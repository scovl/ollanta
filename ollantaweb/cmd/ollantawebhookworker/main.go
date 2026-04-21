package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/config"
	"github.com/scovl/ollanta/ollantaweb/webhook"
)

func main() {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("ollantawebhookworker: connect postgres: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("ollantawebhookworker: migrate: %v", err)
	}

	webhookRepo := postgres.NewWebhookRepository(db)
	webhookJobRepo := postgres.NewWebhookJobRepository(db)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ollantawebhookworker"
	}
	workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	dispatcher := webhook.NewDispatcher(webhookRepo, webhookJobRepo, workerID)
	log.Printf("ollantawebhookworker: started as %s", workerID)
	dispatcher.Start(ctx)
	log.Println("ollantawebhookworker: stopped")
}
