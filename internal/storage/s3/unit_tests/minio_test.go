package s3_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"dating-app/internal/config"
	"dating-app/internal/service"
	"dating-app/internal/storage/s3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var _ service.PhotoURLProvider = (*s3.MinIOURLProvider)(nil)

func TestOpenMinIOStorage(t *testing.T) {
	for _, secure := range []bool{false, true} {
		storage, err := s3.OpenMinIOStorage(config.MinIOConfig{Endpoint: "storage.example:9000", AccessKey: "test-access", SecretKey: "test-secret", Bucket: "photos", UseTLS: secure})
		if err != nil {
			t.Fatal(err)
		}
		wantScheme := "http"
		if secure {
			wantScheme = "https"
		}
		if storage.Bucket != "photos" || storage.Client.EndpointURL().Host != "storage.example:9000" || storage.Client.EndpointURL().Scheme != wantScheme {
			t.Fatalf("unexpected storage: bucket=%q endpoint=%s", storage.Bucket, storage.Client.EndpointURL())
		}
	}
	if storage, err := s3.OpenMinIOStorage(config.MinIOConfig{Endpoint: "https://storage.example/path"}); err == nil || storage != nil {
		t.Fatalf("invalid endpoint: storage=%v err=%v", storage, err)
	}
}

func TestMinIOURLProvider(t *testing.T) {
	// A fixed region avoids bucket-location requests; these tests need no MinIO server.
	client, err := minio.New("storage.example:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("test-access", "test-secret", ""),
		Secure: true, Region: "us-east-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, bucket, key string
		ttl               time.Duration
		wantErr           bool
	}{
		{"signed photo", "photos", "uploads/photo.jpg", 15 * time.Minute, false},
		{"escaped key", "photos", "uploads/моё фото #1.jpg", time.Minute, false},
		{"empty key", "photos", "", time.Minute, true},
		{"invalid bucket", "bad bucket", "photo.jpg", time.Minute, true},
		{"zero TTL", "photos", "photo.jpg", 0, true},
		{"negative TTL", "photos", "photo.jpg", -time.Second, true},
		{"TTL over seven days", "photos", "photo.jpg", 8 * 24 * time.Hour, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := s3.NewMinIOURLProvider(&s3.MinIOStorage{Client: client, Bucket: tc.bucket}, tc.ttl)
			raw, err := provider.GetPhotoURL(context.Background(), tc.key)
			if tc.wantErr {
				if err == nil || raw != "" {
					t.Fatalf("URL=%q err=%v", raw, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if u.Scheme != "https" || u.Host != "storage.example:9000" || u.Path != "/"+tc.bucket+"/"+tc.key {
				t.Fatalf("unexpected URL: %s", u)
			}
			q := u.Query()
			if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" || len(q.Get("X-Amz-Signature")) != 64 || !strings.HasPrefix(q.Get("X-Amz-Credential"), "test-access/") {
				t.Fatalf("missing signing parameters: %v", q)
			}
			expiry, err := time.ParseDuration(q.Get("X-Amz-Expires") + "s")
			if err != nil || expiry != tc.ttl {
				t.Fatalf("expiry=%v err=%v", expiry, err)
			}
			if strings.Contains(raw, "test-secret") {
				t.Fatal("secret key leaked in URL")
			}
		})
	}
}
