package config

import (
	"log/slog"
	"os"
	"strings"
)

// InitLogging sends logs to stdout, which Vercel collects.
// The level comes from LOG_LEVEL (debug, info, warn, error; default info).
func InitLogging() {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
