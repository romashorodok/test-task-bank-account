package cqrs

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs/rmq"
)

type BusContext struct {
	handlers map[string]Handler[any, Request]
}

func (b *BusContext) register(ctx context.Context, requestName string, request Request, handler Handler[any, Request]) {
	b.handlers[requestName] = handler
}

func (b *BusContext) dispatch(ctx context.Context, request Request) (result any, err error) {
	requestName := typeName(request)

	handler, exist := b.handlers[requestName]
	if !exist {
		return result, fmt.Errorf("not found %s handler.", requestName)
	}

	return handler.Handle(ctx, request)
}

func NewBusContext() *BusContext {
	return &BusContext{
		handlers: make(map[string]Handler[any, Request]),
	}
}

func typeName(typeObject any) string {
	h := reflect.TypeOf(typeObject).String()
	return strings.Split(h, ".")[1]
}

const _MESSAGE_UUID_HEADER = "__uuid"

type AmqpMessageMarshaller struct{}

func (AmqpMessageMarshaller) Marshal(msg *Message) amqp.Publishing {
	headers := make(amqp.Table, len(msg.Metadata)+1) // metadata + plus uuid

	for key, value := range msg.Metadata {
		headers[key] = value
	}
	headers[_MESSAGE_UUID_HEADER] = msg.UUID

	return amqp.Publishing{
		Body:    msg.Payload,
		Headers: headers,
	}
}

func (a AmqpMessageMarshaller) Unmarshal(amqpMsg *amqp.Delivery) (*Message, error) {
	msgUUID, err := a.unmarshalMessageUUID(amqpMsg)
	if err != nil {
		return nil, err
	}

	msg := NewMessage(msgUUID, amqpMsg.Body)
	msg.Metadata = make(Metadata, len(amqpMsg.Headers)-1) // headers - minus uuid

	for key, value := range amqpMsg.Headers {
		if key == _MESSAGE_UUID_HEADER {
			continue
		}

		var ok bool
		msg.Metadata[key], ok = value.(string)
		if !ok {
			return nil, fmt.Errorf("metadata %s is not a string, but %#v", key, value)
		}
	}
	return msg, nil
}

func (a AmqpMessageMarshaller) unmarshalMessageUUID(amqpMsg *amqp.Delivery) (string, error) {
	msgUUID, hasMsgUUID := amqpMsg.Headers[_MESSAGE_UUID_HEADER]
	if !hasMsgUUID {
		return "", nil
	}

	var msgUUIDStr string
	msgUUIDStr, hasMsgUUID = msgUUID.(string)
	if !hasMsgUUID {
		return "", fmt.Errorf("message UUID is not a string, but: %#v", msgUUID)
	}

	return msgUUIDStr, nil
}

type BusRabbitMQ struct {
	publisher *rmq.Publisher
	consumer  *rmq.Consumer

	marshaller AmqpMessageMarshaller
}

func (b *BusRabbitMQ) dispatch(ctx context.Context, request Request) (result any, err error) {
	requestName := typeName(request)
	log.Printf("BusRabbitMQ | Dispatch %s handler", requestName)
	msg, err := request.Encode()
	if err != nil {
		var emtpy any
		return emtpy, err
	}

	msg.Metadata.Set("name", requestName)
	msg.Metadata.Set(_AGGREGATE_ID, uuid.New().String())

	pub := b.marshaller.Marshal(msg)

	return nil, b.publisher.Publish(requestName, pub)
}

func (b *BusRabbitMQ) register(ctx context.Context, registerName string, request Request, handler Handler[any, Request]) {
	// log.Printf("BusRabbitMQ | Register %s handler", registerName)
	go b.consumer.Consume(
		ctx,
		registerName,
		registerName,
		func(ctx context.Context, amqpMsg *amqp.Delivery) error {
			log.Printf("BusRabbitMQ | From handler %s handler", registerName)
			msg, err := b.marshaller.Unmarshal(amqpMsg)
			if err != nil {
				log.Println("Unable unmarshal message from rmq")
				return err
			}

			request, err := handler.Factory(msg)
			if err != nil {
				return err
			}
			_, err = handler.Handle(ctx, request)
			return err
		},
	)
}

var (
	_ requestRegistrable  = (*BusRabbitMQ)(nil)
	_ requestDispatchable = (*BusRabbitMQ)(nil)
)

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
