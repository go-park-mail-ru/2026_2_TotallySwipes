package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"dating-app/internal/auth"
	"dating-app/internal/model"
)

type fakeUsers struct {
	created     *model.UserInput
	createdPV   *model.ProfileVersionInput
	createdTags []string
	createID    int64
	createErr   error

	byEmail map[string]*model.User
	getErr  error
}

func (f *fakeUsers) CreateUserWithProfile(_ context.Context, u *model.UserInput, pv *model.ProfileVersionInput, tags []string) (int64, error) {
	f.created, f.createdPV, f.createdTags = u, pv, tags
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

type fakeProfiles struct{ completed bool }

func (f fakeProfiles) IsProfileCompleted(context.Context, int64) (bool, error) {
	return f.completed, nil
}

type fakeSessions struct {
	byHash    map[string]*model.Session
	revoked   []string
	createErr error
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

func (f *fakeSessions) Revoke(_ context.Context, id string) error {
	f.revoked = append(f.revoked, id)
	return nil
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

func newTestService(users *fakeUsers, sessions *fakeSessions, completed bool) *AuthService {
	return NewAuthService(users, fakeProfiles{completed}, sessions, fakeHasher{}, fakeIssuer{}, time.Hour)
}

func validRegisterInput() RegisterInput {
	return RegisterInput{
		Name:          "alex",
		Email:         "alex@example.com",
		Password:      "qwerty123",
		BirthDate:     time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		Sex:           model.SexMale,
		SearchSex:     model.SearchSexFemale,
		DatingGoal:    model.DatingGoalRelationship,
		AboutMe:       "hi",
		SearchAgeFrom: 18,
		SearchAgeTo:   30,
		Tags:          []string{"sport"},
	}
}

// checkSession проверяет, что выданный refresh-токен соответствует сохранённой сессии
func checkSession(t *testing.T, sessions *fakeSessions, res AuthResult, userID int64) {
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
	res, err := newTestService(users, sessions, true).Register(context.Background(), validRegisterInput())
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if res.UserID != 12 || res.ProfileCompleted {
		t.Errorf("result = %+v", res)
	}
	if users.created.PasswordHash != "hash:qwerty123" || users.created.Email != "alex@example.com" {
		t.Errorf("user input = %+v", users.created)
	}
	pv := users.createdPV
	if pv.DatingGoal != model.DatingGoalRelationship || pv.SearchAgeTo != 30 || pv.AboutMe != "hi" || !pv.BirthDate.Equal(users.created.BirthDate) {
		t.Errorf("profile input = %+v", pv)
	}
	checkSession(t, sessions, res, 12)
}

func TestRegister_Errors(t *testing.T) {
	ctx := context.Background()

	_, err := newTestService(&fakeUsers{createErr: model.ErrEmailAlreadyExists}, newFakeSessions(), false).
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
	if _, err := newTestService(&fakeUsers{createID: 1}, sessions, false).Register(ctx, validRegisterInput()); err == nil {
		t.Error("session error must be returned")
	}
}

func TestLogin(t *testing.T) {
	users := &fakeUsers{byEmail: map[string]*model.User{
		"alex@example.com": {ID: 7, Email: "alex@example.com", PasswordHash: "hash:qwerty123"},
	}}
	ctx := context.Background()

	sessions := newFakeSessions()
	res, err := newTestService(users, sessions, true).Login(ctx, "alex@example.com", "qwerty123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.UserID != 7 || !res.ProfileCompleted {
		t.Errorf("result = %+v", res)
	}
	checkSession(t, sessions, res, 7)

	for name, tc := range map[string][2]string{
		"wrong password": {"alex@example.com", "wrong"},
		"unknown email":  {"nobody@example.com", "qwerty123"},
	} {
		sessions := newFakeSessions()
		_, err := newTestService(users, sessions, false).Login(ctx, tc[0], tc[1])
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s: err = %v", name, err)
		}
		if len(sessions.byHash) != 0 {
			t.Errorf("%s: session must not be created", name)
		}
	}

	dbErr := errors.New("db down")
	if _, err := newTestService(&fakeUsers{getErr: dbErr}, newFakeSessions(), false).Login(ctx, "a@b.ru", "x"); !errors.Is(err, dbErr) || errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("db error must not look like invalid credentials: %v", err)
	}
}

func TestLogout(t *testing.T) {
	ctx := context.Background()
	sessions := newFakeSessions()
	sessions.byHash[auth.HashToken("refresh")] = &model.Session{ID: "sid", UserID: 1}
	svc := newTestService(&fakeUsers{}, sessions, false)

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
