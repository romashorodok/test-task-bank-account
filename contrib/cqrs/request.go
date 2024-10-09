package cqrs

import (
	"context"
	"errors"
	"fmt"
)

type Request[F any] interface {
	Encode() (*Message, error)
	Unbox() F
}

type Handler[A any, F Request[A]] interface {
	Handle(ctx context.Context, request F) (A, error)
	Factory(*Message) (Request[A], error)
}

var _ Handler[any, Request[any]] = (*requestWrapper)(nil)

type requestWrapper struct {
	cb        func(ctx context.Context, query Request[any]) (any, error)
	cbFactory func(msg *Message) (Request[any], error)
}

func (r *requestWrapper) Factory(msg *Message) (Request[any], error) {
	return r.cbFactory(msg)
}

func (q *requestWrapper) Handle(ctx context.Context, query Request[any]) (any, error) {
	return q.cb(ctx, query)
}

type requestRegistrable interface {
	register(ctx context.Context, requestName string, request Request[any], handler Handler[any, Request[any]])
}

var ErrRegisteInvalidType = errors.New("invalid type ")

func Register[A any, F Request[A]](bus requestRegistrable, ctx context.Context, query Request[A], handler Handler[A, F]) {
	request := query.(Request[any])
	bus.register(ctx, typeName(query), request, &requestWrapper{
		cb: func(ctx context.Context, query Request[any]) (any, error) {
			typedQuery, ok := query.(F)
			if !ok {
				return nil, errors.Join(ErrRegisteInvalidType, fmt.Errorf("of %s request for handler %s", typeName(query), typeName(handler)))
			}
			return handler.Handle(ctx, typedQuery)
		},
		cbFactory: func(msg *Message) (Request[any], error) {
			result, err := handler.Factory(msg)
			if err != nil {
				return nil, err
			}
			return result.(Request[any]), nil
		},
	})
}

type requestDispatchable interface {
	dispatch(ctx context.Context, request Request[any]) (result any, err error)
}

func Dispatch[F any](bus requestDispatchable, ctx context.Context, request Request[F]) error {
	_, err := bus.dispatch(ctx, request.(Request[any]))
	if err != nil {
		return err
	}
	return err
}

func DispatchQuery[F any](bus *BusContext, ctx context.Context, query Request[F]) (F, error) {
	queryName := typeName(query)
	queryHandler, exist := bus.handlers[queryName]
	if !exist {
		var empty F
		return empty, fmt.Errorf("not found %s handler.", queryName)
	}

	result, err := queryHandler.Handle(ctx, query.(Request[any]))
	if err != nil {
		var empty F
		return empty, err
	}

	return result.(F), err
}
