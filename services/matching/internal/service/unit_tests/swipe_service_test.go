package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dating-app/services/matching/internal/model"
	. "dating-app/services/matching/internal/service"
	"go.uber.org/mock/gomock"
)

func TestSwipePreservesActions(t *testing.T) {

	for _, action := range []model.SwipeAction{model.SwipeLike, model.SwipeDislike, model.SwipeSuperLike} {
		t.Run(string(action), func(t *testing.T) {

			ctrl := gomock.NewController(t)
			profiles, repo, chat := NewMockProfileProvider(ctrl), NewMockSwipeRepository(ctrl), NewMockChatProvider(ctrl)
			swipe := &model.Swipe{AuthorID: 1, TargetID: 2, Action: action}

			gomock.InOrder(
				profiles.EXPECT().GetProfilesBatch(gomock.Any(), []int64{1, 2}, model.MatchFilter{}).
					Return(map[int64]model.Profile{1: profileFixture(1), 2: profileFixture(2)}, nil),
				repo.EXPECT().SaveSwipeAndMatch(gomock.Any(), swipe, int64(101), int64(102)).Return(int64(0), nil),
			)

			if err := NewSwipeService(repo, profiles, chat).Swipe(context.Background(), swipe); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSwipeEnsuresChat(t *testing.T) {

	for _, action := range []model.SwipeAction{model.SwipeLike, model.SwipeSuperLike} {
		for _, created := range []bool{true, false} {
			name := string(action) + "/существующий чат"

			if created {
				name = string(action) + "/новый чат"
			}

			t.Run(name, func(t *testing.T) {

				ctrl := gomock.NewController(t)
				profiles, repo, chat := NewMockProfileProvider(ctrl), NewMockSwipeRepository(ctrl), NewMockChatProvider(ctrl)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				swipe := &model.Swipe{AuthorID: 2, TargetID: 1, Action: action}

				// В репозиторий идут ID анкет, а в Chat — ID пользователей.
				gomock.InOrder(
					profiles.EXPECT().GetProfilesBatch(ctx, []int64{2, 1}, model.MatchFilter{}).
						Return(map[int64]model.Profile{1: profileFixture(1), 2: profileFixture(2)}, nil),
					repo.EXPECT().SaveSwipeAndMatch(ctx, swipe, int64(102), int64(101)).Return(int64(20), nil),
					chat.EXPECT().EnsureChat(ctx, int64(20), int64(2), int64(1)).Return(int64(30), created, nil),
				)

				if err := NewSwipeService(repo, profiles, chat).Swipe(ctx, swipe); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestSwipeRepositoryError(t *testing.T) {

	ctrl := gomock.NewController(t)
	profiles, repo, chat := NewMockProfileProvider(ctrl), NewMockSwipeRepository(ctrl), NewMockChatProvider(ctrl)
	swipe := &model.Swipe{AuthorID: 1, TargetID: 2, Action: model.SwipeLike}

	gomock.InOrder(
		profiles.EXPECT().GetProfilesBatch(gomock.Any(), []int64{1, 2}, model.MatchFilter{}).
			Return(map[int64]model.Profile{1: profileFixture(1), 2: profileFixture(2)}, nil),
		repo.EXPECT().SaveSwipeAndMatch(gomock.Any(), swipe, int64(101), int64(102)).Return(int64(20), failure),
	)

	if err := NewSwipeService(repo, profiles, chat).Swipe(context.Background(), swipe); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
}

func TestSwipeRetriesChatAfterFailure(t *testing.T) {

	ctrl := gomock.NewController(t)
	profiles, repo, chat := NewMockProfileProvider(ctrl), NewMockSwipeRepository(ctrl), NewMockChatProvider(ctrl)
	swipe := &model.Swipe{AuthorID: 1, TargetID: 2, Action: model.SwipeLike}
	var calls []any

	for _, chatErr := range []error{failure, nil} {

		chatID := int64(30)

		if chatErr != nil {
			chatID = 0
		}

		calls = append(calls,
			profiles.EXPECT().GetProfilesBatch(gomock.Any(), []int64{1, 2}, model.MatchFilter{}).
				Return(map[int64]model.Profile{1: profileFixture(1), 2: profileFixture(2)}, nil),
			repo.EXPECT().SaveSwipeAndMatch(gomock.Any(), swipe, int64(101), int64(102)).Return(int64(20), nil),
			chat.EXPECT().EnsureChat(gomock.Any(), int64(20), int64(1), int64(2)).Return(chatID, chatErr == nil, chatErr),
		)
	}

	gomock.InOrder(calls...)
	svc := NewSwipeService(repo, profiles, chat)
	err := svc.Swipe(context.Background(), swipe)

	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "ensure chat for match 20") {
		t.Fatalf("error = %v; want wrapped Chat error with match ID", err)
	}

	if err := svc.Swipe(context.Background(), swipe); err != nil {
		t.Fatalf("retry error = %v", err)
	}
}

func TestSwipeValidation(t *testing.T) {

	tests := []struct {
		name  string
		swipe *model.Swipe
	}{
		{name: "нет свайпа"},
		{name: "неверный автор", swipe: &model.Swipe{TargetID: 2, Action: model.SwipeLike}},
		{name: "неверный получатель", swipe: &model.Swipe{AuthorID: 1, TargetID: -1, Action: model.SwipeLike}},
		{name: "свайп на себя", swipe: &model.Swipe{AuthorID: 1, TargetID: 1, Action: model.SwipeLike}},
		{name: "неверное действие", swipe: &model.Swipe{AuthorID: 1, TargetID: 2, Action: "unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			svc := NewSwipeService(NewMockSwipeRepository(ctrl), NewMockProfileProvider(ctrl), NewMockChatProvider(ctrl))

			if err := svc.Swipe(context.Background(), tt.swipe); !errors.Is(err, model.ErrInvalidSwipe) {
				t.Fatalf("error = %v, want %v", err, model.ErrInvalidSwipe)
			}
		})
	}
}
