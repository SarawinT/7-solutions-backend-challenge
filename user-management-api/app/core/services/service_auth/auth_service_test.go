package service_auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb/mongotest"
	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/core/services/service_auth"
	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/request"
)

// Test doubles for the hasher, ID and token ports. The repository is a real
// MongoDB (see mongotest), so these tests need MONGO_TEST_URI.

type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error) { return "hashed:" + p, nil }

func (fakeHasher) Compare(hash, p string) error {
	if hash != "hashed:"+p {
		return errors.New("mismatch")
	}
	return nil
}

// countingHasher records Compare calls to check the timing defence.
type countingHasher struct {
	fakeHasher
	compares int
}

func (h *countingHasher) Compare(hash, p string) error {
	h.compares++
	return h.fakeHasher.Compare(hash, p)
}

type seqIDs struct{ n int }

func (s *seqIDs) NewID() string { s.n++; return string(rune('a' + s.n - 1)) }

// fakeTokens issues "token:<userID>" and accepts only those.
type fakeTokens struct{}

func (fakeTokens) Issue(userID, email string) (string, *entity.TokenClaimsEntity, error) {
	return "token:" + userID, &entity.TokenClaimsEntity{UserID: userID, Email: email, ExpiresAt: time.Unix(1, 0)}, nil
}

func (fakeTokens) Verify(token string) (*entity.TokenClaimsEntity, error) {
	id, ok := strings.CutPrefix(token, "token:")
	if !ok {
		return nil, entity.ErrInvalidToken
	}
	return &entity.TokenClaimsEntity{UserID: id}, nil
}

func newAuthService(t *testing.T, h ports.PasswordHasherPort) service_auth.AuthServicer {
	t.Helper()
	return service_auth.NewAuthService(mongotest.NewUserRepository(t), h, &seqIDs{}, fakeTokens{})
}

var alice = request.RegisterRequest{Name: "Alice", Email: "alice@example.com", Password: "secret123"}

func TestRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("hashes password via the port", func(t *testing.T) {
		u, err := newAuthService(t, fakeHasher{}).Register(ctx, alice)
		if err != nil {
			t.Fatal(err)
		}
		if u.PasswordHash != "hashed:secret123" {
			t.Fatalf("password not hashed via port: %q", u.PasswordHash)
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		svc := newAuthService(t, fakeHasher{})
		if _, err := svc.Register(ctx, alice); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Register(ctx, alice); !errors.Is(err, entity.ErrEmailAlreadyExists) {
			t.Fatalf("got %v, want ErrEmailAlreadyExists", err)
		}
	})

	t.Run("weak password", func(t *testing.T) {
		req := alice
		req.Password = "short"
		if _, err := newAuthService(t, fakeHasher{}).Register(ctx, req); !errors.Is(err, entity.ErrInvalidPassword) {
			t.Fatalf("got %v, want ErrInvalidPassword", err)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		_, err := newAuthService(t, fakeHasher{}).Register(ctx, request.RegisterRequest{})
		if ve, ok := errors.AsType[*entity.ValidationError](err); !ok || !strings.Contains(ve.Message, "name is required") {
			t.Fatalf("got %v, want ValidationError", err)
		}
	})
}

func TestLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("register then login", func(t *testing.T) {
		svc := newAuthService(t, fakeHasher{})
		u, err := svc.Register(ctx, alice)
		if err != nil {
			t.Fatal(err)
		}
		// Email lookup is case-insensitive.
		res, err := svc.Login(ctx, request.LoginRequest{Email: "  ALICE@example.com ", Password: "secret123"})
		if err != nil {
			t.Fatal(err)
		}
		if res.AccessToken != "token:"+u.ID || res.User.ID != u.ID {
			t.Fatalf("unexpected login result: %+v", res)
		}
		claims, err := svc.VerifyToken(ctx, res.AccessToken)
		if err != nil || claims.UserID != u.ID {
			t.Fatalf("VerifyToken = %+v, %v", claims, err)
		}
	})

	t.Run("failures are indistinguishable", func(t *testing.T) {
		h := &countingHasher{}
		svc := newAuthService(t, h)
		if _, err := svc.Register(ctx, alice); err != nil {
			t.Fatal(err)
		}
		for name, req := range map[string]request.LoginRequest{
			"wrong password":  {Email: "alice@example.com", Password: "wrong-password"},
			"unknown email":   {Email: "nobody@example.com", Password: "secret123"},
			"malformed email": {Email: "not-an-email", Password: "secret123"},
		} {
			if _, err := svc.Login(ctx, req); !errors.Is(err, entity.ErrInvalidCredentials) {
				t.Fatalf("%s: got %v, want ErrInvalidCredentials", name, err)
			}
		}
		// Wrong password and unknown email must both run a hash comparison.
		if h.compares != 2 {
			t.Fatalf("Compare called %d times, want 2", h.compares)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		_, err := newAuthService(t, fakeHasher{}).Login(ctx, request.LoginRequest{Email: "alice@example.com"})
		if _, ok := errors.AsType[*entity.ValidationError](err); !ok {
			t.Fatalf("got %v, want ValidationError", err)
		}
	})
}

func TestVerifyToken_Invalid(t *testing.T) {
	if _, err := newAuthService(t, fakeHasher{}).VerifyToken(context.Background(), "garbage"); !errors.Is(err, entity.ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}
