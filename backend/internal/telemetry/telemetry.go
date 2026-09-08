package telemetry

import (
	"context"
	"log/slog"
	"os"
	"time"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/SSpencer740/fastfso/backend/internal/env"
)

// Init initialises OpenTelemetry metrics. When running in the cloud it exports
// to GCP Cloud Monitoring; locally it creates a no-op provider that discards
// data. It returns a shutdown function that must be called on program exit.
func Init(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) (shutdown func()) {
	var provider *metric.MeterProvider

	if env.IsCloud() {
		exporter, err := mexporter.New()
		if err != nil {
			logger.Error("failed to create Cloud Monitoring exporter", "error", err)
			provider = metric.NewMeterProvider()
		} else {
			res, err := resource.New(ctx,
				resource.WithDetectors(gcp.NewDetector()),
				resource.WithTelemetrySDK(),
				resource.WithAttributes(
					semconv.ServiceName("fastfso"),
					semconv.ServiceNamespace(env.AppEnv()),
					attribute.String("service.instance.id", os.Getenv("K_REVISION")),
				),
			)
			if err != nil {
				logger.Error("failed to create OTel resource", "error", err)
				res = resource.Default()
			}

			provider = metric.NewMeterProvider(
				metric.WithReader(metric.NewPeriodicReader(exporter, metric.WithInterval(60*time.Second))),
				metric.WithResource(res),
			)
		}
	} else {
		// No reader attached — all metrics are silently discarded.
		provider = metric.NewMeterProvider()
	}

	otel.SetMeterProvider(provider)

	if err := initMetrics(pool); err != nil {
		logger.Error("failed to create metric instruments", "error", err)
	}

	logger.Info("telemetry initialised", "cloud", env.IsCloud())

	return func() {
		if err := provider.Shutdown(ctx); err != nil {
			logger.Error("telemetry shutdown error", "error", err)
		}
	}
}
