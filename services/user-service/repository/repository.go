package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mjmichael73/go-uber-clone/services/user-service/model"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, user_type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowContext(ctx, query,
		user.Email, user.PasswordHash, user.FirstName,
		user.LastName, user.Phone, user.UserType,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, phone,
		       user_type, rating, total_ratings, is_active, created_at, updated_at
		FROM users WHERE id = $1`

	user := &model.User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FirstName,
		&user.LastName, &user.Phone, &user.UserType, &user.Rating,
		&user.TotalRatings, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	return user, err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, phone,
		       user_type, rating, total_ratings, is_active, created_at, updated_at
		FROM users WHERE email = $1`

	user := &model.User{}
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FirstName,
		&user.LastName, &user.Phone, &user.UserType, &user.Rating,
		&user.TotalRatings, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	return user, err
}

func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	query := `
		UPDATE users
		SET first_name = $2, last_name = $3, phone = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	return r.db.QueryRowContext(ctx, query,
		user.ID, user.FirstName, user.LastName, user.Phone,
	).Scan(&user.UpdatedAt)
}

func (r *UserRepository) UpdateRating(ctx context.Context, userID string, newRating float32) error {
	query := `
		UPDATE users
		SET rating = (rating * total_ratings + $2) / (total_ratings + 1),
		    total_ratings = total_ratings + 1,
		    updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, userID, newRating)
	return err
}
