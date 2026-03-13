package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/testutil"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func profileRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := testutil.NewTestDB(t)
	h := NewProfileHandler(db, "/tmp/uploads")
	r := gin.New()
	org := r.Group("/api/org/:org")
	org.Use(func(c *gin.Context) {
		c.Set("userEmail", c.GetHeader("X-Test-Email"))
		c.Next()
	})
	org.GET("/profile", h.GetProfile)
	org.PATCH("/profile", h.UpdateProfile)
	org.DELETE("/profile", h.DeleteProfile)
	org.PATCH("/profile/password", h.ChangePassword)
	return r, db
}

func seedMember(t *testing.T, db *gorm.DB, org models.Organization, email, name string, role models.Role) models.User {
	t.Helper()
	user := models.User{Email: email, Name: name}
	require.NoError(t, db.Create(&user).Error)
	m := models.Membership{OrgID: org.ID, UserID: user.ID, Role: role, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&m).Error)
	return user
}

// ── GetProfile ────────────────────────────────────────────────────────────────

func TestGetProfile_success(t *testing.T) {
	r, db := profileRouter(t)
	org, admin := seedAdminWithOrg(t, db, "get-profile-org")

	w := doRequest(r, http.MethodGet, "/api/org/"+org.Slug+"/profile", admin.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, admin.Email, resp["email"])
	assert.Equal(t, admin.Name, resp["name"])
	membership := resp["membership"].(map[string]interface{})
	assert.Equal(t, "admin", membership["role"])
}

func TestGetProfile_org_not_found_returns_404(t *testing.T) {
	r, db := profileRouter(t)
	user := models.User{Email: "ghost@profile.com", Name: "Ghost"}
	require.NoError(t, db.Create(&user).Error)

	w := doRequest(r, http.MethodGet, "/api/org/nonexistent/profile", user.Email, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetProfile_non_member_returns_403(t *testing.T) {
	r, db := profileRouter(t)
	org := models.Organization{Slug: "profile-nonmember-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	outsider := models.User{Email: "outsider@profile.com", Name: "Outsider"}
	require.NoError(t, db.Create(&outsider).Error)

	w := doRequest(r, http.MethodGet, "/api/org/"+org.Slug+"/profile", outsider.Email, nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── UpdateProfile ─────────────────────────────────────────────────────────────

func TestUpdateProfile_success(t *testing.T) {
	r, db := profileRouter(t)
	org, admin := seedAdminWithOrg(t, db, "update-profile-org")

	body := map[string]interface{}{"name": "Updated Name"}
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/profile", admin.Email, body)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	user := resp["user"].(map[string]interface{})
	assert.Equal(t, "Updated Name", user["name"])
}

func TestUpdateProfile_empty_name_returns_400(t *testing.T) {
	r, db := profileRouter(t)
	org, admin := seedAdminWithOrg(t, db, "update-profile-empty-name")

	body := map[string]interface{}{"name": ""}
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/profile", admin.Email, body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Name cannot be empty")
}

func TestUpdateProfile_no_changes_returns_200(t *testing.T) {
	r, db := profileRouter(t)
	org, admin := seedAdminWithOrg(t, db, "update-profile-noop")

	// Empty body → no updates but still 200.
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/profile", admin.Email, map[string]interface{}{})

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateProfile_updates_avatar_url(t *testing.T) {
	r, db := profileRouter(t)
	org, admin := seedAdminWithOrg(t, db, "update-profile-avatar")

	body := map[string]interface{}{"avatarUrl": "https://example.com/avatar.png"}
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/profile", admin.Email, body)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	user := resp["user"].(map[string]interface{})
	assert.Equal(t, "https://example.com/avatar.png", user["avatarUrl"])
}

// ── DeleteProfile (suspend membership) ───────────────────────────────────────

func TestDeleteProfile_suspends_membership(t *testing.T) {
	r, db := profileRouter(t)
	org, admin := seedAdminWithOrg(t, db, "delete-profile-org")

	w := doRequest(r, http.MethodDelete, "/api/org/"+org.Slug+"/profile", admin.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "suspended")

	// Verify membership status is suspended in DB.
	var membership models.Membership
	require.NoError(t, db.Where("org_id = ? AND user_id = ?", org.ID, admin.ID).First(&membership).Error)
	assert.Equal(t, models.MembershipStatusSuspended, membership.Status)
}

func TestDeleteProfile_non_member_returns_403(t *testing.T) {
	r, db := profileRouter(t)
	org := models.Organization{Slug: "delete-profile-nomember", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	outsider := models.User{Email: "del-outsider@profile.com", Name: "Outsider"}
	require.NoError(t, db.Create(&outsider).Error)

	w := doRequest(r, http.MethodDelete, "/api/org/"+org.Slug+"/profile", outsider.Email, nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── ChangePassword ────────────────────────────────────────────────────────────

func TestChangePassword_success(t *testing.T) {
	r, db := profileRouter(t)
	org := models.Organization{Slug: "change-pass-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)

	hash, _ := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	hashStr := string(hash)
	user := models.User{Email: "changeme@pass.com", Name: "Change Pass", PasswordHash: &hashStr}
	require.NoError(t, db.Create(&user).Error)
	m := models.Membership{OrgID: org.ID, UserID: user.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&m).Error)

	body := map[string]interface{}{"currentPassword": "old-password", "newPassword": "new-password123"}
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/profile/password", user.Email, body)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Password updated successfully")

	// Verify the new password works.
	var updatedUser models.User
	require.NoError(t, db.Where("email = ?", user.Email).First(&updatedUser).Error)
	require.NotNil(t, updatedUser.PasswordHash)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*updatedUser.PasswordHash), []byte("new-password123")))
}

func TestChangePassword_wrong_current_password_returns_401(t *testing.T) {
	r, db := profileRouter(t)
	org := models.Organization{Slug: "wrong-pass-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	hashStr := string(hash)
	user := models.User{Email: "wrongcurrent@pass.com", Name: "User", PasswordHash: &hashStr}
	require.NoError(t, db.Create(&user).Error)
	m := models.Membership{OrgID: org.ID, UserID: user.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&m).Error)

	body := map[string]interface{}{"currentPassword": "wrong-password", "newPassword": "new-pass123"}
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/profile/password", user.Email, body)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Current password is incorrect")
}

func TestChangePassword_no_password_set_returns_400(t *testing.T) {
	r, db := profileRouter(t)
	org := models.Organization{Slug: "no-pass-set-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)

	user := models.User{Email: "nopassset@pass.com", Name: "User"} // no password hash
	require.NoError(t, db.Create(&user).Error)
	m := models.Membership{OrgID: org.ID, UserID: user.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&m).Error)

	body := map[string]interface{}{"currentPassword": "any", "newPassword": "newpass123"}
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/profile/password", user.Email, body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "No password set")
}

func TestChangePassword_short_new_password_returns_400(t *testing.T) {
	r, db := profileRouter(t)
	org := models.Organization{Slug: "short-pass-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)

	hash, _ := bcrypt.GenerateFromPassword([]byte("current"), bcrypt.MinCost)
	hashStr := string(hash)
	user := models.User{Email: "short@pass.com", Name: "User", PasswordHash: &hashStr}
	require.NoError(t, db.Create(&user).Error)
	m := models.Membership{OrgID: org.ID, UserID: user.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&m).Error)

	// newPassword must be min=6 chars (from binding tag).
	body := map[string]interface{}{"currentPassword": "current", "newPassword": "abc"}
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/profile/password", user.Email, body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChangePassword_missing_fields_returns_400(t *testing.T) {
	r, db := profileRouter(t)
	org := models.Organization{Slug: "missing-pass-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Email: "missing@pass.com", Name: "User"}
	require.NoError(t, db.Create(&user).Error)
	m := models.Membership{OrgID: org.ID, UserID: user.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&m).Error)

	cases := []map[string]interface{}{
		{"newPassword": "newpass123"},                    // missing currentPassword
		{"currentPassword": "current"},                  // missing newPassword
		{},                                              // empty body
	}
	for _, body := range cases {
		w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/profile/password", user.Email, body)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}
