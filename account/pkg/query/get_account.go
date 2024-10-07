package query

import (
	"context"
	"encoding/json"

	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"gorm.io/gorm"
)

type GetAccountQueryResult struct {
	Accounts []account.Account `json:"accounts"`
}

type GetAccountQuery struct {
	result *GetAccountQueryResult
}

func (g *GetAccountQuery) Encode() ([]byte, error) {
	return json.Marshal(g.result)
}

func (g *GetAccountQuery) Unbox() *GetAccountQueryResult {
	panic("unimplemented")
}

var _ cqrs.Request[*GetAccountQueryResult] = (*GetAccountQuery)(nil)

func NewGetAccountsQuery() *GetAccountQuery {
	return &GetAccountQuery{}
}

type GetAccountQueryHandler struct {
	db *gorm.DB
}

func (g *GetAccountQueryHandler) Factory(data []byte) (cqrs.Request[*GetAccountQueryResult], error) {
	var accounts []*account.Account
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}
	return &GetAccountQuery{
		result: &GetAccountQueryResult{},
	}, nil
}

func (g *GetAccountQueryHandler) Handle(ctx context.Context, query *GetAccountQuery) (*GetAccountQueryResult, error) {
	var accounts []account.Account

	result := g.db.Find(&accounts)
	if result.Error != nil {
		return nil, result.Error
	}

	return &GetAccountQueryResult{
		Accounts: accounts,
	}, nil
}

var _ cqrs.Handler[*GetAccountQueryResult, *GetAccountQuery] = (*GetAccountQueryHandler)(nil)

func NewGetAccountQueryHandler(db *gorm.DB) *GetAccountQueryHandler {
	return &GetAccountQueryHandler{
		db: db,
	}
}
