package service

import (
	"context"
	"errors"
	"testing"

	"dating-app/internal/model"
	"dating-app/internal/psychotest"
	. "dating-app/internal/service"
)

func TestDomainErrorsSurviveServiceWrapping(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{"invalid test request", func() error {
			_, err := NewPsychoTestService(nil, nil, nil, model.Test{}).SubmitTestAnswers(ctx, 1, nil)
			return err
		}, model.ErrInvalidTestRequest},
		{"invalid feed", func() error {
			_, err := NewProfileService(nil, nil, nil, nil).GetNextFeed(ctx, 0, 10, nil)
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
			test, err := psychotest.Load()
			if err != nil {
				return err
			}
			_, err = (&CompatibilityServiceImpl{}).CalculateBigFive(&test, &model.TestAnswers{TestID: test.ID})
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
