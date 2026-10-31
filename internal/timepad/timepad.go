package timepad

import (
	"context"
	"dating-app/internal/model"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TimepadProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

type eventsResponse struct {
	Total  int            `json:"total"`
	Values []timepadEvent `json:"values"`
}

type timepadEvent struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	StartsAt string `json:"starts_at"`
	URL      string `json:"url"`

	PosterImage *struct {
		DefaultURL string `json:"default_url"`
	} `json:"poster_image"`

	Location *struct {
		City    string  `json:"city"`
		Address *string `json:"address"`
	} `json:"location"`

	Categories []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
}

func NewTimepadProvider(client *http.Client, baseURL string, apiKey string) *TimepadProvider {
	return &TimepadProvider{client: client, baseURL: baseURL, apiKey: apiKey}
}

func (p *TimepadProvider) GetEvents(ctx context.Context, eventQuery *model.EventQuery) (*model.EventPage, error) {
	u, err := url.Parse(p.baseURL)

	if err != nil {
		return nil, fmt.Errorf("timepad provider parse URL: %w", err)
	}

	q := u.Query()
	setQuery(&q, eventQuery)

	u.RawQuery = q.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create timepad provider request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+p.apiKey)
	request.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(request)
	if err != nil {
		return nil, timepadRequestError(ctx, "call timepad", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, model.ErrTimepadStatusCode
	}

	page, err := readBody(resp.Body)

	if err != nil {
		return nil, timepadRequestError(ctx, "read body timepad provider", err)
	}

	return page, nil
}

const timepadTimeLayout = "2006-01-02T15:04:05-0700"

func readBody(body io.Reader) (*model.EventPage, error) {

	var response eventsResponse
	decoder := json.NewDecoder(body)

	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode timepad events: %w", err)
	}

	events := make([]model.Event, 0, len(response.Values))

	for _, item := range response.Values {
		startsAt, err := time.Parse(timepadTimeLayout, item.StartsAt)

		if err != nil {
			continue
		}

		categories := make([]model.EventCategory, 0, len(item.Categories))
		for _, category := range item.Categories {
			categories = append(categories, model.EventCategory{
				ID:   category.ID,
				Name: category.Name,
			})
		}

		var imageURL *string
		if item.PosterImage != nil {
			imageURL = optionalString(item.PosterImage.DefaultURL)
		}

		var address *string
		if item.Location != nil {
			if item.Location.Address != nil {
				address = optionalString(*item.Location.Address)
			}
		}

		events = append(events, model.Event{
			ID:       item.ID,
			Name:     item.Name,
			ImageURL: imageURL,
			URL:      item.URL,
			StartsAt: startsAt,
			Location: model.EventLocation{
				City:    item.Location.City,
				Address: address,
			},
			Categories: categories,
		})

	}

	return &model.EventPage{
		Total:  response.Total,
		Events: events,
	}, nil
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func setQuery(query *url.Values, eventQuery *model.EventQuery) {

	if eventQuery.City != nil {
		query.Set("City", *eventQuery.City)
	}

	if eventQuery.Skip != nil {
		query.Set("Skip", strconv.Itoa(*eventQuery.Skip))
	}

	if eventQuery.Limit != nil {
		query.Set("Limit", strconv.Itoa(*eventQuery.Limit))
	}

	if eventQuery.CategoryIDs != nil {
		for _, id := range *eventQuery.CategoryIDs {
			query.Add("category_ids", strconv.FormatInt(id, 64))
		}
	}
}

// Only timeouts of this provider map to TIMEPAD_TIMEOUT; client cancellation does not.
func timepadRequestError(ctx context.Context, op string, err error) error {
	var netErr net.Error
	if !errors.Is(ctx.Err(), context.Canceled) &&
		(errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout())) {
		return fmt.Errorf("%s: %w: %w", op, model.ErrTimepadTimeout, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}
