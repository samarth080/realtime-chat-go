package ws_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/ws"
	"github.com/stretchr/testify/require"
)

func TestHub_RegisterAndSend(t *testing.T) {
	hub := ws.NewHub()

	id := uuid.New()
	client := ws.NewTestClient(id, "alice")
	hub.Register(client)

	msg := []byte(`{"type":"test"}`)
	delivered := hub.Send(id, msg)
	require.True(t, delivered)

	received := <-client.Send()
	require.Equal(t, msg, received)
}

func TestHub_SendToOfflineUser(t *testing.T) {
	hub := ws.NewHub()
	id := uuid.New()

	delivered := hub.Send(id, []byte(`{}`))
	require.False(t, delivered)
}

func TestHub_Unregister(t *testing.T) {
	hub := ws.NewHub()
	id := uuid.New()
	client := ws.NewTestClient(id, "alice")
	hub.Register(client)
	hub.Unregister(client)

	delivered := hub.Send(id, []byte(`{}`))
	require.False(t, delivered)
}

func TestHub_IsOnline(t *testing.T) {
	hub := ws.NewHub()
	id := uuid.New()
	require.False(t, hub.IsOnline(id))

	client := ws.NewTestClient(id, "alice")
	hub.Register(client)
	require.True(t, hub.IsOnline(id))

	hub.Unregister(client)
	require.False(t, hub.IsOnline(id))
}
