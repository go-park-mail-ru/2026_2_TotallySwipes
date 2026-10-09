package service

import (
	"context"
	. "dating-app/internal/service"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"dating-app/internal/auth"
	"dating-app/internal/model"
)

type fakeUsers struct {
	created   *model.UserInput
	createID  int64
	createErr error

	byEmail map[string]*model.User
	getErr  error
}

func (f *fakeUsers) CreateUser(_ context.Context, u *model.UserInput) (int64, error) {
	f.created = u
	return f.createID, f.createErr
}

func (f *fakeUsers) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, model.ErrNotFound
}

type fakePhotos struct {
	saved    map[string][]byte
	deleted  []string
	saveErrN int
	n        int
}

func (f *fakePhotos) Save(_ context.Context, data []byte, ext string) (string, error) {
	f.n++
	if f.n == f.saveErrN {
		return "", errors.New("disk full")
	}
	if f.saved == nil {
		f.saved = map[string][]byte{}
	}
	key := fmt.Sprintf("uploads/%d%s", f.n, ext)
	f.saved[key] = data
	return key, nil
}

func (f *fakePhotos) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

type fakeProfiles struct{ missing []string }

func (f fakeProfiles) Missing(context.Context, int64) ([]string, error) {
	return f.missing, nil
}

type fakeSessions struct {
	byHash    map[string]*model.Session
	revoked   []string
	createErr error
	revokeErr error
}

func newFakeSessions() *fakeSessions { return &fakeSessions{byHash: map[string]*model.Session{}} }

func (f *fakeSessions) Create(_ context.Context, s *model.Session) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.byHash[s.TokenHash] = s
	return nil
}

func (f *fakeSessions) GetByTokenHash(_ context.Context, hash string) (*model.Session, error) {
	if s, ok := f.byHash[hash]; ok {
		return s, nil
	}
	return nil, model.ErrNotFound
}

func (f *fakeSessions) Revoke(_ context.Context, id string) (bool, error) {
	if f.revokeErr != nil {
		return false, f.revokeErr
	}
	for _, s := range f.byHash {
		if s.ID == id && !s.Revoked {
			s.Revoked = true
			f.revoked = append(f.revoked, id)
			return true, nil
		}
	}
	return false, nil
}

// fakeHasher: "hash:<пароль>"
type fakeHasher struct{ hashErr error }

func (f fakeHasher) Hash(p string) (string, error) {
	if f.hashErr != nil {
		return "", f.hashErr
	}
	return "hash:" + p, nil
}

func (fakeHasher) Matches(hash, p string) (bool, error) { return hash == "hash:"+p, nil }

type fakeIssuer struct{}

func (fakeIssuer) Issue(userID int64, sessionID string) (string, time.Time, error) {
	return "access:" + sessionID, time.Now().Add(time.Minute), nil
}

func newTestService(users *fakeUsers, sessions *fakeSessions, missing ...string) *AuthService {
	return NewAuthService(users, fakeProfiles{missing}, sessions, fakeHasher{}, fakeIssuer{}, time.Hour)
}

func validRegisterInput() model.RegisterInput {
	return model.RegisterInput{Email: "alex@example.com", Password: "qwerty123"}
}

var allMissing = []string{"name", "birth_date", "sex", "dating_goal", "search_sex", "search_age", "photos"}

// checkSession проверяет, что выданный refresh-токен соответствует сохранённой сессии
func checkSession(t *testing.T, sessions *fakeSessions, res model.AuthResult, userID int64) {
	t.Helper()
	s, ok := sessions.byHash[auth.HashToken(res.Tokens.Refresh)]
	if !ok {
		t.Fatal("session for issued refresh token not stored")
	}
	if s.UserID != userID || s.TokenHash == res.Tokens.Refresh {
		t.Errorf("session = %+v", s)
	}
	if res.Tokens.Access != "access:"+s.ID {
		t.Errorf("access token must reference session %s, got %q", s.ID, res.Tokens.Access)
	}
	if d := time.Until(res.Tokens.RefreshExpiresAt); d < 59*time.Minute || d > time.Hour {
		t.Errorf("refresh expires in %v, want ~1h", d)
	}
}

func TestRegister_OK(t *testing.T) {
	users := &fakeUsers{createID: 12}
	sessions := newFakeSessions()
	res, err := newTestService(users, sessions).Register(context.Background(), validRegisterInput())
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if res.UserID != 12 || !reflect.DeepEqual(res.Missing, allMissing) {
		t.Errorf("result = %+v", res)
	}
	if users.created.PasswordHash != "hash:qwerty123" || users.created.Email != "alex@example.com" {
		t.Errorf("user input = %+v", users.created)
	}
	checkSession(t, sessions, res, 12)
}

func TestRegister_Errors(t *testing.T) {
	ctx := context.Background()

	_, err := newTestService(&fakeUsers{createErr: model.ErrEmailAlreadyExists}, newFakeSessions()).
		Register(ctx, validRegisterInput())
	if !errors.Is(err, model.ErrEmailAlreadyExists) {
		t.Errorf("email taken: err = %v", err)
	}

	users := &fakeUsers{}
	svc := NewAuthService(users, fakeProfiles{}, newFakeSessions(), fakeHasher{hashErr: model.ErrPasswordTooLong}, fakeIssuer{}, time.Hour)
	if _, err := svc.Register(ctx, validRegisterInput()); !errors.Is(err, model.ErrPasswordTooLong) {
		t.Errorf("hash error: err = %v", err)
	}
	if users.created != nil {
		t.Error("user must not be created when hashing fails")
	}

	sessions := newFakeSessions()
	sessions.createErr = errors.New("redis down")
	res, err := newTestService(&fakeUsers{createID: 1}, sessions).Register(ctx, validRegisterInput())
	if !errors.Is(err, model.ErrSessionNotOpened) {
		t.Errorf("session error: err = %v, want model.ErrSessionNotOpened", err)
	}
	if res.UserID != 1 || res.Tokens != (model.Tokens{}) {
		t.Errorf("session error: result = %+v, want UserID only", res)
	}
}

func TestLogin(t *testing.T) {
	users := &fakeUsers{byEmail: map[string]*model.User{
		"alex@example.com": {ID: 7, Email: "alex@example.com", PasswordHash: "hash:qwerty123"},
	}}
	ctx := context.Background()

	sessions := newFakeSessions()
	res, err := newTestService(users, sessions, "photos").Login(ctx, "alex@example.com", "qwerty123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.UserID != 7 || !reflect.DeepEqual(res.Missing, []string{"photos"}) {
		t.Errorf("result = %+v", res)
	}
	checkSession(t, sessions, res, 7)

	for name, tc := range map[string][2]string{
		"wrong password": {"alex@example.com", "wrong"},
		"unknown email":  {"nobody@example.com", "qwerty123"},
	} {
		sessions := newFakeSessions()
		_, err := newTestService(users, sessions).Login(ctx, tc[0], tc[1])
		if !errors.Is(err, model.ErrInvalidCredentials) {
			t.Errorf("%s: err = %v", name, err)
		}
		if len(sessions.byHash) != 0 {
			t.Errorf("%s: session must not be created", name)
		}
	}

	dbErr := errors.New("db down")
	if _, err := newTestService(&fakeUsers{getErr: dbErr}, newFakeSessions()).Login(ctx, "a@b.ru", "x"); !errors.Is(err, dbErr) || errors.Is(err, model.ErrInvalidCredentials) {
		t.Errorf("db error must not look like invalid credentials: %v", err)
	}
}

func TestLogout(t *testing.T) {
	ctx := context.Background()
	sessions := newFakeSessions()
	sessions.byHash[auth.HashToken("refresh")] = &model.Session{ID: "sid", UserID: 1}
	svc := newTestService(&fakeUsers{}, sessions)

	if err := svc.Logout(ctx, "refresh"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if len(sessions.revoked) != 1 || sessions.revoked[0] != "sid" {
		t.Errorf("revoked = %v", sessions.revoked)
	}

	for _, token := range []string{"", "unknown"} {
		if err := svc.Logout(ctx, token); err != nil {
			t.Errorf("logout %q: %v", token, err)
		}
	}
	if len(sessions.revoked) != 1 {
		t.Errorf("nothing else must be revoked: %v", sessions.revoked)
	}
}

func TestRefresh_OK(t *testing.T) {
	ctx := context.Background()
	sessions := newFakeSessions()
	sessions.byHash[auth.HashToken("old")] = &model.Session{ID: "old-sid", UserID: 7, ExpiresAt: time.Now().Add(time.Hour)}
	svc := newTestService(&fakeUsers{}, sessions)

	tokens, err := svc.Refresh(ctx, "old")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(sessions.revoked) != 1 || sessions.revoked[0] != "old-sid" {
		t.Errorf("old session must be revoked, revoked = %v", sessions.revoked)
	}
	if tokens.Refresh == "old" {
		t.Error("refresh token must be rotated")
	}
	checkSession(t, sessions, model.AuthResult{Tokens: tokens}, 7)

	if _, err := svc.Refresh(ctx, "old"); !errors.Is(err, model.ErrInvalidSession) {
		t.Errorf("reuse: err = %v", err)
	}
	if _, err := svc.Refresh(ctx, tokens.Refresh); err != nil {
		t.Errorf("refresh with new token: %v", err)
	}
}

func TestRefresh_InvalidSession(t *testing.T) {
	ctx := context.Background()
	sessions := newFakeSessions()
	sessions.byHash[auth.HashToken("revoked")] = &model.Session{ID: "r", UserID: 1, ExpiresAt: time.Now().Add(time.Hour), Revoked: true}
	sessions.byHash[auth.HashToken("expired")] = &model.Session{ID: "e", UserID: 1, ExpiresAt: time.Now().Add(-time.Second)}
	svc := newTestService(&fakeUsers{}, sessions)

	for _, token := range []string{"", "unknown", "revoked", "expired"} {
		if _, err := svc.Refresh(ctx, token); !errors.Is(err, model.ErrInvalidSession) {
			t.Errorf("refresh %q: err = %v, want ErrInvalidSession", token, err)
		}
	}
	if len(sessions.revoked) != 0 {
		t.Errorf("nothing must be revoked: %v", sessions.revoked)
	}
}

func TestRefresh_StorageErrors(t *testing.T) {
	ctx := context.Background()
	redisErr := errors.New("redis down")

	sessions := newFakeSessions()
	sessions.byHash[auth.HashToken("t")] = &model.Session{ID: "sid", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}
	sessions.revokeErr = redisErr
	if _, err := newTestService(&fakeUsers{}, sessions).Refresh(ctx, "t"); !errors.Is(err, redisErr) || errors.Is(err, model.ErrInvalidSession) {
		t.Errorf("revoke error must not look like invalid session: %v", err)
	}

	sessions = newFakeSessions()
	sessions.byHash[auth.HashToken("t")] = &model.Session{ID: "sid", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}
	sessions.createErr = redisErr
	if _, err := newTestService(&fakeUsers{}, sessions).Refresh(ctx, "t"); !errors.Is(err, redisErr) {
		t.Errorf("create error: %v", err)
	}
}
