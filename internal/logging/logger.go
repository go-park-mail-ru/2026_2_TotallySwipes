package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func New(dir string) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, nil, fmt.Errorf("create logs directory: %w", err)
	}

	file, err := os.OpenFile(
		filepath.Join(dir, "backend.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0640,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}

	output := io.MultiWriter(os.Stdout, file)
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	return slog.New(NewLogHandler(handler)), file, nil
}
