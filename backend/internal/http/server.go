package http

import (
	"log/slog"
	"net/http"
	"time"
	"fmt"
	"errors"
	"context"

	"github.com/CodeXGautam/echoroom/internal/config"
	"github.com/CodeXGautam/echoroom/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	srv    *http.Server
	logger *slog.Logger
	cfg    	config.HTTPConfig
}

func NewServer (cfg config.HTTPConfig, logger *slog.Logger, pg *pgxpool.Pool, rc *redis.Client) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Recover(logger))

	readTimeout, err := time.ParseDuration(cfg.ReadTimeout)
	if err != nil {
		logger.Warn("invalid_http_read_timeout_using_default", "value", cfg.ReadTimeout, "err", err)
		readTimeout = 10 * time.Second
	}

	writeTimeout, err := time.ParseDuration(cfg.WriteTimeout)
	if err != nil {
		logger.Warn("invalid_http_write_timeout_using_default", "value", cfg.WriteTimeout, "err", err)
		writeTimeout = 30 * time.Second
	}

	return &Server{
		srv: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      r,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
		logger: logger,
		cfg:    cfg,
	}
	
}

func (s *Server) Start () error{
	if s == nil || s.srv == nil {
		return errors.New("http server is not configured")
	}

	s.logger.Info("http_server_starting", "addr", s.srv.Addr)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return errors.New("http server is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	shutdownTimeout, err := time.ParseDuration(s.cfg.ShutdownTimeout)
	if err != nil {
		s.logger.Warn("invalid_http_shutdown_timeout_using_default", "value", s.cfg.ShutdownTimeout, "err", err)
		shutdownTimeout = 15 * time.Second
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	s.logger.Info("http_server_shutting_down")
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}