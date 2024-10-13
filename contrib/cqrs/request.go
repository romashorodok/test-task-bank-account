package cqrs

import (
	"context"
	"errors"
	"fmt"
)

type Request interface {
	Encode() (*Message, error)
}

type Handler[A any, F Request] interface {
	Handle(ctx context.Context, request F) (A, error)
	Factory(*Message) (Request, error)
}

var _ Handler[any, Request] = (*requestWrapper)(nil)

type requestWrapper struct {
	cb        func(ctx context.Context, query Request) (any, error)
	cbFactory func(msg *Message) (Request, error)
}

func (r *requestWrapper) Factory(msg *Message) (Request, error) {
	return r.cbFactory(msg)
}

func (q *requestWrapper) Handle(ctx context.Context, query Request) (any, error) {
	return q.cb(ctx, query)
}

type requestRegistrable interface {
	register(ctx context.Context, requestName string, request Request, handler Handler[any, Request])
}

var ErrRegisteInvalidType = errors.New("invalid type ")

func Register[A any, F Request](bus requestRegistrable, ctx context.Context, request Request, handler Handler[A, F]) {
	bus.register(ctx, typeName(request), request, &requestWrapper{
		cb: func(ctx context.Context, query Request) (any, error) {
			typedQuery, ok := query.(F)
			if !ok {
				return nil, errors.Join(ErrRegisteInvalidType, fmt.Errorf("of %s request for handler %s", typeName(query), typeName(handler)))
			}
			return handler.Handle(ctx, typedQuery)
		},
		cbFactory: func(msg *Message) (Request, error) {
			result, err := handler.Factory(msg)
			if err != nil {
				return nil, err
			}
			return result, nil
		},
	})
}

type requestDispatchable interface {
	dispatch(ctx context.Context, request Request) (result any, err error)
}

func Dispatch(bus requestDispatchable, ctx context.Context, request Request) error {
	_, err := bus.dispatch(ctx, request)
	if err != nil {
		return err
	}
	return err
}

func DispatchQuery[F any](bus *BusContext, ctx context.Context, query Request) (F, error) {
	queryName := typeName(query)
	queryHandler, exist := bus.handlers[queryName]
	if !exist {
		var empty F
		return empty, fmt.Errorf("not found %s handler.", queryName)
	}

	result, err := queryHandler.Handle(ctx, query)
	if err != nil {
		var empty F
		return empty, err
	}

	return result.(F), err
}
