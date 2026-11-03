package routers_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb"
	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb/mongotest"
	"github.com/SarawinT/user-management-api/app/adapters/security"
	"github.com/SarawinT/user-management-api/app/routers"
	authv1 "github.com/SarawinT/user-management-api/gen/go/auth/v1"
	userv1 "github.com/SarawinT/user-management-api/gen/go/user/v1"
)

// End-to-end gRPC tests: real server from routers.GrpcRouter, real
// services and MongoDB (needs MONGO_TEST_URI, see mongotest).

type clients struct {
	auth   authv1.AuthServiceClient
	users  userv1.UserServiceClient
	health healthpb.HealthClient
}

func newClients(t *testing.T) clients {
	t.Helper()
	db := mongotest.NewDatabase(t)
	if err := mongodb.EnsureUserIndexes(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	token, err := security.NewJwtToken(testSecret, "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	srv := routers.GrpcRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db, token, false)

	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return clients{
		auth:   authv1.NewAuthServiceClient(conn),
		users:  userv1.NewUserServiceClient(conn),
		health: healthpb.NewHealthClient(conn),
	}
}

func withToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

// login registers a user and returns a context carrying its access token.
func login(t *testing.T, ctx context.Context, c clients, email string) context.Context {
	t.Helper()
	if _, err := c.auth.Register(ctx, &authv1.RegisterRequest{Name: "Test", Email: email, Password: "secret123"}); err != nil {
		t.Fatal(err)
	}
	res, err := c.auth.Login(ctx, &authv1.LoginRequest{Email: email, Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetTokenType() != "Bearer" || res.GetAccessToken() == "" || res.GetExpiresAt() == nil {
		t.Fatalf("unexpected login response: %v", res)
	}
	return withToken(ctx, res.GetAccessToken())
}

func TestGrpcUserService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := newClients(t)
	ctx = login(t, ctx, c, "admin@example.com")

	// Users are created only via AuthService.Register.
	created, err := c.auth.Register(ctx, &authv1.RegisterRequest{Name: "Alice", Email: "alice@example.com", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	id := created.GetUser().GetId()

	// Duplicate email → AlreadyExists.
	_, err = c.auth.Register(ctx, &authv1.RegisterRequest{Name: "Bob", Email: "alice@example.com", Password: "secret123"})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("got %v, want AlreadyExists", err)
	}

	// Invalid input → InvalidArgument.
	_, err = c.auth.Register(ctx, &authv1.RegisterRequest{Name: "Bob", Email: "bad", Password: "secret123"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v, want InvalidArgument", err)
	}

	newName := "Alicia"
	updated, err := c.users.UpdateUser(ctx, &userv1.UpdateUserRequest{Id: id, Name: &newName})
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetUser().GetName() != "Alicia" {
		t.Fatalf("name not updated: %v", updated.GetUser())
	}

	list, err := c.users.ListUsers(ctx, &userv1.ListUsersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if list.GetTotal() != 2 || len(list.GetUsers()) != 2 { // admin + alice
		t.Fatalf("unexpected list: %v", list)
	}

	if _, err := c.users.DeleteUser(ctx, &userv1.DeleteUserRequest{Id: id}); err != nil {
		t.Fatal(err)
	}
	_, err = c.users.GetUser(ctx, &userv1.GetUserRequest{Id: id})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("got %v, want NotFound", err)
	}
}

func TestGrpcAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := newClients(t)

	t.Run("user service requires a token", func(t *testing.T) {
		_, err := c.users.ListUsers(ctx, &userv1.ListUsersRequest{})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("no token: got %v", err)
		}
		_, err = c.users.ListUsers(withToken(ctx, "garbage"), &userv1.ListUsersRequest{})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("bad token: got %v", err)
		}
		basic := metadata.AppendToOutgoingContext(ctx, "authorization", "Basic dXNlcjpwYXNz")
		if _, err = c.users.ListUsers(basic, &userv1.ListUsersRequest{}); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("basic scheme: got %v", err)
		}
	})

	t.Run("health is public", func(t *testing.T) {
		if _, err := c.health.Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("valid token grants access", func(t *testing.T) {
		authed := login(t, ctx, c, "alice@example.com")
		if _, err := c.users.ListUsers(authed, &userv1.ListUsersRequest{}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("bad credentials", func(t *testing.T) {
		_, err := c.auth.Login(ctx, &authv1.LoginRequest{Email: "alice@example.com", Password: "wrong-password"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("got %v, want Unauthenticated", err)
		}
	})
}
