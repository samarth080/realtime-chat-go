package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/samarth080/peer-chat/db"
	"github.com/stretchr/testify/require"
)

func testPool(t *testing.T) *db.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://chat:chat@localhost:5432/chatdb?sslmode=disable"
	}
	pool, err := db.NewPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestCreateUser(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, pool, "alice_test_"+t.Name(), "hashed")
	require.NoError(t, err)
	require.Equal(t, "alice_test_"+t.Name(), user.Username)
	require.NotEmpty(t, user.ID)

	// cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestGetUserByUsername(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	created, err := db.CreateUser(ctx, pool, "bob_test_"+t.Name(), "hashed")
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID) })

	found, err := db.GetUserByUsername(ctx, pool, "bob_test_"+t.Name())
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
}

func TestGetUserByID(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	created, err := db.CreateUser(ctx, pool, "byid_test_"+t.Name(), "hashed")
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID) })

	found, err := db.GetUserByID(ctx, pool, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
	require.Equal(t, created.Username, found.Username)
}
