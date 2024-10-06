package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5"
	"github.com/romashorodok/test-task-bank-account/account/internal/httphandler"
	"github.com/romashorodok/test-task-bank-account/account/pkg/config"
	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/backoff"
	"github.com/romashorodok/test-task-bank-account/contrib/httputil"

	amqp "github.com/rabbitmq/amqp091-go"
)

var DB_TABLES = []interface{}{
	&account.Account{},
}

type ConnectionWrapper struct {
	address   string
	conn      *amqp.Connection
	bindingMu sync.RWMutex

	wg        sync.WaitGroup
	connected chan struct{}
	closing   chan struct{}
	closed    uint32
}

func (c *ConnectionWrapper) connect() error {
	c.bindingMu.Lock()
	defer c.bindingMu.Unlock()

	log.Printf("Trying connect to AMPQ on %s", c.address)
	conn, err := amqp.Dial(c.address)
	if err != nil {
		return err
	}

	c.conn = conn
	close(c.connected)

	log.Printf("Connected to AMPQ on %s", c.address)

	return nil
}

func (c *ConnectionWrapper) onConnClose() {
	for {
		log.Println("on connect close is waiting for connected")
		<-c.connected
		log.Println("on connect close is connected. Watch amqp err notification")

		notifyCloseConnection := c.conn.NotifyClose(make(chan *amqp.Error))

		select {
		case <-c.closing:
			log.Println("Received close notification from user, reconnecting.")

		case err := <-notifyCloseConnection:
			log.Printf("Received close notification from AMQP, reconnecting. Err: %s", err)

		case <-context.Background().Done():
			log.Println("Received background context done.")
			return
		}

		c.connected = make(chan struct{})
		c.reconnect()
	}
}

func (c *ConnectionWrapper) Close() error {
	if !atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		return nil
	}

	close(c.closing)

	c.wg.Wait()

	if err := c.conn.Close(); err != nil {
		return err
	}

	return nil
}

func (c *ConnectionWrapper) Closed() bool {
	return atomic.LoadUint32(&c.closed) == 1
}

func (c *ConnectionWrapper) reconnect() {
	if err := backoff.Retry(func() error {
		if err := c.connect(); err != nil {
			return err
		}

		if c.Closed() {
			// TODO: may be used a specific error to detect close
			log.Println("Connection is closed. Stop reconnecting...")
			return nil
		}

		return nil
	}, backoff.NewBackoffDefault()); err != nil {
		log.Printf("AMPQ reconnect failed. Err: %s", err)
	}
}

func NewConnectionWrapper(address string) (*ConnectionWrapper, error) {
	conn := &ConnectionWrapper{
		address:   address,
		bindingMu: sync.RWMutex{},
		connected: make(chan struct{}),
		closing:   make(chan struct{}),
	}

	if err := conn.connect(); err != nil {
		return nil, err
	}

	go conn.onConnClose()

	return conn, nil
}

type Publisher struct {
	*ConnectionWrapper

	// Must be a name of command which will have a handler for this event
	exchangeName string
}

var ErrPublisherClosed = errors.New("publisher closed")

func (p *Publisher) Publish(routingKey string, message amqp.Publishing) (err error) {
	if p.Closed() {
		return ErrPublisherClosed
	}

	select {
	case <-p.connected:
	case <-p.closing:
		return ErrPublisherClosed
	}

	p.wg.Add(1)
	defer p.wg.Done()

	channel, err := p.conn.Channel()
	if err != nil {
		log.Printf("Cannot open publish channel of %s. Err: %s", p.exchangeName, err)
		return err
	}
	defer func() {
		if closeErr := channel.Close(); closeErr != nil {
			err = closeErr
		}
	}()
	// NOTE: channel may be a transactional

	if err = channel.ExchangeDeclare(
		p.exchangeName,
		amqp.ExchangeDirect,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		log.Printf("Publisher declare of %s the exchange. Err: %s", p.exchangeName, err)
		return err
	}

	if err = channel.Publish(
		p.exchangeName,
		routingKey,
		false,
		false,
		message,
	); err != nil {
		log.Printf("Publisher %s publish. Err: %s", p.exchangeName, err)
		return err
	}

	return
}

func NewPublisher(address string, exchangeName string) (*Publisher, error) {
	conn, err := NewConnectionWrapper(address)
	if err != nil {
		return nil, err
	}

	return &Publisher{
		ConnectionWrapper: conn,
		exchangeName:      exchangeName,
	}, nil
}

type Consumer struct {
	*ConnectionWrapper

	exchangeName string
}

var ErrConsumerClosed = errors.New("consumer closed")

func (c *Consumer) tryOpenChannel() (*amqp.Channel, error) {
	channel, err := c.conn.Channel()
	if err != nil {
		log.Printf("Cannot open consumer channel of %s. Err: %s", c.exchangeName, err)
		return nil, err
	}
	defer func() {
		if closeErr := channel.Close(); closeErr != nil {
			err = closeErr
		}
	}()
	// TODO: channel.Qos

	if err = channel.ExchangeDeclare(
		c.exchangeName,
		amqp.ExchangeDirect,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		log.Printf("Consumer declare of %s the exchange. Err: %s", c.exchangeName, err)
		return nil, err
	}

	return channel, nil
}

func (c *Consumer) runConsumeSubscription(ctx context.Context, consumerLabel, routingKey string) {
	channel, err := c.conn.Channel()
	if err != nil {
		log.Printf("Cannot open consumer channel of %s. Err: %s", c.exchangeName, err)
		return
	}
	defer func() {
		if err = channel.Close(); err != nil {
			log.Printf("Failed to close %s channel. Err: %s", c.exchangeName, err)
		}
	}()

	notifyCloseChannel := channel.NotifyClose(make(chan *amqp.Error))

	queue, err := channel.QueueDeclare(c.exchangeName, true, false, true, false, nil)
	if err != nil {
		log.Printf("Cannot declare %s consumer of %s. Err: %s", routingKey, c.exchangeName, err)
		return
	}

	if err = channel.QueueBind(queue.Name, routingKey, queue.Name, false, nil); err != nil {
		log.Printf("Unable bind queue name: %s routing key: %s, exchange: %s. Err: %s", queue.Name, routingKey, queue.Name, err)
		return
	}

	sub := subscription{
		notifyCloseChannel: notifyCloseChannel,
		channel:            channel,
		queueName:          queue.Name,
		consumerLabel:      consumerLabel,
		closing:            c.closing,
	}

	sub.ProcessMessage(ctx)
}

func (c *Consumer) Consume(ctx context.Context, consumerLabel, routingKey string) (err error) {
	if c.Closed() {
		return ErrConsumerClosed
	}

	select {
	case <-c.connected:
	case <-c.closing:
		return ErrConsumerClosed
	}

	if _, err = c.tryOpenChannel(); err != nil {
		return err
	}

	c.wg.Add(1)

	go func(ctx context.Context) {
		defer c.wg.Done()

	ReconnectLoop:
		for {
			log.Println("Waiting for s.connected or s.closing in ReconnectLoop")

			select {
			case <-c.closing:
				log.Println("Stopping ReconnectLoop (already closing)")
				break ReconnectLoop
			default:
			}

			select {
			case <-c.closing:
				log.Println("Stopping ReconnectLoop (closing)")
				break ReconnectLoop
			case <-ctx.Done():
				log.Println("Stopping ReconnectLoop (ctx done)")
				break ReconnectLoop

			case <-c.connected:
				log.Println("Connection established in ReconnectLoop")
				c.runConsumeSubscription(ctx, consumerLabel, routingKey)
			}

			time.Sleep(time.Millisecond * 100)
		}
	}(ctx)

	return nil
}

func NewConsumer(address string, exchangeName string) (*Consumer, error) {
	conn, err := NewConnectionWrapper(address)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		ConnectionWrapper: conn,
		exchangeName:      exchangeName,
	}, nil
}

type subscription struct {
	out                chan any
	notifyCloseChannel chan *amqp.Error
	channel            *amqp.Channel
	queueName          string
	consumerLabel      string

	closing chan struct{}
}

const CONSUMER_AUTO_GEN_NAME = ""

type messageTask struct {
	cb   func(ctx context.Context, in <-chan *amqp.Delivery)
	done chan struct{}
}

func (s *subscription) processMessage(ctx context.Context, msg amqp.Delivery) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	in := make(chan *amqp.Delivery)
	defer close(in)

	done := make(chan struct{})

	task := messageTask{
		cb: func(ctx context.Context, in <-chan *amqp.Delivery) {
			defer close(done)

			select {
			case <-ctx.Done():
				return

			case msg := <-in:
				// TODO: here process a message
				// TODO: make a callback which recv error if err is nil then ack the message
				log.Printf("Consumer %s | Process a message here. %+v  %s", s.consumerLabel, msg, msg.Body)
				msg.Ack(false)
			}
		},
		done: done,
	}

	go task.cb(ctx, in)

	select {
	case <-s.closing:
		log.Println("Message not consumed, pub/sub is closing")
		return s.nackMessage(msg)
	case in <- &msg:
		log.Println("Message sent to consumer")
	}

	select {
	case <-s.closing:
		log.Println("Closing pub/sub, message discarded before ack")
		return s.nackMessage(msg)
	case <-task.done:
		return nil
	}
}

func (s *subscription) nackMessage(msg amqp.Delivery) error {
	return msg.Nack(false, false)
}

func (s *subscription) ProcessMessage(ctx context.Context) {
	msgs, err := s.channel.Consume(s.queueName, CONSUMER_AUTO_GEN_NAME, false, false, false, false, nil)
	if err != nil {
		log.Printf("Failed to start subscription %s messages. Err: %s", s.queueName, err)
		return
	}

ConsumingLoop:
	for {
		select {
		case <-ctx.Done():
			log.Println("Closing subscription from ctx")
			break ConsumingLoop
		case <-s.closing:
			log.Println("Closing subscription from closing")
			break ConsumingLoop
		case err := <-s.notifyCloseChannel:
			log.Printf("Closing subscription from amqp. Err: %s", err)
			break ConsumingLoop

		case msg := <-msgs:
			if err = s.processMessage(ctx, msg); err != nil {
				if err = s.nackMessage(msg); err != nil {
					log.Printf("Cannot nack message. Err: %s", err)
					break ConsumingLoop
				}
			}
			continue ConsumingLoop
		}
	}
}

func main() {
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM, syscall.SIGINT)

	db, err := config.NewPostgresConfig().BuildGorm()
	if err != nil {
		panic(err)
	}

	db.AutoMigrate(DB_TABLES...)

	accountHandler := httphandler.NewAccountHandler()
	accountHandler.RegisterHandler()

	amqpConfig := config.NewAmpqConfig()

	consumer, err := NewConsumer(amqpConfig.Address, "logs_direct")
	if err != nil {
		panic(err)
	}
	go consumer.Consume(context.Background(), "info-1", "info")
	go consumer.Consume(context.Background(), "info-2", "info")
	// go consumer.Consume(context.Background(), "warning-1", "warning")
	// go consumer.Consume(context.Background(), "warning-2", "warning")
	// go consumer.Consume(context.Background(), "error-1", "error")
	// go consumer.Consume(context.Background(), "error-2", "error")

	_ = consumer

	ampqConn, err := NewPublisher(amqpConfig.Address, "logs_direct")
	if err != nil {
		panic(err)
	}
	defer ampqConn.Close()
	_ = ampqConn

	go func() {
		ticker := time.NewTicker(time.Second * 2)
		defer ticker.Stop()

		for {
			select {
			case <-context.Background().Done():
				return
			case <-ticker.C:
				ampqConn.Publish(
					"info",
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        []byte(fmt.Sprintf("text from info 1 %s", time.Now())),
					},
				)
				ampqConn.Publish(
					"info",
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        []byte(fmt.Sprintf("text from info 2 %s", time.Now())),
					},
				)
				ampqConn.Publish(
					"info",
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        []byte(fmt.Sprintf("text from info 3 %s", time.Now())),
					},
				)

				// ampqConn.Publish(
				// 	"warning",
				// 	amqp.Publishing{
				// 		ContentType: "text/plain",
				// 		Body:        []byte("text from warning"),
				// 	},
				// )
				// ampqConn.Publish(
				// 	"error",
				// 	amqp.Publishing{
				// 		ContentType: "text/plain",
				// 		Body:        []byte("text from error"),
				// 	},
				// )

			}
		}
	}()

	go func() {
		if err := http.ListenAndServe(":8000", nil); err != nil {
			log.Fatalf("Listen error %s", err)
		}
	}()

	// db.Create(account.Account{
	// 	ID: account.NewID(),
	// 	Balance: account.Money(20),
	// })

	var account account.Account
	db.First(&account, "id = ?", "735a7d00-ea43-4730-b4b2-9377e9ebe5ac")
	log.Printf("%+v", account)

	_ = db

	// gorm.DB()
	httputil.HelloWorld()

	select {
	case <-sigterm:
	case <-context.Background().Done():
	}
}
