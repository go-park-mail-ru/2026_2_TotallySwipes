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

type fakeMatches struct {
	matches    []model.Match
	total      int
	nextCursor *int64
	err        error
	calls      int
	limit      int
	cursor     *int64
	summary    *model.MatchSummary
}

func (f *fakeMatches) GetSummary(context.Context, int64) (*model.MatchSummary, error) {
	f.calls++
	return f.summary, f.err
}

func (f *fakeMatches) GetMatches(_ context.Context, _ int64, limit int, cursor *int64) ([]model.Match, int, *int64, error) {
	f.calls++
	f.limit, f.cursor = limit, cursor
	return f.matches, f.total, f.nextCursor, f.err
}

func (f *fakeMatches) MarkSeen(context.Context, int64, int64) error {
	f.calls++
	return f.err
}

type fakeMatchViewer struct {
	profile *model.Profile
	err     error
}

func (f fakeMatchViewer) GetByUserIDCurrentProfile(context.Context, int64) (*model.Profile, error) {
	return f.profile, f.err
}

func TestMatchServiceGetMatches(t *testing.T) {
	name, key, badKey := "Anna", "profiles/anna.jpg", "../secret.jpg"
	birth := time.Now().AddDate(-25, -1, 0)
	zero, cursor, next := int64(0), int64(30), int64(12)
	low, high := 0.2, 0.6
	failure := errors.New("dependency failure")
	vector := func(v *float64) model.BigFive {
		return model.BigFive{Openness: v, Conscientiousness: v, Extraversion: v, Agreeableness: v, Neuroticism: v}
	}
	viewerPsycho := &model.ProfilePsycho{Openness: &low, Conscientiousness: &low, Extraversion: &low, Agreeableness: &low, Neuroticism: &low}
	viewer := fakeMatchViewer{profile: &model.Profile{UserID: 7, CurrentPsycho: viewerPsycho}}
	anna := model.Match{ID: 20, UserID: 8, Name: &name, BirthDate: &birth, PhotoKey: &key, Psycho: vector(&high), IsNew: true}
	noTest := model.Match{ID: 12, UserID: 9}

	for _, tc := range []struct {
		name              string
		userID            int64
		limit             int
		cursor            *int64
		viewer            fakeMatchViewer
		repo              fakeMatches
		wantErr           error
		wantCalls         int
		wantCompatibility []*float64
	}{
		{name: "invalid user", limit: 10, wantErr: model.ErrInvalidMatchesRequest},
		{name: "limit too big", userID: 7, limit: model.MatchesMaxLimit + 1, wantErr: model.ErrInvalidMatchesRequest},
		{name: "zero cursor", userID: 7, limit: 10, cursor: &zero, wantErr: model.ErrInvalidMatchesRequest},
		{name: "viewer failure", userID: 7, limit: 10, viewer: fakeMatchViewer{err: failure}, wantErr: failure},
		{name: "repository failure", userID: 7, limit: 10, viewer: viewer, repo: fakeMatches{err: failure}, wantErr: failure, wantCalls: 1},
		{name: "invalid photo key", userID: 7, limit: 10, viewer: viewer, repo: fakeMatches{matches: []model.Match{{ID: 1, PhotoKey: &badKey}}},
			wantErr: model.ErrInvalidPhotoStorageKey, wantCalls: 1},
		{name: "empty page", userID: 7, limit: 10, viewer: viewer, repo: fakeMatches{matches: []model.Match{}}, wantCalls: 1},
		{name: "viewer without profile", userID: 7, limit: 10, viewer: fakeMatchViewer{err: model.ErrNotFound},
			repo: fakeMatches{matches: []model.Match{anna}, total: 1}, wantCalls: 1, wantCompatibility: []*float64{nil}},
		{name: "viewer without test", userID: 7, limit: 10, viewer: fakeMatchViewer{profile: &model.Profile{UserID: 7}},
			repo: fakeMatches{matches: []model.Match{anna}, total: 1}, wantCalls: 1, wantCompatibility: []*float64{nil}},
		{name: "page with cursor", userID: 7, limit: 2, cursor: &cursor, viewer: viewer, wantCalls: 1,
			repo:              fakeMatches{matches: []model.Match{anna, noTest}, total: 5, nextCursor: &next},
			wantCompatibility: []*float64{new(0.6), nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := tc.repo
			repo.matches = append([]model.Match(nil), tc.repo.matches...)
			svc := NewMatchService(&repo, tc.viewer, NewCompatibilityService(), NewLocalPhotoURLProvider("https://media.example"))
			page, err := svc.GetMatches(context.Background(), tc.userID, tc.limit, tc.cursor)
			if repo.calls != tc.wantCalls {
				t.Fatalf("repository calls = %d, want %d", repo.calls, tc.wantCalls)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || page != nil {
					t.Fatalf("page=%v error=%v; want %v", page, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if repo.limit != tc.limit || !reflect.DeepEqual(repo.cursor, tc.cursor) {
				t.Fatalf("repository got limit=%d cursor=%v", repo.limit, repo.cursor)
			}
			if len(page.Items) != len(repo.matches) || page.Total != repo.total || !reflect.DeepEqual(page.NextCursor, repo.nextCursor) {
				t.Fatalf("unexpected page: %+v", page)
			}
			for i, m := range page.Items {
				if (m.BirthDate == nil) != (m.Age == nil) || (m.PhotoKey == nil) != (m.MainPhotoURL == nil) {
					t.Fatalf("age or photo url not filled: %+v", m)
				}
				if want := tc.wantCompatibility[i]; (want == nil) != (m.Compatibility == nil) ||
					want != nil && (*m.Compatibility-*want > 1e-9 || *want-*m.Compatibility > 1e-9) {
					t.Fatalf("item %d compatibility = %v, want %v", i, m.Compatibility, want)
				}
			}
			if len(page.Items) > 0 && page.Items[0].Age != nil {
				if *page.Items[0].Age != 25 || *page.Items[0].MainPhotoURL != "https://media.example/cats/profiles/anna.jpg" || !page.Items[0].IsNew {
					t.Fatalf("unexpected first item: %+v", page.Items[0])
				}
			}
		})
	}
}

func TestMatchServiceMarkSeen(t *testing.T) {
	failure := errors.New("dependency failure")
	for _, tc := range []struct {
		name            string
		userID, matchID int64
		repoErr         error
		wantErr         error
		wantCalls       int
	}{
		{name: "invalid user", matchID: 1, wantErr: model.ErrInvalidMatchesRequest},
		{name: "invalid match", userID: 7, wantErr: model.ErrInvalidMatchesRequest},
		{name: "not a participant", userID: 7, matchID: 1, repoErr: model.ErrNotFound, wantErr: model.ErrMatchNotFound, wantCalls: 1},
		{name: "repository failure", userID: 7, matchID: 1, repoErr: failure, wantErr: failure, wantCalls: 1},
		{name: "ok", userID: 7, matchID: 1, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeMatches{err: tc.repoErr}
			svc := NewMatchService(repo, fakeMatchViewer{}, NewCompatibilityService(), NewLocalPhotoURLProvider("https://media.example"))
			err := svc.MarkSeen(context.Background(), tc.userID, tc.matchID)
			if repo.calls != tc.wantCalls || !errors.Is(err, tc.wantErr) || (tc.wantErr == nil) != (err == nil) {
				t.Fatalf("calls = %d, error = %v; want %d, %v", repo.calls, err, tc.wantCalls, tc.wantErr)
			}
		})
	}
}

func TestMatchServiceGetSummary(t *testing.T) {
	failure := errors.New("dependency failure")
	for _, tc := range []struct {
		name      string
		userID    int64
		repo      fakeMatches
		wantErr   error
		wantCalls int
	}{
		{name: "invalid user", wantErr: model.ErrInvalidMatchesRequest},
		{name: "repository failure", userID: 7, repo: fakeMatches{err: failure}, wantErr: failure, wantCalls: 1},
		{name: "ok", userID: 7, repo: fakeMatches{summary: &model.MatchSummary{Total: 12, New: 3}}, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := tc.repo
			svc := NewMatchService(&repo, fakeMatchViewer{}, NewCompatibilityService(), NewLocalPhotoURLProvider("https://media.example"))
			got, err := svc.GetSummary(context.Background(), tc.userID)
			if repo.calls != tc.wantCalls || !errors.Is(err, tc.wantErr) || (tc.wantErr == nil) != (err == nil) {
				t.Fatalf("calls = %d, error = %v; want %d, %v", repo.calls, err, tc.wantCalls, tc.wantErr)
			}
			if tc.wantErr == nil && !reflect.DeepEqual(got, repo.summary) {
				t.Fatalf("summary = %+v, want %+v", got, repo.summary)
			}
		})
	}
}
