package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/auth"
	"github.com/samarth080/peer-chat/middleware"
	"github.com/stretchr/testify/require"
)

func setupRouter(secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", middleware.JWTAuth(secret), func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		c.JSON(http.StatusOK, gin.H{"user_id": userID, "username": username})
	})
	return r
}

func TestJWTAuth_ValidToken(t *testing.T) {
	secret := "testsecret"
	id := uuid.New()
	token, _ := auth.GenerateToken(id, "alice", secret)

	r := setupRouter(secret)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestJWTAuth_TokenInQueryParam(t *testing.T) {
	secret := "testsecret"
	id := uuid.New()
	token, _ := auth.GenerateToken(id, "alice", secret)

	r := setupRouter(secret)
	req := httptest.NewRequest(http.MethodGet, "/protected?token="+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestJWTAuth_MissingToken(t *testing.T) {
	r := setupRouter("secret")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	r := setupRouter("secret")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer notavalidtoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
