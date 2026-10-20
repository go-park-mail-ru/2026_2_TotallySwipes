package s3_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dating-app/internal/config"
	"dating-app/internal/service"
	"dating-app/internal/storage/s3"
)

var _ service.PhotoURLProvider = (*s3.MinIOURLProvider)(nil)

func TestOpenMinIOStorage(t *testing.T) {
	for _, tc := range []struct {
		name        string
		exists      bool
		checkDenied bool
		createCode  string
		wantErr     bool
		wantPuts    int32
	}{
		{name: "existing bucket", exists: true},
		{name: "create missing bucket", wantPuts: 1},
		{name: "concurrent creation", createCode: "BucketAlreadyOwnedByYou", wantPuts: 1},
		{name: "check denied", checkDenied: true, wantErr: true},
		{name: "creation denied", createCode: "AccessDenied", wantErr: true, wantPuts: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var puts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.TrimSuffix(r.URL.Path, "/") != "/photos" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.URL.Query().Has("location") {
					w.Header().Set("Content-Type", "application/xml")
					fmt.Fprint(w, `<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-east-1</LocationConstraint>`)
					return
				}
				switch r.Method {
				case http.MethodHead:
					if tc.checkDenied {
						w.WriteHeader(http.StatusForbidden)
					} else if !tc.exists {
						w.WriteHeader(http.StatusNotFound)
					}
				case http.MethodPut:
					puts.Add(1)
					if tc.createCode != "" {
						w.Header().Set("Content-Type", "application/xml")
						status := http.StatusForbidden
						if tc.createCode == "BucketAlreadyOwnedByYou" {
							status = http.StatusConflict
						}
						w.WriteHeader(status)
						fmt.Fprintf(w, "<Error><Code>%s</Code><BucketName>photos</BucketName></Error>", tc.createCode)
					}
				default:
					t.Errorf("unexpected method: %s", r.Method)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			storage, err := s3.OpenMinIOStorage(ctx, config.MinIOConfig{Endpoint: strings.TrimPrefix(server.URL, "http://"), AccessKey: "test-access", SecretKey: "test-secret", Bucket: "photos"})
			if (err != nil) != tc.wantErr {
				t.Fatalf("storage=%v err=%v", storage, err)
			}
			if tc.wantErr {
				if storage != nil {
					t.Fatal("storage must be nil on error")
				}
			} else if storage == nil || storage.Bucket != "photos" || storage.Client.EndpointURL().String() != server.URL {
				t.Fatalf("unexpected storage: %+v", storage)
			}
			if puts.Load() != tc.wantPuts {
				t.Fatalf("PUT calls=%d want=%d", puts.Load(), tc.wantPuts)
			}
		})
	}
}

func TestOpenMinIOStorageInvalidEndpoint(t *testing.T) {
	storage, err := s3.OpenMinIOStorage(context.Background(), config.MinIOConfig{Endpoint: "https://storage.example/path"})
	if err == nil || storage != nil {
		t.Fatalf("storage=%v err=%v", storage, err)
	}
}

func TestOpenMinIOStorageContext(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint("deadline=", deadline), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer server.Close()
			var ctx context.Context
			var cancel context.CancelFunc
			want := context.Canceled
			if deadline {
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				want = context.DeadlineExceeded
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			cancel()
			storage, err := s3.OpenMinIOStorage(ctx, config.MinIOConfig{Endpoint: strings.TrimPrefix(server.URL, "http://"), AccessKey: "test-access", SecretKey: "test-secret", Bucket: "photos"})
			if storage != nil || !errors.Is(err, want) {
				t.Fatalf("storage=%v err=%v want=%v", storage, err, want)
			}
			if requests.Load() != 0 {
				t.Fatalf("requests with expired context: %d", requests.Load())
			}
		})
	}
}

func TestMinIOURLProvider(t *testing.T) {
	// SDK запрашивает регион у публичного endpoint; реальный MinIO не нужен.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !r.URL.Query().Has("location") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-east-1</LocationConstraint>`)
	}))
	defer server.Close()
	endpoint := strings.TrimPrefix(server.URL, "http://")
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
			provider, err := s3.NewMinIOURLProvider(config.MinIOConfig{
				Endpoint: "internal.invalid:9000", PublicEndpoint: endpoint,
				AccessKey: "test-access", SecretKey: "test-secret",
				Bucket: tc.bucket, PhotoURLTTL: tc.ttl,
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			raw, err := provider.GetPhotoURL(ctx, tc.key)
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
			if u.Scheme != "http" || u.Host != endpoint || u.Path != "/"+tc.bucket+"/"+tc.key {
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

func TestNewMinIOURLProviderInvalidEndpoint(t *testing.T) {
	provider, err := s3.NewMinIOURLProvider(config.MinIOConfig{
		Endpoint: "minio:9000", PublicEndpoint: "https://storage.example/path",
	})
	if err == nil || provider != nil {
		t.Fatalf("provider=%v err=%v", provider, err)
	}
}
