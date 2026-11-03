package grpc

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryInterceptor turns panics into codes.Internal instead of
// crashing the server.
func RecoveryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(ctx, "panic recovered",
					"method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// LoggingInterceptor logs each unary call with its full method name
// (e.g. /user.v1.UserService/GetUser), status code and execution time.
//
// Chain it FIRST (outermost) so calls that panic or are rejected by later
// interceptors (auth) are logged too.
func LoggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		elapsed := time.Since(start)
		code := status.Code(err)

		level := slog.LevelInfo
		switch code {
		case codes.OK:
		case codes.Internal, codes.Unknown, codes.DataLoss, codes.Unavailable:
			level = slog.LevelError
		default:
			level = slog.LevelWarn
		}
		log.Log(ctx, level, "grpc request",
			slog.String("method", info.FullMethod),
			slog.String("code", code.String()),
			slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
		)
		return resp, err
	}
}
