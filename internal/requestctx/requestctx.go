package requestctx

import "context"

type RequestIDKey struct{}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDKey{}).(string)
	return id
}
