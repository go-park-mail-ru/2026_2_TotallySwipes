package config

import (
	"strings"
	"testing"
	"time"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("JWT_SECRET", "secret")
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Redis.Addr != "localhost:6379" || cfg.HTTP.Port != "8080" {
		t.Errorf("defaults: redis = %q, port = %q", cfg.Redis.Addr, cfg.HTTP.Port)
	}
	if cfg.Auth.JWTAccessTTL != 15*time.Minute || !cfg.Auth.CookieSecure {
		t.Errorf("auth defaults = %+v", cfg.Auth)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("want both required vars in error, got %v", err)
	}
}

func TestLoadInvalidValues(t *testing.T) {
	setRequired(t)
	t.Setenv("JWT_ACCESS_TTL", "0s")
	t.Setenv("JWT_REFRESH_TTL", "abc")
	t.Setenv("HTTP_IDLE_TIMEOUT", "-1s")
	t.Setenv("COOKIE_SECURE", "maybe")

	_, err := Load()
	if err == nil {
		t.Fatal("want error")
	}
	for _, key := range []string{"JWT_ACCESS_TTL", "JWT_REFRESH_TTL", "HTTP_IDLE_TIMEOUT", "COOKIE_SECURE"} {
		if strings.Count(err.Error(), key) != 1 {
			t.Errorf("%s must be reported exactly once, got %v", key, err)
		}
	}
}

func TestLoadZeroHTTPTimeoutAllowed(t *testing.T) {
	setRequired(t)
	t.Setenv("HTTP_READ_TIMEOUT", "0s")

	cfg, err := Load()
	if err != nil || cfg.HTTP.ReadTimeout != 0 {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}
