package cqrs

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

var msgDone = sync.OnceValue(func() chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
})

type ackType int

const (
	noAckSent ackType = iota
	ack
	nack
)

type Metadata map[string]string

func (m Metadata) Set(key, value string) {
	m[key] = value
}

func (m Metadata) Get(key string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return ""
}

type Message struct {
	ctx context.Context

	UUID     string
	Metadata Metadata
	Payload  []byte

	// ack is closed when acknowledge is received.
	ack chan struct{}
	// noACk is closed when negative acknowledge is received.
	noAck chan struct{}

	ackMu       sync.Mutex
	ackSentType ackType
}

func (m *Message) Ack() bool {
	m.ackMu.Lock()
	defer m.ackMu.Unlock()

	if m.ackSentType == nack {
		return false
	}
	if m.ackSentType != noAckSent {
		return true
	}

	m.ackSentType = ack
	if m.ack == nil {
		m.ack = msgDone()
	} else {
		close(m.ack)
	}

	return true
}

func (m *Message) Nack() bool {
	m.ackMu.Lock()
	defer m.ackMu.Unlock()

	if m.ackSentType == ack {
		return false
	}
	if m.ackSentType != noAckSent {
		return true
	}

	m.ackSentType = nack

	if m.noAck == nil {
		m.noAck = msgDone()
	} else {
		close(m.noAck)
	}

	return true
}

func (m *Message) Acked() <-chan struct{} {
	return m.ack
}

func (m *Message) Nacked() <-chan struct{} {
	return m.noAck
}

func (m *Message) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

func (m *Message) SetContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *Message) Copy() *Message {
	msg := NewMessage(m.UUID, m.Payload)
	for k, v := range m.Metadata {
		msg.Metadata[k] = v
	}
	return msg
}

func NewMessage(uuid string, data []byte) *Message {
	return &Message{
		UUID:     uuid,
		Metadata: make(Metadata),
		Payload:  data,
		ack:      make(chan struct{}),
		noAck:    make(chan struct{}),
	}
}

type Event interface {
	WithAggregateID(id string)
}

type MessageJsonMarshaller[F Event] struct{}

func (m MessageJsonMarshaller[F]) uuid() string {
	return uuid.New().String()
}

var _AGGREGATE_ID = "aggregateID"

func (m MessageJsonMarshaller[F]) Marshal(val F) (*Message, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	msg := NewMessage(m.uuid(), data)
	return msg, nil
}

func (m MessageJsonMarshaller[F]) Unmarshal(msg *Message, val F) error {
	if err := json.Unmarshal(msg.Payload, val); err != nil {
		return err
	}
	val.WithAggregateID(msg.Metadata.Get(_AGGREGATE_ID))
	return nil
}
