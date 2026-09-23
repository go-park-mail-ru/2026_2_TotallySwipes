package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"

	"dating-app/internal/handler"
	"dating-app/internal/middleware"
)

const port = "8080"

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/health", handler.Health).Methods(http.MethodGet)
	r.PathPrefix("/api/v1").Subrouter()

	// Обёрнуто снаружи роутера, а не через r.Use(): у gorilla/mux r.Use()
	// применяется только к совпавшим роутам, на 404/405 middleware не сработает.
	var h http.Handler = r
	h = middleware.Recovery(h)
	h = middleware.Logging(h)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("starting server", "port", port)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
