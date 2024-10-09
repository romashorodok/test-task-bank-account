package command

// import (
// 	"context"
// 	"encoding/json"
// 	"errors"
// 	"log"
//
// 	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
// 	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
// 	"gorm.io/gorm"
// 	"gorm.io/gorm/clause"
// )
//
// type CreateAccountBody struct {
// 	ID account.ID `json:"id"`
// }
//
// var _ cqrs.Request[*CreateAccountBody] = (*CreateAccountCommand)(nil)
//
// type CreateAccountCommand struct {
// 	body *CreateAccountBody
// }
//
// func (c *CreateAccountCommand) Encode() ([]byte, error) {
// 	return json.Marshal(c.body)
// }
//
// func (c *CreateAccountCommand) Unbox() *CreateAccountBody {
// 	return nil
// }
//
// func NewCreateAccountCommand() *CreateAccountCommand {
// 	return &CreateAccountCommand{
// 		body: &CreateAccountBody{
// 			ID: account.NewID(),
// 		},
// 	}
// }
//
// var _ cqrs.Handler[*CreateAccountBody, *CreateAccountCommand] = (*CreateAccountCommandHandler)(nil)
//
// type CreateAccountCommandHandler struct {
// 	db *gorm.DB
// }
//
// func (c *CreateAccountCommandHandler) Factory(data []byte) (cqrs.Request[*CreateAccountBody], error) {
// 	var body CreateAccountBody
// 	if err := json.Unmarshal(data, &body); err != nil {
// 		return nil, err
// 	}
// 	return &CreateAccountCommand{
// 		body: &body,
// 	}, nil
// }
//
// func (c *CreateAccountCommandHandler) Handle(ctx context.Context, request *CreateAccountCommand) (*CreateAccountBody, error) {
// 	if err := c.db.Create(&account.Account{
// 		ID: request.body.ID,
// 	}).Error; err != nil {
// 		return nil, err
// 	}
//
// 	return nil, nil
// }
//
// func NewCreateAccountCommandHandler(db *gorm.DB) *CreateAccountCommandHandler {
// 	return &CreateAccountCommandHandler{
// 		db: db,
// 	}
// }
//
// type WithdrawAccountBody struct {
// 	AccountID string  `json:"account_id"`
// 	Amount    float64 `json:"amount"`
// }
//
// var _ cqrs.Request[*WithdrawAccountBody] = (*WithdrawAccountCommand)(nil)
//
// type WithdrawAccountCommand struct {
// 	body *WithdrawAccountBody
// }
//
// func (w *WithdrawAccountCommand) Encode() ([]byte, error) {
// 	return json.Marshal(w.body)
// }
//
// func (w *WithdrawAccountCommand) Unbox() *WithdrawAccountBody {
// 	panic("unimplemented")
// }
//
// func NewWithdrawAccountCommand(accountID string, amount float64) *WithdrawAccountCommand {
// 	return &WithdrawAccountCommand{
// 		body: &WithdrawAccountBody{
// 			AccountID: accountID,
// 			Amount:    amount,
// 		},
// 	}
// }
//
// var _ cqrs.Handler[*WithdrawAccountBody, *WithdrawAccountCommand] = (*WithdrawAccountCommandHandler)(nil)
//
// type WithdrawAccountCommandHandler struct {
// 	db *gorm.DB
// }
//
// func (w *WithdrawAccountCommandHandler) Factory(data []byte) (cqrs.Request[*WithdrawAccountBody], error) {
// 	var body WithdrawAccountBody
// 	if err := json.Unmarshal(data, &body); err != nil {
// 		return nil, err
// 	}
// 	return &WithdrawAccountCommand{
// 		body: &body,
// 	}, nil
// }
//
// func (w *WithdrawAccountCommandHandler) Handle(ctx context.Context, request *WithdrawAccountCommand) (*WithdrawAccountBody, error) {
// 	log.Println("Withdraw run")
// 	if err := w.db.Transaction(func(tx *gorm.DB) error {
// 		acc := &account.Account{}
//
// 		if err := tx.Clauses(clause.Locking{
// 			Strength: "UPDATE",
// 		}).First(&acc, "id = ?", request.body.AccountID).Error; err != nil {
// 			return err
// 		}
//
// 		acc.Balance -= account.Money(request.body.Amount)
// 		if acc.Balance < 0 {
// 			log.Println("Account amount cannot be a negative balance", acc.Balance)
// 			// TODO: this may be a reject command or event
// 			return errors.New("Cannot be a negative balance")
// 		}
//
// 		return tx.Save(acc).Error
// 	}); err != nil {
// 		return nil, err
// 	}
// 	return nil, nil
// }
//
// func NewWithdrawAccountCommandHandler(db *gorm.DB) *WithdrawAccountCommandHandler {
// 	return &WithdrawAccountCommandHandler{
// 		db: db,
// 	}
// }
