package db

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/CodeXGautam/echoroom/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultMaxConns int32 = 5
const defaultPingTimeout = 5 * time.Second

type DB struct {
	pool *pgxpool.Pool
}

func NewPool(ctx context.Context, cfg *config.PostgresConfig) (*DB, error) {
	if ctx == nil {
		return nil, errors.New("Postgres context is nil")
	}
	poolconfig, err := buildPoolConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build pool config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolconfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect postgres: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)

	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (p *DB) Close() {
	if p == nil || p.pool == nil {
		return
	}

	p.pool.Close()
}

func (p *DB) Pool() *pgxpool.Pool {
	if p == nil {
		return nil
	}

	return p.pool
}

func buildPoolConfig(cfg *config.PostgresConfig) (*pgxpool.Config, error) {
	if cfg == nil {
		return nil, errors.New("postgres config is nil")
	}

	// DATABASE_URL wins. For Supabase's transaction pooler (port 6543) append
	// default_query_exec_mode=simple_protocol: that pooler does not support
	// the prepared statements pgx uses by default.
	dsn := strings.TrimSpace(cfg.URL)
	if dsn == "" {
		var err error
		dsn, err = buildDSN(cfg)
		if err != nil {
			return nil, fmt.Errorf("build postgres dsn: %w", err)
		}
	}

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	poolConfig.MaxConns = defaultMaxConns
	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = int32(cfg.MaxConns)
	}

	return poolConfig, nil
}

func buildDSN(cfg *config.PostgresConfig) (string, error) {
	if cfg == nil {
		return "", errors.New("postgres config is nil")
	}

	host := strings.TrimSpace(cfg.Host)
	database := strings.TrimSpace(cfg.Database)
	sslMode := strings.TrimSpace(cfg.SSLMode)
	if sslMode == "" {
		sslMode = "disable"
	}

	if host == "" {
		return "", errors.New("postgres host is required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return "", fmt.Errorf("postgres port must be between 1 and 65535: %d", cfg.Port)
	}
	if database == "" {
		return "", errors.New("postgres database is required")
	}
	if cfg.MaxConns < 0 {
		return "", fmt.Errorf("postgres max connections cannot be negative: %d", cfg.MaxConns)
	}

	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   net.JoinHostPort(host, fmt.Sprintf("%d", cfg.Port)),
		Path:   database,
	}

	query := dsn.Query()
	query.Set("sslmode", sslMode)
	dsn.RawQuery = query.Encode()

	return dsn.String(), nil
}
