// Package mongotest provides isolated MongoDB databases for tests.
//
// Tests using it need a running MongoDB and MONGO_TEST_URI set, e.g.
//
//	make up && make test-integration
//
// Without MONGO_TEST_URI the calling test is skipped.
package mongotest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb"
	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/database_init"
)

const uriEnv = "MONGO_TEST_URI"

var (
	clientOnce sync.Once
	client     *mongo.Client
	clientErr  error
)

// NewDatabase returns a fresh, uniquely named database that is dropped when
// the test ends. One client is shared by all tests in the process.
func NewDatabase(t testing.TB) *mongo.Database {
	t.Helper()
	uri := os.Getenv(uriEnv)
	if uri == "" {
		t.Skip(uriEnv + " not set; skipping test that needs MongoDB (run `make up && make test-integration`)")
	}

	clientOnce.Do(func() { client, clientErr = database_init.NewMongoClient(context.Background(), uri) })
	if clientErr != nil {
		t.Fatalf("connect to MongoDB at %s: %v", uri, clientErr)
	}

	suffix := make([]byte, 8)
	_, _ = rand.Read(suffix)
	db := client.Database("test_" + hex.EncodeToString(suffix))
	t.Cleanup(func() { _ = db.Drop(context.Background()) })
	return db
}

// NewUserRepository returns a MongoDB user repository backed by a fresh
// database, with indexes created.
func NewUserRepository(t testing.TB) ports.UserPort {
	t.Helper()
	db := NewDatabase(t)
	if err := mongodb.EnsureUserIndexes(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return mongodb.NewUserRepository(db)
}
