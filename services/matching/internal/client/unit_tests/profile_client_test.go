package client_test

import (
	"context"
	"testing"

	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/services/matching/internal/client"
	"dating-app/services/matching/internal/model"
	"google.golang.org/grpc"
)

type profilesClient struct {
	profilepb.ProfileServiceClient
	batch         *profilepb.GetProfilesBatchResponse
	candidates    *profilepb.SearchCandidatesResponse
	batchRequest  *profilepb.GetProfilesBatchRequest
	searchRequest *profilepb.SearchCandidatesRequest
}

func (c *profilesClient) GetProfilesBatch(_ context.Context, request *profilepb.GetProfilesBatchRequest, _ ...grpc.CallOption) (*profilepb.GetProfilesBatchResponse, error) {

	c.batchRequest = request
	return c.batch, nil
}

func (c *profilesClient) SearchCandidates(_ context.Context, request *profilepb.SearchCandidatesRequest, _ ...grpc.CallOption) (*profilepb.SearchCandidatesResponse, error) {

	c.searchRequest = request
	return c.candidates, nil
}

func ptr[T any](v T) *T {

	return &v
}

func protoProfile() *profilepb.ProfileWithBigFive {

	return &profilepb.ProfileWithBigFive{Profile: &profilepb.Profile{ProfileId: 102, UserId: 2, Name: ptr("Анна"), BirthDate: &profilepb.Date{Year: 2000, Month: 12, Day: 1}, Age: ptr(int32(25)), DatingGoal: ptr("relationship"), Education: ptr("higher"), Height: ptr(int32(170)), Photos: []*profilepb.Photo{{Id: 5, Url: "/media/photo"}}, Tags: []string{"кино"}}, BigFive: &profilepb.BigFive{Openness: ptr(0.5)}}
}

func TestProfileAdapter(t *testing.T) {

	rpc := &profilesClient{batch: &profilepb.GetProfilesBatchResponse{Profiles: []*profilepb.ProfileWithBigFive{protoProfile()}}, candidates: &profilepb.SearchCandidatesResponse{Candidates: []*profilepb.ProfileWithBigFive{protoProfile()}, NextCursor: ptr("2")}}
	adapter := client.NewProfileAdapter(rpc)
	profiles, err := adapter.GetProfilesBatch(context.Background(), []int64{2}, model.MatchFilter{Name: ptr("Анна"), AgeFrom: ptr(20)})

	if err != nil {
		t.Fatal(err)
	}

	p := profiles[2]

	if p.ID != 102 || p.UserID != 2 || *p.Name != "Анна" || *p.Age != 25 || *p.Height != 170 || p.Photos[0].URL != "/media/photo" || *p.Psycho.Openness != 0.5 || rpc.batchRequest.Filter.GetAgeFrom() != 20 {
		t.Fatalf("profile = %#v, request = %v", p, rpc.batchRequest)
	}

	candidates, next, err := adapter.SearchCandidates(context.Background(), 1, 10, ptr(int64(1)))

	if err != nil || len(candidates) != 1 || next == nil || *next != 2 || rpc.searchRequest.GetCursor() != "1" || rpc.searchRequest.Limit != 10 {
		t.Fatalf("candidates = %v, next = %v, error = %v", candidates, next, err)
	}
}

func TestInvalidProfileResponses(t *testing.T) {

	for _, name := range []string{"нет ответа", "нет анкеты", "неверная дата", "чужая анкета", "дубликат"} {
		t.Run(name, func(t *testing.T) {

			rpc := &profilesClient{batch: &profilepb.GetProfilesBatchResponse{Profiles: []*profilepb.ProfileWithBigFive{protoProfile()}}}

			switch name {
			case "нет ответа":

				rpc.batch = nil

			case "нет анкеты":

				rpc.batch.Profiles[0] = nil

			case "неверная дата":

				rpc.batch.Profiles[0].Profile.BirthDate.Day = 32

			case "чужая анкета":

				rpc.batch.Profiles[0].Profile.UserId = 3

			case "дубликат":

				rpc.batch.Profiles = append(rpc.batch.Profiles, protoProfile())
			}

			if _, err := client.NewProfileAdapter(rpc).GetProfilesBatch(context.Background(), []int64{2}, model.MatchFilter{}); err == nil {
				t.Fatal("expected proto validation error")
			}
		})
	}
}
