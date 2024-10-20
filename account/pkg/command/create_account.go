package command

import (
	"context"

	"github.com/google/uuid"
	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs/eventstore"
	"gorm.io/gorm"
)

type CreateAccountCommandHandler struct {
	db   *gorm.DB
	repo *eventstore.Repository[*account.Account]

	marshaller cqrs.MessageJsonMarshaller
}

func (c *CreateAccountCommandHandler) Factory(msg *cqrs.Message) (cqrs.Request, error) {
	var command account.CreateAccountEvent
	if err := c.marshaller.Unmarshal(msg, &command); err != nil {
		return nil, err
	}
	return &command, nil
}

type CreateAccountCommandResult struct{}

func (c *CreateAccountCommandHandler) Handle(ctx context.Context, request *account.CreateAccountEvent) (*CreateAccountCommandResult, error) {
	uuidAccountID, err := uuid.Parse(request.AccountID)
	if err != nil {
		uuidAccountID = uuid.New()
	}

	if err = eventstore.WithTransaction(ctx, c.db, func(tx *gorm.DB) error {
		return c.repo.Add(ctx, tx, &account.Account{
			ID: account.ID(uuidAccountID),
		})
	}); err != nil {
		return nil, err
	}

	return &CreateAccountCommandResult{}, nil
}

var _ cqrs.Handler[*CreateAccountCommandResult, *account.CreateAccountEvent] = (*CreateAccountCommandHandler)(nil)

func NewCreateAccountCommandHandler(db *gorm.DB, accountEntity *eventstore.EventStoreEntity) *CreateAccountCommandHandler {
	repo := eventstore.NewRepository(accountEntity, account.AccountFactory{}, account.AccountEventFactory{})
	return &CreateAccountCommandHandler{
		db:   db,
		repo: repo,
	}
}
