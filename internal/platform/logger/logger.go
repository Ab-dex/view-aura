package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// contextKey is unexported to avoid collisions across packages.
type contextKey struct{}

var LoggerKey = contextKey{}

// Init configures the global zerolog logger.
// Call once at process startup.
func Init(level, format string) {
	zlevel, err := zerolog.ParseLevel(level)
	if err != nil {
		zlevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(zlevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var w io.Writer
	if format == "console" {
		w = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	} else {
		w = os.Stdout
	}

	log.Logger = zerolog.New(w).With().
		Timestamp().
		Caller().
		Logger()
}

// WithContext attaches a child logger carrying contextual fields to ctx.
func WithContext(ctx context.Context, fields map[string]string) context.Context {
	l := zerolog.Ctx(ctx)
	ll := l.With()
	for k, v := range fields {
		ll = ll.Str(k, v)
	}
	logger := ll.Logger()
	return logger.WithContext(ctx)
}

// FromContext extracts the logger stored in ctx, falling back to the global
// logger so callers never need a nil check.
func FromContext(ctx context.Context) *zerolog.Logger {
	l := zerolog.Ctx(ctx)
	if l == nil || l.GetLevel() == zerolog.Disabled {
		return &log.Logger
	}
	return l
}
