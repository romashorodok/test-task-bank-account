package rmq

import (
	"context"
	"log"
	"sync"
	"sync/atomic"

	"github.com/romashorodok/test-task-bank-account/contrib/backoff"

	amqp "github.com/rabbitmq/amqp091-go"
)

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
