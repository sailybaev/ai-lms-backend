package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// NewTestContext creates a *gin.Context backed by a ResponseRecorder.
// It pre-populates "userEmail", "userID", and "isSuperAdmin" so handlers that
// call c.Get("userEmail") work without a real JWT round-trip.
func NewTestContext(method, path, userEmail, userID string, isSuperAdmin bool, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()

	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}

	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("userEmail", userEmail)
	c.Set("userID", userID)
	c.Set("isSuperAdmin", isSuperAdmin)

	return c, w
}

// JSONBody decodes the recorder's body into v.
func JSONBody(w *httptest.ResponseRecorder, v interface{}) error {
	return json.NewDecoder(w.Body).Decode(v)
}
