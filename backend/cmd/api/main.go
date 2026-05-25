package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/CodeXGautam/echoroom/internal/config"
	"github.com/CodeXGautam/echoroom/internal/db"
	"github.com/CodeXGautam/echoroom/internal/http"
	"github.com/CodeXGautam/echoroom/internal/redis"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.LoadConfig()

	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.Default()
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pg, err := db.NewPool(ctx, &cfg.Postgres)
	if err != nil {
		slog.Error("fatal", "err: failed to initialize postgres", err)
		return fmt.Errorf("initialize postgres: %w", err)
	}
	defer pg.Close()

	redis, err := redis.NewClient(ctx, &cfg.Redis)
	if err != nil {
		return fmt.Errorf("initialize redis: %w", err)
	}
	defer func() { _ = redis.Close() }()

	server := http.NewServer(cfg.HTTP, logger, pg.Pool(), redis)

	go func() {
		if err := server.Start(); err != nil {
			logger.Error("http_server_start_failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown_signal_received")

	if err := server.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	return nil
}
