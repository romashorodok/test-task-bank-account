package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

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
			close(notifyCloseConnection)
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

	ampqConn, err := NewConnectionWrapper(amqpConfig.Address)
	if err != nil {
		panic(err)
	}
	defer ampqConn.Close()
	_ = ampqConn

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
