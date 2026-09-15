package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"claim-watcher/internal/config"
	dbrepo "claim-watcher/internal/db"
	"claim-watcher/internal/mailer"
	"claim-watcher/internal/watcher"

	"github.com/joho/godotenv"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)

	// Load .env if present.
	// Existing OS environment variables are NOT overwritten.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf("Error loading .env file: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	repo, err := dbrepo.New(runCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer repo.Close()

	m, err := mailer.New(cfg)
	if err != nil {
		log.Fatalf("mailer configuration error: %v", err)
	}

	service := watcher.New(cfg, repo, m)
	if err := service.Run(runCtx); err != nil {
		log.Fatalf("claim watcher failed: %v", err)
	}
}
