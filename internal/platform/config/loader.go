package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Loader handles config loading (shared infra utility)
type Loader struct {
	v *viper.Viper
}

// New creates a config loader instance
func New(path string, envPrefix string) (*Loader, error) {
	v := viper.New()

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// ENV support
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()

	// IMPORTANT: map ENV like VIEWAURA_DB_DSN → db.dsn
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: read file: %w", err)
	}

	return &Loader{v: v}, nil
}

// Unmarshal maps config into struct
func (l *Loader) Unmarshal(out any) error {
	if err := l.v.Unmarshal(out); err != nil {
		return fmt.Errorf("config: unmarshal: %w", err)
	}
	return nil
}

// Raw access (debug only)
func (l *Loader) All() map[string]any {
	return l.v.AllSettings()
}

// wrapper around LoadConfig to keep internal/platform/config self-contained. App calls Load, which creates a Loader, unmarshals into Config, then validates and sets defaults.
func Load(path string) (*Config, error) {
	loader, err := New(path, "VIEWAURA")
	if err != nil {
		return nil, err
	}

	var cfg Config

	if err := loader.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := LoadConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
