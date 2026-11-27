package client_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"dating-app/gateway/internal/client"
	"dating-app/gateway/internal/grpcerr"
	"dating-app/gateway/internal/model"
	authpb "dating-app/gen/go/auth/v1"

	"google.golang.org/grpc"
)

// stubAuthClient отвечает на Register так, как ответил бы auth-сервис через grpcclient:
// ошибкой, восстановленной из gRPC-статуса.
type stubAuthClient struct {
	authpb.AuthServiceClient
	err error
}

func (s stubAuthClient) Register(context.Context, *authpb.RegisterRequest, ...grpc.CallOption) (*authpb.RegisterResponse, error) {
	return nil, s.err
}

func TestRegisterSessionNotOpenedRestoresUserID(t *testing.T) {
	remote := grpcerr.FromStatus(grpcerr.ToStatus(grpcerr.WithMetadata(model.ErrSessionNotOpened,
		map[string]string{grpcerr.MetadataUserID: "77"})))
	adapter := client.NewAuthAdapter(stubAuthClient{err: remote})

	res, err := adapter.Register(context.Background(), model.RegisterInput{Email: "a@b.c", Password: "secret123"})
	if !errors.Is(err, model.ErrSessionNotOpened) {
		t.Fatalf("err = %v, want ErrSessionNotOpened", err)
	}
	if res.UserID != 77 || !slices.Equal(res.Missing, (&model.Profile{}).Missing()) {
		t.Fatalf("result = %+v, want user 77 with empty-profile missing", res)
	}
}

func TestRegisterOtherErrorsReturnEmptyResult(t *testing.T) {
	remote := grpcerr.FromStatus(grpcerr.ToStatus(model.ErrEmailAlreadyExists))
	adapter := client.NewAuthAdapter(stubAuthClient{err: remote})

	res, err := adapter.Register(context.Background(), model.RegisterInput{Email: "a@b.c", Password: "secret123"})
	if !errors.Is(err, model.ErrEmailAlreadyExists) || res.UserID != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}
