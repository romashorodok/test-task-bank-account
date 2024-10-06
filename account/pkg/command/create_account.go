package command

import (
	"context"
	"log"

	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
)

type CreateAccountParams struct{}

var _ cqrs.Request[*CreateAccountParams] = (*CreateAccountCommand)(nil)

type CreateAccountCommand struct {
	params *CreateAccountParams
}

func (c *CreateAccountCommand) Deserialize([]byte) *CreateAccountParams {
	panic("unimplemented")
}

func (c *CreateAccountCommand) Serialize(params *CreateAccountParams) []byte {
	panic("unimplemented")
}

func (c *CreateAccountCommand) Unbox() *CreateAccountParams {
	return c.params
}

func NewCreateAccountCommand(params *CreateAccountParams) *CreateAccountCommand {
	return &CreateAccountCommand{params}
}

var _ cqrs.Handler[*CreateAccountParams, *CreateAccountCommand] = (*CreateAccountCommandHandler)(nil)

type CreateAccountCommandHandler struct{}

func (c *CreateAccountCommandHandler) Handle(ctx context.Context, command *CreateAccountCommand) (*CreateAccountParams, error) {
	log.Println("handle a create account command")
	return nil, nil
}

func NewCreateAccountCommandHandler() *CreateAccountCommandHandler {
	return &CreateAccountCommandHandler{}
}
