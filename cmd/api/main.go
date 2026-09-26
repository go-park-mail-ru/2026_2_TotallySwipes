package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"

	"dating-app/internal/auth"
	"dating-app/internal/config"
	"dating-app/internal/handler"
	"dating-app/internal/middleware"
	"dating-app/internal/repository"
	"dating-app/internal/service"
	"dating-app/internal/storage"
)

const connectTimeout = 5 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	db, err := storage.OpenPostgres(ctx, cfg.Database.URL)
	if err != nil {
		return err
	}
	defer db.Close()

	rdb, err := storage.OpenRedis(ctx, cfg.Redis.Addr)
	if err != nil {
		return err
	}
	defer rdb.Close()

	authSvc := service.NewAuthService(
		repository.NewUserRepository(db),
		repository.NewProfileRepository(db),
		repository.NewSessionRepository(rdb),
		auth.BcryptHasher{},
		auth.NewJWTIssuer(cfg.Auth.JWTSecret, cfg.Auth.JWTAccessTTL),
		cfg.Auth.JWTRefreshTTL,
	)
	authHandler := handler.NewAuthHandler(authSvc, cfg.Auth.CookieSecure)

	r := mux.NewRouter()
	r.HandleFunc("/health", handler.Health).Methods(http.MethodGet)

	api := r.PathPrefix("/api/v1").Subrouter()
	authRouter := api.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", authHandler.Register).Methods(http.MethodPost)
	authRouter.HandleFunc("/login", authHandler.Login).Methods(http.MethodPost)
	authRouter.HandleFunc("/logout", authHandler.Logout).Methods(http.MethodPost)

	// Обёрнуто снаружи роутера, а не через r.Use(): у gorilla/mux r.Use()
	// применяется только к совпавшим роутам, на 404/405 middleware не сработает.
	var h http.Handler = r
	h = middleware.Recovery(h)
	h = middleware.Logging(h)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           h,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	slog.Info("starting server", "port", cfg.HTTP.Port)
	return srv.ListenAndServe()
}
