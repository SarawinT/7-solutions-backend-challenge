package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpchandler "github.com/SarawinT/user-management-api/app/adapters/grpc"
)

// chain composes interceptors in production order: logging, then recovery.
func chain(log *slog.Logger, h grpc.UnaryHandler) grpc.UnaryHandler {
	logging, recovery := grpchandler.LoggingInterceptor(log), grpchandler.RecoveryInterceptor(log)
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/GetUser"}
	return func(ctx context.Context, req any) (any, error) {
		return logging(ctx, req, info, func(ctx context.Context, req any) (any, error) {
			return recovery(ctx, req, info, h)
		})
	}
}

func TestLoggingInterceptor(t *testing.T) {
	tests := map[string]struct {
		handler     grpc.UnaryHandler
		wantCode    codes.Code
		wantLevel   string
		minDuration float64
	}{
		"ok": {
			handler: func(context.Context, any) (any, error) {
				time.Sleep(10 * time.Millisecond)
				return "resp", nil
			},
			wantCode: codes.OK, wantLevel: "INFO", minDuration: 10,
		},
		"client error": {
			handler:  func(context.Context, any) (any, error) { return nil, status.Error(codes.NotFound, "nope") },
			wantCode: codes.NotFound, wantLevel: "WARN",
		},
		"panic is logged": {
			handler:  func(context.Context, any) (any, error) { panic("boom") },
			wantCode: codes.Internal, wantLevel: "ERROR",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))

			_, err := chain(log, tt.handler)(context.Background(), "req")
			if status.Code(err) != tt.wantCode {
				t.Fatalf("code %v, want %v", status.Code(err), tt.wantCode)
			}

			// Last line is the request log (a panic also emits a recovery line first).
			lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
			var entry map[string]any
			if err := json.Unmarshal(lines[len(lines)-1], &entry); err != nil {
				t.Fatal(err)
			}
			if entry["msg"] != "grpc request" || entry["level"] != tt.wantLevel {
				t.Errorf("msg/level = %v/%v, want grpc request/%s", entry["msg"], entry["level"], tt.wantLevel)
			}
			if entry["method"] != "/user.v1.UserService/GetUser" || entry["code"] != tt.wantCode.String() {
				t.Errorf("method/code = %v/%v", entry["method"], entry["code"])
			}
			if d, ok := entry["duration_ms"].(float64); !ok || d < tt.minDuration {
				t.Errorf("duration_ms = %v, want >= %v", entry["duration_ms"], tt.minDuration)
			}
		})
	}
}
