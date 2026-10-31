package service

import (
	"context"
	"dating-app/internal/model"
	. "dating-app/internal/service"
	"errors"
	"math"
	"reflect"
	"testing"

	"go.uber.org/mock/gomock"
)

func TestProfileServiceGetMatchedProfile(t *testing.T) {
	low, high := 0.2, 0.6
	psycho := func(v *float64) *model.ProfilePsycho {
		return &model.ProfilePsycho{Openness: v, Conscientiousness: v, Extraversion: v, Agreeableness: v, Neuroticism: v}
	}
	about := "Работаю в издательстве"
	target := filledProfile(model.Profile{UserID: 8, CurrentPsycho: psycho(&high), Tags: []model.Tag{{Name: "books"}, {Name: "bicycle"}}}, "Даша")
	target.CurrentVersion.AboutMe = &about
	viewer := filledProfile(model.Profile{UserID: 7, CurrentPsycho: psycho(&low)}, "Илья")
	failure := errors.New("dependency failure")
	ctx := context.Background()

	type deps struct {
		repo   *MockProfileRepository
		urls   *MockPhotoURLProvider
		compat *MockCompatibilityService
	}
	for _, tc := range []struct {
		name              string
		viewerID          int64
		targetID          int64
		setup             func(d deps)
		wantErr           error
		wantCompatibility *float64
	}{
		{name: "self", viewerID: 7, targetID: 7, wantErr: model.ErrProfileNotFound},
		{name: "invalid target", viewerID: 7, wantErr: model.ErrProfileNotFound},
		{name: "no match", viewerID: 7, targetID: 8, wantErr: model.ErrProfileNotFound, setup: func(d deps) {
			d.repo.EXPECT().GetMatchedProfile(ctx, int64(7), int64(8)).Return(nil, int64(0), model.ErrNotFound)
		}},
		{name: "repository failure", viewerID: 7, targetID: 8, wantErr: failure, setup: func(d deps) {
			d.repo.EXPECT().GetMatchedProfile(ctx, int64(7), int64(8)).Return(nil, int64(0), failure)
		}},
		{name: "incomplete target", viewerID: 7, targetID: 8, wantErr: model.ErrProfileNotFound, setup: func(d deps) {
			d.repo.EXPECT().GetMatchedProfile(ctx, int64(7), int64(8)).Return(&model.Profile{UserID: 8}, int64(15), nil)
		}},
		{name: "viewer failure", viewerID: 7, targetID: 8, wantErr: failure, setup: func(d deps) {
			d.repo.EXPECT().GetMatchedProfile(ctx, int64(7), int64(8)).Return(&target, int64(15), nil)
			d.repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(nil, failure)
		}},
		{name: "photo url failure", viewerID: 7, targetID: 8, wantErr: failure, setup: func(d deps) {
			d.repo.EXPECT().GetMatchedProfile(ctx, int64(7), int64(8)).Return(&target, int64(15), nil)
			d.repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(nil, model.ErrNotFound)
			d.urls.EXPECT().GetPhotoURL(ctx, "main.jpg").Return("", failure)
		}},
		{name: "both tested", viewerID: 7, targetID: 8, wantCompatibility: new(0.6), setup: func(d deps) {
			d.repo.EXPECT().GetMatchedProfile(ctx, int64(7), int64(8)).Return(&target, int64(15), nil)
			d.repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&viewer, nil)
			d.compat.EXPECT().CalculateDistance(gomock.Any(), gomock.Any()).Return(0.6, nil)
			d.urls.EXPECT().GetPhotoURL(ctx, "main.jpg").Return("https://media.example/cats/main.jpg", nil)
		}},
		{name: "viewer without profile", viewerID: 7, targetID: 8, setup: func(d deps) {
			d.repo.EXPECT().GetMatchedProfile(ctx, int64(7), int64(8)).Return(&target, int64(15), nil)
			d.repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(nil, model.ErrNotFound)
			d.urls.EXPECT().GetPhotoURL(ctx, "main.jpg").Return("https://media.example/cats/main.jpg", nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			d := deps{NewMockProfileRepository(ctrl), NewMockPhotoURLProvider(ctrl), NewMockCompatibilityService(ctrl)}
			if tc.setup != nil {
				tc.setup(d)
			}
			got, err := NewProfileService(d.repo, d.compat, d.urls, nil).GetMatchedProfile(ctx, tc.viewerID, tc.targetID)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != nil {
					t.Fatalf("got = %+v, err = %v; want %v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			item := got.Item
			if got.MatchID != 15 || item.UserID != 8 || item.Name != "Даша" || item.Age != 25 || !item.LikedMe ||
				item.AboutMe != &about || !reflect.DeepEqual(item.Tags, []string{"books", "bicycle"}) ||
				!reflect.DeepEqual(item.Photos, []model.FeedPhoto{{ID: 1, URL: "https://media.example/cats/main.jpg"}}) {
				t.Fatalf("unexpected profile: %+v", got)
			}
			if (tc.wantCompatibility == nil) != (item.Compatibility == nil) ||
				tc.wantCompatibility != nil && math.Abs(*item.Compatibility-*tc.wantCompatibility) > 1e-9 {
				t.Fatalf("compatibility = %v, want %v", item.Compatibility, tc.wantCompatibility)
			}
		})
	}
}
