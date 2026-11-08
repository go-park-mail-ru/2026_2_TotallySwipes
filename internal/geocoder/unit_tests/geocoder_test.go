package geocoder

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dating-app/internal/geocoder"
	"dating-app/internal/model"
	"dating-app/internal/service"
)

var _ service.ReverseGeocoder = (*geocoder.DaData)(nil)

const testAPIKey = "test-key"

func newServer(t *testing.T, handler http.HandlerFunc) *geocoder.DaData {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return geocoder.NewDaData(srv.Client(), srv.URL, testAPIKey)
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

var moscow = &model.Coordinates{Latitude: 55.878, Longitude: 37.653}

func TestReverseSendsDaDataRequest(t *testing.T) {
	g := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Token "+testAPIKey {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("query must be empty so the key never lands in URLs/logs, got %q", r.URL.RawQuery)
		}

		var body map[string]float64
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["lat"] != moscow.Latitude || body["lon"] != moscow.Longitude {
			t.Errorf("coordinates = %v", body)
		}
		if body["radius_meters"] != 1000 || body["count"] != 10 {
			t.Errorf("radius/count = %v", body)
		}

		reply(http.StatusOK, `{"suggestions":[{"data":{"city":"Москва","city_district":"Бутырский"}}]}`)(w, r)
	})

	addr, err := g.Reverse(context.Background(), moscow)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if addr.City != "Москва" || addr.District == nil || *addr.District != "Бутырский" {
		t.Fatalf("address = %+v", addr)
	}
}

func TestReverseParsing(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantCity     string
		wantDistrict string // "" — района нет
		wantErr      error
	}{
		{
			name:     "district falls back to city_area",
			body:     `{"suggestions":[{"data":{"city":"Москва","city_area":"Северо-восточный"}}]}`,
			wantCity: "Москва", wantDistrict: "Северо-восточный",
		},
		{
			name:     "no district",
			body:     `{"suggestions":[{"data":{"city":"Казань"}}]}`,
			wantCity: "Казань",
		},
		{
			name:     "village takes nearest suggestion with city",
			body:     `{"suggestions":[{"data":{"city":""}},{"data":{"city":" Тула "}}]}`,
			wantCity: "Тула",
		},
		{
			name:     "federal city from region",
			body:     `{"suggestions":[{"data":{"region":"Севастополь","region_type":"г"}}]}`,
			wantCity: "Севастополь",
		},
		{
			name:    "nothing nearby",
			body:    `{"suggestions":[]}`,
			wantErr: model.ErrLocationNotFound,
		},
		{
			name:    "no city at all",
			body:    `{"suggestions":[{"data":{"region":"Тульская","region_type":"обл"}}]}`,
			wantErr: model.ErrCityNotFound,
		},
		{
			name:    "broken json",
			body:    `{"suggestions":`,
			wantErr: model.ErrGeocoderUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newServer(t, reply(http.StatusOK, tt.body))

			addr, err := g.Reverse(context.Background(), moscow)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Reverse: %v", err)
			}
			if addr.City != tt.wantCity {
				t.Errorf("city = %q, want %q", addr.City, tt.wantCity)
			}
			gotDistrict := ""
			if addr.District != nil {
				gotDistrict = *addr.District
			}
			if gotDistrict != tt.wantDistrict {
				t.Errorf("district = %q, want %q", gotDistrict, tt.wantDistrict)
			}
		})
	}
}

func TestReverseStatusErrors(t *testing.T) {
	tests := []struct {
		status  int
		wantErr error
	}{
		{http.StatusTooManyRequests, model.ErrGeocoderQuotaExceeded},
		{http.StatusForbidden, model.ErrGeocoderUnavailable},
		{http.StatusUnauthorized, model.ErrGeocoderUnavailable},
		{http.StatusInternalServerError, model.ErrGeocoderUnavailable},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			g := newServer(t, reply(tt.status, `{"message":"boom"}`))

			_, err := g.Reverse(context.Background(), moscow)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestReverseUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // порт закрыт — как в проде, когда стучались в несуществующий хост

	g := geocoder.NewDaData(&http.Client{}, url, testAPIKey)

	_, err := g.Reverse(context.Background(), moscow)
	if !errors.Is(err, model.ErrGeocoderUnavailable) {
		t.Fatalf("err = %v, want ErrGeocoderUnavailable", err)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error leaks API key: %v", err)
	}
}

func TestReverseTimeout(t *testing.T) {
	g := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := g.Reverse(ctx, moscow)
	if !errors.Is(err, model.ErrGeocoderTimeout) {
		t.Fatalf("err = %v, want ErrGeocoderTimeout", err)
	}
}
