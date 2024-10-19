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

	aggregate := &account.Account{
		ID: account.ID(uuidAccountID),
	}

	if err := c.repo.Add(ctx, aggregate); err != nil {
		return nil, err
	}

	return &CreateAccountCommandResult{}, nil
}

var _ cqrs.Handler[*CreateAccountCommandResult, *account.CreateAccountEvent] = (*CreateAccountCommandHandler)(nil)

func NewCreateAccountCommandHandler(_ *gorm.DB, accountEntity *eventstore.EventStoreEntity) *CreateAccountCommandHandler {
	repo := eventstore.NewRepository(accountEntity, account.AccountFactory{}, account.AccountEventFactory{})
	return &CreateAccountCommandHandler{
		repo: repo,
	}
}
