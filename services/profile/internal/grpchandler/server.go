// Package grpchandler реализует ProfileService поверх сервисов анкеты, фото, теста, фильтров и геолокации.
package grpchandler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"time"

	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/model"
	"dating-app/internal/validate"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProfileService interface {
	GetMyProfile(ctx context.Context, userID int64) (*model.Profile, error)
	UpdateProfile(ctx context.Context, userID int64, patch *model.ProfilePatch) (*model.Profile, error)
	GetShortProfile(ctx context.Context, userID int64) (*model.ProfileShort, error)
	AddPhoto(ctx context.Context, userID int64, upload model.PhotoUpload) ([]model.Photo, error)
	DeletePhoto(ctx context.Context, userID, photoID int64) ([]model.Photo, error)
	ReorderPhotos(ctx context.Context, userID int64, photoIDs []int64) ([]model.Photo, error)
	SearchCandidates(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.FeedCandidate, *int64, error)
	GetProfilesBatch(ctx context.Context, userIDs []int64, filter *model.ProfilesFilter) ([]model.Profile, error)
}

// PhotoReader открывает файл фото по ключу хранилища.
type PhotoReader interface {
	OpenPhoto(ctx context.Context, key string) (io.ReadSeekCloser, time.Time, error)
}

type FilterService interface {
	Get(ctx context.Context, userID int64) (*model.SearchFilter, error)
	Set(ctx context.Context, userID int64, f model.SearchFilter) (*model.SearchFilter, error)
}

type PsychoTestService interface {
	GetCurrentTest(ctx context.Context) (*model.Test, error)
	SubmitTestAnswers(ctx context.Context, userID int64, answers *model.TestAnswers) (*model.TestResult, error)
	GetMyTestResult(ctx context.Context, userID int64) (*model.TestResult, error)
}

type LocationService interface {
	GetLocationPreview(ctx context.Context, userID int64, coordinates *model.Coordinates) (*model.UserLocationPreview, error)
	SaveLocation(ctx context.Context, userID int64, previewID string) (*model.UserLocation, error)
	GetLocation(ctx context.Context, userID int64) (*model.UserLocation, error)
}

type Server struct {
	profilepb.UnimplementedProfileServiceServer

	profiles  ProfileService
	photos    PhotoReader
	filters   FilterService
	tests     PsychoTestService
	locations LocationService
	now       func() time.Time
}

func NewServer(profiles ProfileService, photos PhotoReader, filters FilterService, tests PsychoTestService, locations LocationService) *Server {
	return &Server{profiles: profiles, photos: photos, filters: filters, tests: tests, locations: locations, now: time.Now}
}

// --- анкета ---

func (s *Server) GetMyProfile(ctx context.Context, req *profilepb.GetMyProfileRequest) (*profilepb.GetMyProfileResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	profile, err := s.profiles.GetMyProfile(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return &profilepb.GetMyProfileResponse{Profile: profileToProto(profile, s.now())}, nil
}

func (s *Server) UpdateProfile(ctx context.Context, req *profilepb.UpdateProfileRequest) (*profilepb.UpdateProfileResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	patch, err := patchFromProto(req.GetFields(), req.GetUpdateMask())
	if err != nil {
		return nil, err
	}
	profile, err := s.profiles.UpdateProfile(ctx, req.GetUserId(), patch)
	if err != nil {
		return nil, err
	}
	return &profilepb.UpdateProfileResponse{Profile: profileToProto(profile, s.now())}, nil
}

func (s *Server) GetShortProfile(ctx context.Context, req *profilepb.GetShortProfileRequest) (*profilepb.GetShortProfileResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	short, err := s.profiles.GetShortProfile(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return shortProfileToProto(short), nil
}

// --- фото ---

func (s *Server) AddPhoto(ctx context.Context, req *profilepb.AddPhotoRequest) (*profilepb.AddPhotoResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	if len(req.GetData()) == 0 || len(req.GetData()) > validate.PhotoMaxSize {
		return nil, invalidArgument("photo size must be between 1 and %d bytes", validate.PhotoMaxSize)
	}
	if req.GetExt() == "" {
		return nil, invalidArgument("photo ext is required")
	}
	photos, err := s.profiles.AddPhoto(ctx, req.GetUserId(), model.PhotoUpload{Data: req.GetData(), Ext: req.GetExt()})
	if err != nil {
		return nil, err
	}
	return &profilepb.AddPhotoResponse{Photos: photosToProto(photos)}, nil
}

func (s *Server) GetPhoto(ctx context.Context, req *profilepb.GetPhotoRequest) (*profilepb.GetPhotoResponse, error) {
	body, modTime, err := s.photos.OpenPhoto(ctx, req.GetKey())
	if errors.Is(err, model.ErrInvalidPhotoStorageKey) {
		return nil, fmt.Errorf("%w: %w", model.ErrPhotoNotFound, err)
	}
	if err != nil {
		return nil, err
	}
	defer body.Close()

	// Читаем на байт больше лимита: файл крупнее допустимого не отдаём, а не обрезаем молча.
	data, err := io.ReadAll(io.LimitReader(body, validate.PhotoMaxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read photo %q: %w", req.GetKey(), err)
	}
	if len(data) > validate.PhotoMaxSize {
		return nil, fmt.Errorf("photo %q is larger than %d bytes", req.GetKey(), validate.PhotoMaxSize)
	}

	return &profilepb.GetPhotoResponse{
		Data:         data,
		Filename:     path.Base(req.GetKey()),
		LastModified: timestamppb.New(modTime),
	}, nil
}

func (s *Server) DeletePhoto(ctx context.Context, req *profilepb.DeletePhotoRequest) (*profilepb.DeletePhotoResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	photos, err := s.profiles.DeletePhoto(ctx, req.GetUserId(), req.GetPhotoId())
	if err != nil {
		return nil, err
	}
	return &profilepb.DeletePhotoResponse{Photos: photosToProto(photos)}, nil
}

func (s *Server) ReorderPhotos(ctx context.Context, req *profilepb.ReorderPhotosRequest) (*profilepb.ReorderPhotosResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	photos, err := s.profiles.ReorderPhotos(ctx, req.GetUserId(), req.GetPhotoIds())
	if err != nil {
		return nil, err
	}
	return &profilepb.ReorderPhotosResponse{Photos: photosToProto(photos)}, nil
}

// --- фильтр поиска ---

func (s *Server) GetSearchFilter(ctx context.Context, req *profilepb.GetSearchFilterRequest) (*profilepb.GetSearchFilterResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	filter, err := s.filters.Get(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return &profilepb.GetSearchFilterResponse{Filter: searchFilterToProto(filter)}, nil
}

func (s *Server) SetSearchFilter(ctx context.Context, req *profilepb.SetSearchFilterRequest) (*profilepb.SetSearchFilterResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	in, err := searchFilterFromProto(req.GetFilter())
	if err != nil {
		return nil, err
	}
	filter, err := s.filters.Set(ctx, req.GetUserId(), in)
	if err != nil {
		return nil, err
	}
	return &profilepb.SetSearchFilterResponse{Filter: searchFilterToProto(filter)}, nil
}

// --- психологический тест ---

func (s *Server) GetCurrentTest(ctx context.Context, _ *profilepb.GetCurrentTestRequest) (*profilepb.GetCurrentTestResponse, error) {
	test, err := s.tests.GetCurrentTest(ctx)
	if err != nil {
		return nil, err
	}
	return &profilepb.GetCurrentTestResponse{Test: testToProto(test)}, nil
}

func (s *Server) SubmitTestAnswers(ctx context.Context, req *profilepb.SubmitTestAnswersRequest) (*profilepb.SubmitTestAnswersResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	answers, err := testAnswersFromProto(req)
	if err != nil {
		return nil, err
	}
	result, err := s.tests.SubmitTestAnswers(ctx, req.GetUserId(), answers)
	if err != nil {
		return nil, err
	}
	return &profilepb.SubmitTestAnswersResponse{Result: testResultToProto(result)}, nil
}

func (s *Server) GetMyTestResult(ctx context.Context, req *profilepb.GetMyTestResultRequest) (*profilepb.GetMyTestResultResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	result, err := s.tests.GetMyTestResult(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return &profilepb.GetMyTestResultResponse{Result: testResultToProto(result)}, nil
}

// --- геолокация ---

func (s *Server) PreviewLocation(ctx context.Context, req *profilepb.PreviewLocationRequest) (*profilepb.PreviewLocationResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	coordinates, err := coordinatesFromProto(req.GetCoordinates())
	if err != nil {
		return nil, err
	}
	preview, err := s.locations.GetLocationPreview(ctx, req.GetUserId(), coordinates)
	if err != nil {
		return nil, err
	}
	return &profilepb.PreviewLocationResponse{
		PreviewId: preview.PreviewID,
		City:      preview.Address.City,
		District:  preview.Address.District,
	}, nil
}

func (s *Server) SaveLocation(ctx context.Context, req *profilepb.SaveLocationRequest) (*profilepb.SaveLocationResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	if req.GetPreviewId() == "" {
		return nil, invalidArgument("preview_id is required")
	}
	location, err := s.locations.SaveLocation(ctx, req.GetUserId(), req.GetPreviewId())
	if err != nil {
		return nil, err
	}
	return &profilepb.SaveLocationResponse{Location: locationToProto(location)}, nil
}

// GetLocation возвращает пустой location, если пользователь ещё не указал город.
func (s *Server) GetLocation(ctx context.Context, req *profilepb.GetLocationRequest) (*profilepb.GetLocationResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	location, err := s.locations.GetLocation(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return &profilepb.GetLocationResponse{Location: locationToProto(location)}, nil
}

// --- внутренние вызовы сервисов мэтчинга и чата ---

func (s *Server) SearchCandidates(ctx context.Context, req *profilepb.SearchCandidatesRequest) (*profilepb.SearchCandidatesResponse, error) {
	if err := requireUserID(req.GetUserId()); err != nil {
		return nil, err
	}
	cursor, err := cursorFromProto(req.Cursor)
	if err != nil {
		return nil, err
	}
	candidates, next, err := s.profiles.SearchCandidates(ctx, req.GetUserId(), int(req.GetLimit()), cursor)
	if err != nil {
		return nil, err
	}

	now := s.now()
	resp := &profilepb.SearchCandidatesResponse{
		Candidates: make([]*profilepb.ProfileWithBigFive, 0, len(candidates)),
		NextCursor: cursorToProto(next),
	}
	for i := range candidates {
		resp.Candidates = append(resp.Candidates, profileWithBigFiveToProto(&candidates[i].Profile, now))
	}
	return resp, nil
}

func (s *Server) GetProfilesBatch(ctx context.Context, req *profilepb.GetProfilesBatchRequest) (*profilepb.GetProfilesBatchResponse, error) {
	profiles, err := s.profiles.GetProfilesBatch(ctx, req.GetUserIds(), profilesFilterFromProto(req.GetFilter()))
	if err != nil {
		return nil, err
	}

	now := s.now()
	resp := &profilepb.GetProfilesBatchResponse{Profiles: make([]*profilepb.ProfileWithBigFive, 0, len(profiles))}
	for i := range profiles {
		resp.Profiles = append(resp.Profiles, profileWithBigFiveToProto(&profiles[i], now))
	}
	return resp, nil
}

// GetProfileChatSummaries отдаёт имя, главное фото и теги собеседников; ID без анкеты - в unavailable_user_ids.
func (s *Server) GetProfileChatSummaries(ctx context.Context, req *profilepb.GetProfileChatSummariesRequest) (*profilepb.GetProfileChatSummariesResponse, error) {
	profiles, err := s.profiles.GetProfilesBatch(ctx, req.GetUserIds(), nil)
	if err != nil {
		return nil, err
	}

	resp := &profilepb.GetProfileChatSummariesResponse{
		Profiles:           make([]*profilepb.ProfileChatSummary, 0, len(profiles)),
		UnavailableUserIds: make([]int64, 0),
	}
	found := make(map[int64]bool, len(profiles))
	for _, p := range profiles {
		found[p.UserID] = true
		summary := &profilepb.ProfileChatSummary{
			UserId: p.UserID,
			Name:   p.CurrentVersion.Name,
			Tags:   make([]string, 0, len(p.Tags)),
		}
		if len(p.Photos) > 0 {
			url := p.Photos[0].URL
			summary.PhotoUrl = &url
		}
		for _, tag := range p.Tags {
			summary.Tags = append(summary.Tags, tag.Name)
		}
		resp.Profiles = append(resp.Profiles, summary)
	}
	for _, id := range req.GetUserIds() {
		if !found[id] {
			found[id] = true // каждый недоступный ID - один раз
			resp.UnavailableUserIds = append(resp.UnavailableUserIds, id)
		}
	}
	return resp, nil
}
