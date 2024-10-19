package command

import (
	"context"
	"log"

	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs/eventstore"
	"gorm.io/gorm"
)

type DepositAccountCommandHandler struct {
	db   *gorm.DB
	repo *eventstore.Repository[*account.Account]

	marshaller cqrs.MessageJsonMarshaller
}

func (d *DepositAccountCommandHandler) Factory(msg *cqrs.Message) (cqrs.Request, error) {
	var command account.DepositAccountEvent

	if err := d.marshaller.Unmarshal(msg, &command); err != nil {
		return nil, err
	}

	return &command, nil
}

// type accountRepository struct {
// 	r *cqrs.Repository[*account.Account]
// }

// func NewAccountRepository(es cqrs.EventStore) *accountRepository {
// 	return &accountRepository{
// 		r: cqrs.NewRepository(es, &account.AccountFactory{}, &account.AccountEventFactory{}),
// 	}
// }

type DepositAccountCommandResult struct {
	Test string
}

func (d *DepositAccountCommandHandler) Handle(ctx context.Context, request *account.DepositAccountEvent) (*DepositAccountCommandResult, error) {
	log.Printf("Deposit run %+v %+v", request, request)

	if err := d.repo.UpdateByID(
		ctx,
		request.AccountID,
		func(aggregate *account.Account) error {
			aggregate.Raise(
				account.NewDepositAccountEvent(
					aggregate.AggregateID(),
					request.Amount,
				),
			)
			return nil
		},
	); err != nil {
		return nil, err
	}

	// a := NewAccount()
	//
	// a.AccountID = request.body.AccountID
	// a.Amount = 0
	// // a.SetVersion(2)
	//
	// log.Printf("%+v", a)
	//
	// a.Raise(request.body)

	// repo := NewAccountRepository(d.es)
	//
	// result, err := repo.r.FindByID(ctx, request.AccountID)
	// if err != nil {
	// 	log.Println("Unable find by id", err)
	// 	return nil, err
	// }
	// log.Printf("result: %+v", result)
	//
	// result.Raise(request)
	//
	// if err := repo.r.Update(ctx, result); err != nil {
	// 	log.Println("Unable update a request in es", err)
	// 	return nil, err
	// }

	// if err := repo.r.UpdateByID(ctx, request.body.AccountID, func(aggregate *Account) error {
	// 	// aggregate = a
	// 	return nil
	// }); err != nil {
	// 	log.Println("Unable add a request to es", err)
	//
	// 	return nil, err
	// }

	// if err := repo.r.Update(ctx, a); err != nil {
	// 	return nil, err
	// }

	// if err := repo.r.Add(context.Background(), a); err != nil {
	// 	log.Println("Unable add a request to es", err)
	// 	return nil, err
	// }

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
	// 	a := NewAccount()
	//
	// 	// accountID, err := acc.ID.Value()
	// 	// accountIDStr, ok := accountID.(string)
	// 	// if !ok {
	// 	// 	return err
	// 	// }
	//
	// 	a.AccountID = request.body.AccountID
	// 	a.Amount = float64(acc.Balance)
	//
	// 	a.Raise(request.body)
	//
	// 	repo := NewAccountRepository(d.es)
	//
	// 	if err := repo.r.Add(ctx, a); err != nil {
	// 		log.Println("Unable add a request to es", err)
	// 		return err
	// 	}
	//
	// 	log.Printf("%+v", a)
	//
	// 	// acc.Balance += account.Money(request.body.Amount)
	// 	return tx.Save(acc).Error
	// }); err != nil {
	// 	return nil, err
	// }

	// repo := NewAccountRepository(d.es)
	//
	// request.body.EventRaiserAggregate = cqrs.NewEventRaiserAggregate(request.body.onEvent)
	// request.body.Raise(request.body)
	//
	// log.Println("get changes", request.body.Changes())
	//
	// if err := repo.r.Add(ctx, request.body); err != nil {
	// 	log.Println("Unable add a request to es", err)
	// 	return nil, err
	// }

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
	return &DepositAccountCommandResult{}, nil
}

var _ cqrs.Handler[*DepositAccountCommandResult, *account.DepositAccountEvent] = (*DepositAccountCommandHandler)(nil)

func NewDepositAccountCommandHandler(db *gorm.DB, accountEntity *eventstore.EventStoreEntity) *DepositAccountCommandHandler {
	repo := eventstore.NewRepository(accountEntity, account.AccountFactory{}, account.AccountEventFactory{})
	return &DepositAccountCommandHandler{
		db:   db,
		repo: repo,
	}
}
