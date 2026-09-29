package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"dating-app/internal/model"
	. "dating-app/internal/service"
)

type missingTestRepository struct{ TestRepository }

func (missingTestRepository) GetCurrentTest(context.Context) (*model.Test, error) {
	return nil, fmt.Errorf("repository lookup: %w", model.ErrNotFound)
}

func TestDomainErrorsSurviveServiceWrapping(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{"current test", func() error {
			_, err := NewTestService(missingTestRepository{}, nil, nil).GetCurrentTest(ctx)
			return err
		}, model.ErrActiveTestNotFound},
		{"invalid test request", func() error {
			_, err := NewTestService(nil, nil, nil).SubmitTestAnswers(ctx, 1, nil)
			return err
		}, model.ErrInvalidTestRequest},
		{"invalid feed", func() error {
			_, err := NewProfileService(nil, nil, nil).GetNextFeed(ctx, 0, 10, nil)
			return err
		}, model.ErrInvalidFeedRequest},
		{"invalid photo key", func() error {
			_, err := NewLocalPhotoURLProvider("http://localhost").GetURL(ctx, "../secret")
			return err
		}, model.ErrInvalidPhotoStorageKey},
		{"invalid Big Five", func() error {
			_, err := (&CompatibilityServiceImpl{}).CalculateDistance(model.BigFive{}, model.BigFive{})
			return err
		}, model.ErrInvalidBigFive},
		{"invalid definition", func() error {
			_, err := (&CompatibilityServiceImpl{}).CalculateBigFive(nil, nil)
			return err
		}, model.ErrInvalidTestDefinition},
		{"nested answer validation", func() error {
			test := model.NewTIPITest(1)
			for i := int64(1); i <= 10; i++ {
				test.Questions = append(test.Questions, model.Question{ID: i, OperationID: i})
			}
			_, err := (&CompatibilityServiceImpl{}).CalculateBigFive(&test, &model.TestAnswers{TestID: 1})
			return err
		}, model.ErrInvalidAnswers},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == tc.want || !errors.Is(err, tc.want) {
				t.Fatalf("want wrapped %v, got %v", tc.want, err)
			}
		})
	}
}
