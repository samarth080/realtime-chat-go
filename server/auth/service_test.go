package auth_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/auth"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheck(t *testing.T) {
	hash, err := auth.HashPassword("secret123")
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	require.NoError(t, auth.CheckPassword(hash, "secret123"))
	require.Error(t, auth.CheckPassword(hash, "wrongpassword"))
}

func TestGenerateAndParseToken(t *testing.T) {
	id := uuid.New()
	secret := "testsecret"

	token, err := auth.GenerateToken(id, "alice", secret)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := auth.ParseToken(token, secret)
	require.NoError(t, err)
	require.Equal(t, id, claims.UserID)
	require.Equal(t, "alice", claims.Username)
}

func TestParseToken_WrongSecret(t *testing.T) {
	id := uuid.New()
	token, _ := auth.GenerateToken(id, "alice", "secret1")

	_, err := auth.ParseToken(token, "secret2")
	require.Error(t, err)
}
