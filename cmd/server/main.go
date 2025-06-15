package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	_ "embed"

	"github.com/vikblom/lilygo/pkg/api"
	"github.com/vikblom/lilygo/pkg/db"
)

func run(ctx context.Context) error {
	log.Println("start")

	db, err := db.New("db.sqlite")
	if err != nil {
		return err
	}

	api, err := api.New(db)
	if err != nil {
		return err
	}

	srv := http.Server{
		Addr:    ":9000",
		Handler: api,
	}

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return ctx.Err()
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()

	err := run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
