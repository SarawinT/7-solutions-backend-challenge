// Package mongodb implements the repository ports on MongoDB, one file
// per collection (user_repository.go, ...).
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/models/entity"
)

const usersCollection = "users"

// userDocument is the storage representation of a user. It is kept
// separate from entity.UserEntity so bson tags never leak into the core.
type userDocument struct {
	ID           string    `bson:"_id"`
	Name         string    `bson:"name"`
	Email        string    `bson:"email"`
	PasswordHash string    `bson:"password_hash"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

func toDocument(u *entity.UserEntity) userDocument {
	return userDocument{
		ID:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func (d userDocument) toDomain() *entity.UserEntity {
	return &entity.UserEntity{
		ID:           d.ID,
		Name:         d.Name,
		Email:        d.Email,
		PasswordHash: d.PasswordHash,
		CreatedAt:    d.CreatedAt.UTC(),
		UpdatedAt:    d.UpdatedAt.UTC(),
	}
}

type UserRepository struct {
	coll *mongo.Collection
}

func NewUserRepository(db *mongo.Database) ports.UserPort {
	return &UserRepository{coll: db.Collection(usersCollection)}
}

// EnsureUserIndexes creates the indexes the user repository relies on. It
// is idempotent and should run once at startup. It is a package function,
// not a method, because NewUserRepository returns ports.UserPort and index
// setup is MongoDB-specific, so it doesn't belong in the port.
func EnsureUserIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(usersCollection).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// Enforces email uniqueness atomically (no check-then-insert race).
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetName("uniq_email").SetUnique(true),
		},
		{
			// Supports stable, paginated listing.
			Keys:    bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}},
			Options: options.Index().SetName("created_at_id"),
		},
	})
	if err != nil {
		return fmt.Errorf("create user indexes: %w", err)
	}
	return nil
}

func (r *UserRepository) Create(ctx context.Context, u *entity.UserEntity) error {
	_, err := r.coll.InsertOne(ctx, toDocument(u))
	if mongo.IsDuplicateKeyError(err) {
		return entity.ErrEmailAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*entity.UserEntity, error) {
	var doc userDocument
	err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, entity.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*entity.UserEntity, error) {
	var doc userDocument
	err := r.coll.FindOne(ctx, bson.D{{Key: "email", Value: email}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, entity.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *UserRepository) List(ctx context.Context, offset, limit int) ([]*entity.UserEntity, int64, error) {
	total, err := r.coll.CountDocuments(ctx, bson.D{})
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	cur, err := r.coll.Find(ctx, bson.D{}, options.Find().
		SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, fmt.Errorf("find users: %w", err)
	}

	var docs []userDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, fmt.Errorf("decode users: %w", err)
	}
	users := make([]*entity.UserEntity, len(docs))
	for i, d := range docs {
		users[i] = d.toDomain()
	}
	return users, total, nil
}

func (r *UserRepository) Update(ctx context.Context, u *entity.UserEntity) error {
	res, err := r.coll.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: u.ID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "name", Value: u.Name},
			{Key: "email", Value: u.Email},
			{Key: "password_hash", Value: u.PasswordHash},
			{Key: "updated_at", Value: u.UpdatedAt},
		}}},
	)
	if mongo.IsDuplicateKeyError(err) {
		return entity.ErrEmailAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if res.MatchedCount == 0 {
		return entity.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if res.DeletedCount == 0 {
		return entity.ErrUserNotFound
	}
	return nil
}
