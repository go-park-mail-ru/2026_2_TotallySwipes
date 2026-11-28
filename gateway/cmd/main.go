package main

import (
	"context"
	"dating-app/gateway/internal/auth"
	"dating-app/gateway/internal/client"
	"dating-app/gateway/internal/config"
	"dating-app/gateway/internal/grpcclient"
	"dating-app/gateway/internal/handler"
	"dating-app/gateway/internal/logging"
	"dating-app/gateway/internal/middleware"
	authpb "dating-app/gen/go/auth/v1"
	profilepb "dating-app/gen/go/profile/v1"
	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type handlers struct {
	auth       *handler.AuthHandler
	feed       *handler.FeedHandler
	filter     *handler.FilterHandler
	match      *handler.MatchHandler
	profile    *handler.ProfileHandler
	psychoTest *handler.PsychoTestHandler
	swipe      *handler.SwipeHandler
	location   *handler.LocationHandler
	event      *handler.EventHandler
	photo      *handler.PhotoHandler
	chat       *handler.ChatHTTPHandler
	ws         *handler.WSHandler
}

const maxChatGRPCMessageSize = 12 << 20

func main() {
	if err := start(); err != nil {
		os.Exit(1)
	}
}

func start() error {

	cfg, err := config.Load()

	if err != nil {
		slog.Error("load config", "error", err)
		return err
	}

	logger, logFile, err := logging.New(cfg.LogDir)
	if err != nil {
		slog.Error("init logger", "error", err)
		return err
	}
	defer logFile.Close()

	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		logger.Error("server stopped", "error", err)
		return err
	}

	return nil
}

func run(ctx context.Context, cfg *config.Config) error {

	authConn, err := grpc.NewClient(
		cfg.GRPC.AuthAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(grpcclient.MaxMsgSize), grpc.MaxCallRecvMsgSize(grpcclient.MaxMsgSize)),
		grpc.WithChainUnaryInterceptor(
			grpcclient.UnaryInterceptor(0),
			client.UnaryTimeoutInterceptor(cfg.GRPC.RequestTimeout),
		),
	)

	if err != nil {
		return fmt.Errorf("create auth client: %w", err)
	}
	defer authConn.Close()

	authClient := authpb.NewAuthServiceClient(authConn)
	authAdapter := client.NewAuthAdapter(authClient)
	authHandler := handler.NewAuthHandler(authAdapter, cfg.Auth.CookieSecure)

	profileConn, err := grpc.NewClient(
		cfg.GRPC.ProfileAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(grpcclient.MaxMsgSize), grpc.MaxCallRecvMsgSize(grpcclient.MaxMsgSize)),
		grpc.WithChainUnaryInterceptor(
			grpcclient.UnaryInterceptor(0),
			client.UnaryTimeoutInterceptor(cfg.GRPC.RequestTimeout),
		),
	)

	if err != nil {
		return fmt.Errorf("create profile client: %w", err)
	}
	defer profileConn.Close()

	profileClient := profilepb.NewProfileServiceClient(profileConn)
	profileAdapter := client.NewProfileAdapter(profileClient)
	photoAdapter := client.NewPhotoAdapter(profileClient)
	psychoAdapter := client.NewPsychoTestAdapter(profileClient)
	filterAdapter := client.NewFilterAdapter(profileClient)
	locationAdapter := client.NewLocationAdapter(profileClient)

	profileHandler := handler.NewProfileHandler(profileAdapter)
	photoHandler := handler.NewPhotoHandler(photoAdapter)
	psychoHandler := handler.NewPsychoTestHandler(psychoAdapter)
	filterHandler := handler.NewFilterHandler(filterAdapter)
	locationHandler := handler.NewLocationHandler(locationAdapter)

	matchConn, err := grpc.NewClient(
		cfg.GRPC.MatchingAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(grpcclient.MaxMsgSize), grpc.MaxCallRecvMsgSize(grpcclient.MaxMsgSize)),
		grpc.WithChainUnaryInterceptor(
			grpcclient.UnaryInterceptor(0),
			client.UnaryTimeoutInterceptor(cfg.GRPC.RequestTimeout),
		),
	)

	if err != nil {
		return fmt.Errorf("create matching client: %w", err)
	}
	defer matchConn.Close()

	matchClient := matchpb.NewMatchServiceClient(matchConn)
	matchAdapter := client.NewMatchAdapter(matchClient)
	eventAdapter := client.NewEventAdapter(matchClient)
	swipeAdapter := client.NewSwipeAdapter(matchClient)
	feedAdapter := client.NewFeedAdapter(matchClient)

	matchHandler := handler.NewMatchHandler(matchAdapter)
	eventHandler := handler.NewEventHandler(eventAdapter)
	swipeHandler := handler.NewSwipeHandler(swipeAdapter)
	feedHandler := handler.NewFeedHandler(feedAdapter)

	chatConn, err := grpc.NewClient(
		cfg.GRPC.ChatAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(grpcclient.MaxMsgSize), grpc.MaxCallRecvMsgSize(grpcclient.MaxMsgSize)),
		grpc.WithChainUnaryInterceptor(
			grpcclient.UnaryInterceptor(0),
			client.UnaryTimeoutInterceptor(cfg.GRPC.RequestTimeout),
		),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(maxChatGRPCMessageSize)),
	)

	if err != nil {
		return fmt.Errorf("create chat client: %w", err)
	}
	defer chatConn.Close()

	chatClient := chatpb.NewChatServiceClient(chatConn)
	chatAdapter := client.NewChatAdapter(chatClient)
	chatHandler := handler.NewChatHandler(chatAdapter)
	wsHandler := handler.NewWSHandler(cfg.WebSocket, chatAdapter)

	h := handlers{
		auth:       authHandler,
		feed:       feedHandler,
		filter:     filterHandler,
		match:      matchHandler,
		profile:    profileHandler,
		psychoTest: psychoHandler,
		swipe:      swipeHandler,
		location:   locationHandler,
		event:      eventHandler,
		photo:      photoHandler,
		chat:       chatHandler,
		ws:         wsHandler,
	}

	issuer := auth.NewJWTIssuer(cfg.Auth.JWTSecret, cfg.Auth.JWTAccessTTL)

	var root http.Handler = newRouter(cfg, issuer, h)
	root = middleware.CSRF(root)
	root = middleware.CORS(cfg.HTTP.CORSOrigins)(root)
	root = middleware.Logging(root)
	root = middleware.RequestID(root)
	root = middleware.Recovery(root)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           root,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	listener, err := net.Listen("tcp", srv.Addr)

	if err != nil {
		return fmt.Errorf("listen gateway http: %w", err)
	}

	defer listener.Close()
	slog.Info("starting server", "port", cfg.HTTP.Port)

	return serveHTTP(ctx, srv, listener, wsHandler, cfg)
}

func newRouter(cfg *config.Config, issuer *auth.JWTIssuer, h handlers) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/health", handler.GetHealth).Methods(http.MethodGet)

	// files := http.FileServer(http.Dir(cfg.PhotoDir))
	// r.PathPrefix("/cats/").Handler(http.StripPrefix("/cats/", files)).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(handler.PhotoRoutePrefix+"{key:.+}", h.photo.Get).Methods(http.MethodGet, http.MethodHead)

	api := r.PathPrefix("/api/v1").Subrouter()
	requireAuth := middleware.Auth(issuer)
	r.Handle("/ws", requireAuth(h.ws)).Methods(http.MethodGet)

	authed := func(fn handler.UserHandlerFunc) http.Handler { return requireAuth(handler.WithUser(fn)) }

	authRouter := api.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", h.auth.Register).Methods(http.MethodPost)
	authRouter.HandleFunc("/login", h.auth.Login).Methods(http.MethodPost)
	authRouter.HandleFunc("/logout", h.auth.Logout).Methods(http.MethodPost)
	authRouter.HandleFunc("/refresh", h.auth.Refresh).Methods(http.MethodPost)
	authRouter.HandleFunc("/csrf", h.auth.CSRF).Methods(http.MethodGet)

	api.Handle("/feed", authed(h.feed.Get)).Methods(http.MethodGet)
	api.Handle("/feed/swipe", authed(h.swipe.Swipe)).Methods(http.MethodPost)

	api.Handle("/matches", authed(h.match.List)).Methods(http.MethodGet)
	api.Handle("/matches/summary", authed(h.match.Summary)).Methods(http.MethodGet)
	api.Handle("/matches/{match_id}/seen", authed(h.match.MarkSeen)).Methods(http.MethodPost)

	api.Handle("/chats", authed(h.chat.List)).Methods(http.MethodGet)
	api.Handle("/chats/summary", authed(h.chat.Summary)).Methods(http.MethodGet)
	api.Handle("/chats/{chat_id}", authed(h.chat.Get)).Methods(http.MethodGet)
	api.Handle("/chats/{chat_id}/seen", authed(h.chat.MarkSeen)).Methods(http.MethodPost)
	api.Handle("/chats/{chat_id}/messages", authed(h.chat.GetMessages)).Methods(http.MethodGet)
	api.Handle("/chats/{chat_id}/messages/{message_id}/media", authed(h.chat.GetMedia)).Methods(http.MethodGet)
	api.Handle("/chats/{chat_id}/messages/media", authed(h.chat.SendMedia)).Methods(http.MethodPost)

	api.Handle("/filters/me", authed(h.filter.Get)).Methods(http.MethodGet)
	api.Handle("/filters/me", authed(h.filter.Set)).Methods(http.MethodPut)

	api.Handle("/profile/me", authed(h.profile.Get)).Methods(http.MethodGet)
	api.Handle("/profile/me", authed(h.profile.Update)).Methods(http.MethodPatch)
	api.Handle("/profile/me/short", authed(h.profile.ShortProfile)).Methods(http.MethodGet)
	api.Handle("/profile/me/photos", authed(h.profile.AddPhoto)).Methods(http.MethodPost)
	api.Handle("/profile/me/photos/order", authed(h.profile.ReorderPhotos)).Methods(http.MethodPut)
	api.Handle("/profile/me/photos/{photo_id}", authed(h.profile.DeletePhoto)).Methods(http.MethodDelete)
	api.Handle("/profiles/{user_id}", authed(h.match.GetMatchedProfile)).Methods(http.MethodGet)

	api.Handle("/tests/current", authed(h.psychoTest.Current)).Methods(http.MethodGet)
	api.Handle("/tests/results/me", authed(h.psychoTest.MyResult)).Methods(http.MethodGet)
	api.Handle("/tests/{test_id}/results", authed(h.psychoTest.Submit)).Methods(http.MethodPost)

	api.Handle("/location/me/preview", authed(h.location.Preview)).Methods(http.MethodPost)
	api.Handle("/location/me", authed(h.location.SaveLocation)).Methods(http.MethodPut)
	api.Handle("/location/me", authed(h.location.GetLocation)).Methods(http.MethodGet)
	api.Handle("/matches/{match_id}/search-places", authed(h.event.CreateSearch)).Methods(http.MethodPost)
	api.Handle("/matches/{match_id}/place-searches/{search_id}", authed(h.event.GetSearch)).Methods(http.MethodGet)
	api.Handle("/matches/{match_id}/place-searches/{search_id}/results", authed(h.event.Submit)).Methods(http.MethodPost)

	return r
}
