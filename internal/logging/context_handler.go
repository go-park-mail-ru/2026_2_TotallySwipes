package logging

import (
	"context"
	"dating-app/internal/requestctx"
	"log/slog"
)

type LogHandler struct {
	next slog.Handler
}

func NewLogHandler(next slog.Handler) *LogHandler {
	return &LogHandler{next: next}
}

func (h *LogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *LogHandler) Handle(ctx context.Context, record slog.Record) error {
	if id := requestctx.RequestIDFromContext(ctx); id != "" {
		record = record.Clone()
		record.AddAttrs(slog.String("request_id", id))
	}

	return h.next.Handle(ctx, record)
}

func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogHandler{next: h.next.WithAttrs(attrs)}
}

func (h *LogHandler) WithGroup(name string) slog.Handler {
	return &LogHandler{next: h.next.WithGroup(name)}
}

type Writer interface {
	Write(p []byte) (n int, err error)
}
