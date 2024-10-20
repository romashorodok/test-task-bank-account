package command

import (
	"context"
	"log"

	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs/eventstore"
	"gorm.io/gorm"
)

type DepositAccountCommandHandler struct {
	db   *gorm.DB
	repo *eventstore.Repository[*account.Account]

	marshaller cqrs.MessageJsonMarshaller
}

func (d *DepositAccountCommandHandler) Factory(msg *cqrs.Message) (cqrs.Request, error) {
	var command account.DepositAccountEvent

	if err := d.marshaller.Unmarshal(msg, &command); err != nil {
		return nil, err
	}

	return &command, nil
}

type DepositAccountCommandResult struct {
	Test string
}

func (d *DepositAccountCommandHandler) Handle(ctx context.Context, request *account.DepositAccountEvent) (*DepositAccountCommandResult, error) {
	log.Printf("Deposit run %+v %+v", request, request)

	if err := eventstore.WithTransaction(ctx, d.db, func(tx *gorm.DB) error {
		return d.repo.UpdateByID(ctx, tx, request.AccountID, func(aggregate *account.Account) error {
			aggregate.Raise(account.NewDepositAccountEvent(aggregate.AggregateID(), request.Amount))
			return nil
		})
	}); err != nil {
		return nil, err
	}

	return &DepositAccountCommandResult{}, nil
}

var _ cqrs.Handler[*DepositAccountCommandResult, *account.DepositAccountEvent] = (*DepositAccountCommandHandler)(nil)

func NewDepositAccountCommandHandler(db *gorm.DB, accountEntity *eventstore.EventStoreEntity) *DepositAccountCommandHandler {
	repo := eventstore.NewRepository(accountEntity, account.AccountFactory{}, account.AccountEventFactory{})
	return &DepositAccountCommandHandler{
		db:   db,
		repo: repo,
	}
}
