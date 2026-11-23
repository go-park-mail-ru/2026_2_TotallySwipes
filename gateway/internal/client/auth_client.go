package client

import (
	"context"
	authpb "dating-app/gen/go/auth/v1"
	"dating-app/internal/model"
	"fmt"
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

	tk := response.GetTokens()
	if tk == nil {
		return model.AuthResult{}, fmt.Errorf("auth login: missing tokens")
	}

	if err := tk.GetAccessExpiresAt().CheckValid(); err != nil {
		return model.AuthResult{}, err
	}
	if err := tk.GetRefreshExpiresAt().CheckValid(); err != nil {
		return model.AuthResult{}, err
	}

	tokens := model.Tokens{
		Access:           tk.GetAccessToken(),
		AccessExpiresAt:  tk.GetAccessExpiresAt().AsTime(),
		Refresh:          tk.GetRefreshToken(),
		RefreshExpiresAt: tk.GetRefreshExpiresAt().AsTime(),
	}

	return model.AuthResult{UserID: response.GetUserId(), Missing: response.GetMissing(), Tokens: tokens}, nil
}

func (a *AuthAdapter) Register(ctx context.Context, in model.RegisterInput) (model.AuthResult, error) {

	response, err := a.client.Register(ctx, &authpb.RegisterRequest{Email: in.Email, Password: in.Password})

	if err != nil {
		return model.AuthResult{}, err
	}

	tk := response.GetTokens()
	if tk == nil {
		return model.AuthResult{}, fmt.Errorf("auth register: missing tokens")
	}

	if err := tk.GetAccessExpiresAt().CheckValid(); err != nil {
		return model.AuthResult{}, err
	}

	if err := tk.GetRefreshExpiresAt().CheckValid(); err != nil {
		return model.AuthResult{}, err
	}

	tokens := model.Tokens{
		Access:           tk.GetAccessToken(),
		AccessExpiresAt:  tk.GetAccessExpiresAt().AsTime(),
		Refresh:          tk.GetRefreshToken(),
		RefreshExpiresAt: tk.GetRefreshExpiresAt().AsTime(),
	}

	return model.AuthResult{UserID: response.GetUserId(), Missing: response.GetMissing(), Tokens: tokens}, nil

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

	tk := response.GetTokens()
	if tk == nil {
		return model.Tokens{}, fmt.Errorf("auth refresh: missing tokens")
	}

	if err := tk.GetAccessExpiresAt().CheckValid(); err != nil {
		return model.Tokens{}, err
	}

	if err := tk.GetRefreshExpiresAt().CheckValid(); err != nil {
		return model.Tokens{}, err
	}

	tokens := model.Tokens{
		Access:           tk.GetAccessToken(),
		AccessExpiresAt:  tk.GetAccessExpiresAt().AsTime(),
		Refresh:          tk.GetRefreshToken(),
		RefreshExpiresAt: tk.GetRefreshExpiresAt().AsTime(),
	}

	return tokens, nil
}
