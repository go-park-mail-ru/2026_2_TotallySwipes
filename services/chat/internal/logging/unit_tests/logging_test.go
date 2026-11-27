package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dating-app/services/chat/internal/logging"
	"dating-app/services/chat/internal/requestctx"
)

func TestLoggerFileAndLevel(t *testing.T) {

	dir := filepath.Join(t.TempDir(), "logs")
	logger, closer, err := logging.New(dir, slog.LevelWarn)

	if err != nil {
		t.Fatal(err)
	}

	logger.Info("filtered")
	logger.Warn("saved", "service", "chat")

	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "chat.log"))

	if err != nil {
		t.Fatal(err)
	}

	var record map[string]any

	if err := json.Unmarshal(bytes.TrimSpace(data), &record); err != nil {
		t.Fatal(err)
	}

	if record["msg"] != "saved" || record["level"] != "WARN" || record["service"] != "chat" {
		t.Fatalf("record = %#v", record)
	}
}

func TestContextHandler(t *testing.T) {

	for _, withID := range []bool{false, true} {

		var output bytes.Buffer
		handler := logging.NewLogHandler(slog.NewJSONHandler(&output, nil))
		ctx := context.Background()

		if withID {
			ctx = context.WithValue(ctx, requestctx.RequestIDKey{}, "request-123")
		}

		record := slog.NewRecord(time.Now(), slog.LevelInfo, "request", 0)

		if err := handler.Handle(ctx, record); err != nil {
			t.Fatal(err)
		}
		if record.NumAttrs() != 0 {
			t.Fatal("handler mutated the original record")
		}

		var result map[string]any

		if err := json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if withID && result["request_id"] != "request-123" {
			t.Fatalf("record = %#v", result)
		}
		if !withID && result["request_id"] != nil {
			t.Fatalf("record = %#v", result)
		}

		output.Reset()
		logger := slog.New(handler).With("service", "chat").WithGroup("rpc")
		logger.InfoContext(ctx, "call", "method", "GetSummary")

		if err := json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["service"] != "chat" {
			t.Fatalf("record = %#v", result)
		}
		group, ok := result["rpc"].(map[string]any)
		if !ok || group["method"] != "GetSummary" {
			t.Fatalf("record = %#v", result)
		}
		if withID && group["request_id"] != "request-123" {
			t.Fatalf("record = %#v", result)
		}
	}
}

func TestInvalidLogDirectory(t *testing.T) {

	path := filepath.Join(t.TempDir(), "file")

	if err := os.WriteFile(path, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}

	if logger, closer, err := logging.New(path, slog.LevelInfo); err == nil || logger != nil || closer != nil {
		t.Fatalf("logger = %v, closer = %v, error = %v", logger, closer, err)
	}
}
