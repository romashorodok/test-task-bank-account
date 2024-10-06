package query

import (
	"context"
	"log"

	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
)

type Account struct {
	Name string
}

type GetAccountQuery struct {
	account *Account
}

// Deserialize implements cqrs.Query.
func (g *GetAccountQuery) Deserialize([]byte) *Account {
	panic("unimplemented")
}

// Serialize implements cqrs.Query.
func (g *GetAccountQuery) Serialize(*Account) []byte {
	panic("unimplemented")
}

// Unbox implements cqrs.Query.
func (g *GetAccountQuery) Unbox() *Account {
	panic("unimplemented")
}

var _ cqrs.Request[*Account] = (*GetAccountQuery)(nil)

func NewGetAccountQuery() *GetAccountQuery {
	return &GetAccountQuery{}
}

type GetAccountQueryHandler struct{}

func (g *GetAccountQueryHandler) Handle(ctx context.Context, query *GetAccountQuery) (*Account, error) {
	log.Println("GetAccountQuery handler works")
	return &Account{"test"}, nil
}

var _ cqrs.Handler[*Account, *GetAccountQuery] = (*GetAccountQueryHandler)(nil)
