package telemetry

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// Middleware returns a Gin middleware that records HTTP request duration and
// active request count via OpenTelemetry metrics.
func Middleware() gin.HandlerFunc {
	meter := otel.Meter("fastfso")

	duration, _ := meter.Float64Histogram("http.server.request.duration",
		otelmetric.WithUnit("s"),
		otelmetric.WithDescription("Duration of HTTP server requests in seconds"),
	)

	activeRequests, _ := meter.Int64UpDownCounter("http.server.active_requests",
		otelmetric.WithDescription("Number of active HTTP server requests"),
	)

	return func(c *gin.Context) {
		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		method := c.Request.Method

		attrs := []attribute.KeyValue{
			attribute.String("method", method),
			attribute.String("route", route),
		}

		activeRequests.Add(c.Request.Context(), 1, otelmetric.WithAttributes(attrs...))
		start := time.Now()

		c.Next()

		elapsed := time.Since(start).Seconds()
		statusAttrs := append(attrs, attribute.Int("status_code", c.Writer.Status()))

		duration.Record(c.Request.Context(), elapsed, otelmetric.WithAttributes(statusAttrs...))
		activeRequests.Add(c.Request.Context(), -1, otelmetric.WithAttributes(attrs...))
	}
}
