package grpc

import (
	"context"
	"errors"
	"log/slog"

	"dating-app/services/matching/internal/model"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func matchError(ctx context.Context, err error) error {

	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}

	for _, mapping := range []struct {
		err    error
		code   codes.Code
		reason string
	}{
		{model.ErrInvalidMatchesRequest, codes.InvalidArgument, "VALIDATION_ERROR"},
		{model.ErrInvalidFeedRequest, codes.InvalidArgument, "VALIDATION_ERROR"},
		{model.ErrInvalidSwipe, codes.InvalidArgument, "VALIDATION_ERROR"},
		{model.ErrInvalidEventResults, codes.InvalidArgument, "VALIDATION_ERROR"},
		{model.ErrProfileRequired, codes.FailedPrecondition, "PROFILE_REQUIRED"},
		{model.ErrProfileIncomplete, codes.PermissionDenied, "PROFILE_REQUIRED"},
		{model.ErrLocationRequired, codes.FailedPrecondition, "LOCATION_REQUIRED"},
		{model.ErrMatchNotFound, codes.NotFound, "MATCH_NOT_FOUND"},
		{model.ErrProfileNotFound, codes.NotFound, "PROFILE_NOT_FOUND"},
		{model.ErrNotFound, codes.NotFound, "NOT_FOUND"},
		{model.ErrNoEvents, codes.FailedPrecondition, "NO_EVENTS"},
		{model.ErrResultAlreadySubmitted, codes.AlreadyExists, "RESULT_ALREADY_SUBMITTED"},
		{model.ErrSearchStateConflict, codes.FailedPrecondition, "SEARCH_STATE_CONFLICT"},
		{model.ErrTimepadTimeout, codes.DeadlineExceeded, "TIMEPAD_TIMEOUT"},
	} {
		if errors.Is(err, mapping.err) {
			result, detailsErr := status.New(mapping.code, mapping.err.Error()).WithDetails(&errdetails.ErrorInfo{Reason: mapping.reason, Domain: "matching"})

			if detailsErr == nil {
				return result.Err()
			}
		}
	}

	// Ошибки недоступного Profile сохраняют транспортный код без внутренних подробностей.

	if status.Code(err) == codes.Unavailable {
		return status.Error(codes.Unavailable, "profile service unavailable")
	}

	slog.ErrorContext(ctx, "matching grpc request failed", "error", err)
	return status.Error(codes.Internal, "internal matching error")
}
