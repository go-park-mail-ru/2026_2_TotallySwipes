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

func TestFilterServiceGet(t *testing.T) {
	saved := model.SearchFilter{Sex: model.SearchSexMale, AgeFrom: 25, AgeTo: 35}
	failure := errors.New("db down")
	for _, tc := range []struct {
		name    string
		stored  *model.SearchFilter
		repoErr error
		want    *model.SearchFilter
		wantErr error
	}{
		{"not set", nil, model.ErrNotFound, &model.DefaultSearchFilter, nil},
		{"stored", &saved, nil, &saved, nil},
		{"database error", nil, failure, nil, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			repo := NewMockFilterRepository(ctrl)
			ctx := context.Background()
			repo.EXPECT().Get(ctx, int64(7)).Return(tc.stored, tc.repoErr).Times(1)
			svc := NewFilterService(repo)
			// Act
			got, err := svc.Get(ctx, 7)
			// Assert
			if !errors.Is(err, tc.wantErr) || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Get()=%+v,%v; want %+v,%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestFilterServiceSet(t *testing.T) {
	for _, wantErr := range []error{nil, errors.New("db down")} {
		t.Run(fmtError(wantErr), func(t *testing.T) {
			// Arrange
			ctrl := gomock.NewController(t)
			repo := NewMockFilterRepository(ctrl)
			ctx := context.Background()
			filter := model.SearchFilter{Sex: model.SearchSexFemale, AgeFrom: 20, AgeTo: 30}
			repo.EXPECT().Upsert(ctx, int64(7), &filter).Return(wantErr).Times(1)
			svc := NewFilterService(repo)
			// Act
			got, err := svc.Set(ctx, 7, filter)
			// Assert
			if !errors.Is(err, wantErr) {
				t.Fatalf("error=%v, want %v", err, wantErr)
			}
			if wantErr != nil {
				if got != nil {
					t.Fatal("expected nil filter")
				}
				return
			}
			if got == nil || *got != filter {
				t.Fatalf("filter=%+v", got)
			}
		})
	}
}

func fmtError(err error) string {
	if err == nil {
		return "success"
	}
	return err.Error()
}
