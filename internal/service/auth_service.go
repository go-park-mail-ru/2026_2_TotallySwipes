package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"dating-app/internal/auth"
	"dating-app/internal/model"
)

type AuthUserRepository interface {
	// CreateUserWithProfile атомарно создаёт пользователя, профиль, первую версию, теги и фото.
	// Принимает: контекст ctx, данные user и version, имена tags и метаданные photos.
	// Возвращает: ID пользователя или ошибку; при занятом email — model.ErrEmailAlreadyExists.
	CreateUserWithProfile(ctx context.Context, user *model.UserInput, version *model.ProfileVersionInput, tags []string, photos []model.PhotoInput) (int64, error)
	// GetUserByEmail находит пользователя по email.
	// Принимает: контекст ctx и адрес email.
	// Возвращает: пользователя или ошибку; при отсутствии — model.ErrNotFound.
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
}

type ProfileCompletionChecker interface {
	// IsProfileCompleted проверяет наличие хотя бы одной записи profile_psycho у пользователя.
	// Принимает: контекст ctx и ID пользователя userID.
	// Возвращает: признак наличия записи и ошибку запроса; заполненность пяти координат не проверяет.
	IsProfileCompleted(ctx context.Context, userID int64) (bool, error)
}

type SessionRepository interface {
	// Create сохраняет сессию и индекс её токена в Redis с одинаковым сроком истечения.
	// Принимает: контекст ctx и данные сессии s.
	// Возвращает: nil при успехе или ошибку записи.
	Create(ctx context.Context, s *model.Session) error
	// GetByTokenHash находит refresh-сессию по хешу токена.
	// Принимает: контекст ctx и хеш tokenHash.
	// Возвращает: сессию или ошибку; если ключи отсутствуют, в том числе после истечения TTL, — model.ErrNotFound.
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error)
	// Revoke помечает существующую refresh-сессию отозванной.
	// Принимает: контекст ctx и ID сессии id.
	// Возвращает: nil при успехе или отсутствии сессии, иначе ошибку Redis.
	Revoke(ctx context.Context, id string) error
	// RevokeIfActive атомарно отзывает сессию, если её флаг revoked равен false.
	// Принимает: контекст ctx и ID сессии id.
	// Возвращает: true, если именно этот вызов отозвал сессию, иначе false; при сбое — ошибку.
	RevokeIfActive(ctx context.Context, id string) (bool, error)
}

type PhotoStorage interface {
	// Save сохраняет файл фотографии в хранилище.
	// Принимает: контекст ctx, байты data и расширение ext.
	// Возвращает: ключ сохранённого файла или ошибку.
	Save(ctx context.Context, data []byte, ext string) (key string, err error)
	// Delete удаляет файл фотографии из хранилища.
	// Принимает: контекст ctx и ключ файла key.
	// Возвращает: nil при успехе или ошибку удаления.
	Delete(ctx context.Context, key string) error
}

type PasswordHasher interface {
	// Hash вычисляет хеш пароля.
	// Принимает: пароль plain.
	// Возвращает: хеш или ошибку; при превышении ограничения алгоритма — model.ErrPasswordTooLong.
	Hash(plain string) (string, error)
	// Matches сравнивает пароль с сохранённым хешем.
	// Принимает: хеш hash и пароль plain.
	// Возвращает: признак совпадения и ошибку проверки.
	Matches(hash, plain string) (bool, error)
}

type AccessTokenIssuer interface {
	// Issue выпускает access-токен для пользователя и сессии.
	// Принимает: ID пользователя userID и ID сессии sessionID.
	// Возвращает: токен, время истечения и ошибку выпуска.
	Issue(userID int64, sessionID string) (token string, expiresAt time.Time, err error)
}

type AuthService struct {
	users      AuthUserRepository
	profiles   ProfileCompletionChecker
	sessions   SessionRepository
	photos     PhotoStorage
	hasher     PasswordHasher
	access     AccessTokenIssuer
	refreshTTL time.Duration
	now        func() time.Time

	dummyHashOnce sync.Once
	dummyHash     string
}

// NewAuthService создаёт сервис регистрации, входа и управления сессиями.
// Принимает: репозитории users и sessions, проверку профиля profiles, хранилище photos, hasher, издатель access и срок refreshTTL.
// Возвращает: экземпляр AuthService.
func NewAuthService(users AuthUserRepository, profiles ProfileCompletionChecker, sessions SessionRepository,
	photos PhotoStorage, hasher PasswordHasher, access AccessTokenIssuer, refreshTTL time.Duration) *AuthService {

	return &AuthService{
		users:      users,
		profiles:   profiles,
		sessions:   sessions,
		photos:     photos,
		hasher:     hasher,
		access:     access,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

// Register создаёт пользователя с профилем и фотографиями, затем открывает сессию.
// Принимает: контекст ctx и предварительно проверенные данные регистрации in.
// Возвращает: данные авторизации или ошибку; если пользователь создан, но сессия не открылась, — UserID и model.ErrSessionNotOpened.
func (s *AuthService) Register(ctx context.Context, in model.RegisterInput) (model.AuthResult, error) {
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	photos, err := s.savePhotos(ctx, in.Photos)
	if err != nil {
		return model.AuthResult{}, err
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
			AboutMe:       optionalAboutMe(in.AboutMe),
			Sex:           in.Sex,
			SearchSex:     in.SearchSex,
			SearchAgeFrom: in.SearchAgeFrom,
			SearchAgeTo:   in.SearchAgeTo,
		},
		in.Tags,
		photos,
	)
	if err != nil {
		s.deletePhotos(ctx, photos)
		return model.AuthResult{}, fmt.Errorf("create user: %w", err)
	}

	tokens, err := s.openSession(ctx, userID)
	if err != nil {
		return model.AuthResult{UserID: userID}, fmt.Errorf("register: %w: %w", model.ErrSessionNotOpened, err)
	}

	// Психотест при регистрации ещё не пройден
	return model.AuthResult{UserID: userID, ProfileCompleted: false, Tokens: tokens}, nil
}

// IsEmailAvailable проверяет, свободен ли адрес для регистрации.
// Принимает: контекст ctx и адрес email.
// Возвращает: true, если пользователь не найден, иначе false; при сбое поиска — ошибку.
func (s *AuthService) IsEmailAvailable(ctx context.Context, email string) (bool, error) {
	_, err := s.users.GetUserByEmail(ctx, email)
	if errors.Is(err, model.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}
	return false, nil
}

// Login проверяет пароль пользователя и открывает новую сессию.
// Принимает: контекст ctx, адрес email и пароль password.
// Возвращает: данные авторизации или ошибку; при неверном email или пароле — model.ErrInvalidCredentials.
func (s *AuthService) Login(ctx context.Context, email, password string) (model.AuthResult, error) {
	user, err := s.users.GetUserByEmail(ctx, email)
	if errors.Is(err, model.ErrNotFound) {
		// Всё равно считаем bcrypt, чтобы по времени ответа нельзя было
		// понять, зарегистрирован ли email
		s.matchDummy(password)
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

	completed, err := s.profiles.IsProfileCompleted(ctx, user.ID)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("check profile: %w", err)
	}

	tokens, err := s.openSession(ctx, user.ID)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("login: open session: %w", err)
	}

	return model.AuthResult{UserID: user.ID, ProfileCompleted: completed, Tokens: tokens}, nil
}

// Logout отзывает сессию по refresh-токену.
// Принимает: контекст ctx и токен refreshToken.
// Возвращает: nil при успехе, пустом токене, отсутствующей или уже отозванной сессии; иначе ошибку.
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

// Refresh отзывает действующую сессию и создаёт новую пару токенов.
// Принимает: контекст ctx и токен refreshToken.
// Возвращает: новые access- и refresh-токены со сроками действия либо ошибку; недействующая сессия — model.ErrInvalidSession.
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

	revoked, err := s.sessions.RevokeIfActive(ctx, session.ID)
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
// Принимает: контекст ctx и ID пользователя userID.
// Возвращает: пару токенов со сроками действия или ошибку генерации, сохранения либо выпуска.
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

// savePhotos сохраняет файлы фотографий, назначая позиции начиная с 1.
// Принимает: контекст ctx и загруженные фотографии uploads.
// Возвращает: метаданные сохранённых фото или ошибку; при сбое пытается удалить уже сохранённые файлы.
func (s *AuthService) savePhotos(ctx context.Context, uploads []model.PhotoUpload) ([]model.PhotoInput, error) {
	photos := make([]model.PhotoInput, 0, len(uploads))
	for i, upload := range uploads {
		key, err := s.photos.Save(ctx, upload.Data, upload.Ext)
		if err != nil {
			s.deletePhotos(ctx, photos)
			return nil, fmt.Errorf("save photo %d: %w", i+1, err)
		}
		photos = append(photos, model.PhotoInput{StorageKey: key, Position: i + 1})
	}
	return photos, nil
}

// deletePhotos пытается удалить файлы фотографий, игнорируя отмену исходного контекста.
// Принимает: контекст ctx и метаданные photos с ключами файлов.
// Возвращает: ничего; ошибки удаления записывает в журнал.
func (s *AuthService) deletePhotos(ctx context.Context, photos []model.PhotoInput) {
	ctx = context.WithoutCancel(ctx)
	for _, photo := range photos {
		if err := s.photos.Delete(ctx, photo.StorageKey); err != nil {
			slog.Error("delete orphan photo", "key", photo.StorageKey, "error", err)
		}
	}
}

// matchDummy выполняет сравнение с фиктивным хешем, чтобы уменьшить различия во времени входа.
// Принимает: проверяемый пароль password.
// Возвращает: ничего; результат сравнения и ошибки игнорирует.
func (s *AuthService) matchDummy(password string) {
	s.dummyHashOnce.Do(func() {
		s.dummyHash, _ = s.hasher.Hash("dummy-password-1")
	})
	_, _ = s.hasher.Matches(s.dummyHash, password)
}

// optionalAboutMe преобразует текст описания в необязательное значение.
// Принимает: строку value.
// Возвращает: nil для пустой строки, иначе указатель на исходную строку.
func optionalAboutMe(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
