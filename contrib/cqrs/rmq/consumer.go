package rmq

import (
	"context"
	"errors"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

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

func (c *Consumer) runConsumeSubscription(ctx context.Context, consumerLabel, routingKey string, handler ConsumerHandler) {
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

	queue, err := channel.QueueDeclare(c.exchangeName, true, false, false, false, nil)
	if err != nil {
		log.Printf("Cannot declare %s consumer of %s. Err: %s", routingKey, c.exchangeName, err)
		return
	}

	if err = channel.QueueBind(routingKey, routingKey, queue.Name, false, nil); err != nil {
		log.Printf("Unable bind queue name: %s routing key: %s, exchange: %s. Err: %s", queue.Name, routingKey, queue.Name, err)
		return
	}

	sub := subscription{
		notifyCloseChannel: notifyCloseChannel,
		channel:            channel,
		queueName:          routingKey,
		consumerLabel:      consumerLabel,
		closing:            c.closing,
	}

	sub.ProcessMessage(ctx, handler)
}

type ConsumerHandler = func(ctx context.Context, msg *amqp.Delivery) error

func (c *Consumer) Consume(ctx context.Context, consumerLabel, routingKey string, handler ConsumerHandler) (err error) {
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
				c.runConsumeSubscription(ctx, consumerLabel, routingKey, handler)
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

func (s *subscription) processMessage(ctx context.Context, msg amqp.Delivery, handler ConsumerHandler) error {
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
				// log.Printf("Consumer %s | Process %+v", s.consumerLabel, msg)

				if err := handler(ctx, msg); err != nil {
					_ = s.nackMessage(*msg)
					return
				}

				// log.Printf("Consumer %s | Process a message here. %+v  %s", s.consumerLabel, msg, msg.Body)
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
		// log.Println("Message sent to consumer")
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

func (s *subscription) ProcessMessage(ctx context.Context, handler ConsumerHandler) {
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
			if err = s.processMessage(ctx, msg, handler); err != nil {
				if err = s.nackMessage(msg); err != nil {
					log.Printf("Cannot nack message. Err: %s", err)
					break ConsumingLoop
				}
			}
			continue ConsumingLoop
		}
	}
}
