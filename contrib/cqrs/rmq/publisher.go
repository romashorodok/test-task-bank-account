package rmq

import (
	"errors"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

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
			log.Println("Publisher close error", closeErr)
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
		true,
		false,
		message,
	); err != nil {
		log.Printf("Publisher %s publish. Err: %s", p.exchangeName, err)
		return err
	}

	// go func() {
	// 	ch := make(chan amqp.Return)
	// 	returnCh := channel.NotifyReturn(ch)
	//
	// 	select {
	// 	case returnResult, closed := <-returnCh:
	// 		log.Println("Unable send msg", closed, returnResult)
	// 	}
	// }()

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
