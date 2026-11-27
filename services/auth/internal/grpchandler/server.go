// Package grpchandler реализует AuthService поверх service.AuthService.
package grpchandler

import (
	"context"
	"errors"
	"strconv"

	authpb "dating-app/gen/go/auth/v1"
	"dating-app/internal/grpcerr"
	"dating-app/internal/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AuthService interface {
	Register(ctx context.Context, in model.RegisterInput) (model.AuthResult, error)
	Login(ctx context.Context, email, password string) (model.AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
	Refresh(ctx context.Context, refreshToken string) (model.Tokens, error)
}

type Server struct {
	authpb.UnimplementedAuthServiceServer

	auth AuthService
}

func NewServer(auth AuthService) *Server {
	return &Server{auth: auth}
}

// Register создаёт аккаунт и сессию. Если аккаунт создан, а сессия нет, клиент получает
// ErrSessionNotOpened с ID пользователя в метаданных ошибки.
func (s *Server) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	if req.GetEmail() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	res, err := s.auth.Register(ctx, model.RegisterInput{Email: req.GetEmail(), Password: req.GetPassword()})
	if errors.Is(err, model.ErrSessionNotOpened) {
		return nil, grpcerr.WithMetadata(err, map[string]string{grpcerr.MetadataUserID: strconv.FormatInt(res.UserID, 10)})
	}
	if err != nil {
		return nil, err
	}

	return &authpb.RegisterResponse{UserId: res.UserID, Missing: res.Missing, Tokens: tokensToProto(res.Tokens)}, nil
}

func (s *Server) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	if req.GetEmail() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	res, err := s.auth.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, err
	}

	return &authpb.LoginResponse{UserId: res.UserID, Missing: res.Missing, Tokens: tokensToProto(res.Tokens)}, nil
}

func (s *Server) Refresh(ctx context.Context, req *authpb.RefreshRequest) (*authpb.RefreshResponse, error) {
	tokens, err := s.auth.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return &authpb.RefreshResponse{Tokens: tokensToProto(tokens)}, nil
}

func (s *Server) Logout(ctx context.Context, req *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	if err := s.auth.Logout(ctx, req.GetRefreshToken()); err != nil {
		return nil, err
	}
	return &authpb.LogoutResponse{}, nil
}

func tokensToProto(t model.Tokens) *authpb.Tokens {
	return &authpb.Tokens{
		AccessToken:      t.Access,
		AccessExpiresAt:  timestamppb.New(t.AccessExpiresAt),
		RefreshToken:     t.Refresh,
		RefreshExpiresAt: timestamppb.New(t.RefreshExpiresAt),
	}
}
