package main

import (
	"log"
	"os"

	"github.com/seantiz/vulcan/internal/api"
	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/config"
	"github.com/seantiz/vulcan/internal/engine"
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
	eng := engine.NewEngine(db, reg, logger)
	srv := api.NewServer(cfg.ListenAddr, db, reg, eng, logger)

	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
