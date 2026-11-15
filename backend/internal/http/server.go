package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/CodeXGautam/echoroom/internal/auth"
	"github.com/CodeXGautam/echoroom/internal/config"
	"github.com/CodeXGautam/echoroom/internal/middleware"
	"github.com/CodeXGautam/echoroom/internal/room"
	"github.com/CodeXGautam/echoroom/internal/ws"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	srv    *http.Server
	logger *slog.Logger
	cfg    config.HTTPConfig
}

// Deps are the long-lived services handlers need.
type Deps struct {
	PG       *pgxpool.Pool
	Redis    *redis.Client
	Verifier auth.Verifier
	Rooms    *room.Manager
}

func NewServer(cfg config.HTTPConfig, logger *slog.Logger, deps Deps) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Recover(logger))

	tickets := ws.NewTicketStore(deps.Redis)

	// Liveness: the process is up. Never touches dependencies, so a Redis
	// blip doesn't make Kubernetes/Render restart a healthy process.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// Readiness: dependencies reachable, safe to send traffic here.
	r.Get("/readyz", readyHandler(deps))

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(cors.Handler(cors.Options{
			AllowedOrigins: cfg.AllowedOrigins,
			AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
			AllowedHeaders: []string{"Authorization", "Content-Type"},
			MaxAge:         600,
		}))
		api.With(auth.Middleware(deps.Verifier)).Post("/ws/ticket", ws.TicketHandler(tickets, logger))
	})

	r.Method(http.MethodGet, "/ws", &ws.Handler{
		Tickets:        tickets,
		Rooms:          deps.Rooms,
		OriginPatterns: cfg.AllowedOrigins,
		Log:            logger,
	})

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

func (s *Server) Start() error {
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
