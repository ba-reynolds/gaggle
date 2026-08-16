package logger

import (
	"io"
	"log/slog"
	"os"
)

func NewLogger(logFilePath string) (*slog.Logger, error) {
	// Open the log file
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	// Create a multi-writer for console and file
	mw := io.MultiWriter(os.Stdout, file)

	// Configure slog to output JSON to the file and text to console
	// For simplicity, using a single handler for both, but can be split if needed
	// For production, you might want separate handlers with different levels/formats
	handler := slog.NewJSONHandler(mw, &slog.HandlerOptions{
		AddSource: true, // Add file and line number
		Level:     slog.LevelInfo,
	})

	logger := slog.New(handler)

	return logger, nil
}
