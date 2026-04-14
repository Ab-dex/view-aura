package tracing

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Middleware returns a Gin middleware that:
//  1. Extracts an incoming traceparent / tracestate header (W3C TraceContext).
//  2. Starts a server span for the request.
//  3. Injects the span context into the request context so downstream
//     calls (DB, Redis, gRPC, Kafka) can create child spans.
//  4. Records the HTTP status code and marks the span as error on 5xx.
//
// The serviceName should match TracingConfig.ServiceName so Jaeger / Grafana
// can correlate spans from the same service across requests.
func Middleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
