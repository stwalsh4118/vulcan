package config

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	defaultListenAddr = ":8080"
	defaultDBPath     = "vulcan.db"

	envListenAddr = "VULCAN_LISTEN_ADDR"
	envDBPath     = "VULCAN_DB_PATH"
	envLogLevel   = "VULCAN_LOG_LEVEL"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	ListenAddr string
	DBPath     string
	LogLevel   slog.Level
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	cfg := Config{
		ListenAddr: defaultListenAddr,
		DBPath:     defaultDBPath,
		LogLevel:   slog.LevelInfo,
	}

	if v := os.Getenv(envListenAddr); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv(envDBPath); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv(envLogLevel); v != "" {
		cfg.LogLevel = parseLogLevel(v)
	}

	return cfg
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger creates a structured JSON logger writing to w at the configured level.
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	}))
}
