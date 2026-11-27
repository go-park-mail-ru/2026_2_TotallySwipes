package auth

import (
	. "dating-app/gateway/internal/auth"
	"errors"
	"testing"
	"time"
)

func TestJWTIssuer_IssueAndParse(t *testing.T) {
	j := NewJWTIssuer("secret", 15*time.Minute)

	token, expiresAt, err := j.Issue(12, "sess")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if d := time.Until(expiresAt); d < 14*time.Minute || d > 15*time.Minute {
		t.Errorf("expiresAt in %v, want ~15m", d)
	}

	userID, sessionID, err := j.Parse(token)
	if err != nil || userID != 12 || sessionID != "sess" {
		t.Fatalf("parse = %d, %q, %v", userID, sessionID, err)
	}
}

func TestJWTIssuer_ParseRejects(t *testing.T) {
	j := NewJWTIssuer("secret", time.Minute)
	token, _, _ := j.Issue(12, "sess")

	other := NewJWTIssuer("other-secret", time.Minute)
	if _, _, err := other.Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("wrong secret: err = %v", err)
	}

	expired := NewJWTIssuer("secret", -time.Hour)
	old, _, _ := expired.Issue(12, "sess")
	if _, _, err := j.Parse(old); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expired: err = %v", err)
	}

	if _, _, err := j.Parse("garbage"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("garbage: err = %v", err)
	}
}

func TestRefreshTokens(t *testing.T) {
	a, _ := NewRefreshToken()
	b, _ := NewRefreshToken()
	if a == b || len(a) < 40 {
		t.Errorf("tokens must be random and long: %q %q", a, b)
	}
	if HashToken(a) == a || HashToken(a) != HashToken(a) || len(HashToken(a)) != 64 {
		t.Errorf("hash must be deterministic sha256 hex")
	}
	id, _ := NewSessionID()
	if len(id) != 32 {
		t.Errorf("session id = %q", id)
	}
}
