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
	Auth         AuthConfig         `mapstructure:"auth"`
	EmailClient  EmailConfig        `mapstructure:"email_client"`
	// Resilience controls the three-tier degradation behaviour for every
	// external dependency (Kafka, email, push, Stripe, Temporal, R2).
	// All fields have safe defaults — existing deployments need no changes.
	Resilience ResilienceConfig `mapstructure:"resilience"`
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
	Secret          string        `mapstructure:"secret"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type AuthConfig struct {
	MFAEncryptionKey string      `mapstructure:"mfa_encryption_key"` // 32-byte hex, from Vault
	BaseURL          string      `mapstructure:"base_url"`           // e.g. "https://viewaura.com"
	Google           GoogleOAuth `mapstructure:"google"`
	Apple            AppleOAuth  `mapstructure:"apple"`
}

type EmailConfig struct {
	BaseURL string `mapstructure:"base_url"`
	ApiKey  string `mapstructure:"api_key"`
}

type GoogleOAuth struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

type AppleOAuth struct {
	ClientID   string `mapstructure:"client_id"`
	TeamID     string `mapstructure:"team_id"`
	KeyID      string `mapstructure:"key_id"`
	PrivateKey string `mapstructure:"private_key"`
}

// ─── New config types ─────────────────────────────────────────────────────────

type StripeConfig struct {
	SecretKey     string `mapstructure:"secret_key"`
	WebhookSecret string `mapstructure:"webhook_secret"`
	PriceIDPro    string `mapstructure:"price_id_pro"`
	PriceIDStudio string `mapstructure:"price_id_studio"`
}

type R2Config struct {
	AccountID       string        `mapstructure:"account_id"`
	Bucket          string        `mapstructure:"bucket"`
	AccessKeyID     string        `mapstructure:"access_key_id"`
	SecretAccessKey string        `mapstructure:"secret_access_key"`
	PresignTTL      time.Duration `mapstructure:"presign_ttl"`
}

type TemporalConfig struct {
	HostPort  string `mapstructure:"host_port"`
	Namespace string `mapstructure:"namespace"`
	TaskQueue string `mapstructure:"task_queue"`
}

type KafkaConfig struct {
	Brokers          string `mapstructure:"brokers"`
	GroupID          string `mapstructure:"group_id"`
	SecurityProtocol string `mapstructure:"security_protocol"`
	SASLMechanism    string `mapstructure:"sasl_mechanism"`
	SASLUsername     string `mapstructure:"sasl_username"`
	SASLPassword     string `mapstructure:"sasl_password"`
}

type TracingConfig struct {
	Endpoint       string  `mapstructure:"endpoint"`
	ServiceName    string  `mapstructure:"service_name"`
	ServiceVersion string  `mapstructure:"service_version"`
	Environment    string  `mapstructure:"environment"`
	SampleRate     float64 `mapstructure:"sample_rate"`
}

type ModerationAIConfig struct {
	Endpoint string `mapstructure:"endpoint"`
}

type GatewayConfig struct {
	Addr           string   `mapstructure:"addr"`
	UpstreamURL    string   `mapstructure:"upstream_url"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	RateLimitRPM   int      `mapstructure:"rate_limit_rpm"`
	SkipAuthPaths  []string `mapstructure:"skip_auth_paths"`
}

// ─── Defaults ─────────────────────────────────────────────────────────────────

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

	// Auth base URL defaults per environment.
	if cfg.Auth.BaseURL == "" {
		if cfg.App.Env == "local" {
			cfg.Auth.BaseURL = "http://localhost:3000"
		} else {
			cfg.Auth.BaseURL = "https://viewaura.com"
		}
	}

	setTracingDefaults(cfg)
	setGatewayDefaults(cfg)
	setResilienceDefaults(cfg)
}

func setResilienceDefaults(cfg *Config) {
	if cfg.Resilience.LogDir == "" {
		cfg.Resilience.LogDir = "logs"
	}
	if cfg.Resilience.EmailServiceURL == "" {
		// In local dev the email service is typically not running.
		// Leave empty — mailer degrades to Redis/file outbox silently.
	}
	if cfg.Resilience.TemporalDeferredKey == "" {
		cfg.Resilience.TemporalDeferredKey = "outbox:temporal:deferred"
	}
	if cfg.Resilience.StripePendingKey == "" {
		cfg.Resilience.StripePendingKey = "outbox:payment:pending"
	}
	if cfg.Resilience.PushOutboxKeyPrefix == "" {
		cfg.Resilience.PushOutboxKeyPrefix = "outbox:push:"
	}
	if cfg.Resilience.R2TempUploadDir == "" {
		cfg.Resilience.R2TempUploadDir = "/tmp/viewaura-uploads"
	}
	if cfg.Resilience.R2TempMaxBytes == 0 {
		cfg.Resilience.R2TempMaxBytes = 500 * 1024 * 1024 // 500 MB
	}
}

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

func validate(cfg *Config) error {
	// Database is the only hard requirement — the app cannot function without it.
	if cfg.DB.DSN == "" {
		return fmt.Errorf("db.dsn is required")
	}

	// Redis is optional — absence means the app runs in degraded mode.
	// No error is returned; ProvideRedis returns nil and all components guard.

	// JWT signing must be configured.
	if cfg.JWT.Secret == "" {
		if cfg.JWT.PrivateKeyPath == "" || cfg.JWT.PublicKeyPath == "" {
			return fmt.Errorf("either jwt.secret or both jwt.private_key_path and jwt.public_key_path must be provided")
		}
	}

	if cfg.App.Env != "local" && cfg.App.Env != "staging" && cfg.App.Env != "production" {
		return fmt.Errorf("app.env must be local, staging, or production")
	}

	return nil
}
