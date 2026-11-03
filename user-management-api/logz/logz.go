// Package logz sets up the application logger: JSON lines on stdout via
// the standard library's log/slog.
package logz

import (
	"log/slog"
	"os"
)

// Init creates the JSON logger at the given level ("debug", "info", "warn",
// "error"; unknown values mean "info"), tags every line with the service
// name, and installs it as the slog default.
func Init(level, name string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})).
		With(slog.String("service", name))
	slog.SetDefault(logger)
	return logger
}
