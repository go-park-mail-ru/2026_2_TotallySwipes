package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	profilepb "dating-app/gen/go/profile/v1"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
	delivery "dating-app/services/matching/internal/delivery/grpc"
	"dating-app/services/matching/internal/model"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//go:generate go run go.uber.org/mock/mockgen -source=../match_handler.go -destination=match_mocks_test.go -package=grpc_test

func ptr[T any](v T) *T {

	return &v
}

func TestRPCMappings(t *testing.T) {

	ctrl := gomock.NewController(t)
	matches, feed, swipes, events := NewMockMatchService(ctrl), NewMockFeedService(ctrl), NewMockSwipeService(ctrl), NewMockEventService(ctrl)
	h := delivery.NewMatchHandler(matches, feed, swipes, events)
	ctx := context.Background()
	now := time.Now()
	filter := model.MatchFilter{Name: ptr("Анна"), AgeFrom: ptr(20), Sex: ptr("female")}
	matches.EXPECT().GetMatches(ctx, int64(1), 12, (*int64)(nil), filter).Return(&model.MatchPage{Total: 5, NextCursor: ptr(int64(20)), Items: []model.Match{
		{ID: 20, ChatID: ptr(int64(9007199254740993)), UserID: 2, Name: ptr("Анна"), Age: ptr(25), CreatedAt: now, IsNew: true, CanMeetup: true},
		{ID: 19, UserID: 3, CreatedAt: now},
	}}, nil)
	page, err := h.GetMatches(ctx, &matchpb.GetMatchesRequest{UserId: 1, Filter: &profilepb.MatchFilter{Name: ptr("Анна"), AgeFrom: ptr(int32(20)), Sex: ptr("female")}})

	if err != nil || page.Total != 5 || page.GetNextCursor() != 20 || page.Items[0].GetAge() != 25 || !page.Items[0].IsNew || !page.Items[0].CanMeetup {
		t.Fatalf("page = %v, error = %v", page, err)
	}

	if len(page.Items) != 2 || page.Items[0].GetChatId() != "9007199254740993" || page.Items[1].ChatId != nil {
		t.Fatalf("chat IDs were not preserved: %v", page.Items)
	}

	matches.EXPECT().GetSummary(ctx, int64(1)).Return(&model.MatchSummary{Total: 5, New: 2}, nil)
	summary, err := h.GetSummary(ctx, &matchpb.GetSummaryRequest{UserId: 1})

	if err != nil || summary.Total != 5 || summary.NewCount != 2 || !summary.HasNew {
		t.Fatalf("summary = %v, error = %v", summary, err)
	}

	matches.EXPECT().MarkSeen(ctx, int64(1), int64(20)).Return(nil)

	if _, err := h.MarkSeen(ctx, &matchpb.MarkSeenRequest{UserId: 1, MatchId: 20}); err != nil {
		t.Fatal(err)
	}

	item := model.FeedItem{UserID: 2, Name: "Анна", Age: 25, DatingGoal: "relationship", LikedMe: true, Photos: []model.FeedPhoto{{ID: 10, URL: "/media/photo"}}, Education: ptr(model.Education("higher"))}
	feed.EXPECT().GetNextFeed(ctx, int64(1), 10, ptr(int64(3))).Return(&model.FeedPage{Items: []model.FeedItem{item}, NextCursor: ptr(int64(5))}, nil)
	candidates, err := h.GetNextFeed(ctx, &matchpb.GetNextFeedRequest{UserId: 1, Cursor: ptr(int64(3))})

	if err != nil || candidates.GetNextCursor() != 5 || !candidates.Items[0].LikedMe || candidates.Items[0].Photos[0].Url != "/media/photo" || candidates.Items[0].GetEducation() != "higher" {
		t.Fatalf("feed = %v, error = %v", candidates, err)
	}

	matches.EXPECT().GetMatchedProfile(ctx, int64(1), int64(2)).Return(&model.MatchedProfile{MatchID: 20, Item: item}, nil)
	profile, err := h.GetMatchedProfile(ctx, &matchpb.GetMatchedProfileRequest{UserId: 1, TargetUserId: 2})

	if err != nil || profile.MatchId != 20 || profile.Item.UserId != 2 {
		t.Fatalf("profile = %v, error = %v", profile, err)
	}

	swipes.EXPECT().Swipe(ctx, &model.Swipe{AuthorID: 1, TargetID: 2, Action: model.SwipeSuperLike}).Return(nil)

	if _, err := h.Swipe(ctx, &matchpb.SwipeRequest{UserId: 1, TargetUserId: 2, Action: matchpb.SwipeAction_SWIPE_ACTION_SUPER_LIKE}); err != nil {
		t.Fatal(err)
	}

	id := uuid.New()
	state := &model.SearchEventsState{SearchID: id, City: "Москва", ExpiresAt: now.Add(time.Hour), Status: model.SearchStatusVoting, Events: []model.Event{{ID: 8, Name: "Событие", StartsAt: now, Location: model.EventLocation{City: "Москва"}, Categories: []model.EventCategory{{ID: 9, Name: "Кино"}}}}}
	events.EXPECT().SearchEventsByCity(ctx, int64(1), int64(20), "Москва").Return(state, true, nil)
	search, err := h.SearchEventsByCity(ctx, &matchpb.SearchEventsByCityRequest{UserId: 1, MatchId: 20, City: "Москва"})

	if err != nil || !search.Created || search.State.SearchId != id.String() || search.State.Events[0].Categories[0].Name != "Кино" {
		t.Fatalf("search = %v, error = %v", search, err)
	}

	events.EXPECT().GetSearchEvents(ctx, int64(1), int64(20), id).Return(state, nil)

	if _, err := h.GetSearchEvents(ctx, &matchpb.GetSearchEventsRequest{UserId: 1, MatchId: 20, SearchId: id.String()}); err != nil {
		t.Fatal(err)
	}

	events.EXPECT().SubmitResults(ctx, int64(1), int64(20), id, []int64{8}).Return(state, nil)

	if _, err := h.SubmitResults(ctx, &matchpb.SubmitResultsRequest{UserId: 1, MatchId: 20, SearchId: id.String(), LikedEventIds: []int64{8}}); err != nil {
		t.Fatal(err)
	}
}

func TestErrors(t *testing.T) {

	for _, tc := range []struct {
		err  error
		code codes.Code
	}{
		{model.ErrInvalidMatchesRequest, codes.InvalidArgument}, {model.ErrProfileRequired, codes.FailedPrecondition},
		{model.ErrProfileIncomplete, codes.PermissionDenied}, {model.ErrMatchNotFound, codes.NotFound},
		{model.ErrResultAlreadySubmitted, codes.AlreadyExists}, {context.Canceled, codes.Canceled},
		{context.DeadlineExceeded, codes.DeadlineExceeded}, {errors.New("private database details"), codes.Internal},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {

			svc := NewMockMatchService(gomock.NewController(t))
			svc.EXPECT().GetSummary(gomock.Any(), int64(1)).Return(nil, tc.err)
			_, err := delivery.NewMatchHandler(svc, nil, nil, nil).GetSummary(context.Background(), &matchpb.GetSummaryRequest{UserId: 1})

			if status.Code(err) != tc.code {
				t.Fatalf("error = %v, want %v", err, tc.code)
			}

			if tc.code == codes.Internal && status.Convert(err).Message() != "internal matching error" {
				t.Fatal(err)
			}
		})
	}
}

func TestInvalidRequests(t *testing.T) {

	h := delivery.NewMatchHandler(nil, nil, nil, nil)
	ctx := context.Background()
	checks := []func() error{
		func() error {

			_, err := h.GetMatches(ctx, nil)
			return err
		},
		func() error {

			_, err := h.GetSummary(ctx, nil)
			return err
		},
		func() error {

			_, err := h.MarkSeen(ctx, nil)
			return err
		},
		func() error {

			_, err := h.GetNextFeed(ctx, nil)
			return err
		},
		func() error {

			_, err := h.GetMatchedProfile(ctx, nil)
			return err
		},
		func() error {

			_, err := h.Swipe(ctx, nil)
			return err
		},
		func() error {

			_, err := h.SearchEventsByCity(ctx, nil)
			return err
		},
		func() error {

			_, err := h.GetSearchEvents(ctx, nil)
			return err
		},
		func() error {

			_, err := h.SubmitResults(ctx, nil)
			return err
		},
		func() error {

			_, err := h.GetSearchEvents(ctx, &matchpb.GetSearchEventsRequest{SearchId: "bad"})
			return err
		},
		func() error {

			_, err := h.Swipe(ctx, &matchpb.SwipeRequest{})
			return err
		},
	}

	for _, check := range checks {
		if err := check(); status.Code(err) != codes.InvalidArgument {
			t.Fatal(err)
		}
	}
}
