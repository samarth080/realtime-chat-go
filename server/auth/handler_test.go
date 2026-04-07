package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/samarth080/peer-chat/auth"
	"github.com/samarth080/peer-chat/db"
	"github.com/stretchr/testify/require"
)

func testHandler(t *testing.T) (*auth.Handler, func()) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://chat:chat@localhost:5432/chatdb?sslmode=disable"
	}
	pool, err := db.NewPool(context.Background(), url)
	require.NoError(t, err)
	h := auth.NewHandler(pool, "testsecret")
	return h, func() { pool.Close() }
}

func TestRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, cleanup := testHandler(t)
	defer cleanup()

	r := gin.New()
	r.POST("/auth/register", h.Register)

	body, _ := json.Marshal(map[string]string{"username": "testuser_" + t.Name(), "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["token"])
	require.NotEmpty(t, resp["user_id"])
}

func TestRegister_DuplicateUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, cleanup := testHandler(t)
	defer cleanup()

	r := gin.New()
	r.POST("/auth/register", h.Register)

	username := "dupuser_" + t.Name()
	body, _ := json.Marshal(map[string]string{"username": username, "password": "pass123"})

	req1 := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusCreated, w1.Code)

	body2, _ := json.Marshal(map[string]string{"username": username, "password": "pass123"})
	req2 := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusConflict, w2.Code)
}

func TestLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, cleanup := testHandler(t)
	defer cleanup()

	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)

	username := "loginuser_" + t.Name()
	regBody, _ := json.Marshal(map[string]string{"username": username, "password": "mypassword"})
	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), regReq)

	loginBody, _ := json.Marshal(map[string]string{"username": username, "password": "mypassword"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["token"])
}

func TestLogin_WrongPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, cleanup := testHandler(t)
	defer cleanup()

	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)

	username := "badlogin_" + t.Name()
	regBody, _ := json.Marshal(map[string]string{"username": username, "password": "correct"})
	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), regReq)

	loginBody, _ := json.Marshal(map[string]string{"username": username, "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
