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
	"dating-app/internal/model"
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

	authSvc := service.NewAuthService(
		repository.NewUserRepository(db),
		profileRepo,
		repository.NewSessionRepository(rdb),
		auth.BcryptHasher{},
		issuer,
		cfg.Auth.JWTRefreshTTL,
	)
	urlProvider := service.NewLocalPhotoURLProvider("http://localhost:" + cfg.HTTP.Port)
	authHandler := handler.NewAuthHandler(authSvc, cfg.Auth.CookieSecure)
	profileSvc := service.NewProfileService(
		profileRepo,
		&service.CompatibilityServiceImpl{},
		urlProvider,
	)

	feedHandler := handler.NewGetFeedHandler(profileSvc)
	profileShortHandler := handler.NewGetProfileShortHandler(profileSvc)
	testRepo := repository.NewTestRepository(db, model.NewTIPITest(cfg.CurrentTestID))
	compSvc := service.NewCompatibilityService(1)
	testSvc := service.NewTestService(testRepo, profileSvc, compSvc)
	getCurrentTestHandler := handler.NewGetCurrentTestHandler(testSvc)
	testAnswersHandler := handler.NewPostTestResultsHandler(testSvc)

	r := mux.NewRouter()
	r.HandleFunc("/health", handler.GetHealth).Methods(http.MethodGet)

	files := http.FileServer(http.Dir("./db/data/cats"))
	r.PathPrefix("/cats/").Handler(http.StripPrefix("/cats/", files)).Methods(http.MethodGet, http.MethodHead)

	api := r.PathPrefix("/api/v1").Subrouter()
	requireAuth := middleware.Auth(issuer)

	authRouter := api.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", authHandler.Register).Methods(http.MethodPost)
	authRouter.HandleFunc("/login", authHandler.Login).Methods(http.MethodPost)
	authRouter.HandleFunc("/logout", authHandler.Logout).Methods(http.MethodPost)

	api.Handle("/feed", requireAuth(feedHandler)).Methods(http.MethodGet)
	api.Handle("/profile/me/short", requireAuth(profileShortHandler)).Methods(http.MethodGet)
	api.Handle("/tests/current", requireAuth(getCurrentTestHandler)).Methods(http.MethodGet)
	api.Handle("/tests/{test_id}/results", requireAuth(testAnswersHandler)).Methods(http.MethodPost)

	// Обёрнуто снаружи роутера, а не через r.Use(): у gorilla/mux r.Use()
	// применяется только к совпавшим роутам, на 404/405 middleware не сработает.
	var h http.Handler = r
	h = middleware.Recovery(h)
	h = middleware.CORS([]string{"http://localhost:5173"})(h)
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
