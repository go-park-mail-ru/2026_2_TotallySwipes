package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"dating-app/internal/auth"
	"dating-app/internal/model"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrSessionNotOpened - аккаунт уже создан, но сессию открыть не удалось.
// AuthResult при этом содержит UserID, Tokens пустые: клиенту надо залогиниться
var ErrSessionNotOpened = errors.New("account created, session not opened")

type AuthUserRepository interface {
	// CreateUserWithProfile в одной транзакции создаёт пользователя и профиль
	// Если email занят - model.ErrEmailAlreadyExists
	CreateUserWithProfile(ctx context.Context, user *model.UserInput, version *model.ProfileVersionInput, tags []string) (int64, error)
	// GetUserByEmail - model.ErrNotFound, если пользователя нет
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
}

type ProfileCompletionChecker interface {
	IsProfileCompleted(ctx context.Context, userID int64) (bool, error)
}

type SessionRepository interface {
	Create(ctx context.Context, s *model.Session) error
	// GetByTokenHash - model.ErrNotFound, если сессии нет или она истекла
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error)
	Revoke(ctx context.Context, id string) error
}

type PasswordHasher interface {
	// Hash - model.ErrPasswordTooLong, если пароль не влезает в алгоритм
	Hash(plain string) (string, error)
	Matches(hash, plain string) (bool, error)
}

type AccessTokenIssuer interface {
	Issue(userID int64, sessionID string) (token string, expiresAt time.Time, err error)
}

type RegisterInput struct {
	Name          string
	Email         string
	Password      string
	BirthDate     time.Time
	Sex           model.Sex
	SearchSex     model.SearchSex
	DatingGoal    model.DatingGoal
	AboutMe       string
	SearchAgeFrom int
	SearchAgeTo   int
	Tags          []string
}

// Tokens - пара токенов новой сессии
type Tokens struct {
	Access           string
	AccessExpiresAt  time.Time
	Refresh          string
	RefreshExpiresAt time.Time
}

type AuthResult struct {
	UserID           int64
	ProfileCompleted bool
	Tokens           Tokens
}

type AuthService struct {
	users      AuthUserRepository
	profiles   ProfileCompletionChecker
	sessions   SessionRepository
	hasher     PasswordHasher
	access     AccessTokenIssuer
	refreshTTL time.Duration
	now        func() time.Time

	dummyHashOnce sync.Once
	dummyHash     string
}

func NewAuthService(users AuthUserRepository, profiles ProfileCompletionChecker, sessions SessionRepository,
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

// Register создаёт пользователя с профилем и сразу открывает сессию.
// Ввод должен быть уже провалидирован. Если пользователь создан, а сессия
// нет - возвращает ErrSessionNotOpened вместе с UserID
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (AuthResult, error) {
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	userID, err := s.users.CreateUserWithProfile(ctx,
		&model.UserInput{
			Name:         in.Name,
			Email:        in.Email,
			PasswordHash: hash,
			BirthDate:    in.BirthDate,
		},
		&model.ProfileVersionInput{
			BirthDate:     in.BirthDate,
			DatingGoal:    in.DatingGoal,
			AboutMe:       in.AboutMe,
			Sex:           in.Sex,
			SearchSex:     in.SearchSex,
			SearchAgeFrom: in.SearchAgeFrom,
			SearchAgeTo:   in.SearchAgeTo,
		},
		in.Tags,
	)
	if err != nil {
		return AuthResult{}, fmt.Errorf("create user: %w", err)
	}

	tokens, err := s.openSession(ctx, userID)
	if err != nil {
		return AuthResult{UserID: userID}, fmt.Errorf("%w: %w", ErrSessionNotOpened, err)
	}

	// Психотест при регистрации ещё не пройден
	return AuthResult{UserID: userID, ProfileCompleted: false, Tokens: tokens}, nil
}

// Login - ErrInvalidCredentials, если нет такого email или пароль не подошёл
func (s *AuthService) Login(ctx context.Context, email, password string) (AuthResult, error) {
	user, err := s.users.GetUserByEmail(ctx, email)
	if errors.Is(err, model.ErrNotFound) {
		// Всё равно считаем bcrypt, чтобы по времени ответа нельзя было
		// понять, зарегистрирован ли email
		s.matchDummy(password)
		return AuthResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return AuthResult{}, fmt.Errorf("get user: %w", err)
	}

	ok, err := s.hasher.Matches(user.PasswordHash, password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("compare password: %w", err)
	}
	if !ok {
		return AuthResult{}, ErrInvalidCredentials
	}

	completed, err := s.profiles.IsProfileCompleted(ctx, user.ID)
	if err != nil {
		return AuthResult{}, fmt.Errorf("check profile: %w", err)
	}

	tokens, err := s.openSession(ctx, user.ID)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{UserID: user.ID, ProfileCompleted: completed, Tokens: tokens}, nil
}

// Logout отзывает сессию по refresh-токену. Нет токена или сессии - не ошибка
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
	if session.Revoked {
		return nil
	}

	if err := s.sessions.Revoke(ctx, session.ID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *AuthService) openSession(ctx context.Context, userID int64) (Tokens, error) {
	sessionID, err := auth.NewSessionID()
	if err != nil {
		return Tokens{}, fmt.Errorf("new session id: %w", err)
	}
	refresh, err := auth.NewRefreshToken()
	if err != nil {
		return Tokens{}, fmt.Errorf("new refresh token: %w", err)
	}
	refreshExpiresAt := s.now().Add(s.refreshTTL)

	err = s.sessions.Create(ctx, &model.Session{
		ID:        sessionID,
		UserID:    userID,
		TokenHash: auth.HashToken(refresh),
		ExpiresAt: refreshExpiresAt,
	})
	if err != nil {
		return Tokens{}, fmt.Errorf("create session: %w", err)
	}

	access, accessExpiresAt, err := s.access.Issue(userID, sessionID)
	if err != nil {
		return Tokens{}, fmt.Errorf("issue access token: %w", err)
	}

	return Tokens{
		Access:           access,
		AccessExpiresAt:  accessExpiresAt,
		Refresh:          refresh,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (s *AuthService) matchDummy(password string) {
	s.dummyHashOnce.Do(func() {
		s.dummyHash, _ = s.hasher.Hash("dummy-password-1")
	})
	_, _ = s.hasher.Matches(s.dummyHash, password)
}
