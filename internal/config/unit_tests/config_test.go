package config

import (
	. "dating-app/internal/config"
	"reflect"
	"strings"
	"testing"
	"time"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("GEOCODER_API_KEY", "test-geocoder-key")
	t.Setenv("TIMEPAD_API_TOKEN", "test-timepad-token")
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Redis.Addr != "localhost:6379" || cfg.HTTP.Port != "8080" || cfg.HTTP.PublicURL != "http://localhost:8080" {
		t.Errorf("defaults: redis = %q, port = %q, public url = %q", cfg.Redis.Addr, cfg.HTTP.Port, cfg.HTTP.PublicURL)
	}
	if cfg.Auth.JWTAccessTTL != 15*time.Minute || !cfg.Auth.CookieSecure {
		t.Errorf("auth defaults = %+v", cfg.Auth)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("GEOCODER_API_KEY", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "JWT_SECRET") || !strings.Contains(err.Error(), "GEOCODER_API_KEY") {
		t.Fatalf("want all required vars in error, got %v", err)
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

func TestLoadPublicURL(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "9090")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.PublicURL != "http://localhost:9090" {
		t.Errorf("default must follow PORT, got %q", cfg.HTTP.PublicURL)
	}

	t.Setenv("PUBLIC_URL", "https://api.example.com")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.PublicURL != "https://api.example.com" {
		t.Errorf("public url = %q", cfg.HTTP.PublicURL)
	}
}

func TestLoadCORSOrigins(t *testing.T) {
	setRequired(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(cfg.HTTP.CORSOrigins, []string{
		"http://localhost:5173",
		"http://totally-swipes.ru",
		"https://totally-swipes.ru"}) {
		t.Errorf("default = %q", cfg.HTTP.CORSOrigins)
	}

	t.Setenv("CORS_ORIGINS", "http://localhost:5173, http://totally-swipes.ru, https://totally-swipes.ru")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(cfg.HTTP.CORSOrigins, []string{
		"http://localhost:5173",
		"http://totally-swipes.ru",
		"https://totally-swipes.ru"}) {
		t.Errorf("origins = %q", cfg.HTTP.CORSOrigins)
	}
}

func TestLoadMinIOPublicEndpoint(t *testing.T) {
	setRequired(t)
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinIO.PublicEndpoint != "localhost:9000" {
		t.Fatalf("default public endpoint=%q", cfg.MinIO.PublicEndpoint)
	}
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "192.0.2.10:9000")
	t.Setenv("MINIO_PHOTO_URL_TTL", "10m")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinIO.Endpoint != "minio:9000" || cfg.MinIO.PublicEndpoint != "192.0.2.10:9000" || cfg.MinIO.PhotoURLTTL != 10*time.Minute {
		t.Fatal("internal/public endpoints or photo TTL loaded incorrectly")
	}
}

func TestLoadMissingGeocoderAPIKey(t *testing.T) {
	setRequired(t)
	t.Setenv("GEOCODER_API_KEY", "")
	cfg, err := Load()
	if cfg != nil || err == nil || !strings.Contains(err.Error(), "GEOCODER_API_KEY") {
		t.Fatalf("want missing geocoder key error, got cfg=%+v, err=%v", cfg, err)
	}
}

func TestLoadTimepad(t *testing.T) {
	setRequired(t)
	t.Setenv("TIMEPAD_BASE_URL", "")
	t.Setenv("TIMEPAD_HTTP_CLIENT_TIMEOUT", "")
	t.Setenv("EVENT_SEARCH_TTL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timepad.BaseURL != "https://api.timepad.ru/v1/events.json" || cfg.Timepad.HTTPClientTimeout != 5*time.Second || cfg.Redis.EventRepositoryTTL != 24*time.Hour || cfg.Timepad.APIKey != "test-timepad-token" {
		t.Fatal("incorrect Timepad defaults")
	}
	t.Setenv("TIMEPAD_BASE_URL", "http://localhost:9001/events")
	t.Setenv("TIMEPAD_HTTP_CLIENT_TIMEOUT", "2s")
	t.Setenv("EVENT_SEARCH_TTL", "12h")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timepad.BaseURL != "http://localhost:9001/events" || cfg.Timepad.HTTPClientTimeout != 2*time.Second || cfg.Redis.EventRepositoryTTL != 12*time.Hour {
		t.Fatal("incorrect Timepad overrides")
	}
}

func TestLoadInvalidTimepad(t *testing.T) {
	setRequired(t)
	t.Setenv("TIMEPAD_API_TOKEN", "")
	t.Setenv("TIMEPAD_HTTP_CLIENT_TIMEOUT", "0s")
	t.Setenv("EVENT_SEARCH_TTL", "-1h")
	_, err := Load()
	if err == nil {
		t.Fatal("expected configuration error")
	}
	for _, key := range []string{"TIMEPAD_API_TOKEN", "TIMEPAD_HTTP_CLIENT_TIMEOUT", "EVENT_SEARCH_TTL"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("missing error for %s", key)
		}
	}
}
