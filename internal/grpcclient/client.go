// Package grpcclient открывает соединения к внутренним gRPC-сервисам с общими интерсепторами.
package grpcclient

import (
	"context"
	"fmt"
	"time"

	"dating-app/internal/grpcerr"
	"dating-app/internal/requestctx"
	"dating-app/internal/validate"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// MaxMsgSize совпадает с лимитом сервера: вмещает загрузку и отдачу фото максимального размера.
const MaxMsgSize = validate.PhotoMaxSize + 1<<20

// Dial создаёт клиентское соединение (без TLS - сервисы общаются во внутренней сети).
// Каждый вызов получает request ID из контекста, таймаут timeout (если у контекста нет дедлайна)
// и ошибку, переведённую обратно в доменную через grpcerr.FromStatus.
func Dial(addr string, timeout time.Duration) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMsgSize), grpc.MaxCallSendMsgSize(MaxMsgSize)),
		grpc.WithChainUnaryInterceptor(UnaryInterceptor(timeout)),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc client %s: %w", addr, err)
	}
	return conn, nil
}

// UnaryInterceptor пробрасывает request ID, ограничивает вызов по времени и восстанавливает доменные ошибки.
func UnaryInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if id := requestctx.RequestIDFromContext(ctx); id != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, requestctx.MetadataKey, id)
		}

		if _, ok := ctx.Deadline(); !ok && timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		return grpcerr.FromStatus(invoker(ctx, method, req, reply, cc, opts...))
	}
}
