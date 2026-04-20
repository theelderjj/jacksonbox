// Package obs centralizes structured logging. See docs/DESIGN.md §14.
// Deliberately slog-only — Prometheus is v1.1 if the project needs it.
package obs

import (
	"log/slog"
	"os"
)

// New returns a JSON slog.Logger with the standard attributes set.
// level is one of "debug", "info", "warn", "error"; default is "info".
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h).With("svc", "trivia")
}
