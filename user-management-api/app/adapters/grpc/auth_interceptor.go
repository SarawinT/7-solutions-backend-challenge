package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/models/entity"
	authv1 "github.com/SarawinT/user-management-api/gen/go/auth/v1"
)

// publicMethods may be called without a token. Everything else is denied
// by default, so newly added RPCs are protected automatically.
var publicMethods = map[string]bool{
	authv1.AuthService_Register_FullMethodName: true,
	authv1.AuthService_Login_FullMethodName:    true,
}

// publicPrefixes cover infrastructure services (health probes, reflection).
var publicPrefixes = []string{
	"/grpc.health.v1.Health/",
	"/grpc.reflection.",
}

func isPublic(fullMethod string) bool {
	if publicMethods[fullMethod] {
		return true
	}
	for _, p := range publicPrefixes {
		if strings.HasPrefix(fullMethod, p) {
			return true
		}
	}
	return false
}

type claimsCtxKey struct{}

// ClaimsFromContext returns the authenticated caller's claims, set by the
// auth interceptors on protected RPCs.
func ClaimsFromContext(ctx context.Context) (*entity.TokenClaimsEntity, bool) {
	c, ok := ctx.Value(claimsCtxKey{}).(*entity.TokenClaimsEntity)
	return c, ok
}

// authenticate validates "authorization: Bearer <jwt>" metadata.
func authenticate(ctx context.Context, token ports.TokenPort) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
	}
	scheme, raw, ok := strings.Cut(values[0], " ")
	raw = strings.TrimSpace(raw)
	if !ok || !strings.EqualFold(scheme, "Bearer") || raw == "" {
		return nil, status.Error(codes.Unauthenticated, "authorization must be \"Bearer <token>\"")
	}

	claims, err := token.Verify(raw)
	if err != nil {
		return nil, toStatus(err)
	}
	return context.WithValue(ctx, claimsCtxKey{}, claims), nil
}

// AuthUnaryInterceptor enforces JWT authentication on unary RPCs.
func AuthUnaryInterceptor(token ports.TokenPort) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isPublic(info.FullMethod) {
			return handler(ctx, req)
		}
		ctx, err := authenticate(ctx, token)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// AuthStreamInterceptor enforces JWT authentication on streaming RPCs.
func AuthStreamInterceptor(token ports.TokenPort) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if isPublic(info.FullMethod) {
			return handler(srv, ss)
		}
		ctx, err := authenticate(ss.Context(), token)
		if err != nil {
			return err
		}
		return handler(srv, &authedStream{ServerStream: ss, ctx: ctx})
	}
}

type authedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authedStream) Context() context.Context { return s.ctx }
