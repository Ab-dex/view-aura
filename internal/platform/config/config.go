package config

import (
	"fmt"
	"time"

	jwtlib "github.com/Ab-dex/view-aura/internal/platform/auth/jwt"
)

// Config is the root application configuration.
type Config struct {
	App          AppConfig          `mapstructure:"app"`
	HTTP         HTTPConfig         `mapstructure:"http"`
	DB           DBConfig           `mapstructure:"db"`
	Redis        RedisConfig        `mapstructure:"redis"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	Log          LogConfig          `mapstructure:"log"`
	Stripe       StripeConfig       `mapstructure:"stripe"`
	R2           R2Config           `mapstructure:"r2"`
	Temporal     TemporalConfig     `mapstructure:"temporal"`
	Kafka        KafkaConfig        `mapstructure:"kafka"`
	Tracing      TracingConfig      `mapstructure:"tracing"`
	ModerationAI ModerationAIConfig `mapstructure:"moderation_ai"`
	Gateway      GatewayConfig      `mapstructure:"gateway"`
}

// ─── Existing config types (unchanged) ───────────────────────────────────────

type AppConfig struct {
	Name            string        `mapstructure:"name"`
	Env             string        `mapstructure:"env"`
	Version         string        `mapstructure:"version"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type HTTPConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type DBConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
}

type JWTConfig struct {
	PrivateKeyPath  string        `mapstructure:"private_key_path"`
	PublicKeyPath   string        `mapstructure:"public_key_path"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	Issuer          string        `mapstructure:"issuer"`
	Method          jwtlib.Method `mapstructure:"method"`
	Secret          []byte        `mapstructure:"secret"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// ─── New config types ─────────────────────────────────────────────────────────

// StripeConfig holds Stripe payment credentials.
type StripeConfig struct {
	SecretKey     string `mapstructure:"secret_key"`
	WebhookSecret string `mapstructure:"webhook_secret"`

	// Price IDs from the Stripe dashboard — set once per environment.
	PriceIDPro    string `mapstructure:"price_id_pro"`
	PriceIDStudio string `mapstructure:"price_id_studio"`
}

// R2Config holds Cloudflare R2 object storage credentials.
type R2Config struct {
	AccountID       string `mapstructure:"account_id"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	// PresignTTL is how long presigned upload/download URLs remain valid.
	// Defaults to 15 minutes if zero.
	PresignTTL time.Duration `mapstructure:"presign_ttl"`
}

// TemporalConfig holds Temporal workflow server connection details.
type TemporalConfig struct {
	HostPort  string `mapstructure:"host_port"`  // e.g. "temporal:7233"
	Namespace string `mapstructure:"namespace"`  // e.g. "viewaura"
	TaskQueue string `mapstructure:"task_queue"` // e.g. "viewaura-main"
}

// KafkaConfig holds Apache Kafka broker and producer/consumer settings.
type KafkaConfig struct {
	Brokers          string `mapstructure:"brokers"`
	GroupID          string `mapstructure:"group_id"`
	SecurityProtocol string `mapstructure:"security_protocol"`
	SASLMechanism    string `mapstructure:"sasl_mechanism"`
	SASLUsername     string `mapstructure:"sasl_username"`
	SASLPassword     string `mapstructure:"sasl_password"`
}

// TracingConfig holds OpenTelemetry exporter settings.
//
// config.yaml example:
//
//	tracing:
//	  endpoint:        "otel-collector:4317"   # OTLP gRPC endpoint; empty = no-op
//	  service_name:    "viewaura-api"
//	  service_version: "1.0.0"
//	  environment:     "production"
//	  sample_rate:     0.1                      # 10% of successful requests
type TracingConfig struct {
	// Endpoint is the OTLP gRPC address of the collector.
	// Leave empty to disable tracing (no-op provider used).
	Endpoint string `mapstructure:"endpoint"`

	// ServiceName is the logical name of this service in traces.
	ServiceName string `mapstructure:"service_name"`

	// ServiceVersion is embedded in every span for deployment correlation.
	ServiceVersion string `mapstructure:"service_version"`

	// Environment is "local" | "staging" | "production".
	Environment string `mapstructure:"environment"`

	// SampleRate controls what fraction of successful traces are exported.
	// 0.1 = 10% (PRD spec). Errors are always sampled (ParentBased sampler).
	// Range: 0.0–1.0. Defaults to 0.1 when zero.
	SampleRate float64 `mapstructure:"sample_rate"`
}

// ModerationAIConfig configures the Python moderation-ai service client.
type ModerationAIConfig struct {
	// Endpoint is the base URL of the Python moderation-ai service.
	// Leave empty to use the NoopAIClient (always approves — for local dev).
	Endpoint string `mapstructure:"endpoint"`
}

// GatewayConfig configures the lightweight Go API gateway used in local
// development and integration tests. In production this role is fulfilled
// by Kong; these settings are ignored when Kong is the entry point.
//
// config.yaml example:
//
//	gateway:
//	  addr:           ":8000"
//	  upstream_url:   "http://cinemaos-api:8080"
//	  allowed_origins:
//	    - "https://cinemaos.com"
//	    - "https://www.cinemaos.com"
//	  rate_limit_rpm: 1000
//	  skip_auth_paths:
//	    - "/api/v1/users/register"
//	    - "/api/v1/users/login"
//	    - "/api/v1/users/refresh"
//	    - "/api/v1/movies"
//	    - "/api/v1/movies/genres"
//	    - "/api/v1/webhook/stripe"
type GatewayConfig struct {
	// Addr is the listen address for the gateway server.
	// Defaults to ":8000".
	Addr string `mapstructure:"addr"`

	// UpstreamURL is the base URL of the monolith API service the gateway
	// proxies to. Defaults to "http://cinemaos-api:8080".
	UpstreamURL string `mapstructure:"upstream_url"`

	// AllowedOrigins is the list of exact CORS origins the gateway permits.
	// The wildcard "*" is intentionally not supported to prevent
	// credential-bearing cross-site requests.
	AllowedOrigins []string `mapstructure:"allowed_origins"`

	// RateLimitRPM is the sliding-window rate limit applied per client IP,
	// expressed as requests per minute. Defaults to 100 for unauthenticated
	// and 1000 for authenticated callers.
	RateLimitRPM int `mapstructure:"rate_limit_rpm"`

	// SkipAuthPaths is the list of URL path prefixes that bypass JWT
	// validation. Stripe webhooks and public read endpoints belong here.
	SkipAuthPaths []string `mapstructure:"skip_auth_paths"`
}

// ─── Defaults and helpers ─────────────────────────────────────────────────────

func setTracingDefaults(cfg *Config) {
	if cfg.Tracing.ServiceName == "" {
		cfg.Tracing.ServiceName = cfg.App.Name
	}
	if cfg.Tracing.Environment == "" {
		cfg.Tracing.Environment = cfg.App.Env
	}
	if cfg.Tracing.SampleRate == 0 {
		cfg.Tracing.SampleRate = 0.1
	}
}

func setGatewayDefaults(cfg *Config) {
	if cfg.Gateway.Addr == "" {
		cfg.Gateway.Addr = ":8000"
	}
	if cfg.Gateway.UpstreamURL == "" {
		cfg.Gateway.UpstreamURL = "http://cinemaos-api:8080"
	}
	if cfg.Gateway.RateLimitRPM == 0 {
		cfg.Gateway.RateLimitRPM = 100
	}
	if len(cfg.Gateway.AllowedOrigins) == 0 {
		cfg.Gateway.AllowedOrigins = []string{
			"http://localhost:3000",
			"http://localhost:5173",
		}
	}
	if len(cfg.Gateway.SkipAuthPaths) == 0 {
		cfg.Gateway.SkipAuthPaths = []string{
			"/api/v1/users/register",
			"/api/v1/users/login",
			"/api/v1/users/refresh",
			"/api/v1/movies",
			"/api/v1/movies/genres",
			"/api/v1/webhook/stripe",
		}
	}
}

// ─── Lifecycle ────────────────────────────────────────────────────────────────

func LoadConfig(cfg *Config) error {
	setDefaults(cfg)
	if err := validate(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}

func setDefaults(cfg *Config) {
	if cfg.App.Name == "" {
		cfg.App.Name = "viewaura"
	}
	if cfg.App.Env == "" {
		cfg.App.Env = "local"
	}
	if cfg.App.ShutdownTimeout == 0 {
		cfg.App.ShutdownTimeout = 30 * time.Second
	}
	if cfg.HTTP.Host == "" {
		cfg.HTTP.Host = "0.0.0.0"
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 8080
	}
	if cfg.HTTP.ReadTimeout == 0 {
		cfg.HTTP.ReadTimeout = 10 * time.Second
	}
	if cfg.HTTP.WriteTimeout == 0 {
		cfg.HTTP.WriteTimeout = 30 * time.Second
	}
	if cfg.HTTP.IdleTimeout == 0 {
		cfg.HTTP.IdleTimeout = 120 * time.Second
	}
	if cfg.DB.MaxOpenConns == 0 {
		cfg.DB.MaxOpenConns = 50
	}
	if cfg.DB.MaxIdleConns == 0 {
		cfg.DB.MaxIdleConns = 10
	}
	if cfg.DB.ConnMaxLifetime == 0 {
		cfg.DB.ConnMaxLifetime = 30 * time.Minute
	}
	if cfg.DB.ConnMaxIdleTime == 0 {
		cfg.DB.ConnMaxIdleTime = 5 * time.Minute
	}
	if cfg.Redis.DialTimeout == 0 {
		cfg.Redis.DialTimeout = 5 * time.Second
	}
	if cfg.Redis.ReadTimeout == 0 {
		cfg.Redis.ReadTimeout = 3 * time.Second
	}
	if cfg.Redis.WriteTimeout == 0 {
		cfg.Redis.WriteTimeout = 3 * time.Second
	}
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = 20
	}
	if cfg.Redis.MinIdleConns == 0 {
		cfg.Redis.MinIdleConns = 5
	}
	if cfg.JWT.AccessTokenTTL == 0 {
		cfg.JWT.AccessTokenTTL = time.Hour
	}
	if cfg.JWT.RefreshTokenTTL == 0 {
		cfg.JWT.RefreshTokenTTL = 30 * 24 * time.Hour
	}
	if cfg.JWT.Issuer == "" {
		cfg.JWT.Issuer = "https://auth.viewaura.com"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	if cfg.Temporal.HostPort == "" {
		cfg.Temporal.HostPort = "localhost:7233"
	}
	if cfg.Temporal.Namespace == "" {
		cfg.Temporal.Namespace = "viewaura"
	}
	if cfg.Temporal.TaskQueue == "" {
		cfg.Temporal.TaskQueue = "viewaura-main"
	}
	if cfg.Kafka.GroupID == "" {
		cfg.Kafka.GroupID = "viewaura-worker"
	}

	setTracingDefaults(cfg)
	setGatewayDefaults(cfg)
}

func validate(cfg *Config) error {
	if cfg.DB.DSN == "" {
		return fmt.Errorf("db.dsn is required")
	}
	if cfg.Redis.Addr == "" {
		return fmt.Errorf("redis.addr is required")
	}
	if cfg.JWT.PrivateKeyPath == "" {
		return fmt.Errorf("jwt.private_key_path is required")
	}
	if cfg.JWT.PublicKeyPath == "" {
		return fmt.Errorf("jwt.public_key_path is required")
	}
	if cfg.App.Env != "local" && cfg.App.Env != "staging" && cfg.App.Env != "production" {
		return fmt.Errorf("app.env must be local, staging, or production")
	}
	return nil
}
