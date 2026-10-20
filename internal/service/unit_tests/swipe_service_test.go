package service

import (
	"context"
	"dating-app/internal/model"
	. "dating-app/internal/service"
	"errors"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestSwipeService_Swipe(t *testing.T) {
	failure := errors.New("db down")
	for _, tc := range []struct {
		name                         string
		self, incomplete             bool
		profileErr, saveErr, wantErr error
	}{
		{name: "success"},
		{name: "self swipe", self: true, wantErr: model.ErrInvalidSwipe},
		{name: "author not found", profileErr: model.ErrNotFound, wantErr: model.ErrProfileIncomplete},
		{name: "profile read error", profileErr: failure, wantErr: failure},
		{name: "incomplete author", incomplete: true, wantErr: model.ErrProfileIncomplete},
		{name: "target not found", saveErr: model.ErrNotFound, wantErr: model.ErrProfileNotFound},
		{name: "save error", saveErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			repo := NewMockSwipeRepository(ctrl)
			profiles := NewMockProfileRepository(ctrl)
			ctx := context.Background()
			swipe := &model.Swipe{AuthorID: 7, TargetID: 8}
			svc := NewSwipeService(repo, profiles)
			if tc.self {
				swipe.TargetID = swipe.AuthorID
			} else {
				profile := filledProfile(model.Profile{UserID: 7}, "Anna")
				if tc.incomplete {
					profile.Photos = nil
				}
				get := profiles.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&profile, tc.profileErr).Times(1)
				if tc.profileErr == nil && !tc.incomplete {
					repo.EXPECT().SaveSwipeAndMatch(ctx, swipe).After(get).Return(tc.saveErr).Times(1)
				}
			}
			// Act
			err := svc.Swipe(ctx, swipe)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
		})
	}
}
