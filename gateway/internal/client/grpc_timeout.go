package client

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

func UnaryTimeoutInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {

	return func(ctx context.Context, method string, request, response any, conn *grpc.ClientConn, invoke grpc.UnaryInvoker, options ...grpc.CallOption) error {

		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return invoke(callCtx, method, request, response, conn, options...)
	}
}
