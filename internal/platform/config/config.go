package config

import (
	"fmt"
	"time"

	jwtlib "github.com/Ab-dex/view-aura/internal/platform/auth/jwt"
)

// Config is the root application configuration.
// It is composed of all service configuration domains.
type Config struct {
	App   AppConfig   `mapstructure:"app"`
	HTTP  HTTPConfig  `mapstructure:"http"`
	DB    DBConfig    `mapstructure:"db"`
	Redis RedisConfig `mapstructure:"redis"`
	JWT   JWTConfig   `mapstructure:"jwt"`
	Log   LogConfig   `mapstructure:"log"`
}

type AppConfig struct {
	Name            string        `mapstructure:"name"`
	Env             string        `mapstructure:"env"` // local | staging | production
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
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // json | console
}

// LoadConfig is the final entrypoint after Loader unmarshals data.
func LoadConfig(cfg *Config) error {
	setDefaults(cfg)

	if err := validate(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	return nil
}

func setDefaults(cfg *Config) {
	if cfg.App.Name == "" {
		cfg.App.Name = "cinemaos"
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
		cfg.JWT.Issuer = "https://auth.cinemaos.com"
	}

	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
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
	if cfg.App.Env != "local" &&
		cfg.App.Env != "staging" &&
		cfg.App.Env != "production" {
		return fmt.Errorf("app.env must be local, staging, or production")
	}

	return nil
}
