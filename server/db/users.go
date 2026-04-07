package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

func CreateUser(ctx context.Context, pool *pgxpool.Pool, username, passwordHash string) (User, error) {
	var u User
	err := pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, username, password_hash, created_at`,
		username, passwordHash,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

func GetUserByUsername(ctx context.Context, pool *pgxpool.Pool, username string) (User, error) {
	var u User
	err := pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at
		 FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == pgx.ErrNoRows {
		return User{}, pgx.ErrNoRows
	}
	return u, err
}

func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (User, error) {
	var u User
	err := pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == pgx.ErrNoRows {
		return User{}, pgx.ErrNoRows
	}
	return u, err
}
