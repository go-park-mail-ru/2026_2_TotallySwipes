package grpchandler_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"

	authpb "dating-app/gen/go/auth/v1"
	"dating-app/internal/grpcclient"
	"dating-app/internal/grpcerr"
	"dating-app/internal/grpcserver"
	"dating-app/internal/model"
	"dating-app/services/auth/internal/grpchandler"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeAuth struct {
	result       model.AuthResult
	tokens       model.Tokens
	err          error
	logoutToken  string
	refreshToken string
}

func (f *fakeAuth) Register(context.Context, model.RegisterInput) (model.AuthResult, error) {
	return f.result, f.err
}
func (f *fakeAuth) Login(context.Context, string, string) (model.AuthResult, error) {
	return f.result, f.err
}
func (f *fakeAuth) Logout(_ context.Context, token string) error {
	f.logoutToken = token
	return f.err
}
func (f *fakeAuth) Refresh(_ context.Context, token string) (model.Tokens, error) {
	f.refreshToken = token
	return f.tokens, f.err
}

func startServer(t *testing.T, auth *fakeAuth) authpb.AuthServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpcserver.New()
	authpb.RegisterAuthServiceServer(srv, grpchandler.NewServer(auth))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryInterceptor(time.Second)),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return authpb.NewAuthServiceClient(conn)
}

var (
	accessExp  = time.Date(2026, 11, 27, 12, 15, 0, 0, time.UTC)
	refreshExp = time.Date(2026, 12, 27, 12, 0, 0, 0, time.UTC)
	tokens     = model.Tokens{Access: "access", AccessExpiresAt: accessExp, Refresh: "refresh", RefreshExpiresAt: refreshExp}
)

func TestLoginMapsTokens(t *testing.T) {
	client := startServer(t, &fakeAuth{result: model.AuthResult{UserID: 5, Missing: []string{"photos"}, Tokens: tokens}})

	resp, err := client.Login(context.Background(), &authpb.LoginRequest{Email: "a@b.c", Password: "secret123"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	tk := resp.GetTokens()
	if resp.GetUserId() != 5 || !slices.Equal(resp.GetMissing(), []string{"photos"}) ||
		tk.GetAccessToken() != "access" || !tk.GetAccessExpiresAt().AsTime().Equal(accessExp) ||
		tk.GetRefreshToken() != "refresh" || !tk.GetRefreshExpiresAt().AsTime().Equal(refreshExp) {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestLoginErrors(t *testing.T) {
	client := startServer(t, &fakeAuth{err: fmt.Errorf("login: %w", model.ErrInvalidCredentials)})

	_, err := client.Login(context.Background(), &authpb.LoginRequest{Email: "a@b.c", Password: "wrong"})
	if !errors.Is(err, model.ErrInvalidCredentials) || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("err = %v, want ErrInvalidCredentials/Unauthenticated", err)
	}

	_, err = client.Login(context.Background(), &authpb.LoginRequest{Email: "a@b.c"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty password: code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestRegisterEmailTaken(t *testing.T) {
	client := startServer(t, &fakeAuth{err: fmt.Errorf("create user: %w", model.ErrEmailAlreadyExists)})

	_, err := client.Register(context.Background(), &authpb.RegisterRequest{Email: "a@b.c", Password: "secret123"})
	if !errors.Is(err, model.ErrEmailAlreadyExists) {
		t.Fatalf("err = %v, want ErrEmailAlreadyExists", err)
	}
}

func TestRegisterSessionNotOpenedKeepsUserID(t *testing.T) {
	client := startServer(t, &fakeAuth{
		result: model.AuthResult{UserID: 77},
		err:    fmt.Errorf("register: %w: %w", model.ErrSessionNotOpened, errors.New("redis down")),
	})

	_, err := client.Register(context.Background(), &authpb.RegisterRequest{Email: "a@b.c", Password: "secret123"})
	if !errors.Is(err, model.ErrSessionNotOpened) {
		t.Fatalf("err = %v, want ErrSessionNotOpened", err)
	}
	if got := grpcerr.Metadata(err)[grpcerr.MetadataUserID]; got != "77" {
		t.Fatalf("user_id metadata = %q, want 77", got)
	}
}

func TestRefreshAndLogout(t *testing.T) {
	auth := &fakeAuth{tokens: tokens}
	client := startServer(t, auth)

	resp, err := client.Refresh(context.Background(), &authpb.RefreshRequest{RefreshToken: "old"})
	if err != nil || auth.refreshToken != "old" || resp.GetTokens().GetAccessToken() != "access" {
		t.Fatalf("refresh: resp=%v err=%v token=%q", resp, err, auth.refreshToken)
	}

	if _, err := client.Logout(context.Background(), &authpb.LogoutRequest{RefreshToken: "old"}); err != nil || auth.logoutToken != "old" {
		t.Fatalf("logout: err=%v token=%q", err, auth.logoutToken)
	}

	auth.err = model.ErrInvalidSession
	_, err = client.Refresh(context.Background(), &authpb.RefreshRequest{RefreshToken: "revoked"})
	if !errors.Is(err, model.ErrInvalidSession) {
		t.Fatalf("err = %v, want ErrInvalidSession", err)
	}
}
