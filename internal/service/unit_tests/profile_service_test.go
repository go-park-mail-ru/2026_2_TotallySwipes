package service

import (
	"context"
	"dating-app/internal/model"
	. "dating-app/internal/service"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestProfileServiceGetNextFeed(t *testing.T) {
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
	viewer := filledProfile(model.Profile{ID: 99, UserID: 7, CurrentPsycho: complete}, "Viewer")
	candidate := filledProfile(model.Profile{UserID: 20, CurrentPsycho: complete}, "Anna")
	noTest := filledProfile(model.Profile{UserID: 21}, "Alex")
	partial := filledProfile(model.Profile{UserID: 22, CurrentPsycho: incomplete}, "Kate")
	viewerNoTest := filledProfile(model.Profile{UserID: 7}, "Viewer")
	viewerPartial := filledProfile(model.Profile{UserID: 7, CurrentPsycho: incomplete}, "Viewer")
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
		{name: "viewer with incomplete profile", userID: 7, limit: 10, viewer: &model.Profile{UserID: 7}, wantErr: true, wantViewerCalls: 1},
		{name: "feed lookup failure", userID: 7, limit: 10, viewer: &viewer, feedErr: failure, wantErr: true, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "empty feed and minimum limit", userID: 7, limit: 1, viewer: &viewer, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "viewer without test", userID: 7, limit: 10, viewer: &viewerNoTest, profiles: []model.Profile{candidate}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "incomplete viewer test", userID: 7, limit: 10, viewer: &viewerPartial, profiles: []model.Profile{candidate}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "candidate without test", userID: 7, limit: 10, viewer: &viewer, profiles: []model.Profile{noTest}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "both without test", userID: 7, limit: 10, viewer: &viewerNoTest, profiles: []model.Profile{noTest}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "incomplete candidate test", userID: 7, limit: 10, viewer: &viewer, profiles: []model.Profile{partial}, wantViewerCalls: 1, wantFeedCalls: 1},
		{name: "valid zero", userID: 7, limit: 10, viewer: &viewer, profiles: []model.Profile{candidate}, scores: []float64{0}, wantViewerCalls: 1, wantFeedCalls: 1, wantCalcCalls: 1},
		{name: "maximum score", userID: 7, limit: 10, viewer: &viewer, profiles: []model.Profile{candidate}, scores: []float64{1}, wantViewerCalls: 1, wantFeedCalls: 1, wantCalcCalls: 1},
		{name: "mixed page with cursor", userID: 7, limit: 10, cursor: &cursor, viewer: &viewer, profiles: []model.Profile{candidate, noTest, partial, candidate}, nextCursor: &next, scores: []float64{0.82, 0.35}, wantViewerCalls: 1, wantFeedCalls: 1, wantCalcCalls: 2},
		{name: "calculation failure", userID: 7, limit: 10, viewer: &viewer, profiles: []model.Profile{candidate}, calcErr: failure, wantErr: true, wantViewerCalls: 1, wantFeedCalls: 1, wantCalcCalls: 1},
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
			svc := NewProfileService(repo, calc, NewLocalPhotoURLProvider("https://media.example"), nil)
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
				if item.UserID != tc.profiles[i].UserID || item.Name != *tc.profiles[i].CurrentVersion.Name || item.Tags == nil || item.Photos == nil {
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
	ProfileRepository
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

func TestAgeAt(t *testing.T) {
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
			if got := model.AgeAt(tc.birth, tc.today); got != tc.want {
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
		{name: "empty optional fields", wantTags: []string{},
			photos:     []model.Photo{{ID: 8, StorageKey: "first.jpg", Position: 1}},
			wantPhotos: []model.FeedPhoto{{ID: 8, URL: "https://media.example/cats/first.jpg"}}},
		{name: "populated optional fields", about: &about,
			tags:       []model.Tag{{ID: 4, Name: "music"}, {ID: 9, Name: "sport"}},
			photos:     []model.Photo{{ID: 8, StorageKey: "first.jpg", Position: 0}, {ID: 3, StorageKey: "second.jpg", Position: 1}},
			wantTags:   []string{"music", "sport"},
			wantPhotos: []model.FeedPhoto{{ID: 8, URL: "https://media.example/cats/first.jpg"}, {ID: 3, URL: "https://media.example/cats/second.jpg"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			birth := time.Now().AddDate(-25, -6, 0)
			candidate := filledProfile(model.Profile{UserID: 42, Tags: tc.tags, Photos: tc.photos}, "Anna")
			candidate.CurrentVersion.BirthDate, candidate.CurrentVersion.AboutMe = &birth, tc.about
			viewer := filledProfile(model.Profile{UserID: 7}, "Viewer")
			repo := &feedRepositoryMock{t: t,
				viewer: func(context.Context, int64) (*model.Profile, error) { return &viewer, nil },
				feed: func(context.Context, int64, int, *int64) ([]model.Profile, *int64, error) {
					return []model.Profile{candidate}, nil, nil
				},
			}
			calc := &feedCompatibilityMock{t: t}
			svc := NewProfileService(repo, calc, NewLocalPhotoURLProvider("https://media.example"), nil)
			page, err := svc.GetNextFeed(context.Background(), 7, 10, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := model.FeedItem{UserID: 42, Name: "Anna", Age: 25, DatingGoal: model.DatingGoalRelationship, AboutMe: tc.about, Tags: tc.wantTags, Photos: tc.wantPhotos}
			if page == nil || len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0], want) {
				t.Fatalf("page=%+v, want item=%+v", page, want)
			}
			if repo.viewerCalls != 1 || repo.feedCalls != 1 || calc.calls != 0 {
				t.Fatal("unexpected dependency calls")
			}
		})
	}
}

func TestLocalPhotoURLProvider(t *testing.T) {
	for _, tc := range []struct {
		name, key, want string
		wantErr         bool
	}{
		{"filename", "image_1.jpg", "http://localhost:8080/cats/image_1.jpg", false},
		{"nested and escaped", "profile/my photo.jpg", "http://localhost:8080/cats/profile/my%20photo.jpg", false},
		{"empty", "", "", true},
		{"parent path", "../secret", "", true},
		{"absolute path", "/image_1.jpg", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewLocalPhotoURLProvider("http://localhost:8080/")
			got, err := provider.GetURL(context.Background(), tc.key)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("GetURL() = %q, %v; want %q, error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

type failingURLProvider struct{ err error }

func (p failingURLProvider) GetURL(context.Context, string) (string, error) { return "", p.err }

func TestProfileServicePropagatesURLProviderError(t *testing.T) {
	failure := errors.New("URL provider failed")
	viewer := filledProfile(model.Profile{UserID: 7}, "Viewer")
	repo := &feedRepositoryMock{t: t,
		viewer: func(context.Context, int64) (*model.Profile, error) { return &viewer, nil },
		feed: func(context.Context, int64, int, *int64) ([]model.Profile, *int64, error) {
			return []model.Profile{filledProfile(model.Profile{}, "Anna")}, nil, nil
		},
	}
	svc := NewProfileService(repo, &feedCompatibilityMock{t: t}, failingURLProvider{err: failure}, nil)
	page, err := svc.GetNextFeed(context.Background(), 7, 10, nil)
	if page != nil || !errors.Is(err, failure) {
		t.Fatalf("page=%v, err=%v; want provider error", page, err)
	}
}

func filledProfile(p model.Profile, name string) model.Profile {
	birth := time.Now().AddDate(-25, -6, 0)
	sex, goal := model.SexFemale, model.DatingGoalRelationship
	p.CurrentVersion.ProfileFields = model.ProfileFields{Name: &name, BirthDate: &birth, Sex: &sex, DatingGoal: &goal}
	p.HasSearchFilter = true
	if len(p.Photos) == 0 {
		p.Photos = []model.Photo{{ID: 1, StorageKey: "main.jpg", Position: 1}}
	}
	return p
}

func TestProfileMissing(t *testing.T) {
	if got := (&model.Profile{}).Missing(); !reflect.DeepEqual(got, []string{"name", "birth_date", "sex", "dating_goal", "search_filter", "photos"}) {
		t.Fatalf("empty profile missing = %v", got)
	}
	p := filledProfile(model.Profile{}, "Anna")
	if got := p.Missing(); len(got) != 0 {
		t.Fatalf("filled profile missing = %v", got)
	}
	p.CurrentVersion.DatingGoal, p.HasSearchFilter, p.Photos = nil, false, nil
	if got := p.Missing(); !reflect.DeepEqual(got, []string{"dating_goal", "search_filter", "photos"}) {
		t.Fatalf("missing = %v", got)
	}
}

func TestProfilePatchApply(t *testing.T) {
	p := filledProfile(model.Profile{}, "Anna")
	about, newName := "привет", "Аня"
	p.CurrentVersion.AboutMe = &about

	got := model.ProfilePatch{Name: &newName}.Apply(p.CurrentVersion.ProfileFields)
	if *got.Name != "Аня" || got.AboutMe == nil || *got.Sex != model.SexFemale {
		t.Fatalf("patch must change only name: %+v", got)
	}
	if got := (model.ProfilePatch{AboutMe: model.Optional[string]{Set: true}}).Apply(p.CurrentVersion.ProfileFields); got.AboutMe != nil {
		t.Fatal("about_me must be cleared")
	}
	height := 175
	got = model.ProfilePatch{Height: model.Optional[int]{Set: true, Value: &height}}.Apply(p.CurrentVersion.ProfileFields)
	if *got.Height != 175 || got.AboutMe == nil {
		t.Fatalf("patch must change only height: %+v", got)
	}
	if !(model.ProfilePatch{}).IsEmpty() || (model.ProfilePatch{Smoking: model.Optional[model.Attitude]{Set: true}}).IsEmpty() {
		t.Fatal("IsEmpty is wrong")
	}
}

type photoRepositoryMock struct {
	ProfileRepository
	addErr    error
	deleteKey string
	deleteErr error
	orderErr  error
	added     []string
	order     []int64
}

func (m *photoRepositoryMock) ReorderPhotos(_ context.Context, _ int64, photoIDs []int64) error {
	m.order = photoIDs
	return m.orderErr
}

func (m *photoRepositoryMock) AddPhoto(_ context.Context, _ int64, key string) error {
	m.added = append(m.added, key)
	return m.addErr
}
func (m *photoRepositoryMock) DeletePhoto(context.Context, int64, int64) (string, error) {
	return m.deleteKey, m.deleteErr
}
func (m *photoRepositoryMock) GetByUserIDCurrentProfile(context.Context, int64) (*model.Profile, error) {
	return &model.Profile{Photos: []model.Photo{{ID: 1, StorageKey: "left.jpg"}}}, nil
}

func TestProfileServicePhotoFiles(t *testing.T) {
	ctx := context.Background()
	urls := NewLocalPhotoURLProvider("https://media.example")

	repo, files := &photoRepositoryMock{}, &fakePhotos{}
	photos, err := NewProfileService(repo, nil, urls, files).AddPhoto(ctx, 7, model.PhotoUpload{Data: []byte("a"), Ext: ".jpg"})
	if err != nil || len(photos) != 1 || photos[0].URL != "https://media.example/cats/left.jpg" {
		t.Fatalf("add: photos = %+v, err = %v", photos, err)
	}
	if !reflect.DeepEqual(repo.added, []string{"uploads/1.jpg"}) || len(files.deleted) != 0 {
		t.Fatalf("add: added = %v, deleted = %v", repo.added, files.deleted)
	}

	repo, files = &photoRepositoryMock{addErr: model.ErrPhotoLimit}, &fakePhotos{}
	if _, err := NewProfileService(repo, nil, urls, files).AddPhoto(ctx, 7, model.PhotoUpload{Data: []byte("a"), Ext: ".jpg"}); !errors.Is(err, model.ErrPhotoLimit) {
		t.Fatalf("limit: err = %v", err)
	}
	if !reflect.DeepEqual(files.deleted, []string{"uploads/1.jpg"}) {
		t.Errorf("saved file must be removed when photo is not added, deleted = %v", files.deleted)
	}

	repo, files = &photoRepositoryMock{deleteKey: "gone.jpg"}, &fakePhotos{}
	if _, err := NewProfileService(repo, nil, urls, files).DeletePhoto(ctx, 7, 1); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files.deleted, []string{"gone.jpg"}) {
		t.Errorf("deleted = %v", files.deleted)
	}

	repo = &photoRepositoryMock{}
	if photos, err := NewProfileService(repo, nil, urls, nil).ReorderPhotos(ctx, 7, []int64{1}); err != nil ||
		!reflect.DeepEqual(repo.order, []int64{1}) || photos[0].URL != "https://media.example/cats/left.jpg" {
		t.Fatalf("reorder: photos = %+v, err = %v", photos, err)
	}
	repo = &photoRepositoryMock{orderErr: model.ErrPhotoOrderMismatch}
	if _, err := NewProfileService(repo, nil, urls, nil).ReorderPhotos(ctx, 7, []int64{2}); !errors.Is(err, model.ErrPhotoOrderMismatch) {
		t.Fatalf("reorder mismatch: err = %v", err)
	}

	repo, files = &photoRepositoryMock{deleteErr: model.ErrLastPhoto}, &fakePhotos{}
	if _, err := NewProfileService(repo, nil, urls, files).DeletePhoto(ctx, 7, 1); !errors.Is(err, model.ErrLastPhoto) || len(files.deleted) != 0 {
		t.Errorf("last photo: err = %v, deleted = %v", err, files.deleted)
	}
}

func TestProfileServiceFeedSkipsIncompleteProfiles(t *testing.T) {
	viewer := filledProfile(model.Profile{UserID: 7}, "Viewer")
	incomplete := filledProfile(model.Profile{UserID: 30}, "Broken")
	incomplete.CurrentVersion.Name = nil
	repo := &feedRepositoryMock{t: t,
		viewer: func(context.Context, int64) (*model.Profile, error) { return &viewer, nil },
		feed: func(context.Context, int64, int, *int64) ([]model.Profile, *int64, error) {
			return []model.Profile{incomplete, filledProfile(model.Profile{UserID: 31}, "Anna")}, nil, nil
		},
	}
	svc := NewProfileService(repo, &feedCompatibilityMock{t: t}, NewLocalPhotoURLProvider("https://media.example"), nil)
	page, err := svc.GetNextFeed(context.Background(), 7, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].UserID != 31 {
		t.Fatalf("items = %+v", page.Items)
	}
}
