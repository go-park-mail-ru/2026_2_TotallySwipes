package service

import (
	"context"
	"dating-app/internal/auth"
	"dating-app/internal/model"
	. "dating-app/internal/service"
	"errors"
	"go.uber.org/mock/gomock"
	"reflect"
	"testing"
	"time"
)

func TestRegister(t *testing.T) {
	failure := errors.New("dependency failure")
	for _, tc := range []struct {
		name                                             string
		hashErr, userErr, sessionErr, issuerErr, wantErr error
	}{
		{name: "success"},
		{name: "hash error", hashErr: model.ErrPasswordTooLong, wantErr: model.ErrPasswordTooLong},
		{name: "duplicate email", userErr: model.ErrEmailAlreadyExists, wantErr: model.ErrEmailAlreadyExists},
		{name: "session error", sessionErr: failure, wantErr: model.ErrSessionNotOpened},
		{name: "issuer error", issuerErr: failure, wantErr: model.ErrSessionNotOpened},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			users := NewMockUserRepository(ctrl)
			profiles := NewMockProfileMissingChecker(ctrl)
			sessions := NewMockSessionRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl)
			issuer := NewMockAccessTokenIssuer(ctrl)
			svc := NewAuthService(users, profiles, sessions, hasher, issuer, time.Hour)
			ctx := context.Background()
			input := model.RegisterInput{Email: "alex@example.com", Password: "password"}
			hasher.EXPECT().Hash(input.Password).Return("hash", tc.hashErr).Times(1)
			var saved model.Session
			expiry := time.Now().Add(time.Minute)
			if tc.hashErr == nil {
				users.EXPECT().CreateUser(ctx, &model.UserInput{Email: input.Email, PasswordHash: "hash"}).Return(int64(12), tc.userErr).Times(1)
				if tc.userErr == nil {
					create := sessions.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, session *model.Session) error { saved = *session; return tc.sessionErr }).Times(1)
					if tc.sessionErr == nil {
						issuer.EXPECT().Issue(int64(12), gomock.Any()).After(create).DoAndReturn(func(id int64, sid string) (string, time.Time, error) {
							if sid == "" || sid != saved.ID {
								t.Fatalf("session ID=%q, stored=%q", sid, saved.ID)
							}
							return "access", expiry, tc.issuerErr
						}).Times(1)
					}
				}
			}
			before := time.Now()
			// Act
			got, err := svc.Register(ctx, input)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
			if tc.hashErr != nil || tc.userErr != nil {
				if !reflect.DeepEqual(got, model.AuthResult{}) {
					t.Fatalf("unexpected result: %+v", got)
				}
				return
			}
			if got.UserID != 12 || !reflect.DeepEqual(got.Missing, (&model.Profile{}).Missing()) {
				t.Fatalf("result=%+v", got)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, failure) || got.Tokens != (model.Tokens{}) {
					t.Fatalf("partial registration=%+v, %v", got, err)
				}
				return
			}
			if saved.ID == "" || saved.UserID != 12 || saved.TokenHash != auth.HashToken(got.Tokens.Refresh) || got.Tokens.Refresh == "" {
				t.Fatalf("invalid saved session: %+v", saved)
			}
			if saved.TokenHash == got.Tokens.Refresh || saved.Revoked {
				t.Fatal("invalid session state")
			}
			if got.Tokens.Access != "access" || !got.Tokens.AccessExpiresAt.Equal(expiry) || !got.Tokens.RefreshExpiresAt.Equal(saved.ExpiresAt) {
				t.Fatalf("tokens=%+v", got.Tokens)
			}
			if saved.ExpiresAt.Before(before.Add(time.Hour)) || saved.ExpiresAt.After(time.Now().Add(time.Hour)) {
				t.Fatal("wrong refresh TTL")
			}
		})
	}
}

func TestLogin(t *testing.T) {
	failure := errors.New("dependency failure")
	for _, tc := range []struct {
		name                                                           string
		getErr, compareErr, profileErr, sessionErr, issuerErr, wantErr error
		mismatch                                                       bool
	}{
		{name: "success"},
		{name: "unknown email", getErr: model.ErrNotFound, wantErr: model.ErrInvalidCredentials},
		{name: "database error", getErr: failure, wantErr: failure},
		{name: "wrong password", mismatch: true, wantErr: model.ErrInvalidCredentials},
		{name: "comparison error", compareErr: failure, wantErr: failure},
		{name: "profile error", profileErr: failure, wantErr: failure},
		{name: "session error", sessionErr: failure, wantErr: failure},
		{name: "issuer error", issuerErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			users := NewMockUserRepository(ctrl)
			profiles := NewMockProfileMissingChecker(ctrl)
			sessions := NewMockSessionRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl)
			issuer := NewMockAccessTokenIssuer(ctrl)
			svc := NewAuthService(users, profiles, sessions, hasher, issuer, time.Hour)
			ctx := context.Background()
			users.EXPECT().GetUserByEmail(ctx, "alex@example.com").Return(&model.User{ID: 7, PasswordHash: "hash"}, tc.getErr).Times(1)
			var saved model.Session
			expiry := time.Now().Add(time.Minute)
			if tc.getErr == nil {
				hasher.EXPECT().Matches("hash", "password").Return(!tc.mismatch, tc.compareErr).Times(1)
				if !tc.mismatch && tc.compareErr == nil {
					profiles.EXPECT().Missing(ctx, int64(7)).Return([]string{"photos"}, tc.profileErr).Times(1)
					if tc.profileErr == nil {
						create := sessions.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, s *model.Session) error { saved = *s; return tc.sessionErr }).Times(1)
						if tc.sessionErr == nil {
							issuer.EXPECT().Issue(int64(7), gomock.Any()).After(create).DoAndReturn(func(_ int64, id string) (string, time.Time, error) {
								if id != saved.ID || id == "" {
									t.Fatal("wrong session ID")
								}
								return "access", expiry, tc.issuerErr
							}).Times(1)
						}
					}
				}
			}
			// Act
			got, err := svc.Login(ctx, "alex@example.com", "password")
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !reflect.DeepEqual(got, model.AuthResult{}) {
					t.Fatalf("unexpected result: %+v", got)
				}
				return
			}
			if got.UserID != 7 || !reflect.DeepEqual(got.Missing, []string{"photos"}) || got.Tokens.Access != "access" || got.Tokens.Refresh == "" {
				t.Fatalf("result=%+v", got)
			}
			if saved.UserID != 7 || saved.TokenHash != auth.HashToken(got.Tokens.Refresh) || !got.Tokens.AccessExpiresAt.Equal(expiry) {
				t.Fatal("invalid session/tokens")
			}
		})
	}
}

func TestLogout(t *testing.T) {
	failure := errors.New("redis down")
	for _, tc := range []struct {
		name, token                string
		getErr, revokeErr, wantErr error
		revoked                    bool
	}{
		{name: "empty"}, {name: "unknown", token: "unknown", getErr: model.ErrNotFound},
		{name: "success", token: "refresh", revoked: true}, {name: "already revoked", token: "refresh"},
		{name: "read error", token: "refresh", getErr: failure, wantErr: failure},
		{name: "revoke error", token: "refresh", revokeErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			sessions := NewMockSessionRepository(ctrl)
			ctx := context.Background()
			svc := NewAuthService(nil, nil, sessions, nil, nil, time.Hour)
			if tc.token != "" {
				sessions.EXPECT().GetByTokenHash(ctx, auth.HashToken(tc.token)).Return(&model.Session{ID: "sid"}, tc.getErr).Times(1)
				if tc.getErr == nil {
					sessions.EXPECT().Revoke(ctx, "sid").Return(tc.revoked, tc.revokeErr).Times(1)
				}
			}
			// Act
			err := svc.Logout(ctx, tc.token)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefresh(t *testing.T) {
	failure := errors.New("redis down")
	for _, tc := range []struct {
		name, token                                      string
		getErr, revokeErr, createErr, issuerErr, wantErr error
		revoked, expired, lostRace                       bool
	}{
		{name: "success", token: "old"}, {name: "empty", wantErr: model.ErrInvalidSession},
		{name: "unknown", token: "old", getErr: model.ErrNotFound, wantErr: model.ErrInvalidSession},
		{name: "revoked", token: "old", revoked: true, wantErr: model.ErrInvalidSession},
		{name: "expired", token: "old", expired: true, wantErr: model.ErrInvalidSession},
		{name: "lost race", token: "old", lostRace: true, wantErr: model.ErrInvalidSession},
		{name: "read error", token: "old", getErr: failure, wantErr: failure},
		{name: "revoke error", token: "old", revokeErr: failure, wantErr: failure},
		{name: "create error", token: "old", createErr: failure, wantErr: failure},
		{name: "issuer error", token: "old", issuerErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			sessions := NewMockSessionRepository(ctrl)
			issuer := NewMockAccessTokenIssuer(ctrl)
			svc := NewAuthService(nil, nil, sessions, nil, issuer, time.Hour)
			ctx := context.Background()
			old := &model.Session{ID: "old-sid", UserID: 7, Revoked: tc.revoked, ExpiresAt: time.Now().Add(time.Hour)}
			if tc.expired {
				old.ExpiresAt = time.Now().Add(-time.Second)
			}
			var saved model.Session
			expiry := time.Now().Add(time.Minute)
			if tc.token != "" {
				get := sessions.EXPECT().GetByTokenHash(ctx, auth.HashToken(tc.token)).Return(old, tc.getErr).Times(1)
				if tc.getErr == nil && !tc.expired && !tc.revoked {
					revoke := sessions.EXPECT().Revoke(ctx, old.ID).After(get).Return(!tc.lostRace, tc.revokeErr).Times(1)
					if tc.revokeErr == nil && !tc.lostRace {
						create := sessions.EXPECT().Create(ctx, gomock.Any()).After(revoke).DoAndReturn(func(_ context.Context, s *model.Session) error { saved = *s; return tc.createErr }).Times(1)
						if tc.createErr == nil {
							issuer.EXPECT().Issue(int64(7), gomock.Any()).After(create).DoAndReturn(func(_ int64, id string) (string, time.Time, error) {
								if id == "" || id != saved.ID {
									t.Fatal("wrong session ID")
								}
								return "access", expiry, tc.issuerErr
							}).Times(1)
						}
					}
				}
			}
			// Act
			got, err := svc.Refresh(ctx, tc.token)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (model.Tokens{}) {
					t.Fatal("unexpected tokens")
				}
				return
			}
			if got.Refresh == "" || got.Refresh == tc.token || saved.ID == old.ID || saved.UserID != 7 || saved.TokenHash != auth.HashToken(got.Refresh) {
				t.Fatal("session was not rotated")
			}
			if got.Access != "access" || !got.AccessExpiresAt.Equal(expiry) || !got.RefreshExpiresAt.Equal(saved.ExpiresAt) {
				t.Fatal("incorrect token fields")
			}
		})
	}
}

func TestRefresh_RotationAndReuse(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionRepository(ctrl)
	issuer := NewMockAccessTokenIssuer(ctrl)
	ctx := context.Background()
	svc := NewAuthService(nil, nil, sessions, nil, issuer, time.Hour)
	old := &model.Session{ID: "old-session", UserID: 7, ExpiresAt: time.Now().Add(time.Hour)}
	revoked := *old
	revoked.Revoked = true
	var firstSession, secondSession model.Session
	gomock.InOrder(
		sessions.EXPECT().GetByTokenHash(ctx, auth.HashToken("old")).Return(old, nil).Times(1),
		sessions.EXPECT().Revoke(ctx, old.ID).Return(true, nil).Times(1),
		sessions.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, s *model.Session) error { firstSession = *s; return nil }).Times(1),
		issuer.EXPECT().Issue(int64(7), gomock.Any()).DoAndReturn(func(_ int64, id string) (string, time.Time, error) {
			if id != firstSession.ID {
				t.Fatal("wrong first session")
			}
			return "access-1", time.Now().Add(time.Minute), nil
		}).Times(1),
		sessions.EXPECT().GetByTokenHash(ctx, auth.HashToken("old")).Return(&revoked, nil).Times(1),
		sessions.EXPECT().GetByTokenHash(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, hash string) (*model.Session, error) {
			if hash != firstSession.TokenHash {
				t.Fatal("wrong rotated token hash")
			}
			return &firstSession, nil
		}).Times(1),
		sessions.EXPECT().Revoke(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, id string) (bool, error) {
			if id != firstSession.ID {
				t.Fatal("wrong revoked session")
			}
			return true, nil
		}).Times(1),
		sessions.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, s *model.Session) error { secondSession = *s; return nil }).Times(1),
		issuer.EXPECT().Issue(int64(7), gomock.Any()).DoAndReturn(func(_ int64, id string) (string, time.Time, error) {
			if id != secondSession.ID {
				t.Fatal("wrong second session")
			}
			return "access-2", time.Now().Add(time.Minute), nil
		}).Times(1),
	)
	// Act
	first, err := svc.Refresh(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	_, reuseErr := svc.Refresh(ctx, "old")
	second, err := svc.Refresh(ctx, first.Refresh)
	// Assert
	if !errors.Is(reuseErr, model.ErrInvalidSession) {
		t.Fatalf("old token accepted: %v", reuseErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if first.Refresh == "" || second.Refresh == "" || first.Refresh == second.Refresh || firstSession.ID == secondSession.ID {
		t.Fatal("tokens/sessions not rotated")
	}
	if secondSession.TokenHash != auth.HashToken(second.Refresh) || second.Access != "access-2" {
		t.Fatal("invalid second session")
	}
}
