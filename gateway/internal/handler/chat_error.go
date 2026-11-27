package handler

import (
	"context"
	"log/slog"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func writeChatServiceError(ctx context.Context, w http.ResponseWriter, op string, err error, media bool) {

	switch status.Code(err) {

	case codes.InvalidArgument:
		writeError(ctx, w, http.StatusBadRequest, codeValidationError, "Некорректные данные запроса", nil)

	case codes.Unauthenticated:
		writeError(ctx, w, http.StatusUnauthorized, codeUnauthorized, msgUnauthorized, nil)

	case codes.PermissionDenied:
		writeError(ctx, w, http.StatusForbidden, "CHAT_ACCESS_DENIED", "Доступ к чату запрещён", nil)

	case codes.NotFound:
		if media {
			writeError(ctx, w, http.StatusNotFound, "MEDIA_NOT_FOUND", "Медиафайл не найден", nil)
		} else {
			writeError(ctx, w, http.StatusNotFound, "CHAT_NOT_FOUND", "Чат не найден или недоступен", nil)
		}

	default:
		slog.ErrorContext(ctx, op, "error", err)
		writeError(ctx, w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
	}
}
