package presence_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/presence"
	"github.com/samarth080/peer-chat/ws"
	"github.com/stretchr/testify/require"
)

func TestSetTyping_FanOut(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()

	senderID := uuid.New()
	receiverID := uuid.New()
	chatID := uuid.New()

	hub := ws.NewHub()
	recvClient := ws.NewTestClient(receiverID, "bob")
	hub.Register(recvClient)

	presence.SetTyping(ctx, rdb, hub, chatID, senderID, "alice", []uuid.UUID{senderID, receiverID})

	msg := <-recvClient.Send()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(msg, &out))
	require.Equal(t, "typing", out["type"])
	require.Equal(t, "alice", out["from"])
}
