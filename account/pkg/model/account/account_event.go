package account

import "github.com/romashorodok/test-task-bank-account/contrib/cqrs"

const (
	DEPOSIT_ACCOUNT_EVENT_NAME  = "DepositAccountEvent"
	WITHDRAW_ACCOUNT_EVENT_NAME = "WithdrawAccountEvent"
)

type DepositAccountEvent struct {
	AccountID string  `json:"account_id"`
	Amount    float64 `json:"amount"`
}

func (d *DepositAccountEvent) Encode() (*cqrs.Message, error) {
	msg, err := cqrs.JsonMarshaller.Marshal(d)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (d *DepositAccountEvent) EventName() string {
	return DEPOSIT_ACCOUNT_EVENT_NAME
}

var (
	_ cqrs.Request = (*DepositAccountEvent)(nil)
	_ cqrs.Event   = (*DepositAccountEvent)(nil)
)

func NewDepositAccountEvent(accountID string, amount float64) *DepositAccountEvent {
	return &DepositAccountEvent{
		AccountID: accountID,
		Amount:    amount,
	}
}

type WithdrawAccountEvent struct {
	AccountID string  `json:"account_id"`
	Amount    float64 `json:"amount"`
}

func (w *WithdrawAccountEvent) EventName() string {
	return WITHDRAW_ACCOUNT_EVENT_NAME
}

func (w *WithdrawAccountEvent) Encode() (*cqrs.Message, error) {
	msg, err := cqrs.JsonMarshaller.Marshal(w)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

var (
	_ cqrs.Request = (*WithdrawAccountEvent)(nil)
	_ cqrs.Event   = (*WithdrawAccountEvent)(nil)
)

func NewWithdrawAccountEvent(accountID string, amount float64) *WithdrawAccountEvent {
	return &WithdrawAccountEvent{
		AccountID: accountID,
		Amount:    amount,
	}
}
