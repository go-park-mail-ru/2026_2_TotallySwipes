package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"dating-app/services/matching/internal/model"
	. "dating-app/services/matching/internal/service"
	"go.uber.org/mock/gomock"
)

func ptr[T any](v T) *T {

	return &v
}

var failure = errors.New("dependency failure")

func profileFixture(userID int64) model.Profile {

	return model.Profile{ID: userID + 100, UserID: userID, Name: ptr("Анна"), Age: ptr(25), DatingGoal: ptr(model.DatingGoal("relationship")), City: ptr("Москва"), Tags: []string{"кино"}, Photos: []model.FeedPhoto{{ID: 1, URL: "/media/photo"}}, Psycho: bigFiveTestVector([5]float64{0, 0, 0, 0, 0})}
}

func TestFeedSelection(t *testing.T) {

	ctrl := gomock.NewController(t)
	profiles, swipes := NewMockProfileProvider(ctrl), NewMockFeedRepository(ctrl)
	viewer, first, second, third := profileFixture(1), profileFixture(2), profileFixture(3), profileFixture(4)
	third.Psycho = bigFiveTestVector([5]float64{1, 1, 1, 1, 1})
	profiles.EXPECT().GetProfilesBatch(gomock.Any(), []int64{1}, model.MatchFilter{}).Return(map[int64]model.Profile{1: viewer}, nil)
	profiles.EXPECT().SearchCandidates(gomock.Any(), int64(1), 2, (*int64)(nil)).Return([]model.Profile{first, second}, ptr(int64(3)), nil)
	swipes.EXPECT().GetSwipeStates(gomock.Any(), int64(1), viewer.ID, []model.Profile{first, second}).Return(map[int64]model.SwipeState{2: {Swiped: true}, 3: {LikedMe: true}}, nil)
	profiles.EXPECT().SearchCandidates(gomock.Any(), int64(1), 1, ptr(int64(3))).Return([]model.Profile{third}, (*int64)(nil), nil)
	swipes.EXPECT().GetSwipeStates(gomock.Any(), int64(1), viewer.ID, []model.Profile{third}).Return(map[int64]model.SwipeState{}, nil)
	got, err := NewFeedService(profiles, swipes, NewCompatibilityService()).GetNextFeed(context.Background(), 1, 2, nil)

	if err != nil {
		t.Fatal(err)
	}

	if len(got.Items) != 2 || got.Items[0].UserID != 3 || got.Items[1].UserID != 4 || !got.Items[0].LikedMe || *got.Items[0].Compatibility != 1 || *got.Items[1].Compatibility != 0 || got.NextCursor != nil {
		t.Fatalf("page = %#v", got)
	}

	if !reflect.DeepEqual(got.Items[0].Photos, second.Photos) || !reflect.DeepEqual(got.Items[0].Tags, second.Tags) {
		t.Fatalf("item = %#v", got.Items[0])
	}
}

func TestFeedErrors(t *testing.T) {

	for _, name := range []string{"неверный пользователь", "неверный limit", "неверный cursor", "ошибка профиля", "нет анкеты", "незаполненная анкета", "нет города", "ошибка кандидатов", "ошибка свайпов"} {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			profiles, swipes := NewMockProfileProvider(ctrl), NewMockFeedRepository(ctrl)
			userID, limit := int64(1), 2
			var cursor *int64
			wantErr := model.ErrInvalidFeedRequest

			switch name {
			case "неверный пользователь":

				userID = 0

			case "неверный limit":

				limit = 11

			case "неверный cursor":

				cursor = ptr(int64(0))

			default:

				viewer := profileFixture(1)
				available := map[int64]model.Profile{1: viewer}
				profileErr := error(nil)
				wantErr = failure

				switch name {
				case "ошибка профиля":

					profileErr = failure

				case "нет анкеты":

					available = nil
					wantErr = model.ErrProfileRequired

				case "незаполненная анкета":

					viewer.Missing = []string{"photos"}
					available[1] = viewer
					wantErr = model.ErrProfileRequired

				case "нет города":

					viewer.City = nil
					available[1] = viewer
					wantErr = model.ErrLocationRequired
				}

				profiles.EXPECT().GetProfilesBatch(gomock.Any(), []int64{1}, model.MatchFilter{}).Return(available, profileErr)

				if name == "ошибка кандидатов" {
					profiles.EXPECT().SearchCandidates(gomock.Any(), userID, limit, cursor).Return(nil, nil, failure)
				}

				if name == "ошибка свайпов" {
					profiles.EXPECT().SearchCandidates(gomock.Any(), userID, limit, cursor).Return([]model.Profile{}, nil, nil)
					swipes.EXPECT().GetSwipeStates(gomock.Any(), userID, viewer.ID, []model.Profile{}).Return(nil, failure)
				}
			}

			_, err := NewFeedService(profiles, swipes, NewCompatibilityService()).GetNextFeed(context.Background(), userID, limit, cursor)

			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
		})
	}
}

func TestMatchesFilteredPagination(t *testing.T) {

	ctrl := gomock.NewController(t)
	repo, profiles, events := NewMockMatchRepository(ctrl), NewMockProfileProvider(ctrl), NewMockMatchSearchRepository(ctrl)
	filter := model.MatchFilter{Name: ptr("Анна")}
	viewer, second, third := profileFixture(1), profileFixture(3), profileFixture(4)
	third.City = ptr("Казань")
	profiles.EXPECT().GetProfilesBatch(gomock.Any(), []int64{1}, model.MatchFilter{}).Return(map[int64]model.Profile{1: viewer}, nil)
	repo.EXPECT().GetMatches(gomock.Any(), int64(1), 2, (*int64)(nil)).Return([]model.Match{{ID: 70, ChatID: ptr(int64(700)), UserID: 2}, {ID: 60, ChatID: ptr(int64(600)), UserID: 3, IsNew: true}}, 3, ptr(int64(60)), nil)
	profiles.EXPECT().GetProfilesBatch(gomock.Any(), []int64{2, 3}, filter).Return(map[int64]model.Profile{3: second}, nil)
	events.EXPECT().GetPartnerFinishedByMatchIDs(gomock.Any(), int64(1), []int64{70, 60}).Return(map[int64]bool{60: true}, nil)
	repo.EXPECT().GetMatches(gomock.Any(), int64(1), 1, ptr(int64(60))).Return([]model.Match{{ID: 50, UserID: 4}}, 3, nil, nil)
	profiles.EXPECT().GetProfilesBatch(gomock.Any(), []int64{4}, filter).Return(map[int64]model.Profile{4: third}, nil)
	events.EXPECT().GetPartnerFinishedByMatchIDs(gomock.Any(), int64(1), []int64{50}).Return(map[int64]bool{}, nil)
	got, err := NewMatchService(repo, profiles, NewCompatibilityService(), events).GetMatches(context.Background(), 1, 2, nil, filter)

	if err != nil {
		t.Fatal(err)
	}

	if got.Total != 3 || got.NextCursor != nil || len(got.Items) != 2 || got.Items[0].ID != 60 || got.Items[1].ID != 50 || !got.Items[0].CanMeetup || got.Items[1].CanMeetup || !got.Items[0].PartnerFinished || !got.Items[0].IsNew || got.Items[0].Compatibility == nil {
		t.Fatalf("page = %#v", got)
	}

	if got.Items[0].ChatID == nil || *got.Items[0].ChatID != 600 || got.Items[1].ChatID != nil {
		t.Fatalf("chat IDs = %v, %v; want 600, nil", got.Items[0].ChatID, got.Items[1].ChatID)
	}
}

func TestMatchedProfile(t *testing.T) {

	for _, name := range []string{"успех", "нет мэтча", "неполная анкета"} {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			repo, profiles := NewMockMatchRepository(ctrl), NewMockProfileProvider(ctrl)
			wantErr := error(nil)
			lookupErr := error(nil)

			if name == "нет мэтча" {
				lookupErr, wantErr = model.ErrNotFound, model.ErrProfileNotFound
			}

			repo.EXPECT().GetMatchID(gomock.Any(), int64(1), int64(2)).Return(int64(20), lookupErr)

			if lookupErr == nil {
				candidate := profileFixture(2)

				if name == "неполная анкета" {
					candidate.Missing = []string{"name"}
					wantErr = model.ErrProfileNotFound
				}

				profiles.EXPECT().GetProfilesBatch(gomock.Any(), []int64{1, 2}, model.MatchFilter{}).Return(map[int64]model.Profile{2: candidate}, nil)
			}

			got, err := NewMatchService(repo, profiles, NewCompatibilityService(), nil).GetMatchedProfile(context.Background(), 1, 2)

			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v", err)
			}

			if err == nil && (got.MatchID != 20 || !got.Item.LikedMe || got.Item.Compatibility != nil) {
				t.Fatalf("profile = %#v", got)
			}
		})
	}
}

func TestMatchSummaryAndSeen(t *testing.T) {

	ctrl := gomock.NewController(t)
	repo := NewMockMatchRepository(ctrl)
	svc := NewMatchService(repo, nil, nil, nil)
	repo.EXPECT().GetSummary(gomock.Any(), int64(1)).Return(&model.MatchSummary{Total: 3, New: 2}, nil)
	summary, err := svc.GetSummary(context.Background(), 1)

	if err != nil || summary.Total != 3 || summary.New != 2 {
		t.Fatalf("summary = %v, error = %v", summary, err)
	}

	repo.EXPECT().MarkSeen(gomock.Any(), int64(1), int64(20)).Return(model.ErrNotFound)

	if err := svc.MarkSeen(context.Background(), 1, 20); !errors.Is(err, model.ErrMatchNotFound) {
		t.Fatal(err)
	}

	if _, err := svc.GetSummary(context.Background(), 0); !errors.Is(err, model.ErrInvalidMatchesRequest) {
		t.Fatal(err)
	}

	if err := svc.MarkSeen(context.Background(), 1, 0); !errors.Is(err, model.ErrInvalidMatchesRequest) {
		t.Fatal(err)
	}
}

func TestMatchAge(t *testing.T) {

	if age := model.AgeAt(time.Date(2000, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)); age != 25 {
		t.Fatalf("age = %d", age)
	}
}
