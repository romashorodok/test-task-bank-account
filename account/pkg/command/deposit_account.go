package command

import (
	"context"
	"log"

	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"gorm.io/gorm"
)

type DepositAccountBody struct {
	AccountID   string  `json:"account_id"`
	AggregateID string  `json:"aggregate_id"`
	Amount      float64 `json:"amount"`
}

func (d *DepositAccountBody) WithAggregateID(id string) {
	d.AggregateID = id
}

var _ cqrs.Event = (*DepositAccountBody)(nil)

type DepositAccountCommand struct {
	body *DepositAccountBody

	marshaller cqrs.MessageJsonMarshaller[*DepositAccountBody]
}

func (d *DepositAccountCommand) Encode() (*cqrs.Message, error) {
	msg, err := d.marshaller.Marshal(d.body)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (d *DepositAccountCommand) Unbox() *DepositAccountBody {
	panic("unimplemented")
}

var _ cqrs.Request[*DepositAccountBody] = (*DepositAccountCommand)(nil)

func NewDepositAccountCommand(accountID string, amount float64) *DepositAccountCommand {
	return &DepositAccountCommand{
		body: &DepositAccountBody{
			AccountID: accountID,
			Amount:    amount,
		},
	}
}

type DepositAccountCommandHandler struct {
	db *gorm.DB

	marshaller cqrs.MessageJsonMarshaller[*DepositAccountBody]
}

func (d *DepositAccountCommandHandler) Factory(msg *cqrs.Message) (cqrs.Request[*DepositAccountBody], error) {
	var body DepositAccountBody

	if err := d.marshaller.Unmarshal(msg, &body); err != nil {
		return nil, err
	}

	return &DepositAccountCommand{
		body: &body,
	}, nil
}

func (d *DepositAccountCommandHandler) Handle(ctx context.Context, request *DepositAccountCommand) (*DepositAccountBody, error) {
	log.Printf("Deposit run %+v %+v", request, request.body)

	// if err := d.db.Transaction(func(tx *gorm.DB) error {
	// 	acc := &account.Account{}
	//
	// 	// SELECT * FROM accounts WHERE id = ? FOR UPDATE;
	// 	// NOTE: Hold lock for the row for all tx and release when commit or rollback it
	// 	if err := tx.Clauses(clause.Locking{
	// 		Strength: "UPDATE",
	// 	}).First(&acc, "id = ?", request.body.AccountID).Error; err != nil {
	// 		return err
	// 	}
	//
	// 	acc.Balance += account.Money(request.body.Amount)
	// 	return tx.Save(acc).Error
	// }); err != nil {
	// 	return nil, err
	// }
	return nil, nil
}

var _ cqrs.Handler[*DepositAccountBody, *DepositAccountCommand] = (*DepositAccountCommandHandler)(nil)

func NewDepositAccountCommandHandler(db *gorm.DB) *DepositAccountCommandHandler {
	return &DepositAccountCommandHandler{
		db: db,
	}
}
