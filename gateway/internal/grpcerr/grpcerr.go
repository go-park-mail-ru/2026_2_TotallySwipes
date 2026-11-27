// Package grpcerr переводит доменные ошибки Gateway в gRPC-статусы и обратно,
// чтобы вызывающая сторона снова могла сравнивать их через errors.Is.
package grpcerr

import (
	"context"
	"errors"
	"maps"

	"dating-app/gateway/internal/model"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Domain - домен ErrorInfo, по которому клиент узнаёт ошибки своих сервисов.
const Domain = "totallyswipes"

// MetadataUserID - ключ метаданных ошибки с ID пользователя (например, при ErrSessionNotOpened).
const MetadataUserID = "user_id"

type entry struct {
	err    error
	code   codes.Code
	reason string
}

// registry - соответствие доменных ошибок кодам; выигрывает первая совпавшая по errors.Is,
// поэтому общий model.ErrNotFound стоит последним.
var registry = []entry{
	{model.ErrSessionNotOpened, codes.Unavailable, "SESSION_NOT_OPENED"},

	{model.ErrEmailAlreadyExists, codes.AlreadyExists, "EMAIL_ALREADY_EXISTS"},
	{model.ErrPasswordTooLong, codes.InvalidArgument, "PASSWORD_TOO_LONG"},
	{model.ErrInvalidCredentials, codes.Unauthenticated, "INVALID_CREDENTIALS"},
	{model.ErrInvalidSession, codes.Unauthenticated, "INVALID_SESSION"},

	{model.ErrProfileRequired, codes.FailedPrecondition, "PROFILE_REQUIRED"},
	{model.ErrProfileIncomplete, codes.FailedPrecondition, "PROFILE_INCOMPLETE"},
	{model.ErrProfileNotFound, codes.NotFound, "PROFILE_NOT_FOUND"},
	{model.ErrUnknownTag, codes.InvalidArgument, "UNKNOWN_TAG"},
	{model.ErrPhotoLimit, codes.FailedPrecondition, "PHOTO_LIMIT"},
	{model.ErrLastPhoto, codes.FailedPrecondition, "LAST_PHOTO"},
	{model.ErrPhotoNotFound, codes.NotFound, "PHOTO_NOT_FOUND"},
	{model.ErrPhotoOrderMismatch, codes.InvalidArgument, "PHOTO_ORDER_MISMATCH"},
	{model.ErrInvalidPhotoStorageKey, codes.InvalidArgument, "INVALID_PHOTO_STORAGE_KEY"},

	{model.ErrInvalidTestRequest, codes.InvalidArgument, "INVALID_TEST_REQUEST"},
	{model.ErrInvalidAnswers, codes.InvalidArgument, "INVALID_ANSWERS"},
	{model.ErrTestNotFound, codes.NotFound, "TEST_NOT_FOUND"},
	{model.ErrTestResultNotFound, codes.NotFound, "TEST_RESULT_NOT_FOUND"},
	{model.ErrInvalidBigFive, codes.Internal, "INVALID_BIG_FIVE"},
	{model.ErrInvalidTestDefinition, codes.Internal, "INVALID_TEST_DEFINITION"},

	{model.ErrInvalidCoordinates, codes.InvalidArgument, "INVALID_COORDINATES"},
	{model.ErrLocationRequired, codes.FailedPrecondition, "LOCATION_REQUIRED"},
	{model.ErrLocationPreviewUnavailable, codes.FailedPrecondition, "LOCATION_PREVIEW_UNAVAILABLE"},
	{model.ErrLocationNotFound, codes.NotFound, "LOCATION_NOT_FOUND"},
	{model.ErrCityNotFound, codes.NotFound, "CITY_NOT_FOUND"},
	{model.ErrGeocoderTimeout, codes.DeadlineExceeded, "GEOCODER_TIMEOUT"},
	{model.ErrGeocoderQuotaExceeded, codes.ResourceExhausted, "GEOCODER_QUOTA_EXCEEDED"},
	{model.ErrGeocoderUnavailable, codes.Unavailable, "GEOCODER_UNAVAILABLE"},

	{model.ErrInvalidFeedRequest, codes.InvalidArgument, "INVALID_FEED_REQUEST"},
	{model.ErrInvalidMatchesRequest, codes.InvalidArgument, "INVALID_MATCHES_REQUEST"},
	{model.ErrInvalidSwipe, codes.InvalidArgument, "INVALID_SWIPE"},
	{model.ErrMatchNotFound, codes.NotFound, "MATCH_NOT_FOUND"},

	{model.ErrTimepadTimeout, codes.DeadlineExceeded, "TIMEPAD_TIMEOUT"},
	{model.ErrTimepadStatusCode, codes.Unavailable, "TIMEPAD_STATUS_CODE"},
	{model.ErrNoEvents, codes.FailedPrecondition, "NO_EVENTS"},
	{model.ErrInvalidEventResults, codes.InvalidArgument, "INVALID_EVENT_RESULTS"},
	{model.ErrResultAlreadySubmitted, codes.FailedPrecondition, "RESULT_ALREADY_SUBMITTED"},
	{model.ErrSearchStateConflict, codes.FailedPrecondition, "SEARCH_STATE_CONFLICT"},

	{model.ErrNotFound, codes.NotFound, "NOT_FOUND"},
}

var byReason = func() map[string]entry {
	m := make(map[string]entry, len(registry))
	for _, e := range registry {
		m[e.reason] = e
	}
	return m
}()

// metaError прикрепляет к ошибке метаданные, которые уйдут клиенту в ErrorInfo.
// *Принадлежит компании Meta, деятельность которой признана экстремистской и запрещена в РФ.
type metaError struct {
	err      error
	metadata map[string]string
}

func (e *metaError) Error() string { return e.err.Error() }
func (e *metaError) Unwrap() error { return e.err }

// WithMetadata добавляет к ошибке метаданные для ErrorInfo; nil остаётся nil.
func WithMetadata(err error, metadata map[string]string) error {
	if err == nil {
		return nil
	}
	return &metaError{err: err, metadata: metadata}
}

// IsKnown сообщает, есть ли ошибка в реестре доменных ошибок.
func IsKnown(err error) bool {
	_, ok := lookup(err)
	return ok
}

func lookup(err error) (entry, bool) {
	for _, e := range registry {
		if errors.Is(err, e.err) {
			return e, true
		}
	}
	return entry{}, false
}

// ToStatus переводит ошибку сервиса в gRPC-статус: доменная - код и ErrorInfo с reason,
// отмена контекста - Canceled/DeadlineExceeded, готовый статус - как есть, остальное - Internal без подробностей.
func ToStatus(err error) error {
	if err == nil {
		return nil
	}

	if e, ok := lookup(err); ok {
		info := &errdetails.ErrorInfo{Reason: e.reason, Domain: Domain}
		var me *metaError
		if errors.As(err, &me) && len(me.metadata) > 0 {
			info.Metadata = maps.Clone(me.metadata)
		}

		st, detailsErr := status.New(e.code, e.err.Error()).WithDetails(info)
		if detailsErr != nil {
			return status.Error(e.code, e.err.Error())
		}
		return st.Err()
	}

	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
		return st.Err()
	}

	return status.Error(codes.Internal, "internal error")
}

// Error - ошибка, восстановленная из gRPC-статуса; errors.Is совпадает с исходной доменной ошибкой.
type Error struct {
	Err      error
	Code     codes.Code
	Message  string
	Metadata map[string]string
}

func (e *Error) Error() string { return e.Code.String() + ": " + e.Message }
func (e *Error) Unwrap() error { return e.Err }

// GRPCStatus позволяет пробросить ошибку дальше по цепочке вызовов без потери кода.
func (e *Error) GRPCStatus() *status.Status { return status.New(e.Code, e.Message) }

// FromStatus восстанавливает доменную ошибку из gRPC-статуса по reason в ErrorInfo;
// статусы без известного reason и ошибки не из gRPC возвращаются без изменений.
func FromStatus(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	for _, detail := range st.Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if !ok || info.GetDomain() != Domain {
			continue
		}
		e, ok := byReason[info.GetReason()]
		if !ok {
			continue
		}
		return &Error{Err: e.err, Code: st.Code(), Message: st.Message(), Metadata: info.GetMetadata()}
	}

	return err
}

// Metadata возвращает метаданные ErrorInfo из ошибки, восстановленной FromStatus.
func Metadata(err error) map[string]string {
	var e *Error
	if errors.As(err, &e) {
		return e.Metadata
	}
	return nil
}
