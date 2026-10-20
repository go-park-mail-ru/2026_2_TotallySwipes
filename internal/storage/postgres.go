package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenPostgres открывает пул соединений через pgx и проверяет, что база доступна
func OpenPostgres(ctx context.Context, url string, tracer pgx.QueryTracer) (*sql.DB, error) {

	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	cfg.Tracer = tracer

	db := stdlib.OpenDB(*cfg)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}
