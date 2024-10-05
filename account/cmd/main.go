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
	"github.com/romashorodok/test-task-bank-account/account/pkg/config"
	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"
	"github.com/romashorodok/test-task-bank-account/contrib/httputil"
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

	accountHandler := httphandler.NewAccountHandler()
	accountHandler.RegisterHandler()

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
	httputil.HelloWorld()

	select {
	case <-sigterm:
	case <-context.Background().Done():
	}
}
