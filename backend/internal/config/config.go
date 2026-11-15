package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env      string // "development" or "production"
	Postgres PostgresConfig
	Redis    RedisConfig
	HTTP     HTTPConfig
	Auth     AuthConfig
}

// AuthConfig controls how access tokens are verified.
type AuthConfig struct {
	// SupabaseURL is the project URL, e.g. https://abcd.supabase.co.
	// JWTs are verified against <SupabaseURL>/auth/v1/.well-known/jwks.json.
	SupabaseURL string
	// LegacyJWTSecret enables HS256 verification for Supabase projects that
	// still use the legacy shared JWT secret. Leave empty for signing keys.
	LegacyJWTSecret string
	// DevMode accepts "dev:<id>" tokens without any signature check.
	// Refused when Env is "production".
	DevMode bool
}

type PostgresConfig struct {
	// URL, if set, overrides every individual field below.
	// Supabase gives you this as the "connection string".
	URL      string
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
	// AllowedOrigins are full origins (scheme://host[:port]) allowed for
	// CORS and WebSocket upgrades. path.Match wildcards are allowed,
	// e.g. https://*.vercel.app.
	AllowedOrigins []string
}

type RedisConfig struct {
	// URL, if set (redis:// or rediss://), overrides every field below
	// except PingTimeout. Render Key Value gives you this.
	URL           string
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

	cfg.Env = getEnv("APP_ENV", "development")

	// Load auth configuration
	cfg.Auth.SupabaseURL = strings.TrimRight(getEnv("SUPABASE_URL", ""), "/")
	cfg.Auth.LegacyJWTSecret = getEnv("SUPABASE_JWT_SECRET", "")
	cfg.Auth.DevMode = getEnvAsBool("AUTH_DEV_MODE", false, &errs)

	// Load PostgreSQL configuration
	cfg.Postgres.URL = getEnv("DATABASE_URL", "")
	cfg.Postgres.Username = getEnv("POSTGRES_USERNAME", defaultPostgresUsername)
	cfg.Postgres.Password = getEnv("POSTGRES_PASSWORD", defaultPostgresPassword)
	cfg.Postgres.Host = getEnv("POSTGRES_HOST", defaultPostgresHost)
	cfg.Postgres.Port = getEnvAsInt("POSTGRES_PORT", defaultPostgresPort, &errs)
	cfg.Postgres.Database = getEnv("POSTGRES_DATABASE", defaultPostgresDatabase)
	cfg.Postgres.SSLMode = getEnv("POSTGRES_SSLMODE", defaultPostgresSSLMode)
	cfg.Postgres.MaxConns = getEnvAsInt("POSTGRES_MAX_CONNS", defaultPostgresMaxConns, &errs)

	// Load Redis configuration
	cfg.Redis.URL = getEnv("REDIS_URL", "")
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
	// Render (and most PaaS) inject PORT; it wins over HTTP_PORT.
	cfg.HTTP.Port = getEnvAsInt("HTTP_PORT", defaultHTTPPort, &errs)
	if _, ok := os.LookupEnv("PORT"); ok {
		cfg.HTTP.Port = getEnvAsInt("PORT", defaultHTTPPort, &errs)
	}
	cfg.HTTP.AllowedOrigins = splitList(getEnv("ALLOWED_ORIGINS", "http://localhost:3000"))
	cfg.HTTP.ReadTimeout = getEnv("HTTP_READ_TIMEOUT", defaultHTTPReadTimeout)
	cfg.HTTP.WriteTimeout = getEnv("HTTP_WRITE_TIMEOUT", defaultHTTPWriteTimeout)
	cfg.HTTP.ShutdownTimeout = getEnv("HTTP_SHUTDOWN_TIMEOUT", defaultHTTPShutdownTimeout)

	validateConfig(&cfg, &errs)
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	return &cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int, errs *[]error) int {
	if value, exists := os.LookupEnv(key); exists {
		i, err := strconv.Atoi(value)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("%s must be an integer: %w", key, err))
			return defaultValue
		}
		return i
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool, errs *[]error) bool {
	if value, exists := os.LookupEnv(key); exists {
		i, err := strconv.ParseBool(value)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("%s must be bool: %w", key, err))
			return defaultValue
		}
		return i
	}
	return defaultValue
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validateConfig(cfg *Config, errs *[]error) {
	if cfg.Env != "development" && cfg.Env != "production" {
		*errs = append(*errs, fmt.Errorf("APP_ENV must be development or production: %q", cfg.Env))
	}
	if cfg.Auth.DevMode && cfg.Env == "production" {
		*errs = append(*errs, errors.New("AUTH_DEV_MODE must not be enabled when APP_ENV=production"))
	}
	if !cfg.Auth.DevMode && cfg.Auth.SupabaseURL == "" {
		*errs = append(*errs, errors.New("SUPABASE_URL is required unless AUTH_DEV_MODE=true"))
	}
	if len(cfg.HTTP.AllowedOrigins) == 0 {
		*errs = append(*errs, errors.New("ALLOWED_ORIGINS must list at least one origin"))
	}
	for _, o := range cfg.HTTP.AllowedOrigins {
		if o == "*" {
			*errs = append(*errs, errors.New("ALLOWED_ORIGINS must not be *"))
		}
	}

	// A URL replaces the individual fields, so skip their checks.
	if cfg.Postgres.URL == "" {
		validatePostgresFields(cfg, errs)
	}
	if cfg.Postgres.MaxConns <= 0 {
		*errs = append(*errs, fmt.Errorf("POSTGRES_MAX_CONNS must be greater than 0: %d", cfg.Postgres.MaxConns))
	}
	if cfg.Redis.URL == "" {
		validateRedisFields(cfg, errs)
	}
	validateDuration("REDIS_PING_TIMEOUT", cfg.Redis.PingTimeout, errs)

	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		*errs = append(*errs, fmt.Errorf("HTTP_PORT must be between 1 and 65535: %d", cfg.HTTP.Port))
	}
	validateDuration("HTTP_READ_TIMEOUT", cfg.HTTP.ReadTimeout, errs)
	validateDuration("HTTP_WRITE_TIMEOUT", cfg.HTTP.WriteTimeout, errs)
	validateDuration("HTTP_SHUTDOWN_TIMEOUT", cfg.HTTP.ShutdownTimeout, errs)
}

func validatePostgresFields(cfg *Config, errs *[]error) {
	if cfg.Postgres.Host == "" {
		*errs = append(*errs, errors.New("POSTGRES_HOST is required"))
	}
	if cfg.Postgres.Port <= 0 || cfg.Postgres.Port > 65535 {
		*errs = append(*errs, fmt.Errorf("POSTGRES_PORT must be between 1 and 65535: %d", cfg.Postgres.Port))
	}
	if cfg.Postgres.Database == "" {
		*errs = append(*errs, errors.New("POSTGRES_DATABASE is required"))
	}
}

func validateRedisFields(cfg *Config, errs *[]error) {
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
