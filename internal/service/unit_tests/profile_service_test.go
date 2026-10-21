package service

import (
	"context"
	"dating-app/internal/model"
	"dating-app/internal/service"
	. "dating-app/internal/service"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
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
			// Arrange
			ctrl := gomock.NewController(t)
			repo := NewMockProfileRepository(ctrl)
			calc := NewMockCompatibilityService(ctrl)
			urls := NewMockPhotoURLProvider(ctrl)
			var ordered []any
			if tc.wantViewerCalls > 0 {
				ordered = append(ordered, repo.EXPECT().GetByUserIDCurrentProfile(ctx, tc.userID).Return(tc.viewer, tc.viewerErr).Times(1))
			}
			if tc.wantFeedCalls > 0 {
				ordered = append(ordered, repo.EXPECT().GetProfilesByCursorAndLimit(ctx, tc.userID, tc.limit, tc.cursor).Return(feedCandidates(tc.profiles...), tc.nextCursor, tc.feedErr).Times(1))
			}
			expected := model.BigFive{Openness: &value, Conscientiousness: &value, Extraversion: &value, Agreeableness: &value, Neuroticism: &value}
			for i := 0; i < tc.wantCalcCalls; i++ {
				score := 0.0
				if tc.calcErr == nil {
					score = tc.scores[i]
				}
				ordered = append(ordered, calc.EXPECT().CalculateDistance(expected, expected).Return(score, tc.calcErr).Times(1))
			}
			gomock.InOrder(ordered...)
			if tc.feedErr == nil && tc.calcErr == nil {
				for _, profile := range tc.profiles {
					for _, photo := range profile.Photos {
						urls.EXPECT().GetPhotoURL(ctx, photo.StorageKey).Return("https://media.example/cats/"+photo.StorageKey, nil).Times(1)
					}
				}
			}
			svc := NewProfileService(repo, calc, urls, nil)
			// Act
			page, err := svc.GetNextFeed(ctx, tc.userID, tc.limit, tc.cursor)
			// Assert
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
		likedMe    bool
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
		{name: "populated optional fields", likedMe: true, about: &about,
			tags:       []model.Tag{{ID: 4, Name: "music"}, {ID: 9, Name: "movies"}},
			photos:     []model.Photo{{ID: 8, StorageKey: "first.jpg", Position: 0}, {ID: 3, StorageKey: "second.jpg", Position: 1}},
			wantTags:   []string{"music", "movies"},
			wantPhotos: []model.FeedPhoto{{ID: 8, URL: "https://media.example/cats/first.jpg"}, {ID: 3, URL: "https://media.example/cats/second.jpg"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			birth := time.Now().AddDate(-25, -6, 0)
			candidate := filledProfile(model.Profile{UserID: 42, Tags: tc.tags, Photos: tc.photos}, "Anna")
			candidate.CurrentVersion.BirthDate, candidate.CurrentVersion.AboutMe = &birth, tc.about
			viewer := filledProfile(model.Profile{UserID: 7}, "Viewer")
			// Arrange
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			repo := NewMockProfileRepository(ctrl)
			calc := NewMockCompatibilityService(ctrl)
			urls := NewMockPhotoURLProvider(ctrl)
			repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&viewer, nil).Times(1)
			repo.EXPECT().GetProfilesByCursorAndLimit(ctx, int64(7), 10, (*int64)(nil)).Return([]model.FeedCandidate{{Profile: candidate, LikedMe: tc.likedMe}}, nil, nil).Times(1)
			for _, photo := range tc.photos {
				urls.EXPECT().GetPhotoURL(ctx, photo.StorageKey).Return("https://media.example/cats/"+photo.StorageKey, nil).Times(1)
			}
			svc := NewProfileService(repo, calc, urls, nil)
			// Act
			page, err := svc.GetNextFeed(ctx, 7, 10, nil)
			// Assert
			if err != nil {
				t.Fatal(err)
			}
			want := model.FeedItem{LikedMe: tc.likedMe, UserID: 42, Name: "Anna", Age: 25, DatingGoal: model.DatingGoalRelationship, AboutMe: tc.about, Tags: tc.wantTags, Photos: tc.wantPhotos}
			if page == nil || len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0], want) {
				t.Fatalf("page=%+v, want item=%+v", page, want)
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

func TestProfileServicePropagatesURLProviderError(t *testing.T) {
	failure := errors.New("URL provider failed")
	viewer := filledProfile(model.Profile{UserID: 7}, "Viewer")
	// Arrange
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := NewMockProfileRepository(ctrl)
	urls := NewMockPhotoURLProvider(ctrl)
	repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&viewer, nil).Times(1)
	repo.EXPECT().GetProfilesByCursorAndLimit(ctx, int64(7), 10, (*int64)(nil)).Return(feedCandidates(filledProfile(model.Profile{}, "Anna")), nil, nil).Times(1)
	urls.EXPECT().GetPhotoURL(ctx, "main.jpg").Return("", failure).Times(1)
	svc := NewProfileService(repo, NewMockCompatibilityService(ctrl), urls, nil)
	// Act
	page, err := svc.GetNextFeed(ctx, 7, 10, nil)
	// Assert
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

func TestProfileServiceFeedSkipsIncompleteProfiles(t *testing.T) {
	viewer := filledProfile(model.Profile{UserID: 7}, "Viewer")
	incomplete := filledProfile(model.Profile{UserID: 30}, "Broken")
	incomplete.CurrentVersion.Name = nil
	// Arrange
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := NewMockProfileRepository(ctrl)
	urls := NewMockPhotoURLProvider(ctrl)
	repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&viewer, nil).Times(1)
	repo.EXPECT().GetProfilesByCursorAndLimit(ctx, int64(7), 10, (*int64)(nil)).Return(feedCandidates(incomplete, filledProfile(model.Profile{UserID: 31}, "Anna")), nil, nil).Times(1)
	urls.EXPECT().GetPhotoURL(ctx, "main.jpg").Return("https://media.example/cats/main.jpg", nil).Times(1)
	svc := NewProfileService(repo, NewMockCompatibilityService(ctrl), urls, nil)
	// Act
	page, err := svc.GetNextFeed(ctx, 7, 10, nil)
	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].UserID != 31 {
		t.Fatalf("items = %+v", page.Items)
	}
}

func feedCandidates(profiles ...model.Profile) []model.FeedCandidate {
	candidates := make([]model.FeedCandidate, len(profiles))
	for i, profile := range profiles {
		candidates[i] = model.FeedCandidate{Profile: profile}
	}
	return candidates
}

func TestProfileServiceGetMyProfilePhotoURLs(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint("provider error=", fail), func(t *testing.T) {
			ctx := context.Background()
			failure := errors.New("signing failed")
			// Arrange
			ctrl := gomock.NewController(t)
			repo := NewMockProfileRepository(ctrl)
			provider := NewMockPhotoURLProvider(ctrl)
			repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&model.Profile{Photos: []model.Photo{{ID: 1, StorageKey: "left.jpg"}}}, nil).Times(1)
			url := "https://storage.example/photos/left.jpg?signature=test"
			var providerErr error
			if fail {
				url = ""
				providerErr = failure
			}
			provider.EXPECT().GetPhotoURL(ctx, "left.jpg").Return(url, providerErr).Times(1)
			svc := NewProfileService(repo, nil, provider, nil)
			// Act
			profile, err := svc.GetMyProfile(ctx, 7)
			// Assert
			if fail {
				if profile != nil || !errors.Is(err, failure) {
					t.Fatalf("profile=%v error=%v", profile, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if profile.Photos[0].URL != "https://storage.example/photos/left.jpg?signature=test" || profile.Photos[0].StorageKey != "left.jpg" {
				t.Fatalf("unexpected photo: %+v", profile.Photos[0])
			}
		})
	}
}

func TestProfileService_AddPhoto_SaveError(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)

	repo := NewMockProfileRepository(ctrl)
	photos := NewMockPhotoStorage(ctrl)
	urls := NewMockPhotoURLProvider(ctrl)

	service := service.NewProfileService(repo, nil, urls, photos)

	ctx := context.Background()
	userID := int64(42)
	upload := model.PhotoUpload{
		Data: []byte("photo bytes"),
		Ext:  ".jpg",
	}

	wantErr := errors.New("storage unavalible")

	photos.EXPECT().
		Save(ctx, upload.Data, upload.Ext).Return("", wantErr).Times(1)

	// Act
	result, err := service.AddPhoto(ctx, userID, upload)

	// Assert
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected storage error, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil photos, got %#v", result)
	}
}

func TestProfileService_AddPhoto_RepositoryErrorDeleteFile(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)

	repo := NewMockProfileRepository(ctrl)
	photos := NewMockPhotoStorage(ctrl)
	urls := NewMockPhotoURLProvider(ctrl)

	service := service.NewProfileService(repo, nil, urls, photos)

	ctx := context.Background()
	userID := int64(42)
	upload := model.PhotoUpload{
		Data: []byte("photo bytes"),
		Ext:  ".jpg",
	}

	key := "photos/test.jpg"
	wantErr := model.ErrPhotoLimit

	gomock.InOrder(
		photos.EXPECT().
			Save(ctx, upload.Data, upload.Ext).
			Return(key, nil).Times(1),
		repo.EXPECT().
			AddPhoto(ctx, userID, key).Return(wantErr).Times(1),

		photos.EXPECT().Delete(gomock.Any(), key).Return(nil).Times(1),
	)

	// Act
	result, err := service.AddPhoto(ctx, userID, upload)

	// Assert
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected photo limit error, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil photos, got %#v", result)
	}
}

func TestProfileService_GetMyProfile_ProfilePhotoOK(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	repo := NewMockProfileRepository(ctrl)
	urls := NewMockPhotoURLProvider(ctrl)

	profileService := service.NewProfileService(repo, nil, urls, nil)
	userID := int64(42)
	ctx := context.Background()

	profile := &model.Profile{
		UserID: userID,
		Photos: []model.Photo{
			{ID: 1, StorageKey: "photos/cat1.jpg"},
			{ID: 2, StorageKey: "photos/cat2.jpg"},
		},
	}

	repo.EXPECT().
		GetByUserIDCurrentProfile(ctx, userID).
		Return(profile, nil).
		Times(1)

	urls.EXPECT().
		GetPhotoURL(ctx, "photos/cat1.jpg").
		Return("https://example.com/cat1.jpg", nil).
		Times(1)

	urls.EXPECT().
		GetPhotoURL(ctx, "photos/cat2.jpg").
		Return("https://example.com/cat2.jpg", nil).
		Times(1)

	// Act
	got, err := profileService.GetMyProfile(ctx, userID)
	// Assert
	if err != nil {
		t.Fatalf("GetMyProfile() error: %v", err)
	}

	if got == nil {
		t.Fatal("expected profile, got nil")
	}

	if got.UserID != userID {
		t.Fatalf("UserID = %d, want %d", got.UserID, userID)
	}

	wantPhotos := []model.Photo{
		{
			ID:         1,
			StorageKey: "photos/cat1.jpg",
			URL:        "https://example.com/cat1.jpg",
		},
		{
			ID:         2,
			StorageKey: "photos/cat2.jpg",
			URL:        "https://example.com/cat2.jpg",
		},
	}

	if !reflect.DeepEqual(got.Photos, wantPhotos) {
		t.Fatalf("Photos = %#v, want %#v", got.Photos, wantPhotos)
	}
}
