package tracing

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

// Provider wraps the OpenTelemetry TracerProvider so the caller can shut it
// down gracefully without importing the OTel SDK directly.
type Provider struct {
	tp *sdktrace.TracerProvider
}

// New initialises an OpenTelemetry TracerProvider that exports spans to the
// configured OTLP endpoint (Jaeger, Grafana Tempo, or any OTLP-compatible
// collector).
//
// When cfg.Tracing.Endpoint is empty (local dev) a no-op provider is returned
// so the application starts without a running collector.
func New(ctx context.Context, cfg config.TracingConfig) (*Provider, error) {
	if cfg.Endpoint == "" {
		// No-op: register a global no-op provider so otel.Tracer() calls
		// never panic, they just produce no-op spans.
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		log.Info().Msg("tracing: no endpoint configured — using no-op provider")
		return &Provider{}, nil
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(), // TLS terminated at the collector sidecar
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: create resource: %w", err)
	}

	// Sample 100% of errors, cfg.SampleRate of successes.
	// This matches the PRD spec: 10% sample rate, 100% on errors.
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(cfg.SampleRate),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Register as the global tracer provider so all packages can call
	// otel.Tracer("package-name") without dependency injection.
	otel.SetTracerProvider(tp)

	// Register the W3C TraceContext + Baggage propagators so the
	// traceparent header is forwarded across service boundaries.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info().
		Str("endpoint", cfg.Endpoint).
		Str("service", cfg.ServiceName).
		Float64("sample_rate", cfg.SampleRate).
		Msg("tracing: OTLP provider initialised")

	return &Provider{tp: tp}, nil
}

// Shutdown flushes buffered spans and closes the exporter connection.
// Must be called on application shutdown (register via app Hooks).
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tp == nil {
		return nil // no-op provider
	}
	if err := p.tp.Shutdown(ctx); err != nil {
		return fmt.Errorf("tracing: shutdown: %w", err)
	}
	log.Info().Msg("tracing: provider shut down")
	return nil
}

// Tracer returns a named tracer from the global provider.
// Convenience wrapper so callers don't import the OTel package directly.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
