package service_user_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb/mongotest"
	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/core/services/service_user"
	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/request"
	"github.com/SarawinT/user-management-api/app/utils"
)

// newService returns a UserServicer on a fresh MongoDB database (see
// mongotest) plus the repository, for seeding users directly.
func newService(t *testing.T) (service_user.UserServicer, ports.UserPort) {
	t.Helper()
	repo := mongotest.NewUserRepository(t)
	return service_user.NewUserService(repo), repo
}

func seedUser(t *testing.T, repo ports.UserPort, i int) *entity.UserEntity {
	t.Helper()
	u, err := entity.NewUserEntity(fmt.Sprintf("id-%d", i), fmt.Sprintf("User %d", i),
		fmt.Sprintf("user%d@example.com", i), "hash", utils.NowUTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestInquiryUser(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)
	u := seedUser(t, repo, 1)

	got, err := svc.InquiryUser(ctx, u.ID)
	if err != nil || got.Email != u.Email {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := svc.InquiryUser(ctx, "missing"); !errors.Is(err, entity.ErrUserNotFound) {
		t.Fatalf("missing: got %v, want ErrUserNotFound", err)
	}
}

func TestListUser(t *testing.T) {
	svc, repo := newService(t)
	for i := 1; i <= 3; i++ {
		seedUser(t, repo, i)
	}

	res, err := svc.ListUser(context.Background(), request.ListUserRequest{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 || len(res.Users) != 1 || res.Users[0].ID != "id-3" {
		t.Fatalf("unexpected page: total=%d users=%v", res.Total, res.Users)
	}
}

func TestUpdateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("partial update", func(t *testing.T) {
		svc, repo := newService(t)
		u := seedUser(t, repo, 1)

		updated, err := svc.UpdateUser(ctx, request.UpdateUserRequest{ID: u.ID, Name: new("Alicia")})
		if err != nil {
			t.Fatal(err)
		}
		if updated.Name != "Alicia" || updated.Email != u.Email {
			t.Fatalf("unexpected result: %+v", updated)
		}
	})

	t.Run("no fields", func(t *testing.T) {
		svc, repo := newService(t)
		u := seedUser(t, repo, 1)
		if _, err := svc.UpdateUser(ctx, request.UpdateUserRequest{ID: u.ID}); !isValidation(err) {
			t.Fatalf("got %v, want ValidationError", err)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		svc, repo := newService(t)
		u := seedUser(t, repo, 1)
		if _, err := svc.UpdateUser(ctx, request.UpdateUserRequest{ID: u.ID, Email: new("bad")}); !errors.Is(err, entity.ErrInvalidEmail) {
			t.Fatalf("got %v, want ErrInvalidEmail", err)
		}
	})
}

func TestDeleteUser(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)
	u := seedUser(t, repo, 1)

	if err := svc.DeleteUser(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteUser(ctx, u.ID); !errors.Is(err, entity.ErrUserNotFound) {
		t.Fatalf("second delete: got %v, want ErrUserNotFound", err)
	}
	if err := svc.DeleteUser(ctx, ""); !isValidation(err) {
		t.Fatalf("empty id: got %v, want ValidationError", err)
	}
}

func isValidation(err error) bool {
	_, ok := errors.AsType[*entity.ValidationError](err)
	return ok
}
