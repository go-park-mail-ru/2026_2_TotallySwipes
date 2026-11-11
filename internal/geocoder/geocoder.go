package geocoder

import (
	"bytes"
	"context"
	"dating-app/internal/model"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

const (
	searchRadiusMeters = 1000
	resultsCount       = 10
	errorBodyLimit     = 512
	federalCityType    = "г"
)

type DaData struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

type geolocateRequest struct {
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Count        int     `json:"count"`
	RadiusMeters int     `json:"radius_meters"`
}

type geolocateResponse struct {
	Suggestions []struct {
		Data struct {
			City         string `json:"city"`
			CityDistrict string `json:"city_district"`
			CityArea     string `json:"city_area"`
			Region       string `json:"region"`
			RegionType   string `json:"region_type"`
		} `json:"data"`
	} `json:"suggestions"`
}

func NewDaData(client *http.Client, baseURL, apiKey string) *DaData {
	return &DaData{client: client, baseURL: baseURL, apiKey: apiKey}
}

func (g *DaData) Reverse(ctx context.Context, coordinates *model.Coordinates) (*model.ResolvedAddress, error) {

	payload, err := json.Marshal(geolocateRequest{
		Lat:          coordinates.Latitude,
		Lon:          coordinates.Longitude,
		Count:        resultsCount,
		RadiusMeters: searchRadiusMeters,
	})

	if err != nil {
		return nil, fmt.Errorf("encode geocoder request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create geocoder request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Token "+g.apiKey)

	resp, err := g.client.Do(request)
	if err != nil {
		return nil, geocoderRequestError(ctx, "call geocoder", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("geocoder resp: %w", model.ErrGeocoderQuotaExceeded)
		}

		return nil, fmt.Errorf("geocoder resp: %w: %d", model.ErrGeocoderUnavailable, resp.StatusCode)
	}

	address, err := readBody(resp.Body)
	if err != nil {
		return nil, geocoderRequestError(ctx, "read geocoder body", err)
	}

	return address, nil
}

func readBody(body io.Reader) (*model.ResolvedAddress, error) {

	var data geolocateResponse
	if err := json.NewDecoder(body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}

	if len(data.Suggestions) == 0 {
		return nil, model.ErrLocationNotFound
	}

	for _, s := range data.Suggestions {

		city := strings.TrimSpace(s.Data.City)
		if city == "" && s.Data.RegionType == federalCityType {
			city = strings.TrimSpace(s.Data.Region)
		}

		if city == "" {
			continue
		}

		address := &model.ResolvedAddress{City: city}
		district := strings.TrimSpace(s.Data.CityDistrict)
		if district == "" {
			district = strings.TrimSpace(s.Data.CityArea)
		}

		if district != "" {
			address.District = &district
		}

		return address, nil
	}

	return nil, model.ErrCityNotFound
}

func geocoderRequestError(ctx context.Context, op string, err error) error {

	if errors.Is(ctx.Err(), context.Canceled) {
		return fmt.Errorf("%s: %w", op, err)
	}

	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("%s: %w: %w", op, model.ErrGeocoderTimeout, err)
	}

	if errors.Is(err, model.ErrGeocoderUnavailable) {
		return fmt.Errorf("%s: %w", op, err)
	}

	return fmt.Errorf("%s: %w: %w", op, model.ErrGeocoderUnavailable, err)
}
