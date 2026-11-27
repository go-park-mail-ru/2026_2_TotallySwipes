package requestctx

import "context"

type RequestIDKey struct{}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDKey{}).(string)
	return id
}

// MetadataKey - ключ gRPC-метаданных, в котором request ID передаётся между сервисами.
const MetadataKey = "x-request-id"
