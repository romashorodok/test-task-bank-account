package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5"
	"github.com/romashorodok/test-task-bank-account/account/internal/httphandler"

	"github.com/romashorodok/test-task-bank-account/account/pkg/command"
	"github.com/romashorodok/test-task-bank-account/account/pkg/config"
	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs/espgx"
)

var DB_TABLES = []interface{}{
	&account.Account{},
}

type Command struct {
	Key string `json:"key"`
}

func main() {
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM, syscall.SIGINT)

	pgconfig := config.NewPostgresConfig()
	db, err := pgconfig.BuildGorm()
	if err != nil {
		panic(err)
	}

	// db.AutoMigrate(DB_TABLES...)

	estore := account.NewEventStoreGorm(db)

	rel := estore.Register(&account.Account{})
	_ = rel
	idStr := "735a7d00-ea43-4730-b4b2-9377e9ebe5ac"
	id := account.ID(uuid.MustParse(idStr))
	_ = id

	// rel.AddAggregate(context.Background(), id)

	// test2, _ := json.Marshal(&Command{"Withdrow some"})
	// test1, _ := json.Marshal(&Command{"Deposit some"})

	// if err = rel.AppendEvents(context.Background(), id, []cqrs.RawEvent{
	// 	{
	// 		Name: "DepositCommadn",
	// 		Data: test1,
	// 	},
	// 	{
	// 		Name: "WithdrawCommand",
	// 		Data: test2,
	// 	},
	// }); err != nil {
	// 	log.Println(err)
	// 	return
	// }

	// test3, _ := json.Marshal(&Command{"Snapshot"})
	// if err = rel.AddSnapshot(context.Background(), id, cqrs.RawSnapshot{
	// 	Version: 3,
	// 	Data:    test3,
	// }); err != nil {
	// 	log.Println("Snapshot error", err)
	// 	return
	// }

	// latest, err := rel.LatestSnapshots(context.Background(), id)
	// if err != nil {
	// 	log.Println("Latest snapshot error", err)
	// 	return
	// }
	//
	// log.Printf("Found latest %+v", latest)

	// acc := account.NewAccount()
	// acc.ID = id
	// acc.Raise(account.NewDepositAccountEvent(idStr, 20))

	// result := repo.Add(context.Background(), acc)
	// log.Println("result error", result)

	// log.Println("repo result", result)

	repo := account.NewRepository[*account.Account](rel, account.AccountFactory{}, account.AccountEventFactory{})

	err = repo.UpdateByID(context.Background(), idStr, func(aggregate *account.Account) error {
		aggregate.Raise(account.NewDepositAccountEvent(aggregate.AggregateID(), 20))
		return nil
	})
	log.Println("Update by id error", err)

	pool, err := pgconfig.BuildPool()
	if err != nil {
		panic(err)
	}

	es := espgx.NewEventStore(pool)

	amqpConfig := config.NewAmpqConfig()
	bus, err := cqrs.NewBusRabbitMQ(amqpConfig.Address, "events")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	// cqrs.Register(bus, ctx, &command.CreateAccountCommand{}, command.NewCreateAccountCommandHandler(db))
	cqrs.Register(bus, ctx, &account.DepositAccountEvent{}, command.NewDepositAccountCommandHandler(db, es))
	// cqrs.Register(bus, ctx, &command.WithdrawAccountCommand{}, command.NewWithdrawAccountCommandHandler(db))

	queryBus := cqrs.NewBusContext()
	// cqrs.Register(queryBus, ctx, &query.GetAccountQuery{}, query.NewGetAccountQueryHandler(db))

	router := chi.NewRouter()

	accountHandler := httphandler.NewAccountHandler(queryBus, bus)
	accountHandler.RegisterHandler(router)

	go func() {
		if err := http.ListenAndServe(":8000", router); err != nil {
			log.Fatalf("Listen error %s", err)
		}
	}()

	select {
	case <-sigterm:
	case <-context.Background().Done():
	}
}
