package main

import (
	"fmt"
	"os"

	"github.com/fastfso/fastfso/backend/internal/database"
	"github.com/fastfso/fastfso/backend/internal/logging"
	"github.com/fastfso/fastfso/backend/internal/migrate"
)

func cmdMigrate() {
	root := logging.Setup()
	logger := logging.Component(root, "migrate")

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: fastfso migrate <up|down>\n")
		os.Exit(1)
	}

	dsn := database.DSN()

	switch os.Args[2] {
	case "up":
		logger.Info("running migrations up")
		if err := migrate.Up(dsn); err != nil {
			logger.Error("migration failed", "error", err)
			os.Exit(1)
		}
		logger.Info("migrations complete")
	case "down":
		logger.Info("running migrations down")
		if err := migrate.Down(dsn); err != nil {
			logger.Error("rollback failed", "error", err)
			os.Exit(1)
		}
		logger.Info("rollback complete")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s (use up or down)\n", os.Args[2])
		os.Exit(1)
	}
}
