package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/CodeXGautam/echoroom/internal/auth"
	"github.com/CodeXGautam/echoroom/internal/config"
	"github.com/CodeXGautam/echoroom/internal/db"
	"github.com/CodeXGautam/echoroom/internal/http"
	"github.com/CodeXGautam/echoroom/internal/logger"
	"github.com/CodeXGautam/echoroom/internal/redis"
	"github.com/CodeXGautam/echoroom/internal/room"
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

	log := logger.Log
	slog.SetDefault(log)

	// ctx ends on Ctrl-C locally, or SIGTERM from Render / Kubernetes.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pg, err := db.NewPool(ctx, &cfg.Postgres)
	if err != nil {
		return fmt.Errorf("initialize postgres: %w", err)
	}
	defer pg.Close()

	rdb, err := redis.NewClient(ctx, &cfg.Redis)
	if err != nil {
		return fmt.Errorf("initialize redis: %w", err)
	}
	defer func() { _ = rdb.Close() }()

	// Background work (JWKS refresh, room loops) gets its own context so it
	// keeps running while the HTTP server drains, then stops after.
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	verifier, err := newVerifier(bgCtx, cfg.Auth)
	if err != nil {
		return fmt.Errorf("initialize auth: %w", err)
	}
	if cfg.Auth.DevMode {
		log.Warn("auth_dev_mode_enabled", "detail", "tokens are NOT verified; never use in production")
	}

	rooms := room.NewManager(bgCtx, log)

	server := http.NewServer(cfg.HTTP, log, http.Deps{
		PG:       pg.Pool(),
		Redis:    rdb,
		Verifier: verifier,
		Rooms:    rooms,
	})

	go func() {
		if err := server.Start(); err != nil {
			log.Error("http_server_start_failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutdown_signal_received")

	// 1. Stop accepting new HTTP requests and finish in-flight ones.
	//    http.Server.Shutdown does NOT wait for hijacked WebSocket conns…
	shutdownErr := server.Shutdown(context.Background())
	// 2. …so stop the rooms: every client gets close code 1001 and the
	//    frontend reconnects (in Kubernetes, to another pod).
	bgCancel()
	rooms.Wait()
	log.Info("shutdown_complete")

	if shutdownErr != nil {
		return fmt.Errorf("shutdown http server: %w", shutdownErr)
	}
	return nil
}

func newVerifier(ctx context.Context, cfg config.AuthConfig) (auth.Verifier, error) {
	if cfg.DevMode {
		return auth.DevVerifier{}, nil
	}
	return auth.NewSupabaseVerifier(ctx, cfg.SupabaseURL, cfg.LegacyJWTSecret)
}
