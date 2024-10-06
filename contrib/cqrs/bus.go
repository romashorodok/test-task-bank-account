package cqrs

import (
	"reflect"
	"strings"
)

type Serializable[F any] interface {
	Serialize(F) []byte
	Deserialize([]byte) F
	Unbox() F
}

type BusContext struct {
	handlers map[string]Handler[any, Request[any]]
}

func (b *BusContext) register(commandName string, handler Handler[any, Request[any]]) {
	b.handlers[commandName] = handler
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
