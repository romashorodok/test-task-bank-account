package cqrs

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs/rmq"
)

type BusContext struct {
	handlers map[string]Handler[any, Request[any]]
}

func (b *BusContext) register(ctx context.Context, requestName string, request Request[any], handler Handler[any, Request[any]]) {
	b.handlers[requestName] = handler
}

func (b *BusContext) dispatch(ctx context.Context, request Request[any]) (result any, err error) {
	requestName := typeName(request)

	handler, exist := b.handlers[requestName]
	if !exist {
		return result, fmt.Errorf("not found %s handler.", requestName)
	}

	return handler.Handle(ctx, request)
}

func NewBusContext() *BusContext {
	return &BusContext{
		handlers: make(map[string]Handler[any, Request[any]]),
	}
}

func typeName(typeObject any) string {
	h := reflect.TypeOf(typeObject).String()
	return strings.Split(h, ".")[1]
}

var (
	_ requestRegistrable  = (*BusRabbitMQ)(nil)
	_ requestDispatchable = (*BusRabbitMQ)(nil)
)

type BusRabbitMQ struct {
	publisher *rmq.Publisher
	consumer  *rmq.Consumer
}

func (b *BusRabbitMQ) dispatch(ctx context.Context, request Request[any]) (result any, err error) {
	requestName := typeName(request)
	msg, err := request.Encode()
	if err != nil {
		var emtpy any
		return emtpy, err
	}
	return nil, b.publisher.Publish(requestName, amqp.Publishing{
		Body: msg,
	})
}

func (b *BusRabbitMQ) register(ctx context.Context, registerName string, request Request[any], handler Handler[any, Request[any]]) {
	log.Printf("BusRabbitMQ | Register %s handler", registerName)
	go b.consumer.Consume(
		ctx,
		registerName,
		registerName,
		func(ctx context.Context, msg *amqp.Delivery) error {
			request, err := handler.Factory(msg.Body)
			if err != nil {
				return err
			}
			_, err = handler.Handle(ctx, request)
			return err
		},
	)
}

func NewBusRabbitMQ(address string, exchangeName string) (*BusRabbitMQ, error) {
	consumer, err := rmq.NewConsumer(address, exchangeName)
	if err != nil {
		return nil, err
	}
	publisher, err := rmq.NewPublisher(address, exchangeName)
	if err != nil {
		return nil, err
	}

	return &BusRabbitMQ{
		publisher: publisher,
		consumer:  consumer,
	}, nil
}
