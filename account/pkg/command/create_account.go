package command

import (
	"context"
	"log"

	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
)

type CreateAccountBody struct{}

var _ cqrs.Request[*CreateAccountBody] = (*CreateAccountCommand)(nil)

type CreateAccountCommand struct {
	params *CreateAccountBody
}

func (c *CreateAccountCommand) Encode() ([]byte, error) {
	return nil, nil
}

func (c *CreateAccountCommand) Unbox() *CreateAccountBody {
	return c.params
}

func NewCreateAccountCommand(params *CreateAccountBody) *CreateAccountCommand {
	return &CreateAccountCommand{params}
}

var _ cqrs.Handler[*CreateAccountBody, *CreateAccountCommand] = (*CreateAccountCommandHandler)(nil)

type CreateAccountCommandHandler struct{}

func (c *CreateAccountCommandHandler) Factory(data []byte) (cqrs.Request[*CreateAccountBody], error) {
	panic("unimplemented")
}

func (c *CreateAccountCommandHandler) Handle(ctx context.Context, request *CreateAccountCommand) (*CreateAccountBody, error) {
	log.Println("handle a create account command")

	return nil, nil
}

func NewCreateAccountCommandHandler() *CreateAccountCommandHandler {
	return &CreateAccountCommandHandler{}
}
