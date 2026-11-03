package mongodb_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb/mongotest"
	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/models/entity"
)

// Integration test against a real MongoDB: needs MONGO_TEST_URI (see
// mongotest), skipped otherwise. Each subtest gets a fresh database.
func TestUserRepository(t *testing.T) {
	newRepo := func(t *testing.T) ports.UserPort { return mongotest.NewUserRepository(t) }

	// MongoDB stores ms-precision BSON dates.
	base := time.Now().UTC().Truncate(time.Millisecond)

	newUser := func(t *testing.T, i int) *entity.UserEntity {
		t.Helper()
		u, err := entity.NewUserEntity(fmt.Sprintf("id-%02d", i), fmt.Sprintf("User %d", i),
			fmt.Sprintf("user%d@example.com", i), "hash", base.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		return u
	}

	t.Run("create and get", func(t *testing.T) {
		ctx, repo := context.Background(), newRepo(t)
		u := newUser(t, 1)
		mustNoErr(t, repo.Create(ctx, u))

		got, err := repo.GetByID(ctx, u.ID)
		mustNoErr(t, err)
		if got.ID != u.ID || got.Name != u.Name || got.Email != u.Email ||
			got.PasswordHash != u.PasswordHash ||
			!got.CreatedAt.Equal(u.CreatedAt) || !got.UpdatedAt.Equal(u.UpdatedAt) {
			t.Fatalf("got %+v, want %+v", got, u)
		}
	})

	t.Run("get missing", func(t *testing.T) {
		_, err := newRepo(t).GetByID(context.Background(), "missing")
		mustErrIs(t, err, entity.ErrUserNotFound)
	})

	t.Run("get by email", func(t *testing.T) {
		ctx, repo := context.Background(), newRepo(t)
		u := newUser(t, 1)
		mustNoErr(t, repo.Create(ctx, u))

		got, err := repo.GetByEmail(ctx, u.Email)
		mustNoErr(t, err)
		if got.ID != u.ID || got.PasswordHash != u.PasswordHash {
			t.Fatalf("got %+v, want %+v", got, u)
		}

		_, err = repo.GetByEmail(ctx, "missing@example.com")
		mustErrIs(t, err, entity.ErrUserNotFound)
	})

	t.Run("create duplicate email", func(t *testing.T) {
		ctx, repo := context.Background(), newRepo(t)
		mustNoErr(t, repo.Create(ctx, newUser(t, 1)))
		dup := newUser(t, 2)
		dup.Email = "user1@example.com"
		mustErrIs(t, repo.Create(ctx, dup), entity.ErrEmailAlreadyExists)
	})

	t.Run("list paginates in creation order", func(t *testing.T) {
		ctx, repo := context.Background(), newRepo(t)
		for i := 1; i <= 5; i++ {
			mustNoErr(t, repo.Create(ctx, newUser(t, i)))
		}

		page, total, err := repo.List(ctx, 2, 2)
		mustNoErr(t, err)
		if total != 5 {
			t.Fatalf("total = %d, want 5", total)
		}
		if len(page) != 2 || page[0].ID != "id-03" || page[1].ID != "id-04" {
			t.Fatalf("unexpected page: %v", ids(page))
		}

		empty, _, err := repo.List(ctx, 10, 2)
		mustNoErr(t, err)
		if len(empty) != 0 {
			t.Fatalf("expected empty page past the end, got %v", ids(empty))
		}
	})

	t.Run("update", func(t *testing.T) {
		ctx, repo := context.Background(), newRepo(t)
		u := newUser(t, 1)
		mustNoErr(t, repo.Create(ctx, u))

		mustNoErr(t, u.Rename("Renamed", base.Add(time.Hour)))
		mustNoErr(t, u.ChangeEmail("new@example.com", base.Add(time.Hour)))
		mustNoErr(t, repo.Update(ctx, u))

		got, err := repo.GetByID(ctx, u.ID)
		mustNoErr(t, err)
		if got.Name != "Renamed" || got.Email != "new@example.com" || !got.UpdatedAt.Equal(u.UpdatedAt) {
			t.Fatalf("update not persisted: %+v", got)
		}

		// The old email must be free again.
		mustNoErr(t, repo.Create(ctx, withID(newUser(t, 1), "id-99")))
	})

	t.Run("update to taken email", func(t *testing.T) {
		ctx, repo := context.Background(), newRepo(t)
		mustNoErr(t, repo.Create(ctx, newUser(t, 1)))
		u2 := newUser(t, 2)
		mustNoErr(t, repo.Create(ctx, u2))

		u2.Email = "user1@example.com"
		mustErrIs(t, repo.Update(ctx, u2), entity.ErrEmailAlreadyExists)
	})

	t.Run("update missing", func(t *testing.T) {
		mustErrIs(t, newRepo(t).Update(context.Background(), newUser(t, 1)), entity.ErrUserNotFound)
	})

	t.Run("delete", func(t *testing.T) {
		ctx, repo := context.Background(), newRepo(t)
		u := newUser(t, 1)
		mustNoErr(t, repo.Create(ctx, u))
		mustNoErr(t, repo.Delete(ctx, u.ID))

		_, err := repo.GetByID(ctx, u.ID)
		mustErrIs(t, err, entity.ErrUserNotFound)
		mustErrIs(t, repo.Delete(ctx, u.ID), entity.ErrUserNotFound)

		// Email is released after delete.
		mustNoErr(t, repo.Create(ctx, withID(newUser(t, 1), "id-99")))
	})
}

func ids(us []*entity.UserEntity) []string {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = u.ID
	}
	return out
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustErrIs(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("got error %v, want %v", err, want)
	}
}

func withID(u *entity.UserEntity, id string) *entity.UserEntity {
	u.ID = id
	return u
}
