package db_test

import (
	"context"
	"testing"

	"github.com/samarth080/peer-chat/db"
	"github.com/stretchr/testify/require"
)

func TestFindOrCreateChat(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	userA, err := db.CreateUser(ctx, pool, "chatA_"+t.Name(), "h")
	require.NoError(t, err)
	userB, err := db.CreateUser(ctx, pool, "chatB_"+t.Name(), "h")
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1,$2)", userA.ID, userB.ID)
	})

	chatID1, err := db.FindOrCreateChat(ctx, pool, userA.ID, userB.ID)
	require.NoError(t, err)
	require.NotEmpty(t, chatID1)

	// calling again returns same chat
	chatID2, err := db.FindOrCreateChat(ctx, pool, userA.ID, userB.ID)
	require.NoError(t, err)
	require.Equal(t, chatID1, chatID2)
}

func TestInsertAndGetMessages(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	userA, err := db.CreateUser(ctx, pool, "msgA_"+t.Name(), "h")
	require.NoError(t, err)
	userB, err := db.CreateUser(ctx, pool, "msgB_"+t.Name(), "h")
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1,$2)", userA.ID, userB.ID)
	})

	chatID, err := db.FindOrCreateChat(ctx, pool, userA.ID, userB.ID)
	require.NoError(t, err)

	msg, err := db.InsertMessage(ctx, pool, chatID, userA.ID, "hello world")
	require.NoError(t, err)
	require.Equal(t, "hello world", msg.Body)
	require.Equal(t, "sent", msg.Status)
}
