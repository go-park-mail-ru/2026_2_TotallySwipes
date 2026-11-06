package storage

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

type QueryTracer struct {
	logger *slog.Logger
}

type startTimeKey struct{}

func (t *QueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	start := time.Now()

	tracerCtx := context.WithValue(ctx, startTimeKey{}, start)
	return tracerCtx
}

func NewTracer(logger *slog.Logger) *QueryTracer {
	return &QueryTracer{logger: logger}
}

func (t *QueryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {

	level := slog.LevelInfo
	outcome := "ok"

	attrs := []any{
		"command_tag", data.CommandTag.String(),
	}

	if start, ok := ctx.Value(startTimeKey{}).(time.Time); ok {
		attrs = append(attrs, "duration_ms", float64(time.Since(start))/float64(time.Millisecond))
	}

	if data.Err != nil {
		level = slog.LevelError
		outcome = "error"
		attrs = append(attrs, "error", data.Err)
	}
	attrs = append(attrs, "outcome", outcome)

	t.logger.Log(ctx, level, "database query", attrs...)
}
