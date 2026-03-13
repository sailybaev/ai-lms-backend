package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/testutil"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const testJWTSecret = "test-secret"

func loginRouter(t *testing.T) (*gin.Engine, func(email, name, password string) models.User) {
	t.Helper()
	db := testutil.NewTestDB(t)
	h := NewAuthHandler(db, testJWTSecret)
	r := gin.New()
	r.POST("/api/auth/login", h.Login)

	createUser := func(email, name, password string) models.User {
		var hash []byte
		if password != "" {
			var err error
			hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
			require.NoError(t, err)
		}
		user := models.User{Email: email, Name: name}
		if hash != nil {
			s := string(hash)
			user.PasswordHash = &s
		}
		require.NoError(t, db.Create(&user).Error)
		return user
	}

	return r, createUser
}

func doLogin(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// ── Login ────────────────────────────────────────────────────────────────────

func TestLogin_success(t *testing.T) {
	r, create := loginRouter(t)
	create("login@test.com", "Login User", "secret123")

	w := doLogin(r, `{"email":"login@test.com","password":"secret123"}`)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["token"])
	userMap := resp["user"].(map[string]interface{})
	assert.Equal(t, "login@test.com", userMap["email"])
}

func TestLogin_invalid_email_format_returns_400(t *testing.T) {
	r, _ := loginRouter(t)
	w := doLogin(r, `{"email":"not-an-email","password":"secret"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_missing_password_returns_400(t *testing.T) {
	r, _ := loginRouter(t)
	w := doLogin(r, `{"email":"a@b.com"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_missing_email_returns_400(t *testing.T) {
	r, _ := loginRouter(t)
	w := doLogin(r, `{"password":"pass"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_empty_body_returns_400(t *testing.T) {
	r, _ := loginRouter(t)
	w := doLogin(r, `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_wrong_password_returns_401(t *testing.T) {
	r, create := loginRouter(t)
	create("wrongpass@test.com", "User", "correct-pass")

	w := doLogin(r, `{"email":"wrongpass@test.com","password":"wrong-pass"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid email or password")
}

func TestLogin_user_not_found_returns_401(t *testing.T) {
	r, _ := loginRouter(t)
	w := doLogin(r, `{"email":"ghost@test.com","password":"anypass"}`)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_user_has_no_password_returns_401(t *testing.T) {
	r, create := loginRouter(t)
	create("nopass@test.com", "No Pass User", "") // password="" → no hash

	w := doLogin(r, `{"email":"nopass@test.com","password":"whatever"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_superadmin_flag_reflected_in_response(t *testing.T) {
	db := testutil.NewTestDB(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("adminpass"), bcrypt.MinCost)
	hashStr := string(hash)
	user := models.User{Email: "superadmin@test.com", Name: "Super", PasswordHash: &hashStr, IsSuperAdmin: true}
	require.NoError(t, db.Create(&user).Error)

	h := NewAuthHandler(db, testJWTSecret)
	r := gin.New()
	r.POST("/api/auth/login", h.Login)

	w := doLogin(r, `{"email":"superadmin@test.com","password":"adminpass"}`)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["user"].(map[string]interface{})["isSuperAdmin"])
}
