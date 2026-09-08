package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/env"
	"github.com/SSpencer740/fastfso/backend/internal/logging"
	"github.com/SSpencer740/fastfso/backend/internal/router"
	"github.com/SSpencer740/fastfso/backend/internal/telemetry"
)

func cmdServe() {
	root := logging.Setup()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dbLogger := logging.Component(root, "database")
	pool, err := database.Connect(ctx, dbLogger)
	if err != nil {
		root.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	telemetryLogger := logging.Component(root, "telemetry")
	shutdownTelemetry := telemetry.Init(ctx, pool, telemetryLogger)
	defer shutdownTelemetry()

	adapted := &database.PoolAdapter{Pool: pool}
	sqlLogger := logging.Component(root, "sql")
	db := telemetry.WithMetrics(database.WithLogging(adapted, sqlLogger))

	port := env.Port()

	routerLogger := logging.Component(root, "router")
	r, err := router.New(ctx, db, routerLogger)
	if err != nil {
		root.Error("failed to initialize router", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		root.Info("listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			root.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	root.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		root.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
	root.Info("shutdown complete")
}
