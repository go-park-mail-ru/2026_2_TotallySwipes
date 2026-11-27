package mapper

import (
	"dating-app/gateway/internal/model"
	profilepb "dating-app/gen/go/profile/v1"
	"fmt"
)

func TestResultFromProto(result *profilepb.TestResult) (*model.TestResult, error) {

	if result == nil {
		return nil, fmt.Errorf("proto: missing test result")
	}

	if err := result.GetCompletedAt().CheckValid(); err != nil {
		return nil, fmt.Errorf("proto: invalid test_result.completed_at: %w", err)
	}
	bigFive := result.GetBigFive()

	if bigFive == nil {
		return nil, fmt.Errorf("proto: missing test_result.big_five")
	}

	return &model.TestResult{
		ID:         result.GetResultId(),
		TestID:     result.GetTestId(),
		Revision:   int(result.GetRevision()),
		CompletedA: result.GetCompletedAt().AsTime(),
		BigFive: model.BigFive{
			Openness:          bigFive.Openness,
			Conscientiousness: bigFive.Conscientiousness,
			Extraversion:      bigFive.Extraversion,
			Agreeableness:     bigFive.Agreeableness,
			Neuroticism:       bigFive.Neuroticism,
		},
		PersonalityType:      model.PersonalityType(result.GetPersonalityType()),
		AboutPersonalityType: result.GetAboutPersonalityType()}, nil
}

func CurrentTestFromProto(response *profilepb.GetCurrentTestResponse) (*model.Test, error) {
	test := response.GetTest()

	if test == nil {
		return nil, fmt.Errorf("proto: missing test")
	}
	result := &model.Test{
		ID:            test.GetTestId(),
		Title:         test.GetTitle(),
		Instructions:  test.GetInstructions(),
		AnswerOptions: make([]model.AnswerOption, 0, len(test.GetAnswerOptions())),
		Questions:     make([]model.Question, 0, len(test.GetQuestions())),
	}

	for _, option := range test.GetAnswerOptions() {

		if option == nil {
			return nil, fmt.Errorf("proto: missing test.answer_option")
		}

		result.AnswerOptions = append(result.AnswerOptions, model.AnswerOption{Value: int(option.GetValue()), Label: option.GetLabel()})
	}

	for _, question := range test.GetQuestions() {

		if question == nil {
			return nil, fmt.Errorf("proto: missing test.question")
		}

		result.Questions = append(result.Questions, model.Question{ID: question.GetId(), Body: question.GetBody()})
	}

	return result, nil
}

func TestAnswersToProto(userID int64, answers *model.TestAnswers) (*profilepb.SubmitTestAnswersRequest, error) {
	if answers == nil {
		return nil, fmt.Errorf("missing test answers")
	}

	items := make([]*profilepb.TestAnswer, 0, len(answers.Answers))

	for _, answer := range answers.Answers {
		value := int32(answer.Value)
		items = append(items, &profilepb.TestAnswer{QuestionId: answer.QuestionID, Value: &value})
	}

	return &profilepb.SubmitTestAnswersRequest{UserId: userID, TestId: answers.TestID, Answers: items}, nil
}
