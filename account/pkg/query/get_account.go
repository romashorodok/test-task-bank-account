package query

import (
	"context"
	"encoding/json"
	"log"

	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
)

type Account struct {
	Name string `json:"name"`
}

type GetAccountQuery struct {
	account *Account
}

func (g *GetAccountQuery) Encode() ([]byte, error) {
	return json.Marshal(g.account)
}

func (g *GetAccountQuery) Unbox() *Account {
	panic("unimplemented")
}

var _ cqrs.Request[*Account] = (*GetAccountQuery)(nil)

func NewGetAccountQuery() *GetAccountQuery {
	return &GetAccountQuery{}
}

type GetAccountQueryHandler struct{}

func (g *GetAccountQueryHandler) Factory(data []byte) (cqrs.Request[*Account], error) {
	log.Println("Get account data decode", data)
	var account Account
	if err := json.Unmarshal(data, &account); err != nil {
		return nil, err
	}
	return &GetAccountQuery{
		account: &account,
	}, nil
}

func (g *GetAccountQueryHandler) Handle(ctx context.Context, query *GetAccountQuery) (*Account, error) {
	log.Println("GetAccountQuery handler works")
	return &Account{"test"}, nil
}

var _ cqrs.Handler[*Account, *GetAccountQuery] = (*GetAccountQueryHandler)(nil)
