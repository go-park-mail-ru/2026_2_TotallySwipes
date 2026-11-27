package grpc

import (
	"context"
	"errors"
	"log/slog"

	"dating-app/services/chat/internal/model"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// chatError переводит ошибки сервиса в единый публичный статус gRPC.
func chatError(ctx context.Context, err error) error {

	if err == nil {
		return nil
	}

	switch {

	case errors.Is(err, model.ErrUnsupportedMedia):
		result, detailsErr := status.New(codes.InvalidArgument, model.ErrUnsupportedMedia.Error()).WithDetails(
			&errdetails.ErrorInfo{Reason: "UNSUPPORTED_MEDIA_TYPE", Domain: "chat"},
		)

		if detailsErr != nil {
			return status.Error(codes.Internal, "internal chat error")
		}

		return result.Err()

	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")

	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}

	for _, mapping := range []struct {
		domainErr error
		code      codes.Code
	}{
		{model.ErrInvalidMedia, codes.InvalidArgument},
		{model.ErrMediaTooLong, codes.InvalidArgument},
		{model.ErrMediaTooLarge, codes.ResourceExhausted},
		{model.ErrInvalidUserID, codes.InvalidArgument},
		{model.ErrInvalidMatchID, codes.InvalidArgument},
		{model.ErrInvalidChatID, codes.InvalidArgument},
		{model.ErrInvalidMessageID, codes.InvalidArgument},
		{model.ErrInvalidChatMembers, codes.InvalidArgument},
		{model.ErrInvalidMessagesQuery, codes.InvalidArgument},
		{model.ErrInvalidClientMessageID, codes.InvalidArgument},
		{model.ErrInvalidMessageText, codes.InvalidArgument},
		{model.ErrInvalidReadSeq, codes.InvalidArgument},
		{model.ErrChatNotFound, codes.NotFound},
		{model.ErrMessageNotFound, codes.NotFound},
		{model.ErrMediaNotFound, codes.NotFound},
		{model.ErrChatMemberBlocked, codes.PermissionDenied},
		{model.ErrMessagingNotAllowed, codes.PermissionDenied},
		{model.ErrIdempotencyConflict, codes.AlreadyExists},
	} {

		if errors.Is(err, mapping.domainErr) {
			return status.Error(mapping.code, mapping.domainErr.Error())
		}
	}

	slog.ErrorContext(ctx, "chat grpc request failed", "error", err)

	return status.Error(codes.Internal, "internal chat error")
}
