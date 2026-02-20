package main

import (
	"log"
	"os"

	"github.com/seantiz/vulcan/internal/api"
	"github.com/seantiz/vulcan/internal/backend"
	fc "github.com/seantiz/vulcan/internal/backend/firecracker"
	"github.com/seantiz/vulcan/internal/config"
	"github.com/seantiz/vulcan/internal/engine"
	"github.com/seantiz/vulcan/internal/model"
	"github.com/seantiz/vulcan/internal/store"
)

func main() {
	cfg := config.Load()
	logger := config.NewLogger(os.Stdout, cfg.LogLevel)

	logger.Info("vulcan: starting",
		"listen_addr", cfg.ListenAddr,
		"db_path", cfg.DBPath,
	)

	db, err := store.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	reg := backend.NewRegistry()

	// Register Firecracker backend if configured.
	fcCfg := fc.LoadConfig()
	if fcCfg.KernelPath != "" && fcCfg.FirecrackerBin != "" {
		fcBackend, err := fc.NewBackend(fcCfg, logger)
		if err != nil {
			logger.Warn("firecracker backend unavailable", "error", err)
		} else if verifyErr := fcBackend.Verify(); verifyErr != nil {
			logger.Warn("firecracker backend: plugin verification failed, skipping registration", "error", verifyErr)
		} else {
			reg.Register(model.IsolationMicroVM, fcBackend)
			logger.Info("firecracker backend registered")
		}
	}

	eng := engine.NewEngine(db, reg, logger)
	srv := api.NewServer(cfg.ListenAddr, db, reg, eng, logger)

	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
