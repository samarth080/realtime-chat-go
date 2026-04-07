package chat_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samarth080/peer-chat/chat"
	"github.com/samarth080/peer-chat/db"
	"github.com/samarth080/peer-chat/ws"
	"github.com/stretchr/testify/require"
)

func TestHandleGroupMessage_FanOut(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	creator, err := db.CreateUser(ctx, pool, "gcreator_"+t.Name(), "h")
	require.NoError(t, err)
	member, err := db.CreateUser(ctx, pool, "gmember_"+t.Name(), "h")
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1,$2)", creator.ID, member.ID)
	})

	group, err := db.CreateGroup(ctx, pool, "testgroup_"+t.Name(), creator.ID)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM groups WHERE id = $1", group.ID)
	})

	require.NoError(t, db.AddGroupMember(ctx, pool, group.ID, member.ID, "member"))

	hub := ws.NewHub()
	creatorClient := ws.NewTestClient(creator.ID, creator.Username)
	memberClient := ws.NewTestClient(member.ID, member.Username)
	hub.Register(creatorClient)
	hub.Register(memberClient)

	raw, _ := json.Marshal(map[string]string{
		"type":     "group_message",
		"group_id": group.ID.String(),
		"body":     "hello group",
	})

	err = chat.HandleGroupMessage(ctx, pool, hub, creator.ID, creator.Username, raw)
	require.NoError(t, err)

	// member receives the fan-out
	msg := <-memberClient.Send()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(msg, &out))
	require.Equal(t, "group_message", out["type"])
	require.Equal(t, "hello group", out["body"])
}
