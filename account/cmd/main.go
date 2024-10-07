package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
	// cqrs.Register(bus, ctx, &command.CreateAccountCommand{}, command.NewCreateAccountCommandHandler())
	cqrs.Register(bus, ctx, &command.DepositAccountCommand{}, command.NewDepositAccountCommandHandler(db))
	cqrs.Register(bus, ctx, &command.WithdrawAccountCommand{}, command.NewWithdrawAccountCommandHandler(db))

	queryBus := cqrs.NewBusContext()
	cqrs.Register(queryBus, context.Background(), &query.GetAccountQuery{}, query.NewGetAccountQueryHandler(db))

	accountHandler := httphandler.NewAccountHandler(queryBus, bus)
	accountHandler.RegisterHandler()

	// cqrs.Register(bus, context.Background(), (*command.CreateAccountCommand)(nil), &command.CreateAccountCommandHandler{})

	// go func() {
	// 	ticker := time.NewTicker(time.Second * 2)
	// 	defer ticker.Stop()
	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			cqrs.Dispatch(bus, context.Background(), command.NewCreateAccountCommand(&command.CreateAccountBody{}))
	// 		}
	// 	}
	// }()

	// cqrs.Dispatch(bus, context.Background(), query.NewGetAccountQuery())
	// cqrs.Dispatch(bus, context.Background(), command.NewCreateAccountCommand(&command.CreateAccountParams{}))

	// TODO: Options pattern and uber.fx ???
	// cqrs.RegisterCommand(
	// 	bus,
	// 	(*command.CreateAccountCommand)(nil),
	// 	command.NewCreateAccountCommandHandler(),
	// )
	//
	// cqrs.DispatchCommand(
	// 	bus,
	// 	context.Background(),
	// 	command.NewCreateAccountCommand(&command.CreateAccountParams{}),
	// )

	// amqpConfig := config.NewAmpqConfig()
	//
	// consumer, err := NewConsumer(amqpConfig.Address, "logs_direct")
	// if err != nil {
	// 	panic(err)
	// }
	// go consumer.Consume(context.Background(), "info-1", "info")
	// go consumer.Consume(context.Background(), "info-2", "info")
	// // go consumer.Consume(context.Background(), "warning-1", "warning")
	// // go consumer.Consume(context.Background(), "warning-2", "warning")
	// // go consumer.Consume(context.Background(), "error-1", "error")
	// // go consumer.Consume(context.Background(), "error-2", "error")
	// // _ = consumer
	//
	// ampqConn, err := NewPublisher(amqpConfig.Address, "logs_direct")
	// if err != nil {
	// 	panic(err)
	// }
	// defer ampqConn.Close()
	// _ = ampqConn
	//
	// go func() {
	// 	ticker := time.NewTicker(time.Second * 2)
	// 	defer ticker.Stop()
	//
	// 	for {
	// 		select {
	// 		case <-context.Background().Done():
	// 			return
	// 		case <-ticker.C:
	// 			ampqConn.Publish(
	// 				"info",
	// 				amqp.Publishing{
	// 					ContentType: "text/plain",
	// 					Body:        []byte(fmt.Sprintf("text from info 1 %s", time.Now())),
	// 				},
	// 			)
	// 			ampqConn.Publish(
	// 				"info",
	// 				amqp.Publishing{
	// 					ContentType:  "text/plain",
	// 					Body:         []byte(fmt.Sprintf("text from info 2 %s", time.Now())),
	// 					DeliveryMode: 2,
	// 				},
	// 			)
	// 			ampqConn.Publish(
	// 				"info",
	// 				amqp.Publishing{
	// 					ContentType:  "text/plain",
	// 					Body:         []byte(fmt.Sprintf("text from info 3 %s", time.Now())),
	// 					DeliveryMode: 2,
	// 				},
	// 			)
	//
	// 			// ampqConn.Publish(
	// 			// 	"warning",
	// 			// 	amqp.Publishing{
	// 			// 		ContentType: "text/plain",
	// 			// 		Body:        []byte("text from warning"),
	// 			// 	},
	// 			// )
	// 			// ampqConn.Publish(
	// 			// 	"error",
	// 			// 	amqp.Publishing{
	// 			// 		ContentType: "text/plain",
	// 			// 		Body:        []byte("text from error"),
	// 			// 	},
	// 			// )
	//
	// 		}
	// 	}
	// }()

	go func() {
		if err := http.ListenAndServe(":8000", nil); err != nil {
			log.Fatalf("Listen error %s", err)
		}
	}()

	// db.Create(account.Account{
	// 	ID: account.NewID(),
	// 	Balance: account.Money(20),
	// })

	var account account.Account
	db.First(&account, "id = ?", "735a7d00-ea43-4730-b4b2-9377e9ebe5ac")
	log.Printf("%+v", account)

	_ = db

	// gorm.DB()
	// httputil.HelloWorld()

	select {
	case <-sigterm:
	case <-context.Background().Done():
	}
}
