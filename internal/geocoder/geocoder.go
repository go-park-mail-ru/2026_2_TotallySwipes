package geocoder

import (
	"context"
	"dating-app/internal/model"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type GeoQoder struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

type reverseResponse struct {
	Data []struct {
		Attributes struct {
			City     string `json:"city"`
			District string `json:"district"`
			Distance string `json:"distance"`
		} `json:"attributes"`
	} `json:"data"`
}

func NewGeoQoder(client *http.Client, baseURL, apiKey string) *GeoQoder {
	return &GeoQoder{client: client, baseURL: baseURL, apiKey: apiKey}
}

func (g *GeoQoder) Reverse(ctx context.Context, coordinates *model.Coordinates) (*model.ResolvedAddress, error) {

	u, err := url.Parse(g.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse geocoder URL: %w", err)
	}

	query := u.Query()
	query.Set("radius", "200")
	query.Set("lat", strconv.FormatFloat(coordinates.Latitude, 'f', -1, 64))
	query.Set("lng", strconv.FormatFloat(coordinates.Longitude, 'f', -1, 64))
	query.Set("token", g.apiKey)
	u.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create geocoder request: %w", err)
	}
	request.Header.Set("Authorization", g.apiKey)
	request.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(request)

	if err != nil {
		return nil, fmt.Errorf("call geocoder: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, model.ErrGeocoderQuotaExceeded
		}

		return nil, fmt.Errorf("geocoder returned status %d", resp.StatusCode)
	}

	return readBody(resp.Body)
}

func readBody(body io.ReadCloser) (*model.ResolvedAddress, error) {

	var data reverseResponse
	b := json.NewDecoder(body)

	if err := b.Decode(&data); err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}

	if len(data.Data) == 0 {
		return nil, model.ErrLocationNotFound
	}

	attrs := data.Data[0].Attributes
	city := strings.TrimSpace(attrs.City)
	if city == "" {
		return nil, model.ErrCityNotFound
	}

	address := &model.ResolvedAddress{
		City:     city,
		District: nil,
	}

	if district := strings.TrimSpace(attrs.District); district != "" {
		address.District = &district
	}

	return address, nil
}
