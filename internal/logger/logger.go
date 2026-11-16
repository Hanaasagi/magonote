package logger

import (
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

// InitLogger configures the default slog logger to write to path. If the log
// directory or file cannot be created/opened, it degrades to stderr with a
// warning rather than terminating the process: an unwritable log location is
// not a fatal condition for a clipboard/selection tool.
func InitLogger(path, level string) {
	loglevel, _ := levelFromString(level)
	opts := &slog.HandlerOptions{Level: loglevel}

	logDir := filepath.Dir(path)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
		slog.Warn("Failed to create log directory; logging to stderr", "path", logDir, "error", err)
		return
	}

	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
		slog.Warn("Failed to open log file; logging to stderr", "path", path, "error", err)
		return
	}

	// https://cs.opensource.google/go/go/+/refs/tags/go1.24.1:src/log/slog/handler.go;l=265-315;drc=3d61de41a28b310fedc345d76320829bd08146b3
	// slog defaults to logging in the order: time, level, msg, then attributes.
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, opts)))
}
