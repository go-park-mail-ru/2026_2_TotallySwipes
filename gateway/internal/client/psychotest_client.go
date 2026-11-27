package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	"dating-app/gateway/internal/model"
	profilepb "dating-app/gen/go/profile/v1"
)

type PsychoTestAdapter struct {
	client profilepb.ProfileServiceClient
}

func NewPsychoTestAdapter(client profilepb.ProfileServiceClient) *PsychoTestAdapter {
	return &PsychoTestAdapter{client: client}
}

func (a *PsychoTestAdapter) GetCurrentTest(ctx context.Context) (*model.Test, error) {
	response, err := a.client.GetCurrentTest(ctx, &profilepb.GetCurrentTestRequest{})

	if err != nil {
		return nil, err
	}
	return mapper.CurrentTestFromProto(response)
}

func (a *PsychoTestAdapter) SubmitTestAnswers(ctx context.Context, userID int64, answers *model.TestAnswers) (*model.TestResult, error) {

	request, err := mapper.TestAnswersToProto(userID, answers)

	if err != nil {
		return nil, err
	}

	response, err := a.client.SubmitTestAnswers(ctx, request)

	if err != nil {
		return nil, err
	}

	return mapper.TestResultFromProto(response.GetResult())
}

func (a *PsychoTestAdapter) GetMyTestResult(ctx context.Context, userID int64) (*model.TestResult, error) {
	response, err := a.client.GetMyTestResult(ctx, &profilepb.GetMyTestResultRequest{UserId: userID})

	if err != nil {
		return nil, err
	}

	return mapper.TestResultFromProto(response.GetResult())
}
