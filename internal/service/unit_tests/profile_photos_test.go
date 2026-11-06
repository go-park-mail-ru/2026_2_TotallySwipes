package service

import (
	"context"
	"dating-app/internal/model"
	. "dating-app/internal/service"
	"errors"
	"go.uber.org/mock/gomock"
	"reflect"
	"testing"
)

func TestProfileServicePhotoFiles(t *testing.T) {
	failure := errors.New("storage down")
	for _, tc := range []struct {
		name, operation    string
		repoErr, deleteErr error
	}{
		{name: "add", operation: "add"},
		{name: "delete", operation: "delete"},
		{name: "delete file error still returns photos", operation: "delete", deleteErr: failure},
		{name: "last photo", operation: "delete", repoErr: model.ErrLastPhoto},
		{name: "reorder", operation: "reorder"},
		{name: "reorder mismatch", operation: "reorder", repoErr: model.ErrPhotoOrderMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			repo := NewMockProfileRepository(ctrl)
			files := NewMockPhotoStorage(ctrl)
			urls := NewMockPhotoURLProvider(ctrl)
			svc := NewProfileService(repo, nil, urls, files)
			ctx := context.Background()
			upload := model.PhotoUpload{Data: []byte("a"), Ext: ".jpg"}
			var ordered []any
			switch tc.operation {
			case "add":
				ordered = append(ordered, files.EXPECT().Save(ctx, upload.Data, upload.Ext).Return("new.jpg", nil).Times(1), repo.EXPECT().AddPhoto(ctx, int64(7), "new.jpg").Return(tc.repoErr).Times(1))
			case "delete":
				ordered = append(ordered, repo.EXPECT().DeletePhoto(ctx, int64(7), int64(1)).Return("gone.jpg", tc.repoErr).Times(1))
				if tc.repoErr == nil {
					ordered = append(ordered, files.EXPECT().Delete(gomock.Any(), "gone.jpg").Return(tc.deleteErr).Times(1))
				}
			case "reorder":
				ordered = append(ordered, repo.EXPECT().ReorderPhotos(ctx, int64(7), []int64{1}).Return(tc.repoErr).Times(1))
			}
			if tc.repoErr == nil {
				ordered = append(ordered, repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&model.Profile{Photos: []model.Photo{{ID: 1, StorageKey: "left.jpg"}}}, nil).Times(1), urls.EXPECT().GetPhotoURL(ctx, "left.jpg").Return("https://media.example/cats/left.jpg", nil).Times(1))
			}
			gomock.InOrder(ordered...)
			// Act
			var got []model.Photo
			var err error
			switch tc.operation {
			case "add":
				got, err = svc.AddPhoto(ctx, 7, upload)
			case "delete":
				got, err = svc.DeletePhoto(ctx, 7, 1)
			case "reorder":
				got, err = svc.ReorderPhotos(ctx, 7, []int64{1})
			}
			// Assert
			if !errors.Is(err, tc.repoErr) {
				t.Fatalf("error=%v, want %v", err, tc.repoErr)
			}
			if tc.repoErr != nil {
				if got != nil {
					t.Fatal("expected nil photos")
				}
				return
			}
			want := []model.Photo{{ID: 1, StorageKey: "left.jpg", URL: "https://media.example/cats/left.jpg"}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("photos=%+v, want %+v", got, want)
			}
		})
	}
}

func TestProfileService_AddPhoto_CleanupWithoutCancellation(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	repo := NewMockProfileRepository(ctrl)
	files := NewMockPhotoStorage(ctrl)
	svc := NewProfileService(repo, nil, NewMockPhotoURLProvider(ctrl), files)
	type requestKey struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), requestKey{}, "request-id"))
	defer cancel()
	upload := model.PhotoUpload{Data: []byte("data"), Ext: ".jpg"}
	cleanupErr := errors.New("cleanup failed")
	gomock.InOrder(
		files.EXPECT().Save(ctx, upload.Data, upload.Ext).Return("saved.jpg", nil).Times(1),
		repo.EXPECT().AddPhoto(ctx, int64(7), "saved.jpg").DoAndReturn(func(context.Context, int64, string) error { cancel(); return model.ErrPhotoLimit }).Times(1),
		files.EXPECT().Delete(gomock.Any(), "saved.jpg").DoAndReturn(func(cleanup context.Context, _ string) error {
			if cleanup.Err() != nil || cleanup.Done() != nil || cleanup.Value(requestKey{}) != "request-id" {
				t.Fatal("cleanup must retain request values without cancellation")
			}
			return cleanupErr
		}).Times(1),
	)
	// Act
	got, err := svc.AddPhoto(ctx, 7, upload)
	// Assert
	if got != nil || !errors.Is(err, model.ErrPhotoLimit) || errors.Is(err, cleanupErr) {
		t.Fatalf("result=%v error=%v", got, err)
	}
}

func TestProfileService_GetMyProfile_StopsOnSecondURLError(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	repo := NewMockProfileRepository(ctrl)
	urls := NewMockPhotoURLProvider(ctrl)
	ctx := context.Background()
	failure := errors.New("signing failed")
	repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&model.Profile{Photos: []model.Photo{{StorageKey: "1.jpg"}, {StorageKey: "2.jpg"}, {StorageKey: "3.jpg"}}}, nil).Times(1)
	urls.EXPECT().GetPhotoURL(ctx, "1.jpg").Return("https://example.com/1.jpg", nil).Times(1)
	urls.EXPECT().GetPhotoURL(ctx, "2.jpg").Return("", failure).Times(1)
	svc := NewProfileService(repo, nil, urls, nil)
	// Act
	got, err := svc.GetMyProfile(ctx, 7)
	// Assert
	if got != nil || !errors.Is(err, failure) {
		t.Fatalf("profile=%v,error=%v", got, err)
	}
}

func TestProfileService_UpdateProfile(t *testing.T) {
	failure := errors.New("db down")
	for _, tc := range []struct {
		name                       string
		patchErr, readErr, wantErr error
	}{
		{name: "success"}, {name: "missing profile", patchErr: model.ErrNotFound, wantErr: model.ErrProfileRequired},
		{name: "unknown tag", patchErr: model.ErrUnknownTag, wantErr: model.ErrUnknownTag},
		{name: "read error", readErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			repo := NewMockProfileRepository(ctrl)
			urls := NewMockPhotoURLProvider(ctrl)
			ctx := context.Background()
			name := "Anna"
			patch := &model.ProfilePatch{Name: &name}
			profile := &model.Profile{UserID: 7}
			update := repo.EXPECT().PatchProfile(ctx, int64(7), patch).Return(tc.patchErr).Times(1)
			if tc.patchErr == nil {
				repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).After(update).Return(profile, tc.readErr).Times(1)
			}
			svc := NewProfileService(repo, nil, urls, nil)
			// Act
			got, err := svc.UpdateProfile(ctx, 7, patch)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatal("expected nil profile")
				}
				return
			}
			if !reflect.DeepEqual(got, profile) {
				t.Fatalf("profile=%+v", got)
			}
		})
	}
}

func TestProfileService_Missing(t *testing.T) {
	failure := errors.New("db down")
	for _, wantErr := range []error{nil, failure} {
		t.Run(fmtError(wantErr), func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			repo := NewMockProfileRepository(ctrl)
			ctx := context.Background()
			repo.EXPECT().GetByUserIDCurrentProfile(ctx, int64(7)).Return(&model.Profile{}, wantErr).Times(1)
			svc := NewProfileService(repo, nil, nil, nil)
			// Act
			got, err := svc.Missing(ctx, 7)
			// Assert
			if !errors.Is(err, wantErr) {
				t.Fatalf("error=%v", err)
			}
			if wantErr != nil {
				if got != nil {
					t.Fatal("expected nil missing fields")
				}
				return
			}
			if !reflect.DeepEqual(got, []string{"name", "birth_date", "sex", "dating_goal", "search_filter", "photos"}) {
				t.Fatalf("missing=%v", got)
			}
		})
	}
}
