package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// New builds a structured slog.Logger that writes to both stdout and a
// rotating-by-date log file under logsDir, per spec section 55.
func New(logsDir string) (*slog.Logger, error) {
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, err
	}

	fileName := filepath.Join(logsDir, time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	multi := io.MultiWriter(os.Stdout, f)

	handler := slog.NewJSONHandler(multi, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, nil
}
