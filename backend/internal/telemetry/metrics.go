package telemetry

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// Package-level instruments populated by initMetrics.
var (
	loginTotal         otelmetric.Int64Counter
	sessionCreated     otelmetric.Int64Counter
	rateLimitBlocked   otelmetric.Int64Counter
	frontendErrorTotal otelmetric.Int64Counter
	aiTokensTotal      otelmetric.Int64Counter
	queryDurationHist  otelmetric.Float64Histogram
)

// initMetrics creates all application metric instruments and registers
// observable callbacks for the database connection pool.
func initMetrics(pool *pgxpool.Pool) error {
	var errs []error
	meter := otel.Meter("fastfso")

	var err error

	loginTotal, err = meter.Int64Counter("auth.login.total",
		otelmetric.WithDescription("Total login attempts"),
	)
	errs = append(errs, err)

	sessionCreated, err = meter.Int64Counter("auth.session.created",
		otelmetric.WithDescription("Total sessions created"),
	)
	errs = append(errs, err)

	rateLimitBlocked, err = meter.Int64Counter("auth.ratelimit.blocked",
		otelmetric.WithDescription("Requests blocked by rate limiting"),
	)
	errs = append(errs, err)

	frontendErrorTotal, err = meter.Int64Counter("frontend.error.total",
		otelmetric.WithDescription("Total frontend errors reported"),
	)
	errs = append(errs, err)

	aiTokensTotal, err = meter.Int64Counter("ai.tokens.total",
		otelmetric.WithDescription("Total AI tokens consumed, labeled by feature and direction"),
	)
	errs = append(errs, err)

	queryDurationHist, err = meter.Float64Histogram("db.query.duration",
		otelmetric.WithUnit("s"),
		otelmetric.WithDescription("Duration of database queries in seconds"),
	)
	errs = append(errs, err)

	poolGauge, err := meter.Int64ObservableGauge("db.pool.connections",
		otelmetric.WithDescription("Database connection pool state"),
	)
	errs = append(errs, err)

	_, err = meter.RegisterCallback(func(_ context.Context, o otelmetric.Observer) error {
		stat := pool.Stat()
		o.ObserveInt64(poolGauge, int64(stat.IdleConns()),
			otelmetric.WithAttributes(attribute.String("state", "idle")))
		o.ObserveInt64(poolGauge, int64(stat.AcquiredConns()),
			otelmetric.WithAttributes(attribute.String("state", "in_use")))
		o.ObserveInt64(poolGauge, int64(stat.TotalConns()),
			otelmetric.WithAttributes(attribute.String("state", "total")))
		return nil
	}, poolGauge)
	errs = append(errs, err)

	return errors.Join(errs...)
}

// RecordLogin increments the auth.login.total counter.
func RecordLogin(ctx context.Context, method string, success bool) {
	result := "failure"
	if success {
		result = "success"
	}
	loginTotal.Add(ctx, 1,
		otelmetric.WithAttributes(
			attribute.String("method", method),
			attribute.String("result", result),
		),
	)
}

// RecordSessionCreated increments the auth.session.created counter.
func RecordSessionCreated(ctx context.Context) {
	sessionCreated.Add(ctx, 1)
}

// RecordRateLimitBlocked increments the auth.ratelimit.blocked counter.
func RecordRateLimitBlocked(ctx context.Context) {
	rateLimitBlocked.Add(ctx, 1)
}

// RecordFrontendError increments the frontend.error.total counter.
func RecordFrontendError(ctx context.Context, source string) {
	if frontendErrorTotal == nil {
		return
	}
	frontendErrorTotal.Add(ctx, 1,
		otelmetric.WithAttributes(attribute.String("source", source)),
	)
}

// RecordAITokens increments the ai.tokens.total counter for both prompt and
// response directions. `feature` identifies the calling subsystem (e.g.
// "chat") so usage can be broken down in Cloud Monitoring.
func RecordAITokens(ctx context.Context, feature string, promptTokens, responseTokens int32) {
	if aiTokensTotal == nil {
		return
	}
	if promptTokens > 0 {
		aiTokensTotal.Add(ctx, int64(promptTokens),
			otelmetric.WithAttributes(
				attribute.String("feature", feature),
				attribute.String("direction", "prompt"),
			),
		)
	}
	if responseTokens > 0 {
		aiTokensTotal.Add(ctx, int64(responseTokens),
			otelmetric.WithAttributes(
				attribute.String("feature", feature),
				attribute.String("direction", "response"),
			),
		)
	}
}

// recordQueryDuration records a database query duration to the histogram.
func recordQueryDuration(ctx context.Context, name string, d time.Duration, _ error) {
	queryDurationHist.Record(ctx, d.Seconds(),
		otelmetric.WithAttributes(attribute.String("name", name)),
	)
}
