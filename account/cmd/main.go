package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5"
	"github.com/romashorodok/test-task-bank-account/account/internal/httphandler"

	"github.com/romashorodok/test-task-bank-account/account/pkg/command"
	"github.com/romashorodok/test-task-bank-account/account/pkg/config"
	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/account/pkg/query"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
)

var DB_TABLES = []interface{}{
	&account.Account{},
}

func main() {
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM, syscall.SIGINT)

	db, err := config.NewPostgresConfig().BuildGorm()
	if err != nil {
		panic(err)
	}

	db.AutoMigrate(DB_TABLES...)

	amqpConfig := config.NewAmpqConfig()
	bus, err := cqrs.NewBusRabbitMQ(amqpConfig.Address, "events")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	cqrs.Register(bus, ctx, &command.CreateAccountCommand{}, command.NewCreateAccountCommandHandler(db))
	cqrs.Register(bus, ctx, &command.DepositAccountCommand{}, command.NewDepositAccountCommandHandler(db))
	cqrs.Register(bus, ctx, &command.WithdrawAccountCommand{}, command.NewWithdrawAccountCommandHandler(db))

	queryBus := cqrs.NewBusContext()
	cqrs.Register(queryBus, ctx, &query.GetAccountQuery{}, query.NewGetAccountQueryHandler(db))

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
