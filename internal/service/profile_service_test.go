package service

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestProfileServiceGetNextFeed(t *testing.T) {
	// Классы: невалидные аргументы, ошибки зависимостей, пустая/непустая
	// лента, отсутствие/неполнота теста, граничные и промежуточные оценки.
	type testCase struct {
		name                                          string
		userID                                        int64
		limit                                         int
		cursor                                        *int64
		viewer                                        *model.Profile
		profiles                                      []model.Profile
		nextCursor                                    *int64
		viewerErr, feedErr, calcErr                   error
		scores                                        []float64
		wantErr                                       bool
		wantViewerCalls, wantFeedCalls, wantCalcCalls int
	}
	value := 0.5
	complete := &model.ProfilePsycho{Openness: &value, Conscientiousness: &value, Extraversion: &value, Agreeableness: &value, Neuroticism: &value}
	incomplete := &model.ProfilePsycho{Openness: &value}
	viewer := &model.Profile{ID: 99, UserID: 7, CurrentPsycho: complete}
	candidate := model.Profile{UserID: 20, Name: "Anna", CurrentPsycho: complete}
	noTest := model.Profile{UserID: 21, Name: "Alex"}
	partial := model.Profile{UserID: 22, CurrentPsycho: incomplete}
	zero, negative, cursor, next := int64(0), int64(-1), int64(10), int64(20)
	failure := errors.New("dependency failure")
	tests := []testCase{
		{name: "zero user ID", limit: 10, wantErr: true},
		{name: "negative user ID", userID: -1, limit: 10, wantErr: true},
		{name: "zero limit", userID: 7, wantErr: true},
		{name: "negative limit", userID: 7, limit: -1, wantErr: true},
		{name: "zero cursor", userID: 7, limit: 10, cursor: &zero, wantErr: true},
		{name: "negative cursor", userID: 7, limit: 10, cursor: &negative, wantErr: true},
		{name: "viewer lookup failure", userID: 7, limit: 10, viewerErr: failure, wantErr: true, wantViewerCalls: 1},
		{name: "nil viewer", userID: 7, limit: 10, wantErr: true, wantViewerCalls: 1},
		{name: "feed lookup failure", userID: 7, limit: 10, viewer: viewer, feedErr: failure, wantErr: true, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "empty feed and minimum limit", userID: 7, limit: 1, viewer: viewer, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "viewer without test", userID: 7, limit: 10, viewer: &model.Profile{UserID: 7}, profiles: []model.Profile{candidate}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "incomplete viewer test", userID: 7, limit: 10, viewer: &model.Profile{CurrentPsycho: incomplete}, profiles: []model.Profile{candidate}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "candidate without test", userID: 7, limit: 10, viewer: viewer, profiles: []model.Profile{noTest}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "both without test", userID: 7, limit: 10, viewer: &model.Profile{}, profiles: []model.Profile{noTest}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "incomplete candidate test", userID: 7, limit: 10, viewer: viewer, profiles: []model.Profile{partial}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "valid zero", userID: 7, limit: 10, viewer: viewer, profiles: []model.Profile{candidate}, scores: []float64{0}, wantViewerCalls: 1, wantFeedCalls: 1, wantCalcCalls: 1},
		{name: "maximum score", userID: 7, limit: 10, viewer: viewer, profiles: []model.Profile{candidate}, scores: []float64{1}, wantViewerCalls: 1, wantFeedCalls: 1, wantCalcCalls: 1},
		{name: "mixed page with cursor", userID: 7, limit: 10, cursor: &cursor, viewer: viewer, profiles: []model.Profile{candidate, noTest, partial, candidate}, nextCursor: &next, scores: []float64{0.82, 0.35}, wantViewerCalls: 1, wantFeedCalls: 1, wantCalcCalls: 2},
		{name: "calculation failure", userID: 7, limit: 10, viewer: viewer, profiles: []model.Profile{candidate}, calcErr: failure, wantErr: true, wantViewerCalls: 1, wantFeedCalls: 1, wantCalcCalls: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := []string{}
			repo := &feedRepositoryMock{t: t}
			repo.viewer = func(gotCtx context.Context, id int64) (*model.Profile, error) {
				calls = append(calls, "viewer")
				if gotCtx != ctx || id != tc.userID {
					t.Fatalf("unexpected viewer arguments: ctx=%v, id=%d", gotCtx, id)
				}
				return tc.viewer, tc.viewerErr
			}
			repo.feed = func(gotCtx context.Context, id int64, limit int, cursor *int64) ([]model.Profile, *int64, error) {
				calls = append(calls, "feed")
				if gotCtx != ctx || id != tc.userID || limit != tc.limit || !reflect.DeepEqual(cursor, tc.cursor) {
					t.Fatal("unexpected feed arguments")
				}
				return tc.profiles, tc.nextCursor, tc.feedErr
			}
			calc := &feedCompatibilityMock{t: t}
			calc.calculate = func(a, b model.BigFive) (float64, error) {
				calls = append(calls, "calculate")
				expected := model.BigFive{Openness: &value, Conscientiousness: &value, Extraversion: &value, Agreeableness: &value, Neuroticism: &value}
				if !reflect.DeepEqual(a, expected) || !reflect.DeepEqual(b, expected) {
					t.Fatal("incorrect Big Five arguments")
				}
				if tc.calcErr != nil {
					return 0, tc.calcErr
				}
				if calc.calls > len(tc.scores) {
					t.Fatal("unexpected calculation call")
				}
				return tc.scores[calc.calls-1], nil
			}
			svc := NewProfileService(repo, calc, LocalPhotoURLProvider{})
			page, err := svc.GetNextFeed(ctx, tc.userID, tc.limit, tc.cursor)
			if repo.viewerCalls != tc.wantViewerCalls || repo.feedCalls != tc.wantFeedCalls || calc.calls != tc.wantCalcCalls {
				t.Fatalf("calls: viewer=%d feed=%d calc=%d", repo.viewerCalls, repo.feedCalls, calc.calls)
			}
			expectedCalls := []string{}
			if tc.wantViewerCalls > 0 {
				expectedCalls = append(expectedCalls, "viewer")
			}
			if tc.wantFeedCalls > 0 {
				expectedCalls = append(expectedCalls, "feed")
			}
			for range tc.wantCalcCalls {
				expectedCalls = append(expectedCalls, "calculate")
			}
			if !reflect.DeepEqual(calls, expectedCalls) {
				t.Fatalf("call order=%v, want=%v", calls, expectedCalls)
			}
			if tc.wantErr {
				if err == nil || page != nil {
					t.Fatalf("page=%v error=%v; want nil page and error", page, err)
				}
				for _, cause := range []error{tc.viewerErr, tc.feedErr, tc.calcErr} {
					if cause != nil && !errors.Is(err, cause) {
						t.Fatalf("lost error cause: %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if page == nil || page.Items == nil || len(page.Items) != len(tc.profiles) || !reflect.DeepEqual(page.NextCursor, tc.nextCursor) {
				t.Fatalf("unexpected page: %+v", page)
			}
			scoreIndex := 0
			for i, item := range page.Items {
				if item.UserID != tc.profiles[i].UserID || item.Name != tc.profiles[i].Name || item.Tags == nil || item.Photos == nil {
					t.Fatalf("incorrect item: %+v", item)
				}
				hasScore := tc.wantCalcCalls > 0 && tc.profiles[i].CurrentPsycho == complete
				if !hasScore {
					if item.Compatibility != nil {
						t.Fatal("missing test must give nil score")
					}
					continue
				}
				if item.Compatibility == nil || *item.Compatibility != tc.scores[scoreIndex] {
					t.Fatalf("incorrect score: %v", item.Compatibility)
				}
				scoreIndex++
			}
		})
	}
}

type feedRepositoryMock struct {
	ProfileRepository      // Unexpected unused interface methods panic rather than silently succeed.
	t                      *testing.T
	viewer                 func(context.Context, int64) (*model.Profile, error)
	feed                   func(context.Context, int64, int, *int64) ([]model.Profile, *int64, error)
	viewerCalls, feedCalls int
}

func (m *feedRepositoryMock) GetByUserIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error) {
	m.t.Helper()
	m.viewerCalls++
	if m.viewer == nil {
		m.t.Fatal("unexpected viewer lookup")
	}
	return m.viewer(ctx, id)
}
func (m *feedRepositoryMock) GetProfilesByCursorAndLimit(ctx context.Context, id int64, limit int, cursor *int64) ([]model.Profile, *int64, error) {
	m.t.Helper()
	m.feedCalls++
	if m.feed == nil {
		m.t.Fatal("unexpected feed lookup")
	}
	return m.feed(ctx, id, limit, cursor)
}

type feedCompatibilityMock struct {
	CompatibilityService
	t         *testing.T
	calculate func(model.BigFive, model.BigFive) (float64, error)
	calls     int
}

func (m *feedCompatibilityMock) CalculateDistance(a, b model.BigFive) (float64, error) {
	m.t.Helper()
	m.calls++
	if m.calculate == nil {
		m.t.Fatal("unexpected compatibility calculation")
	}
	return m.calculate(a, b)
}

func TestCalculateAge(t *testing.T) {
	type testCase struct {
		name         string
		birth, today time.Time
		want         int
	}
	date := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	tests := []testCase{
		{"before birthday", date(2000, 6, 15), date(2026, 6, 14), 25},
		{"on birthday", date(2000, 6, 15), date(2026, 6, 15), 26},
		{"after birthday", date(2000, 6, 15), date(2026, 6, 16), 26},
		{"previous month", date(2000, 6, 15), date(2026, 5, 20), 25},
		{"next month", date(2000, 6, 15), date(2026, 7, 1), 26},
		{"new year", date(2000, 12, 31), date(2026, 1, 1), 25},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := calculateAge(tc.birth, tc.today); got != tc.want {
				t.Fatalf("age=%d, want=%d", got, tc.want)
			}
		})
	}
}

func TestProfileServiceFeedItemMapping(t *testing.T) {
	type testCase struct {
		name       string
		about      *string
		tags       []model.Tag
		photos     []model.Photo
		wantTags   []string
		wantPhotos []model.FeedPhoto
	}
	about := "Люблю путешествия"
	tests := []testCase{
		{name: "empty optional fields", wantTags: []string{}, wantPhotos: []model.FeedPhoto{}},
		{name: "populated optional fields", about: &about,
			tags:       []model.Tag{{ID: 4, Name: "music"}, {ID: 9, Name: "sport"}},
			photos:     []model.Photo{{ID: 8, StorageKey: "first.jpg", Position: 0}, {ID: 3, StorageKey: "second.jpg", Position: 1}},
			wantTags:   []string{"music", "sport"},
			wantPhotos: []model.FeedPhoto{{ID: 8, URL: "https://media.example/uploads/first.jpg"}, {ID: 3, URL: "https://media.example/uploads/second.jpg"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Берём дату в середине текущего года жизни, чтобы тест не зависел от полуночи дня рождения.
			birth := time.Now().AddDate(-25, -6, 0)
			candidate := model.Profile{UserID: 42, Name: "Anna", CurrentVersion: model.ProfileVersion{BirthDate: birth, AboutMe: tc.about}, Tags: tc.tags, Photos: tc.photos}
			repo := &feedRepositoryMock{t: t,
				viewer: func(context.Context, int64) (*model.Profile, error) { return &model.Profile{}, nil },
				feed: func(context.Context, int64, int, *int64) ([]model.Profile, *int64, error) {
					return []model.Profile{candidate}, nil, nil
				},
			}
			calc := &feedCompatibilityMock{t: t}
			svc := NewProfileService(repo, calc, LocalPhotoURLProvider{baseURL: "https://media.example"})
			page, err := svc.GetNextFeed(context.Background(), 7, 10, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := model.FeedItem{UserID: 42, Name: "Anna", Age: 25, AboutMe: tc.about, Tags: tc.wantTags, Photos: tc.wantPhotos}
			if page == nil || len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0], want) {
				t.Fatalf("page=%+v, want item=%+v", page, want)
			}
			if repo.viewerCalls != 1 || repo.feedCalls != 1 || calc.calls != 0 {
				t.Fatal("unexpected dependency calls")
			}
		})
	}
}
