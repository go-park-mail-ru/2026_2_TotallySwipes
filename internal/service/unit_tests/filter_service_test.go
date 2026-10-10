package service

import (
	"context"
	"errors"
	"testing"

	"dating-app/internal/model"
	. "dating-app/internal/service"
)

type filterRepositoryMock struct {
	filter *model.SearchFilter
	err    error
}

func (m *filterRepositoryMock) Get(context.Context, int64) (*model.SearchFilter, error) {
	return m.filter, m.err
}
func (m *filterRepositoryMock) Upsert(_ context.Context, _ int64, f *model.SearchFilter) error {
	m.filter = f
	return m.err
}

func TestFilterServiceGet(t *testing.T) {
	saved := model.SearchFilter{Sex: model.SearchSexMale, AgeFrom: 25, AgeTo: 35}
	failure := errors.New("db down")
	for _, tc := range []struct {
		name    string
		repo    *filterRepositoryMock
		want    *model.SearchFilter
		wantErr error
	}{
		{"не задан", &filterRepositoryMock{err: model.ErrNotFound}, &model.DefaultSearchFilter, nil},
		{"задан", &filterRepositoryMock{filter: &saved}, &saved, nil},
		{"ошибка хранилища", &filterRepositoryMock{err: failure}, nil, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewFilterService(tc.repo).Get(context.Background(), 7)
			if !errors.Is(err, tc.wantErr) || (tc.want == nil) != (got == nil) || (got != nil && *got != *tc.want) {
				t.Fatalf("Get() = %+v, %v; want %+v, %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestFilterServiceSet(t *testing.T) {
	repo := &filterRepositoryMock{}
	f := model.SearchFilter{Sex: model.SearchSexFemale, AgeFrom: 20, AgeTo: 30}
	got, err := NewFilterService(repo).Set(context.Background(), 7, f)
	if err != nil || *got != f || *repo.filter != f {
		t.Fatalf("Set() = %+v, %v; saved %+v", got, err, repo.filter)
	}
}
