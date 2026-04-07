package chat_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/chat"
	"github.com/samarth080/peer-chat/db"
	"github.com/samarth080/peer-chat/ws"
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

func TestHandleDM_DeliverToOnlineReceiver(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	sender, err := db.CreateUser(ctx, pool, "dm_sender_"+t.Name(), "h")
	require.NoError(t, err)
	receiver, err := db.CreateUser(ctx, pool, "dm_recv_"+t.Name(), "h")
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1,$2)", sender.ID, receiver.ID)
	})

	hub := ws.NewHub()
	recvClient := ws.NewTestClient(receiver.ID, receiver.Username)
	senderClient := ws.NewTestClient(sender.ID, sender.Username)
	hub.Register(recvClient)
	hub.Register(senderClient)

	raw, _ := json.Marshal(map[string]string{
		"type": "message",
		"to":   receiver.ID.String(),
		"body": "hello there",
		"id":   uuid.New().String(),
	})

	err = chat.HandleDM(ctx, pool, hub, nil, sender.ID, sender.Username, raw)
	require.NoError(t, err)

	// receiver gets the message
	msg := <-recvClient.Send()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(msg, &out))
	require.Equal(t, "message", out["type"])
	require.Equal(t, "hello there", out["body"])

	// sender gets ack
	ack := <-senderClient.Send()
	var ackOut map[string]interface{}
	require.NoError(t, json.Unmarshal(ack, &ackOut))
	require.Equal(t, "sent", ackOut["type"])
}
