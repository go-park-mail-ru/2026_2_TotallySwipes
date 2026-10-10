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

type handlers struct {
	auth       *handler.AuthHandler
	feed       *handler.FeedHandler
	filter     *handler.FilterHandler
	profile    *handler.ProfileHandler
	psychoTest *handler.PsychoTestHandler
}

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

	psychoTest, err := psychotest.Load()
	if err != nil {
		return err
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

	userRepo := repository.NewUserRepository(db)
	profileRepo := repository.NewProfileRepository(db)
	filterRepo := repository.NewFilterRepository(db)
	psychoTestRepo := repository.NewPsychoTestRepository(db)
	sessionRepo := repository.NewSessionRepository(rdb)

	issuer := auth.NewJWTIssuer(cfg.Auth.JWTSecret, cfg.Auth.JWTAccessTTL)
	photoStorage := storage.NewLocalPhotoStorage(cfg.PhotoDir)
	urlProvider := service.NewLocalPhotoURLProvider(cfg.HTTP.PublicURL)

	compatibilitySvc := service.NewCompatibilityService()
	profileSvc := service.NewProfileService(profileRepo, compatibilitySvc, urlProvider, photoStorage)
	authSvc := service.NewAuthService(userRepo, profileSvc, sessionRepo, auth.BcryptHasher{}, issuer, cfg.Auth.JWTRefreshTTL)
	psychoTestSvc := service.NewPsychoTestService(psychoTestRepo, profileRepo, compatibilitySvc, psychoTest)
	filterSvc := service.NewFilterService(filterRepo)

	h := handlers{
		auth:       handler.NewAuthHandler(authSvc, cfg.Auth.CookieSecure),
		feed:       handler.NewFeedHandler(profileSvc),
		filter:     handler.NewFilterHandler(filterSvc),
		profile:    handler.NewProfileHandler(profileSvc),
		psychoTest: handler.NewPsychoTestHandler(psychoTestSvc),
	}

	var root http.Handler = newRouter(cfg, issuer, h)
	root = middleware.Recovery(root)
	root = middleware.CORS(cfg.HTTP.CORSOrigins)(root)
	root = middleware.Logging(root)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           root,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	slog.Info("starting server", "port", cfg.HTTP.Port)
	return srv.ListenAndServe()
}

func newRouter(cfg *config.Config, issuer *auth.JWTIssuer, h handlers) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/health", handler.GetHealth).Methods(http.MethodGet)

	files := http.FileServer(http.Dir(cfg.PhotoDir))
	r.PathPrefix("/cats/").Handler(http.StripPrefix("/cats/", files)).Methods(http.MethodGet, http.MethodHead)

	api := r.PathPrefix("/api/v1").Subrouter()
	requireAuth := middleware.Auth(issuer)
	authed := func(fn handler.UserHandlerFunc) http.Handler { return requireAuth(handler.WithUser(fn)) }

	authRouter := api.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", h.auth.Register).Methods(http.MethodPost)
	authRouter.HandleFunc("/login", h.auth.Login).Methods(http.MethodPost)
	authRouter.HandleFunc("/logout", h.auth.Logout).Methods(http.MethodPost)
	authRouter.HandleFunc("/refresh", h.auth.Refresh).Methods(http.MethodPost)

	api.Handle("/feed", authed(h.feed.Get)).Methods(http.MethodGet)

	api.Handle("/filters/me", authed(h.filter.Get)).Methods(http.MethodGet)
	api.Handle("/filters/me", authed(h.filter.Set)).Methods(http.MethodPut)

	api.Handle("/profile/me", authed(h.profile.Get)).Methods(http.MethodGet)
	api.Handle("/profile/me", authed(h.profile.Update)).Methods(http.MethodPatch)
	api.Handle("/profile/me/short", authed(h.profile.ShortProfile)).Methods(http.MethodGet)
	api.Handle("/profile/me/photos", authed(h.profile.AddPhoto)).Methods(http.MethodPost)
	api.Handle("/profile/me/photos/order", authed(h.profile.ReorderPhotos)).Methods(http.MethodPut)
	api.Handle("/profile/me/photos/{photo_id}", authed(h.profile.DeletePhoto)).Methods(http.MethodDelete)

	api.Handle("/tests/current", authed(h.psychoTest.Current)).Methods(http.MethodGet)
	api.Handle("/tests/results/me", authed(h.psychoTest.MyResult)).Methods(http.MethodGet)
	api.Handle("/tests/{test_id}/results", authed(h.psychoTest.Submit)).Methods(http.MethodPost)

	return r
}
