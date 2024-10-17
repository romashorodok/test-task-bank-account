package account

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
)

type Account struct {
	cqrs.EventRaiserAggregate

	ID      ID    `gorm:"type:uuid"`
	Balance Money `gorm:"type:float"`
}

func (a *Account) AggregateID() string {
	value, err := a.ID.Value()
	if err != nil {
		return ""
	}
	valueStr, ok := value.(string)
	if !ok {
		return ""
	}
	return valueStr
}

func (a *Account) Snapshot() ([]byte, error) {
	// return json.Marshal(account.Account{
	// 	ID:      account.ID(uuid.MustParse(a.AccountID)),
	// 	Balance: account.Money(a.Amount),
	// })
	return json.Marshal(a)
}

func (a *Account) onEvent(event cqrs.Event) {
	switch evt := event.(type) {
	case *DepositAccountEvent:
		a.Balance += Money(evt.Amount)
	case *WithdrawAccountEvent:
		a.Balance -= Money(evt.Amount)
	}
}

func NewAccount() *Account {
	a := &Account{}
	a.EventRaiserAggregate = cqrs.NewEventRaiserAggregate(a.onEvent)
	return a
}

type AccountFactory struct{}

func (AccountFactory) NewAggregateFromEvents([]cqrs.Event) (*Account, error) {
	acc := Account{}
	acc.EventRaiserAggregate = cqrs.NewEventRaiserAggregate(acc.onEvent)
	return &acc, nil
}

func (AccountFactory) NewAggregateFromSnapshotAndEvents(snapshot cqrs.RawSnapshot, events []cqrs.Event) (*Account, error) {
	var accSnapshot Account
	err := json.Unmarshal(snapshot.Data, &accSnapshot)
	if err != nil {
		return nil, err
	}

	acc := &Account{
		ID: accSnapshot.ID,
		// Amount:    float64(accSnapshot.Balance),
	}

	log.Println("AccountFactory from snapshot version", snapshot.Version, "event factory", events)

	acc.EventRaiserAggregate = cqrs.NewEventRaiserAggregateFromEvents(0, events, acc.onEvent)

	return acc, nil
}

var _ cqrs.AggregateFactory[*Account] = (*AccountFactory)(nil)

type AccountEventFactory struct{}

func (AccountEventFactory) CreateEmptyEvent(name string) (cqrs.Event, error) {
	var e cqrs.Event
	switch name {
	case DEPOSIT_ACCOUNT_EVENT_NAME:
		e = &DepositAccountEvent{}

	case WITHDRAW_ACCOUNT_EVENT_NAME:
		e = &WithdrawAccountEvent{}

	default:
		return nil, fmt.Errorf("unkown factory event name: %s", name)
	}
	return e, nil
}

var _ cqrs.EventFactory = (*AccountEventFactory)(nil)
