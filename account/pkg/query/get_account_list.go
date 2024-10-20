package query

import (
	"context"

	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs/eventstore"
	"gorm.io/gorm"
)

type GetAccountListQueryResult struct {
	Accounts []*account.Account `json:"accounts"`
}

type GetAccountListQueryHandler struct {
	db   *gorm.DB
	repo *eventstore.Repository[*account.Account]

	marshaller cqrs.MessageJsonMarshaller
}

func (g *GetAccountListQueryHandler) Factory(msg *cqrs.Message) (cqrs.Request, error) {
	var query account.DepositAccountEvent
	if err := g.marshaller.Unmarshal(msg, &query); err != nil {
		return nil, err
	}
	return &query, nil
}

func (g *GetAccountListQueryHandler) Handle(ctx context.Context, request *account.GetAccountListEvent) (*GetAccountListQueryResult, error) {
	var result []*account.Account
	var err error

	result, err = g.repo.ListByOffset(ctx, g.db, request.Offset, request.Limit)
	if err != nil {
		return nil, err
	}

	return &GetAccountListQueryResult{
		Accounts: result,
	}, nil
}

var _ cqrs.Handler[*GetAccountListQueryResult, *account.GetAccountListEvent] = (*GetAccountListQueryHandler)(nil)

func NewGetAccountListQueryHandler(db *gorm.DB, accountEntity *eventstore.EventStoreEntity) *GetAccountListQueryHandler {
	repo := eventstore.NewRepository(accountEntity, account.AccountFactory{}, account.AccountEventFactory{})
	return &GetAccountListQueryHandler{
		db:   db,
		repo: repo,
	}
}
