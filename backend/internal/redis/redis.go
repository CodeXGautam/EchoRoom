package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"fmt"
	"time"

	"github.com/CodeXGautam/echoroom/internal/config"
	"github.com/redis/go-redis/v9"
)

func  NewClient(ctx context.Context, cfg *config.RedisConfig) (*redis.Client, error){
	tlsConfig , err := buildTlsConfig(cfg)
	if err != nil {
		return nil , err
	}

	client := redis.NewClient(&redis.Options{
		Addr:      fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Username:  cfg.Username,
		Password:  cfg.Password,
		DB:        cfg.DB,
		TLSConfig: tlsConfig,
	})

	pingTimeout , err := time.ParseDuration(cfg.PingTimeout)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("parse redis ping timeout: %w", err)
	}
	pingCtx,cancel := context.WithTimeout(ctx,pingTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return client, nil
}

func buildTlsConfig (cfg *config.RedisConfig) (*tls.Config, error){
	if !cfg.TLSEnabled {
		return nil , nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		InsecureSkipVerify: cfg.SkipTLSVerify,
	}

	if cfg.CACertPath != ""{
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
