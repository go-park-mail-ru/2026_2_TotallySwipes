package mapper

import (
	authpb "dating-app/gen/go/auth/v1"
	"dating-app/internal/model"
	"fmt"
)

func TokensFromProto(tk *authpb.Tokens) (model.Tokens, error) {

	if tk == nil {
		return model.Tokens{}, fmt.Errorf("proto: missing tokens")
	}

	if err := tk.GetAccessExpiresAt().CheckValid(); err != nil {
		return model.Tokens{}, err
	}

	if err := tk.GetRefreshExpiresAt().CheckValid(); err != nil {
		return model.Tokens{}, err
	}

	return model.Tokens{Access: tk.GetAccessToken(), AccessExpiresAt: tk.GetAccessExpiresAt().AsTime(), Refresh: tk.GetRefreshToken(), RefreshExpiresAt: tk.GetRefreshExpiresAt().AsTime()}, nil
}

func LoginResponseFromProto(response *authpb.LoginResponse) (model.AuthResult, error) {

	tokens, err := TokensFromProto(response.GetTokens())

	if err != nil {
		return model.AuthResult{}, err
	}

	return model.AuthResult{UserID: response.GetUserId(), Missing: response.GetMissing(), Tokens: tokens}, nil
}

func RegisterResponseFromProto(response *authpb.RegisterResponse) (model.AuthResult, error) {

	tokens, err := TokensFromProto(response.GetTokens())

	if err != nil {
		return model.AuthResult{}, err
	}

	return model.AuthResult{UserID: response.GetUserId(), Missing: response.GetMissing(), Tokens: tokens}, nil
}
