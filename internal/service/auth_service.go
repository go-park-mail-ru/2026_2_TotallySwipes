package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dating-app/internal/auth"
	"dating-app/internal/model"
)

type UserRepository interface {
	// CreateUser создаёт пользователя с пустой анкетой; model.ErrEmailAlreadyExists при занятой почте.
	CreateUser(ctx context.Context, user *model.UserInput) (int64, error)
	// GetUserByEmail возвращает пользователя или model.ErrNotFound.
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
}

type ProfileMissingChecker interface {
	// Missing возвращает незаполненные обязательные поля анкеты.
	Missing(ctx context.Context, userID int64) ([]string, error)
}

type SessionRepository interface {
	Create(ctx context.Context, s *model.Session) error
	// GetByTokenHash возвращает сессию или model.ErrNotFound, в том числе после истечения TTL.
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error)
	// Revoke атомарно отзывает активную сессию; true, если отозвал именно этот вызов.
	Revoke(ctx context.Context, id string) (bool, error)
}

type PasswordHasher interface {
	// Hash возвращает model.ErrPasswordTooLong, если пароль длиннее ограничения алгоритма.
	Hash(plain string) (string, error)
	Matches(hash, plain string) (bool, error)
}

type AccessTokenIssuer interface {
	Issue(userID int64, sessionID string) (token string, expiresAt time.Time, err error)
}

type AuthService struct {
	users      UserRepository
	profiles   ProfileMissingChecker
	sessions   SessionRepository
	hasher     PasswordHasher
	access     AccessTokenIssuer
	refreshTTL time.Duration
	now        func() time.Time
}

func NewAuthService(users UserRepository, profiles ProfileMissingChecker, sessions SessionRepository,
	hasher PasswordHasher, access AccessTokenIssuer, refreshTTL time.Duration) *AuthService {

	return &AuthService{
		users:      users,
		profiles:   profiles,
		sessions:   sessions,
		hasher:     hasher,
		access:     access,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

// Register создаёт пользователя с пустой анкетой и открывает сессию; при сбое сессии - UserID и model.ErrSessionNotOpened
func (s *AuthService) Register(ctx context.Context, in model.RegisterInput) (model.AuthResult, error) {
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	userID, err := s.users.CreateUser(ctx, &model.UserInput{Email: in.Email, PasswordHash: hash})
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("create user: %w", err)
	}

	// Анкета только что создана пустой - не хватает всех обязательных полей
	missing := (&model.Profile{}).Missing()

	tokens, err := s.openSession(ctx, userID)
	if err != nil {
		return model.AuthResult{UserID: userID, Missing: missing}, fmt.Errorf("register: %w: %w", model.ErrSessionNotOpened, err)
	}
	return model.AuthResult{UserID: userID, Missing: missing, Tokens: tokens}, nil
}

// Login проверяет пароль пользователя и открывает новую сессию.
func (s *AuthService) Login(ctx context.Context, email, password string) (model.AuthResult, error) {
	user, err := s.users.GetUserByEmail(ctx, email)
	if errors.Is(err, model.ErrNotFound) {
		return model.AuthResult{}, fmt.Errorf("login: %w", model.ErrInvalidCredentials)
	}
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("get user: %w", err)
	}

	ok, err := s.hasher.Matches(user.PasswordHash, password)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("compare password: %w", err)
	}
	if !ok {
		return model.AuthResult{}, fmt.Errorf("login: %w", model.ErrInvalidCredentials)
	}

	missing, err := s.profiles.Missing(ctx, user.ID)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("check profile: %w", err)
	}

	tokens, err := s.openSession(ctx, user.ID)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("login: open session: %w", err)
	}

	return model.AuthResult{UserID: user.ID, Missing: missing, Tokens: tokens}, nil
}

// Logout отзывает сессию; пустой токен и уже отозванная сессия ошибкой не считаются
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}

	session, err := s.sessions.GetByTokenHash(ctx, auth.HashToken(refreshToken))
	if errors.Is(err, model.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	// Уже отозванная сессия - тоже успешный выход, поэтому результат не важен
	if _, err := s.sessions.Revoke(ctx, session.ID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// Refresh отзывает действующую сессию и создаёт новую пару токенов.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (model.Tokens, error) {
	if refreshToken == "" {
		return model.Tokens{}, fmt.Errorf("refresh: %w", model.ErrInvalidSession)
	}

	session, err := s.sessions.GetByTokenHash(ctx, auth.HashToken(refreshToken))
	if errors.Is(err, model.ErrNotFound) {
		return model.Tokens{}, fmt.Errorf("refresh: %w", model.ErrInvalidSession)
	}
	if err != nil {
		return model.Tokens{}, fmt.Errorf("get session: %w", err)
	}
	if session.Revoked || !s.now().Before(session.ExpiresAt) {
		return model.Tokens{}, fmt.Errorf("refresh session id=%s: %w", session.ID, model.ErrInvalidSession)
	}

	// При гонке двух refresh новую пару получит только запрос, отозвавший старую сессию
	revoked, err := s.sessions.Revoke(ctx, session.ID)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("revoke session: %w", err)
	}
	if !revoked {
		return model.Tokens{}, fmt.Errorf("refresh session id=%s: %w", session.ID, model.ErrInvalidSession)
	}

	tokens, err := s.openSession(ctx, session.UserID)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("refresh: open session: %w", err)
	}
	return tokens, nil
}

// openSession создаёт refresh-сессию и выпускает access-токен.
func (s *AuthService) openSession(ctx context.Context, userID int64) (model.Tokens, error) {
	sessionID, err := auth.NewSessionID()
	if err != nil {
		return model.Tokens{}, fmt.Errorf("new session id: %w", err)
	}
	refresh, err := auth.NewRefreshToken()
	if err != nil {
		return model.Tokens{}, fmt.Errorf("new refresh token: %w", err)
	}
	refreshExpiresAt := s.now().Add(s.refreshTTL)

	err = s.sessions.Create(ctx, &model.Session{
		ID:        sessionID,
		UserID:    userID,
		TokenHash: auth.HashToken(refresh),
		ExpiresAt: refreshExpiresAt,
	})
	if err != nil {
		return model.Tokens{}, fmt.Errorf("create session: %w", err)
	}

	access, accessExpiresAt, err := s.access.Issue(userID, sessionID)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("issue access token: %w", err)
	}

	return model.Tokens{
		Access:           access,
		AccessExpiresAt:  accessExpiresAt,
		Refresh:          refresh,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}
