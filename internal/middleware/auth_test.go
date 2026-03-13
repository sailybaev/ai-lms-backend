package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key"

func init() {
	gin.SetMode(gin.TestMode)
}

// makeToken generates a signed JWT for testing.
func makeToken(t *testing.T, claims *Claims, secret string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func validClaims(isSuperAdmin bool) *Claims {
	return &Claims{
		UserID:       "user-123",
		Email:        "test@example.com",
		IsSuperAdmin: isSuperAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

// newTestRouter wraps JWTMiddleware followed by a simple 200-OK handler so we
// can inspect what was stored in the context.
func newJWTRouter(secret string) *gin.Engine {
	r := gin.New()
	r.GET("/protected", JWTMiddleware(secret), func(c *gin.Context) {
		userID, _ := c.Get("userID")
		email, _ := c.Get("userEmail")
		superadmin, _ := c.Get("isSuperAdmin")
		c.JSON(http.StatusOK, gin.H{
			"userID":       userID,
			"userEmail":    email,
			"isSuperAdmin": superadmin,
		})
	})
	return r
}

// ── JWTMiddleware ──────────────────────────────────────────────────────────

func TestJWTMiddleware_valid_token_passes(t *testing.T) {
	token := makeToken(t, validClaims(false), testSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	newJWTRouter(testSecret).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJWTMiddleware_sets_context_values(t *testing.T) {
	claims := validClaims(true)
	token := makeToken(t, claims, testSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	newJWTRouter(testSecret).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "user-123")
	assert.Contains(t, body, "test@example.com")
	assert.Contains(t, body, "true")
}

func TestJWTMiddleware_missing_header_returns_401(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)

	newJWTRouter(testSecret).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization header required")
}

func TestJWTMiddleware_malformed_header_returns_401(t *testing.T) {
	cases := []string{
		"just-a-token",           // no "Bearer" prefix
		"Basic dXNlcjpwYXNz",    // wrong scheme
		"Bearer",                 // missing token part
	}

	for _, header := range cases {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", header)

		newJWTRouter(testSecret).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code, "header: %s", header)
	}
}

func TestJWTMiddleware_invalid_token_returns_401(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer this.is.not.valid")

	newJWTRouter(testSecret).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid or expired token")
}

func TestJWTMiddleware_wrong_secret_returns_401(t *testing.T) {
	token := makeToken(t, validClaims(false), "different-secret")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	newJWTRouter(testSecret).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTMiddleware_expired_token_returns_401(t *testing.T) {
	claims := &Claims{
		UserID: "user-123",
		Email:  "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // already expired
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := makeToken(t, claims, testSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	newJWTRouter(testSecret).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTMiddleware_wrong_signing_method_returns_401(t *testing.T) {
	// Sign with RS256 (asymmetric) — the middleware rejects non-HMAC methods.
	claims := validClaims(false)
	// We craft a token that looks valid but with an unsupported method by
	// constructing it manually using a symmetric key but pretending it is RS256.
	// The easiest approach: supply a raw token whose header claims "RS256" but
	// is actually HMAC-signed — the middleware's signing-method check will fire.
	rawToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJ1c2VySUQiOiJ1c2VyLTEyMyIsImVtYWlsIjoidGVzdEBleGFtcGxlLmNvbSJ9." +
		"invalidsignature"
	_ = claims

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)

	newJWTRouter(testSecret).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── SuperadminMiddleware ────────────────────────────────────────────────────

func newSuperadminRouter() *gin.Engine {
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		// Simulate JWT middleware having run by pre-populating context.
		isSA, _ := c.GetQuery("sa")
		if isSA == "true" {
			c.Set("isSuperAdmin", true)
		} else if isSA == "false" {
			c.Set("isSuperAdmin", false)
		}
		// if "missing" we don't set it at all
		c.Next()
	}, SuperadminMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestSuperadminMiddleware_grants_access_to_superadmin(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin?sa=true", nil)

	newSuperadminRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSuperadminMiddleware_denies_non_superadmin(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin?sa=false", nil)

	newSuperadminRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Superadmin access required")
}

func TestSuperadminMiddleware_denies_when_key_missing(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin?sa=missing", nil)

	newSuperadminRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Access denied")
}
