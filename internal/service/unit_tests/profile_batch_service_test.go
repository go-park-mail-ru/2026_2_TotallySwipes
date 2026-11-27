package service

import (
	"context"
	"errors"
	"testing"

	"dating-app/internal/model"
	. "dating-app/internal/service"

	"go.uber.org/mock/gomock"
)

func TestProfileServiceSearchCandidates(t *testing.T) {
	ctx := context.Background()
	viewer := filledProfile(model.Profile{UserID: 7}, "Viewer")
	incomplete := model.Profile{UserID: 12}
	next := int64(11)

	t.Run("returns complete candidates with photo URLs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMockProfileRepository(ctrl)
		urls := NewMockPhotoURLProvider(ctrl)
		repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&viewer, nil)
		candidates := feedCandidates(filledProfile(model.Profile{UserID: 11}, "Anna"), incomplete)
		repo.EXPECT().GetCandidatesByCursorAndLimit(ctx, int64(7), 5, (*int64)(nil)).Return(candidates, &next, nil)
		urls.EXPECT().GetPhotoURL(ctx, "main.jpg").Return("https://x/media/main.jpg", nil)

		got, gotNext, err := NewProfileService(repo, nil, urls, nil).SearchCandidates(ctx, 7, 5, nil)
		if err != nil {
			t.Fatalf("SearchCandidates: %v", err)
		}
		if len(got) != 1 || got[0].Profile.UserID != 11 || got[0].Profile.Photos[0].URL != "https://x/media/main.jpg" {
			t.Fatalf("candidates = %+v", got)
		}
		if gotNext == nil || *gotNext != 11 {
			t.Fatalf("next = %v", gotNext)
		}
	})

	for name, tc := range map[string]struct {
		viewer  *model.Profile
		limit   int
		wantErr error
	}{
		"limit too big":     {viewer: &viewer, limit: MaxFeedLimit + 1, wantErr: model.ErrInvalidFeedRequest},
		"profile missing":   {viewer: nil, limit: 5, wantErr: model.ErrProfileRequired},
		"no city":           {viewer: func() *model.Profile { p := viewer; p.City = nil; return &p }(), limit: 5, wantErr: model.ErrLocationRequired},
		"incomplete viewer": {viewer: &model.Profile{UserID: 7}, limit: 5, wantErr: model.ErrProfileRequired},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockProfileRepository(ctrl)
			if tc.limit <= MaxFeedLimit {
				if tc.viewer == nil {
					repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(nil, model.ErrNotFound)
				} else {
					repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(tc.viewer, nil)
				}
			}

			_, _, err := NewProfileService(repo, nil, nil, nil).SearchCandidates(ctx, 7, tc.limit, nil)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestProfileServiceGetProfilesBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("fills photo URLs and passes filter", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMockProfileRepository(ctrl)
		urls := NewMockPhotoURLProvider(ctrl)
		name := "an"
		filter := &model.ProfilesFilter{Name: &name}
		profiles := []model.Profile{filledProfile(model.Profile{UserID: 11}, "Anna")}
		repo.EXPECT().GetByUserIDs(ctx, []int64{11, 12}, filter).Return(profiles, nil)
		urls.EXPECT().GetPhotoURL(ctx, "main.jpg").Return("https://x/media/main.jpg", nil)

		got, err := NewProfileService(repo, nil, urls, nil).GetProfilesBatch(ctx, []int64{11, 12}, filter)
		if err != nil || len(got) != 1 || got[0].Photos[0].URL != "https://x/media/main.jpg" {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})

	tooMany := make([]int64, MaxProfilesBatch+1)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	from, to := 30, 20
	for name, tc := range map[string]struct {
		ids    []int64
		filter *model.ProfilesFilter
	}{
		"too many ids":       {ids: tooMany},
		"non-positive id":    {ids: []int64{1, 0}},
		"inverted age range": {ids: []int64{1}, filter: &model.ProfilesFilter{AgeFrom: &from, AgeTo: &to}},
	} {
		t.Run(name, func(t *testing.T) {
			repo := NewMockProfileRepository(gomock.NewController(t))
			_, err := NewProfileService(repo, nil, nil, nil).GetProfilesBatch(ctx, tc.ids, tc.filter)
			if !errors.Is(err, model.ErrInvalidMatchesRequest) {
				t.Fatalf("err = %v, want ErrInvalidMatchesRequest", err)
			}
		})
	}
}
