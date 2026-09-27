package service

import (
	"context"
	"dating-app/internal/model"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type feedRepoStub struct {
	ProfileRepository
	viewer    *model.Profile
	candidate model.Profile
	err       error
	gotUserID int64
}

func (r *feedRepoStub) GetByUserIDCurrentProfile(_ context.Context, id int64) (*model.Profile, error) {
	r.gotUserID = id
	return r.viewer, r.err
}
func (r *feedRepoStub) GetProfilesByCursorAndLimit(context.Context, int64, int, *int64) ([]model.Profile, *int64, error) {
	return []model.Profile{r.candidate}, nil, nil
}

type compatibilityStub struct {
	calls int
	score float64
	err   error
}

func (s *compatibilityStub) Calculate(model.BigFive, model.BigFive) (float64, error) {
	s.calls++
	return s.score, s.err
}
func TestFeedOptionalCompatibility(t *testing.T) {
	value := 0.5
	complete := &model.ProfilePsycho{Openness: &value, Conscientiousness: &value, Extraversion: &value, Agreeableness: &value, Neuroticism: &value}
	failure := errors.New("calculation failed")
	for _, tc := range []struct {
		name              string
		viewer, candidate *model.ProfilePsycho
		score             float64
		calcErr           error
		wantCalls         int
	}{
		{name: "viewer without test", candidate: complete},
		{name: "candidate without test", viewer: complete},
		{name: "both without test"},
		{name: "incomplete test", viewer: complete, candidate: &model.ProfilePsycho{Openness: &value}},
		{name: "valid zero", viewer: complete, candidate: complete, wantCalls: 1},
		{name: "calculated score", viewer: complete, candidate: complete, score: 0.82, wantCalls: 1},
		{name: "calculation failure", viewer: complete, candidate: complete, calcErr: failure, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &feedRepoStub{viewer: &model.Profile{ID: 99, CurrentPsycho: tc.viewer}, candidate: model.Profile{UserID: 20, CurrentPsycho: tc.candidate}}
			calc := &compatibilityStub{score: tc.score, err: tc.calcErr}
			svc := NewProfileService(repo, calc, LocalPhotoURLProvider{})
			page, err := svc.GetNextFeed(context.Background(), 7, 10, nil)
			if calc.calls != tc.wantCalls || repo.gotUserID != 7 {
				t.Fatalf("calls=%d userID=%d", calc.calls, repo.gotUserID)
			}
			if tc.calcErr != nil {
				if !errors.Is(err, tc.calcErr) {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			item := page.Items[0]
			if tc.wantCalls == 0 {
				if item.Compatibility != nil {
					t.Fatal("expected nil")
				}
				data, err := json.Marshal(item)
				if err != nil || !strings.Contains(string(data), `"compatibility":null`) {
					t.Fatalf("JSON=%s error=%v", data, err)
				}
			} else if item.Compatibility == nil || *item.Compatibility != tc.score {
				t.Fatalf("score=%v", item.Compatibility)
			}
		})
	}
}
func TestFeedViewerLookupFailure(t *testing.T) {
	failure := errors.New("database unavailable")
	svc := NewProfileService(&feedRepoStub{err: failure}, &compatibilityStub{}, LocalPhotoURLProvider{})
	if _, err := svc.GetNextFeed(context.Background(), 7, 10, nil); !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
}
