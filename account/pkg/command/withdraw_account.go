package command

import (
	"context"
	"log"

	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs/eventstore"
	"gorm.io/gorm"
)

type WithdrawAccountCommandHandler struct {
	db         *gorm.DB
	repo       *eventstore.Repository[*account.Account]
	marshaller cqrs.MessageJsonMarshaller
}

func (w *WithdrawAccountCommandHandler) Factory(msg *cqrs.Message) (cqrs.Request, error) {
	var command account.WithdrawAccountEvent

	if err := w.marshaller.Unmarshal(msg, &command); err != nil {
		return nil, err
	}

	return &command, nil
}

func (w *WithdrawAccountCommandHandler) Handle(ctx context.Context, request *account.WithdrawAccountEvent) (*WithdrawAccountCommandResult, error) {
	log.Printf("Withdraw run %+v %+v", request, request)

	if err := eventstore.WithTransaction(ctx, w.db, func(tx *gorm.DB) error {
		return w.repo.UpdateByID(ctx, tx, request.AccountID, func(aggregate *account.Account) error {
			aggregate.Raise(account.NewWithdrawAccountEvent(aggregate.AggregateID(), request.Amount))
			return nil
		})
	}); err != nil {
		return nil, err
	}

	return nil, nil
}

type WithdrawAccountCommandResult struct{}

var _ cqrs.Handler[*WithdrawAccountCommandResult, *account.WithdrawAccountEvent] = (*WithdrawAccountCommandHandler)(nil)

func NewWithdrawAccountCommandHandler(db *gorm.DB, accountEntity *eventstore.EventStoreEntity) *WithdrawAccountCommandHandler {
	repo := eventstore.NewRepository(accountEntity, account.AccountFactory{}, account.AccountEventFactory{})
	return &WithdrawAccountCommandHandler{
		db:   db,
		repo: repo,
	}
}
