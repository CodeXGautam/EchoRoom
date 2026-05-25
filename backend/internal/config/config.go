package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Postgres PostgresConfig
	Redis    RedisConfig
	HTTP     HTTPConfig
}

type PostgresConfig struct {
	Username string
	Password string
	Host     string
	Port     int
	Database string
	SSLMode  string
	MaxConns int
}

type HTTPConfig struct {
	Port            int
	ReadTimeout     string
	WriteTimeout    string
	ShutdownTimeout string
}

type RedisConfig struct {
	Username      string
	Password      string
	Host          string
	Port          int
	DB            int
	TLSEnabled    bool
	SkipTLSVerify bool
	CACertPath    string
	PingTimeout   string
}

const (
	defaultPostgresUsername = "postgres"
	defaultPostgresPassword = "postgres"
	defaultPostgresHost     = "localhost"
	defaultPostgresPort     = 5432
	defaultPostgresDatabase = "postgres"
	defaultPostgresSSLMode  = "disable"
	defaultPostgresMaxConns = 5

	defaultRedisUsername      = ""
	defaultRedisPassword      = ""
	defaultRedisHost          = "localhost"
	defaultRedisPort          = 6379
	defaultRedisDB            = 0
	defaultRedisTLSEnabled    = false
	defaultRedisSkipTLSVerify = false
	defaultRedisCACertPath    = ""
	defaultRedisPingTimeout   = "5s"

	defaultHTTPPort            = 8080
	defaultHTTPReadTimeout     = "10s"
	defaultHTTPWriteTimeout    = "30s"
	defaultHTTPShutdownTimeout = "15s"
)

func LoadConfig() (*Config, error) {
	var cfg Config
	var errs []error

	// Load PostgreSQL configuration
	cfg.Postgres.Username = getEnv("POSTGRES_USERNAME", defaultPostgresUsername)
	cfg.Postgres.Password = getEnv("POSTGRES_PASSWORD", defaultPostgresPassword)
	cfg.Postgres.Host = getEnv("POSTGRES_HOST", defaultPostgresHost)
	cfg.Postgres.Port = getEnvAsInt("POSTGRES_PORT", defaultPostgresPort, &errs)
	cfg.Postgres.Database = getEnv("POSTGRES_DATABASE", defaultPostgresDatabase)
	cfg.Postgres.SSLMode = getEnv("POSTGRES_SSLMODE", defaultPostgresSSLMode)
	cfg.Postgres.MaxConns = getEnvAsInt("POSTGRES_MAX_CONNS", defaultPostgresMaxConns, &errs)

	// Load Redis configuration
	cfg.Redis.Username = getEnv("REDIS_USERNAME", defaultRedisUsername)
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", defaultRedisPassword)
	cfg.Redis.Host = getEnv("REDIS_HOST", defaultRedisHost)
	cfg.Redis.Port = getEnvAsInt("REDIS_PORT", defaultRedisPort, &errs)
	cfg.Redis.DB = getEnvAsInt("REDIS_DB", defaultRedisDB, &errs)
	cfg.Redis.TLSEnabled = getEnvAsBool("REDIS_TLS_ENABLED", defaultRedisTLSEnabled, &errs)
	cfg.Redis.SkipTLSVerify = getEnvAsBool("REDIS_SKIP_TLS_VERIFY", defaultRedisSkipTLSVerify, &errs)
	cfg.Redis.CACertPath = getEnv("REDIS_CA_CERT_PATH", defaultRedisCACertPath)
	cfg.Redis.PingTimeout = getEnv("REDIS_PING_TIMEOUT", defaultRedisPingTimeout)

	// Load HTTP configuration
	cfg.HTTP.Port = getEnvAsInt("HTTP_PORT", defaultHTTPPort, &errs)
	cfg.HTTP.ReadTimeout = getEnv("HTTP_READ_TIMEOUT", defaultHTTPReadTimeout)
	cfg.HTTP.WriteTimeout = getEnv("HTTP_WRITE_TIMEOUT", defaultHTTPWriteTimeout)
	cfg.HTTP.ShutdownTimeout = getEnv("HTTP_SHUTDOWN_TIMEOUT", defaultHTTPShutdownTimeout)

	validateConfig(&cfg, &errs)
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	return &cfg, nil
}

func getEnv (key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt (key string, defaultValue int, errs *[]error) int {
	if value, exists := os.LookupEnv(key);exists{
		i, err := strconv.Atoi(value)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("%s must be an integer: %w", key, err))
			return defaultValue
		}
		return i
	}
	return defaultValue
} 

func getEnvAsBool (key string , defaultValue bool ,errs *[]error) bool{
	if value , exists := os.LookupEnv(key); exists{
		i, err := strconv.ParseBool(value)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("%s must be bool: %w", key , err))
			return defaultValue
		}
		return i
	}
	return defaultValue
}

func validateConfig(cfg *Config, errs *[]error) {
	if cfg.Postgres.Host == "" {
		*errs = append(*errs, errors.New("POSTGRES_HOST is required"))
	}
	if cfg.Postgres.Port <= 0 || cfg.Postgres.Port > 65535 {
		*errs = append(*errs, fmt.Errorf("POSTGRES_PORT must be between 1 and 65535: %d", cfg.Postgres.Port))
	}
	if cfg.Postgres.Database == "" {
		*errs = append(*errs, errors.New("POSTGRES_DATABASE is required"))
	}
	if cfg.Postgres.MaxConns <= 0 {
		*errs = append(*errs, fmt.Errorf("POSTGRES_MAX_CONNS must be greater than 0: %d", cfg.Postgres.MaxConns))
	}

	if cfg.Redis.Host == "" {
		*errs = append(*errs, errors.New("REDIS_HOST is required"))
	}
	if cfg.Redis.Port <= 0 || cfg.Redis.Port > 65535 {
		*errs = append(*errs, fmt.Errorf("REDIS_PORT must be between 1 and 65535: %d", cfg.Redis.Port))
	}
	if cfg.Redis.DB < 0 {
		*errs = append(*errs, fmt.Errorf("REDIS_DB must be greater than or equal to 0: %d", cfg.Redis.DB))
	}
	if !cfg.Redis.TLSEnabled {
		if cfg.Redis.CACertPath != "" {
			*errs = append(*errs, errors.New("REDIS_TLS_ENABLED must be true when REDIS_CA_CERT_PATH is set"))
		}
	}
	validateDuration("REDIS_PING_TIMEOUT", cfg.Redis.PingTimeout, errs)

	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		*errs = append(*errs, fmt.Errorf("HTTP_PORT must be between 1 and 65535: %d", cfg.HTTP.Port))
	}
	validateDuration("HTTP_READ_TIMEOUT", cfg.HTTP.ReadTimeout, errs)
	validateDuration("HTTP_WRITE_TIMEOUT", cfg.HTTP.WriteTimeout, errs)
	validateDuration("HTTP_SHUTDOWN_TIMEOUT", cfg.HTTP.ShutdownTimeout, errs)
}

func validateDuration(key, value string, errs *[]error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be a valid duration: %w", key, err))
		return
	}
	if duration <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be greater than 0: %s", key, value))
	}
}