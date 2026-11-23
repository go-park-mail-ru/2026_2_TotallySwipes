package middleware

import (
	"context"
	"dating-app/internal/requestctx"
	"net/http"

	"github.com/google/uuid"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()

		ctx := context.WithValue(r.Context(), requestctx.RequestIDKey{}, id)
		w.Header().Set("X-Request-ID", id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestctx.RequestIDKey{}).(string)
	return id
}
