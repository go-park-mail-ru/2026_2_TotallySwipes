package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	authpb "dating-app/gen/go/auth/v1"
	"dating-app/internal/model"
)

type AuthAdapter struct {
	client authpb.AuthServiceClient
}

func NewAuthAdapter(client authpb.AuthServiceClient) *AuthAdapter {
	return &AuthAdapter{client: client}
}

func (a *AuthAdapter) Login(ctx context.Context, email, password string) (model.AuthResult, error) {
	response, err := a.client.Login(ctx, &authpb.LoginRequest{Email: email, Password: password})

	if err != nil {
		return model.AuthResult{}, err
	}

	return mapper.LoginResponseFromProto(response)
}

func (a *AuthAdapter) Register(ctx context.Context, in model.RegisterInput) (model.AuthResult, error) {

	response, err := a.client.Register(ctx, &authpb.RegisterRequest{Email: in.Email, Password: in.Password})

	if err != nil {
		return model.AuthResult{}, err
	}

	return mapper.RegisterResponseFromProto(response)
}

func (a *AuthAdapter) Logout(ctx context.Context, refreshToken string) error {

	_, err := a.client.Logout(ctx, &authpb.LogoutRequest{RefreshToken: refreshToken})

	if err != nil {
		return err
	}

	return nil
}

func (a *AuthAdapter) Refresh(ctx context.Context, refreshToken string) (model.Tokens, error) {

	response, err := a.client.Refresh(ctx, &authpb.RefreshRequest{RefreshToken: refreshToken})

	if err != nil {
		return model.Tokens{}, err
	}

	return mapper.TokensFromProto(response.GetTokens())
}
