package command

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

type DepositAccountBody struct {
	AccountID string  `json:"account_id"`
	Amount    float64 `json:"amount"`
}

var _ cqrs.Request[*DepositAccountBody] = (*DepositAccountCommand)(nil)

type DepositAccountCommand struct {
	body *DepositAccountBody
}

func (d *DepositAccountCommand) Encode() ([]byte, error) {
	return json.Marshal(d.body)
}

func (d *DepositAccountCommand) Unbox() *DepositAccountBody {
	panic("unimplemented")
}

func NewDepositAccountCommand(accountID string, amount float64) *DepositAccountCommand {
	return &DepositAccountCommand{
		body: &DepositAccountBody{
			AccountID: accountID,
			Amount:    amount,
		},
	}
}

var _ cqrs.Handler[*DepositAccountBody, *DepositAccountCommand] = (*DepositAccountCommandHandler)(nil)

type DepositAccountCommandHandler struct {
	db *gorm.DB
}

func (d *DepositAccountCommandHandler) Factory(data []byte) (cqrs.Request[*DepositAccountBody], error) {
	var body DepositAccountBody
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return &DepositAccountCommand{
		body: &body,
	}, nil
}

func (d *DepositAccountCommandHandler) Handle(ctx context.Context, request *DepositAccountCommand) (*DepositAccountBody, error) {
	log.Println("Deposit run")
	if err := d.db.Transaction(func(tx *gorm.DB) error {
		acc := &account.Account{}

		// SELECT * FROM accounts WHERE id = ? FOR UPDATE;
		// NOTE: Hold lock for the row for all tx and release when commit or rollback it
		if err := tx.Clauses(clause.Locking{
			Strength: "UPDATE",
		}).First(&acc, "id = ?", request.body.AccountID).Error; err != nil {
			return err
		}

		acc.Balance += account.Money(request.body.Amount)
		return tx.Save(acc).Error
	}); err != nil {
		return nil, err
	}
	return nil, nil
}

func NewDepositAccountCommandHandler(db *gorm.DB) *DepositAccountCommandHandler {
	return &DepositAccountCommandHandler{
		db: db,
	}
}

type WithdrawAccountBody struct {
	AccountID string  `json:"account_id"`
	Amount    float64 `json:"amount"`
}

var _ cqrs.Request[*WithdrawAccountBody] = (*WithdrawAccountCommand)(nil)

type WithdrawAccountCommand struct {
	body *WithdrawAccountBody
}

func (w *WithdrawAccountCommand) Encode() ([]byte, error) {
	return json.Marshal(w.body)
}

func (w *WithdrawAccountCommand) Unbox() *WithdrawAccountBody {
	panic("unimplemented")
}

func NewWithdrawAccountCommand(accountID string, amount float64) *WithdrawAccountCommand {
	return &WithdrawAccountCommand{
		body: &WithdrawAccountBody{
			AccountID: accountID,
			Amount:    amount,
		},
	}
}

var _ cqrs.Handler[*WithdrawAccountBody, *WithdrawAccountCommand] = (*WithdrawAccountCommandHandler)(nil)

type WithdrawAccountCommandHandler struct {
	db *gorm.DB
}

func (w *WithdrawAccountCommandHandler) Factory(data []byte) (cqrs.Request[*WithdrawAccountBody], error) {
	var body WithdrawAccountBody
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return &WithdrawAccountCommand{
		body: &body,
	}, nil
}

func (w *WithdrawAccountCommandHandler) Handle(ctx context.Context, request *WithdrawAccountCommand) (*WithdrawAccountBody, error) {
	log.Println("Withdraw run")
	if err := w.db.Transaction(func(tx *gorm.DB) error {
		acc := &account.Account{}

		if err := tx.Clauses(clause.Locking{
			Strength: "UPDATE",
		}).First(&acc, "id = ?", request.body.AccountID).Error; err != nil {
			return err
		}

		acc.Balance -= account.Money(request.body.Amount)
		if acc.Balance < 0 {
			log.Println("Account amount cannot be a negative balance", acc.Balance)
			// TODO: this may be a reject command or event
			return errors.New("Cannot be a negative balance")
		}

		return tx.Save(acc).Error
	}); err != nil {
		return nil, err
	}
	return nil, nil
}

func NewWithdrawAccountCommandHandler(db *gorm.DB) *WithdrawAccountCommandHandler {
	return &WithdrawAccountCommandHandler{
		db: db,
	}
}
