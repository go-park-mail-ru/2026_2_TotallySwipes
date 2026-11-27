package grpchandler_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"slices"
	"testing"
	"time"

	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/grpcclient"
	"dating-app/internal/grpcserver"
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"dating-app/services/profile/internal/grpchandler"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// fakeProfiles записывает аргументы и отдаёт заранее заданные ответы.
type fakeProfiles struct {
	profile  *model.Profile
	err      error
	patch    *model.ProfilePatch
	upload   model.PhotoUpload
	cursor   *int64
	limit    int
	batchIDs []int64
	filter   *model.ProfilesFilter
	batch    []model.Profile
	next     *int64
}

func (f *fakeProfiles) GetMyProfile(context.Context, int64) (*model.Profile, error) {
	return f.profile, f.err
}
func (f *fakeProfiles) UpdateProfile(_ context.Context, _ int64, p *model.ProfilePatch) (*model.Profile, error) {
	f.patch = p
	return f.profile, f.err
}
func (f *fakeProfiles) GetShortProfile(context.Context, int64) (*model.ProfileShort, error) {
	return nil, f.err
}
func (f *fakeProfiles) AddPhoto(_ context.Context, _ int64, u model.PhotoUpload) ([]model.Photo, error) {
	f.upload = u
	return f.profile.Photos, f.err
}
func (f *fakeProfiles) DeletePhoto(context.Context, int64, int64) ([]model.Photo, error) {
	return nil, f.err
}
func (f *fakeProfiles) ReorderPhotos(context.Context, int64, []int64) ([]model.Photo, error) {
	return nil, f.err
}
func (f *fakeProfiles) SearchCandidates(_ context.Context, _ int64, limit int, cursor *int64) ([]model.FeedCandidate, *int64, error) {
	f.limit, f.cursor = limit, cursor
	out := make([]model.FeedCandidate, 0, len(f.batch))
	for _, p := range f.batch {
		out = append(out, model.FeedCandidate{Profile: p})
	}
	return out, f.next, f.err
}
func (f *fakeProfiles) GetProfilesBatch(_ context.Context, ids []int64, filter *model.ProfilesFilter) ([]model.Profile, error) {
	f.batchIDs, f.filter = ids, filter
	return f.batch, f.err
}

type fakePhotos struct {
	data []byte
	err  error
}

type nopReadSeekCloser struct{ *bytes.Reader }

func (nopReadSeekCloser) Close() error { return nil }

func (f *fakePhotos) OpenPhoto(context.Context, string) (io.ReadSeekCloser, time.Time, error) {
	if f.err != nil {
		return nil, time.Time{}, f.err
	}
	return nopReadSeekCloser{bytes.NewReader(f.data)}, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), nil
}

type fakeFilters struct{}

func (fakeFilters) Get(context.Context, int64) (*model.SearchFilter, error) {
	f := model.DefaultSearchFilter
	return &f, nil
}
func (fakeFilters) Set(_ context.Context, _ int64, f model.SearchFilter) (*model.SearchFilter, error) {
	return &f, nil
}

type fakeTests struct{ answers *model.TestAnswers }

func (fakeTests) GetCurrentTest(context.Context) (*model.Test, error) { return &model.Test{ID: 1}, nil }
func (f *fakeTests) SubmitTestAnswers(_ context.Context, _ int64, a *model.TestAnswers) (*model.TestResult, error) {
	f.answers = a
	return &model.TestResult{ID: 5, TestID: a.TestID}, nil
}
func (fakeTests) GetMyTestResult(context.Context, int64) (*model.TestResult, error) {
	return nil, model.ErrTestResultNotFound
}

type fakeLocations struct{ location *model.UserLocation }

func (fakeLocations) GetLocationPreview(context.Context, int64, *model.Coordinates) (*model.UserLocationPreview, error) {
	return nil, model.ErrGeocoderTimeout
}
func (fakeLocations) SaveLocation(context.Context, int64, string) (*model.UserLocation, error) {
	return nil, model.ErrLocationPreviewUnavailable
}
func (f fakeLocations) GetLocation(context.Context, int64) (*model.UserLocation, error) {
	return f.location, nil
}

type deps struct {
	profiles  *fakeProfiles
	photos    *fakePhotos
	tests     *fakeTests
	locations fakeLocations
}

// startServer поднимает настоящий gRPC-сервер с интерсепторами и клиент с grpcclient.UnaryInterceptor.
func startServer(t *testing.T, d deps) profilepb.ProfileServiceClient {
	t.Helper()
	if d.profiles == nil {
		d.profiles = &fakeProfiles{}
	}
	if d.photos == nil {
		d.photos = &fakePhotos{}
	}
	if d.tests == nil {
		d.tests = &fakeTests{}
	}

	lis := bufconn.Listen(grpcserver.MaxMsgSize * 2)
	srv := grpcserver.New()
	profilepb.RegisterProfileServiceServer(srv, grpchandler.NewServer(d.profiles, d.photos, fakeFilters{}, d.tests, d.locations))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcclient.MaxMsgSize), grpc.MaxCallSendMsgSize(grpcclient.MaxMsgSize)),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryInterceptor(time.Second)),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return profilepb.NewProfileServiceClient(conn)
}

func ptr[T any](v T) *T { return &v }

func filledProfile(userID int64) *model.Profile {
	birth := time.Date(2000, 5, 17, 0, 0, 0, 0, time.UTC)
	sex, goal := model.SexFemale, model.DatingGoalFriendship
	p := &model.Profile{
		ID:              userID + 100,
		UserID:          userID,
		HasSearchFilter: true,
		City:            ptr("Москва"),
		Tags:            []model.Tag{{ID: 1, Name: "coffee"}},
		Photos:          []model.Photo{{ID: 9, URL: "https://x/media/photos/a.jpg"}},
		CurrentPsycho:   &model.ProfilePsycho{Openness: ptr(0.5)},
	}
	p.CurrentVersion.ProfileFields = model.ProfileFields{Name: ptr("Anna"), BirthDate: &birth, Sex: &sex, DatingGoal: &goal}
	return p
}

func TestGetMyProfileMapsFields(t *testing.T) {
	client := startServer(t, deps{profiles: &fakeProfiles{profile: filledProfile(7)}})

	resp, err := client.GetMyProfile(context.Background(), &profilepb.GetMyProfileRequest{UserId: 7})
	if err != nil {
		t.Fatalf("GetMyProfile: %v", err)
	}
	p := resp.GetProfile()
	if p.GetUserId() != 7 || p.GetProfileId() != 107 || p.GetName() != "Anna" || p.GetSex() != "female" || p.GetCity() != "Москва" {
		t.Fatalf("profile = %+v", p)
	}
	if d := p.GetBirthDate(); d.GetYear() != 2000 || d.GetMonth() != 5 || d.GetDay() != 17 {
		t.Fatalf("birth_date = %+v", d)
	}
	if p.Age == nil || len(p.GetMissing()) != 0 || !slices.Equal(p.GetTags(), []string{"coffee"}) || p.GetPhotos()[0].GetId() != 9 {
		t.Fatalf("profile = %+v", p)
	}
}

func TestUpdateProfileBuildsPatchFromMask(t *testing.T) {
	fake := &fakeProfiles{profile: filledProfile(7)}
	client := startServer(t, deps{profiles: fake})

	_, err := client.UpdateProfile(context.Background(), &profilepb.UpdateProfileRequest{
		UserId: 7,
		Fields: &profilepb.ProfileFields{
			Name:      ptr("Ann"),
			BirthDate: &profilepb.Date{Year: 1999, Month: 2, Day: 3},
			Height:    ptr(int32(170)),
			Work:      ptr("ignored: not in mask"),
			Tags:      []string{},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name", "birth_date", "height", "about_me", "tags"}},
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	p := fake.patch
	if p.Name == nil || *p.Name != "Ann" || p.BirthDate == nil || p.BirthDate.Year() != 1999 {
		t.Fatalf("patch = %+v", p)
	}
	if !p.Height.Set || p.Height.Value == nil || *p.Height.Value != 170 {
		t.Fatalf("height = %+v", p.Height)
	}
	if !p.AboutMe.Set || p.AboutMe.Value != nil {
		t.Fatalf("about_me must be cleared: %+v", p.AboutMe)
	}
	if p.Work.Set || p.Sex != nil {
		t.Fatalf("fields outside mask must stay untouched: %+v", p)
	}
	if p.Tags == nil || len(*p.Tags) != 0 {
		t.Fatalf("tags must be replaced with empty list: %v", p.Tags)
	}
}

func TestUpdateProfileRejectsBadMask(t *testing.T) {
	client := startServer(t, deps{profiles: &fakeProfiles{profile: filledProfile(7)}})

	for name, req := range map[string]*profilepb.UpdateProfileRequest{
		"unknown path":      {UserId: 7, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"password"}}},
		"clear required":    {UserId: 7, Fields: &profilepb.ProfileFields{}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}}},
		"invalid date":      {UserId: 7, Fields: &profilepb.ProfileFields{BirthDate: &profilepb.Date{Year: 2001, Month: 2, Day: 30}}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"birth_date"}}},
		"non-positive user": {UserId: 0},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := client.UpdateProfile(context.Background(), req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument (%v)", status.Code(err), err)
			}
		})
	}
}

func TestDomainErrorsReachClient(t *testing.T) {
	client := startServer(t, deps{profiles: &fakeProfiles{err: model.ErrLastPhoto}})

	_, err := client.DeletePhoto(context.Background(), &profilepb.DeletePhotoRequest{UserId: 7, PhotoId: 1})
	if !errors.Is(err, model.ErrLastPhoto) {
		t.Fatalf("err = %v, want ErrLastPhoto", err)
	}

	_, err = client.GetMyTestResult(context.Background(), &profilepb.GetMyTestResultRequest{UserId: 7})
	if !errors.Is(err, model.ErrTestResultNotFound) {
		t.Fatalf("err = %v, want ErrTestResultNotFound", err)
	}

	_, err = client.PreviewLocation(context.Background(), &profilepb.PreviewLocationRequest{UserId: 7})
	if !errors.Is(err, model.ErrInvalidCoordinates) {
		t.Fatalf("missing coordinates: err = %v, want ErrInvalidCoordinates", err)
	}
}

func TestUnexpectedErrorIsInternal(t *testing.T) {
	client := startServer(t, deps{profiles: &fakeProfiles{err: errors.New("connection reset by peer")}})

	_, err := client.GetMyProfile(context.Background(), &profilepb.GetMyProfileRequest{UserId: 7})
	if status.Code(err) != codes.Internal || status.Convert(err).Message() != "internal error" {
		t.Fatalf("err = %v, want bare Internal", err)
	}
}

func TestAddPhotoLimits(t *testing.T) {
	fake := &fakeProfiles{profile: filledProfile(7)}
	client := startServer(t, deps{profiles: fake})

	data := bytes.Repeat([]byte{1}, validate.PhotoMaxSize)
	resp, err := client.AddPhoto(context.Background(), &profilepb.AddPhotoRequest{UserId: 7, Data: data, Ext: ".jpg"})
	if err != nil {
		t.Fatalf("max-size photo must pass through gRPC: %v", err)
	}
	if len(fake.upload.Data) != validate.PhotoMaxSize || fake.upload.Ext != ".jpg" || len(resp.GetPhotos()) != 1 {
		t.Fatalf("upload = %d bytes %q, photos = %v", len(fake.upload.Data), fake.upload.Ext, resp.GetPhotos())
	}

	_, err = client.AddPhoto(context.Background(), &profilepb.AddPhotoRequest{UserId: 7, Data: append(data, 1), Ext: ".jpg"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("oversized photo: code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestGetPhoto(t *testing.T) {
	client := startServer(t, deps{photos: &fakePhotos{data: []byte("jpeg")}})

	resp, err := client.GetPhoto(context.Background(), &profilepb.GetPhotoRequest{Key: "photos/a.jpg"})
	if err != nil {
		t.Fatalf("GetPhoto: %v", err)
	}
	if string(resp.GetData()) != "jpeg" || resp.GetFilename() != "a.jpg" || resp.GetLastModified().AsTime().Year() != 2026 {
		t.Fatalf("resp = %+v", resp)
	}

	client = startServer(t, deps{photos: &fakePhotos{err: model.ErrInvalidPhotoStorageKey}})
	_, err = client.GetPhoto(context.Background(), &profilepb.GetPhotoRequest{Key: "../etc/passwd"})
	if !errors.Is(err, model.ErrPhotoNotFound) {
		t.Fatalf("invalid key: err = %v, want ErrPhotoNotFound", err)
	}
}

func TestSearchCandidatesCursor(t *testing.T) {
	fake := &fakeProfiles{batch: []model.Profile{*filledProfile(11)}, next: ptr(int64(11))}
	client := startServer(t, deps{profiles: fake})

	resp, err := client.SearchCandidates(context.Background(), &profilepb.SearchCandidatesRequest{UserId: 7, Limit: 5, Cursor: ptr("3")})
	if err != nil {
		t.Fatalf("SearchCandidates: %v", err)
	}
	if fake.limit != 5 || fake.cursor == nil || *fake.cursor != 3 {
		t.Fatalf("limit=%d cursor=%v", fake.limit, fake.cursor)
	}
	if resp.GetNextCursor() != "11" || len(resp.GetCandidates()) != 1 || resp.GetCandidates()[0].GetBigFive().GetOpenness() != 0.5 || resp.GetCandidates()[0].GetProfile().GetProfileId() != 111 {
		t.Fatalf("resp = %+v", resp)
	}

	_, err = client.SearchCandidates(context.Background(), &profilepb.SearchCandidatesRequest{UserId: 7, Limit: 5, Cursor: ptr("abc")})
	if !errors.Is(err, model.ErrInvalidFeedRequest) {
		t.Fatalf("bad cursor: err = %v, want ErrInvalidFeedRequest", err)
	}
}

func TestGetProfilesBatchFilter(t *testing.T) {
	fake := &fakeProfiles{batch: []model.Profile{*filledProfile(11)}}
	client := startServer(t, deps{profiles: fake})

	_, err := client.GetProfilesBatch(context.Background(), &profilepb.GetProfilesBatchRequest{
		UserIds: []int64{11, 12},
		Filter:  &profilepb.MatchFilter{Name: ptr("an"), AgeFrom: ptr(int32(20)), Sex: ptr("all")},
	})
	if err != nil {
		t.Fatalf("GetProfilesBatch: %v", err)
	}
	f := fake.filter
	if !slices.Equal(fake.batchIDs, []int64{11, 12}) || f == nil || *f.Name != "an" || *f.AgeFrom != 20 || f.AgeTo != nil || f.Sex != nil {
		t.Fatalf("ids=%v filter=%+v", fake.batchIDs, f)
	}
}

func TestGetProfileChatSummaries(t *testing.T) {
	fake := &fakeProfiles{batch: []model.Profile{*filledProfile(11)}}
	client := startServer(t, deps{profiles: fake})

	resp, err := client.GetProfileChatSummaries(context.Background(), &profilepb.GetProfileChatSummariesRequest{UserIds: []int64{11, 12, 12}})
	if err != nil {
		t.Fatalf("GetProfileChatSummaries: %v", err)
	}
	if len(resp.GetProfiles()) != 1 || resp.GetProfiles()[0].GetPhotoUrl() == "" || resp.GetProfiles()[0].GetName() != "Anna" {
		t.Fatalf("profiles = %+v", resp.GetProfiles())
	}

	wantAge := int32(model.AgeAt(*fake.batch[0].CurrentVersion.BirthDate, time.Now()))

	if resp.GetProfiles()[0].Age == nil || resp.GetProfiles()[0].GetAge() != wantAge {
		t.Fatalf("chat profile age = %v, want %d", resp.GetProfiles()[0].Age, wantAge)
	}

	if !slices.Equal(resp.GetUnavailableUserIds(), []int64{12}) {
		t.Fatalf("unavailable = %v", resp.GetUnavailableUserIds())
	}
	if fake.filter != nil {
		t.Fatalf("chat summaries must not filter: %+v", fake.filter)
	}
}

func TestSubmitTestAnswersRequiresValues(t *testing.T) {
	tests := &fakeTests{}
	client := startServer(t, deps{tests: tests})

	_, err := client.SubmitTestAnswers(context.Background(), &profilepb.SubmitTestAnswersRequest{
		UserId: 7, TestId: 1, Answers: []*profilepb.TestAnswer{{QuestionId: 1}},
	})
	if !errors.Is(err, model.ErrInvalidAnswers) {
		t.Fatalf("err = %v, want ErrInvalidAnswers", err)
	}

	resp, err := client.SubmitTestAnswers(context.Background(), &profilepb.SubmitTestAnswersRequest{
		UserId: 7, TestId: 1, Answers: []*profilepb.TestAnswer{{QuestionId: 1, Value: ptr(int32(4))}},
	})
	if err != nil || resp.GetResult().GetTestId() != 1 || tests.answers.Answers[0].Value != 4 {
		t.Fatalf("resp=%v err=%v answers=%+v", resp, err, tests.answers)
	}
}

func TestGetLocationEmpty(t *testing.T) {
	client := startServer(t, deps{})

	resp, err := client.GetLocation(context.Background(), &profilepb.GetLocationRequest{UserId: 7})
	if err != nil || resp.GetLocation() != nil {
		t.Fatalf("resp=%v err=%v, want empty location", resp, err)
	}
}
