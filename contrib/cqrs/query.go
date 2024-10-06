package cqrs

import (
	"context"
	"errors"
	"fmt"
)

type Request[F any] interface {
	Serializable[F]
}

type Handler[A any, F Request[A]] interface {
	Handle(ctx context.Context, query F) (A, error)
}

var _ Handler[any, Request[any]] = (*requestWrapper)(nil)

type requestWrapper struct {
	cb func(ctx context.Context, query Request[any]) (any, error)
}

func (q *requestWrapper) Handle(ctx context.Context, query Request[any]) (any, error) {
	return q.cb(ctx, query)
}

type requestRegistrable interface {
	register(queryName string, handler Handler[any, Request[any]])
}

var ErrRegisteInvalidType = errors.New("invalid type ")

func Register[A any, F Request[A]](bus requestRegistrable, query F, handler Handler[A, F]) {
	bus.register(typeName(query), &requestWrapper{
		cb: func(ctx context.Context, query Request[any]) (any, error) {
			typedQuery, ok := query.(F)
			if !ok {
				return nil, errors.Join(ErrRegisteInvalidType, fmt.Errorf("of %s request for handler %s", typeName(query), typeName(handler)))
			}

			return handler.Handle(ctx, typedQuery)
		},
	})
}

func Dispatch[F any](bus *BusContext, ctx context.Context, query Request[F]) (F, error) {
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
