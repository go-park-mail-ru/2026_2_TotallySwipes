package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/gorilla/mux"

	"dating-app/internal/config"
	"dating-app/internal/handler"
	"dating-app/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	r := mux.NewRouter()
	r.HandleFunc("/health", handler.Health).Methods(http.MethodGet)
	r.PathPrefix("/api/v1").Subrouter()

	// Обёрнуто снаружи роутера, а не через r.Use(): у gorilla/mux r.Use()
	// применяется только к совпавшим роутам, на 404/405 middleware не сработает.
	var h http.Handler = r
	h = middleware.Recovery(h)
	h = middleware.Logging(h)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	slog.Info("starting server", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
