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
	"dating-app/internal/psychotest"
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

	issuer := auth.NewJWTIssuer(cfg.Auth.JWTSecret, cfg.Auth.JWTAccessTTL)
	profileRepo := repository.NewProfileRepository(db)

	urlProvider := service.NewLocalPhotoURLProvider(cfg.HTTP.PublicURL)
	compatibilitySvc := service.NewCompatibilityService()
	profileSvc := service.NewProfileService(
		profileRepo,
		compatibilitySvc,
		urlProvider,
		storage.NewLocalPhotoStorage(cfg.PhotoDir),
	)
	authSvc := service.NewAuthService(
		repository.NewUserRepository(db),
		profileSvc,
		repository.NewSessionRepository(rdb),
		auth.BcryptHasher{},
		issuer,
		cfg.Auth.JWTRefreshTTL,
	)
	authHandler := handler.NewAuthHandler(authSvc, cfg.Auth.CookieSecure)

	feedHandler := handler.NewFeedHandler(profileSvc)
	profileHandler := handler.NewProfileHandler(profileSvc)
	psychoTest, err := psychotest.Load()
	if err != nil {
		return err
	}
	psychoTestRepo := repository.NewPsychoTestRepository(db)
	psychoTestSvc := service.NewPsychoTestService(psychoTestRepo, profileSvc, compatibilitySvc, psychoTest)
	psychoTestHandler := handler.NewPsychoTestHandler(psychoTestSvc)

	r := mux.NewRouter()
	r.HandleFunc("/health", handler.GetHealth).Methods(http.MethodGet)

	files := http.FileServer(http.Dir(cfg.PhotoDir))
	r.PathPrefix("/cats/").Handler(http.StripPrefix("/cats/", files)).Methods(http.MethodGet, http.MethodHead)

	api := r.PathPrefix("/api/v1").Subrouter()
	requireAuth := middleware.Auth(issuer)
	authed := func(fn handler.UserHandlerFunc) http.Handler { return requireAuth(handler.WithUser(fn)) }

	authRouter := api.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", authHandler.Register).Methods(http.MethodPost)
	authRouter.HandleFunc("/login", authHandler.Login).Methods(http.MethodPost)
	authRouter.HandleFunc("/logout", authHandler.Logout).Methods(http.MethodPost)
	authRouter.HandleFunc("/refresh", authHandler.Refresh).Methods(http.MethodPost)

	api.Handle("/feed", authed(feedHandler.Get)).Methods(http.MethodGet)
	api.Handle("/profile/me", authed(profileHandler.Get)).Methods(http.MethodGet)
	api.Handle("/profile/me", authed(profileHandler.Update)).Methods(http.MethodPatch)
	api.Handle("/profile/me/short", authed(profileHandler.ShortProfile)).Methods(http.MethodGet)
	api.Handle("/profile/me/photos", authed(profileHandler.AddPhoto)).Methods(http.MethodPost)
	api.Handle("/profile/me/photos/{photo_id}", authed(profileHandler.DeletePhoto)).Methods(http.MethodDelete)
	api.Handle("/tests/current", authed(psychoTestHandler.Current)).Methods(http.MethodGet)
	api.Handle("/tests/results/me", authed(psychoTestHandler.MyResult)).Methods(http.MethodGet)
	api.Handle("/tests/{test_id}/results", authed(psychoTestHandler.Submit)).Methods(http.MethodPost)

	var h http.Handler = r
	h = middleware.Recovery(h)
	h = middleware.CORS(cfg.HTTP.CORSOrigins)(h)
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
