package s3_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"dating-app/services/chat/internal/config"
	"dating-app/services/chat/internal/model"
	"dating-app/services/chat/internal/storage/s3"
)

func storageConfig(server *httptest.Server) config.MinIOConfig {
	return config.MinIOConfig{
		Endpoint:       strings.TrimPrefix(server.URL, "http://"),
		AccessKey:      "test-access",
		SecretKey:      "test-secret",
		Bucket:         "chat-media",
		DownloadURLTTL: 5 * time.Minute,
	}
}

func TestOpenMinIOStorage(t *testing.T) {
	for _, name := range []string{"существующий бакет", "создание", "конкурентное создание", "ошибка проверки", "ошибка создания"} {
		t.Run(name, func(t *testing.T) {
			puts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.TrimSuffix(r.URL.Path, "/") != "/chat-media" {
					t.Errorf("path = %s", r.URL.Path)
				}
				switch r.Method {
				case http.MethodHead:
					if name == "ошибка проверки" {
						w.WriteHeader(http.StatusForbidden)
					} else if name != "существующий бакет" {
						w.WriteHeader(http.StatusNotFound)
					}
				case http.MethodPut:
					puts++
					if name == "конкурентное создание" || name == "ошибка создания" {
						code, responseStatus := "AccessDenied", http.StatusForbidden
						if name == "конкурентное создание" {
							code, responseStatus = "BucketAlreadyOwnedByYou", http.StatusConflict
						}
						w.Header().Set("Content-Type", "application/xml")
						w.WriteHeader(responseStatus)
						fmt.Fprintf(w, "<Error><Code>%s</Code><BucketName>chat-media</BucketName></Error>", code)
					}
				default:
					t.Errorf("method = %s", r.Method)
				}
			}))
			defer server.Close()
			storage, err := s3.OpenMinIOStorage(context.Background(), storageConfig(server))
			wantErr := strings.HasPrefix(name, "ошибка")
			if (err != nil) != wantErr {
				t.Fatalf("storage = %v, error = %v", storage, err)
			}
			if !wantErr && (storage.Bucket != "chat-media" || storage.Client.EndpointURL().String() != server.URL) {
				t.Fatalf("storage = %#v", storage)
			}
			wantPuts := 1
			if name == "существующий бакет" || name == "ошибка проверки" {
				wantPuts = 0
			}
			if puts != wantPuts {
				t.Fatalf("PUT calls = %d, want %d", puts, wantPuts)
			}
		})
	}
}

func TestSaveDeleteAndDownloadURL(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, prefix string
		kind                      model.ChatMessageType
	}{
		{"голос", "audio/webm", "voice/", model.ChatMessageTypeVoice},
		{"видео", "video/mp4", "video_note/", model.ChatMessageTypeVideoNote},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := make(map[string]bool)
			deleted := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodHead:
					if strings.TrimSuffix(r.URL.Path, "/") != "/chat-media" {
						t.Errorf("HEAD path = %s", r.URL.Path)
					}
				case http.MethodPut:
					key := strings.TrimPrefix(r.URL.Path, "/chat-media/")
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					if !strings.HasPrefix(key, tc.prefix) || r.Header.Get("Content-Type") != tc.contentType || !strings.Contains(string(body), "media bytes") {
						t.Errorf("key = %s, content type = %s, body = %q", key, r.Header.Get("Content-Type"), body)
					}
					saved[key] = true
					w.Header().Set("ETag", `"etag"`)
				case http.MethodDelete:
					deleted = strings.TrimPrefix(r.URL.Path, "/chat-media/")
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("method = %s", r.Method)
				}
			}))
			defer server.Close()
			storage, err := s3.OpenMinIOStorage(context.Background(), storageConfig(server))
			if err != nil {
				t.Fatal(err)
			}
			key, err := storage.Save(context.Background(), []byte("media bytes"), tc.kind, tc.contentType)
			if err != nil || !saved[key] {
				t.Fatalf("key = %q, error = %v", key, err)
			}
			second, err := storage.Save(context.Background(), []byte("media bytes"), tc.kind, tc.contentType)
			if err != nil || second == key {
				t.Fatalf("second key = %q, error = %v", second, err)
			}
			rawURL, err := storage.GetPresignedDownloadURL(context.Background(), key)
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(rawURL)
			if err != nil {
				t.Fatal(err)
			}
			if u.Host != strings.TrimPrefix(server.URL, "http://") || u.Path != "/chat-media/"+key || u.Query().Get("X-Amz-Expires") != "300" || len(u.Query().Get("X-Amz-Signature")) != 64 || strings.Contains(rawURL, "test-secret") {
				t.Fatalf("URL = %s", rawURL)
			}
			if err := storage.Delete(context.Background(), key); err != nil {
				t.Fatal(err)
			}
			if deleted != key {
				t.Fatalf("deleted = %q, want %q", deleted, key)
			}
		})
	}
}

func TestInvalidMedia(t *testing.T) {
	// Неверные параметры должны отклоняться до обращения к SDK.
	storage := &s3.MinIOStorage{}
	for _, tc := range []struct {
		kind        model.ChatMessageType
		contentType string
		data        []byte
	}{
		{model.ChatMessageTypeText, "text/plain", []byte("data")},
		{model.ChatMessageTypeVoice, "video/mp4", []byte("data")},
		{model.ChatMessageTypeVideoNote, "audio/webm", []byte("data")},
		{model.ChatMessageTypeVoice, "invalid", []byte("data")},
		{model.ChatMessageTypeVoice, "audio/webm", nil},
	} {
		if key, err := storage.Save(context.Background(), tc.data, tc.kind, tc.contentType); err == nil || key != "" {
			t.Fatalf("key = %q, error = %v", key, err)
		}
	}
	for _, key := range []string{"", "voice/", "video_note/", "photos/image", "voice/../secret", "../secret", `voice\file`} {
		if err := storage.Delete(context.Background(), key); err == nil {
			t.Errorf("Delete accepted %q", key)
		}
		if _, err := storage.GetPresignedDownloadURL(context.Background(), key); err == nil {
			t.Errorf("GetPresignedDownloadURL accepted %q", key)
		}
	}
}

func TestStorageErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "<Error><Code>AccessDenied</Code></Error>")
	}))
	defer server.Close()
	storage, err := s3.OpenMinIOStorage(context.Background(), storageConfig(server))
	if err != nil {
		t.Fatal(err)
	}
	if key, err := storage.Save(context.Background(), []byte("data"), model.ChatMessageTypeVoice, "audio/ogg"); err == nil || key != "" {
		t.Fatalf("key = %q, error = %v", key, err)
	}
	if err := storage.Delete(context.Background(), "voice/key"); err == nil {
		t.Fatal("expected delete error")
	}
	for _, ttl := range []time.Duration{-time.Second, time.Millisecond, 8 * 24 * time.Hour} {
		cfg := storageConfig(server)
		cfg.DownloadURLTTL = ttl
		if _, err := s3.OpenMinIOStorage(context.Background(), cfg); err == nil {
			t.Errorf("accepted TTL %v", ttl)
		}
	}
}
