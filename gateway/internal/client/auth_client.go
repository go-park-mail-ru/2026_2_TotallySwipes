package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	"dating-app/gateway/internal/grpcerr"
	"dating-app/gateway/internal/model"
	authpb "dating-app/gen/go/auth/v1"
	"errors"
	"strconv"
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

	if errors.Is(err, model.ErrSessionNotOpened) {
		// Аккаунт создан с пустой анкетой, но без сессии: хендлер ответит 201 без cookie.
		userID, _ := strconv.ParseInt(grpcerr.Metadata(err)[grpcerr.MetadataUserID], 10, 64)
		return model.AuthResult{UserID: userID, Missing: (&model.Profile{}).Missing()}, err
	}

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
