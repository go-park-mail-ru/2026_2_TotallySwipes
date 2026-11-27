package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"dating-app/gateway/internal/config"
	"dating-app/gateway/internal/handler"
)

func serveHTTP(ctx context.Context, server *http.Server, listener net.Listener, ws *handler.WSHandler, cfg *config.Config) error {

	defer server.Close()

	serveErr := make(chan error, 1)

	go func() {

		if cfg.TLS.CertFile != "" {
			serveErr <- server.ServeTLS(listener, cfg.TLS.CertFile, cfg.TLS.KeyFile)
			return
		}

		serveErr <- server.Serve(listener)
	}()

	var serverErr error
	var serveFinished bool

	select {
	case serverErr = <-serveErr:
		serveFinished = true

	case <-ctx.Done():
	}

	slog.Info("stopping gateway")

	// Отмена сигнального контекста не должна обрывать выполняющиеся HTTP-запросы.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	httpDone := make(chan error, 1)

	go func() {
		httpDone <- server.Shutdown(shutdownCtx)
	}()

	var wsErr error

	if ws != nil {
		wsErr = ws.Shutdown(shutdownCtx)
	}

	httpErr := <-httpDone

	if httpErr != nil {
		slog.Warn("gateway shutdown timeout exceeded; closing active HTTP connections", "error", httpErr)
		_ = server.Close()
	}

	if !serveFinished {
		serverErr = <-serveErr
	}

	if errors.Is(serverErr, http.ErrServerClosed) {
		serverErr = nil
	}

	var result error

	if serverErr != nil {
		result = errors.Join(result, fmt.Errorf("serve gateway http: %w", serverErr))
	}

	if httpErr != nil {
		result = errors.Join(result, fmt.Errorf("shutdown gateway http: %w", httpErr))
	}

	if wsErr != nil {
		result = errors.Join(result, fmt.Errorf("shutdown gateway websocket: %w", wsErr))
	}

	return result
}
