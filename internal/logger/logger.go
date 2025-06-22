package logger

import (
	"log"

	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func levelFromString(s string) (l slog.Level, ok bool) {
	switch strings.ToLower(s) {
	case "debug", "dbg":
		return slog.LevelDebug, true
	case "info", "inf":
		return slog.LevelInfo, true
	case "warn", "wrn":
		return slog.LevelWarn, true
	case "error", "err":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

func InitLogger(path, level string) {
	loglevel, _ := levelFromString(level)

	logDir := filepath.Dir(path)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Fatal("Failed to create log directory:", err)
	}

	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}

	// https://cs.opensource.google/go/go/+/refs/tags/go1.24.1:src/log/slog/handler.go;l=265-315;drc=3d61de41a28b310fedc345d76320829bd08146b3
	// slog defaults to logging in the order of time, level, msg, and other attributes.
	handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: loglevel})

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
