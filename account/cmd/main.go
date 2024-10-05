package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/romashorodok/test-task-bank-account/contrib/httputil"
	_ "gorm.io/gorm"
)

func main() {
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM, syscall.SIGINT)

	// gorm.DB()
	httputil.HelloWorld()

	select {
	case <-sigterm:
	case <-context.Background().Done():
	}
}
