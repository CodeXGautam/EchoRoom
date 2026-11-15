package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/CodeXGautam/echoroom/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, cfg *config.RedisConfig) (*redis.Client, error) {
	opts, err := buildOptions(cfg)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)

	pingTimeout, err := time.ParseDuration(cfg.PingTimeout)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("parse redis ping timeout: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return client, nil
}

// buildOptions prefers REDIS_URL (redis:// or rediss:// for TLS), which is
// what Render Key Value and most hosted Redis providers hand out.
func buildOptions(cfg *config.RedisConfig) (*redis.Options, error) {
	if cfg.URL != "" {
		opts, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("parse REDIS_URL: %w", err)
		}
		return opts, nil
	}

	tlsConfig, err := buildTlsConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &redis.Options{
		Addr:      fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Username:  cfg.Username,
		Password:  cfg.Password,
		DB:        cfg.DB,
		TLSConfig: tlsConfig,
	}, nil
}

func buildTlsConfig(cfg *config.RedisConfig) (*tls.Config, error) {
	if !cfg.TLSEnabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.SkipTLSVerify,
	}

	if cfg.CACertPath != "" {
		caCert, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("read redis CA certificate: %w", err)
		}

		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system cert pool: %w", err)
		}
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("append redis CA certificate: invalid PEM in %s", cfg.CACertPath)
		}
		tlsConfig.RootCAs = rootCAs
	}
	return tlsConfig, nil
}
