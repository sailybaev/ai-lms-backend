package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/testutil"
	"gorm.io/gorm"
)

// ── helpers ─────────────────────────────────────────────────────────────────

func seedOrgUserMembership(t *testing.T, db *gorm.DB, slug string, role models.Role) (models.Organization, models.User, models.Membership) {
	t.Helper()
	org := models.Organization{Slug: slug, Name: "Org " + slug}
	require.NoError(t, db.Create(&org).Error)

	user := models.User{Email: slug + "@example.com", Name: "User " + slug}
	require.NoError(t, db.Create(&user).Error)

	membership := models.Membership{
		OrgID:  org.ID,
		UserID: user.ID,
		Role:   role,
		Status: models.MembershipStatusActive,
	}
	require.NoError(t, db.Create(&membership).Error)

	return org, user, membership
}

// ── GetOrgAndVerifyRole ──────────────────────────────────────────────────────

func TestGetOrgAndVerifyRole_success_with_matching_role(t *testing.T) {
	db := testutil.NewTestDB(t)
	org, user, _ := seedOrgUserMembership(t, db, "slug-admin", models.RoleAdmin)

	retOrg, retUser, err := GetOrgAndVerifyRole(db, org.Slug, user.Email, "admin")

	require.NoError(t, err)
	assert.Equal(t, org.ID, retOrg.ID)
	assert.Equal(t, user.ID, retUser.ID)
}

func TestGetOrgAndVerifyRole_allows_multiple_roles(t *testing.T) {
	db := testutil.NewTestDB(t)
	org, user, _ := seedOrgUserMembership(t, db, "slug-teacher", models.RoleTeacher)

	_, _, err := GetOrgAndVerifyRole(db, org.Slug, user.Email, "admin", "teacher")

	require.NoError(t, err)
}

func TestGetOrgAndVerifyRole_no_roles_skips_role_check(t *testing.T) {
	db := testutil.NewTestDB(t)

	org := models.Organization{Slug: "any-role-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Email: "student@norole.com", Name: "Student"}
	require.NoError(t, db.Create(&user).Error)
	// No membership created — no role check should be performed.

	retOrg, retUser, err := GetOrgAndVerifyRole(db, org.Slug, user.Email)

	require.NoError(t, err)
	assert.Equal(t, org.ID, retOrg.ID)
	assert.Equal(t, user.ID, retUser.ID)
}

func TestGetOrgAndVerifyRole_org_not_found(t *testing.T) {
	db := testutil.NewTestDB(t)

	user := models.User{Email: "nobody@example.com", Name: "Nobody"}
	require.NoError(t, db.Create(&user).Error)

	_, _, err := GetOrgAndVerifyRole(db, "nonexistent-org", user.Email, "admin")

	assert.ErrorIs(t, err, ErrOrgNotFound)
}

func TestGetOrgAndVerifyRole_user_not_found(t *testing.T) {
	db := testutil.NewTestDB(t)

	org := models.Organization{Slug: "existing-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)

	_, _, err := GetOrgAndVerifyRole(db, org.Slug, "ghost@example.com", "admin")

	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestGetOrgAndVerifyRole_forbidden_when_not_member(t *testing.T) {
	db := testutil.NewTestDB(t)

	org := models.Organization{Slug: "org-no-member", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Email: "nonmember@example.com", Name: "Non Member"}
	require.NoError(t, db.Create(&user).Error)
	// No membership — role check should return ErrForbidden.

	_, _, err := GetOrgAndVerifyRole(db, org.Slug, user.Email, "admin")

	assert.ErrorIs(t, err, ErrForbidden)
}

func TestGetOrgAndVerifyRole_forbidden_when_wrong_role(t *testing.T) {
	db := testutil.NewTestDB(t)
	org, user, _ := seedOrgUserMembership(t, db, "slug-student", models.RoleStudent)

	// Requires "admin" but user is "student".
	_, _, err := GetOrgAndVerifyRole(db, org.Slug, user.Email, "admin")

	assert.ErrorIs(t, err, ErrForbidden)
}

// ── GetOrgAndMembership ──────────────────────────────────────────────────────

func TestGetOrgAndMembership_success(t *testing.T) {
	db := testutil.NewTestDB(t)
	org, user, membership := seedOrgUserMembership(t, db, "member-org", models.RoleStudent)

	retOrg, retUser, retMembership, err := GetOrgAndMembership(db, org.Slug, user.Email)

	require.NoError(t, err)
	assert.Equal(t, org.ID, retOrg.ID)
	assert.Equal(t, user.ID, retUser.ID)
	assert.Equal(t, membership.ID, retMembership.ID)
	assert.Equal(t, models.RoleStudent, retMembership.Role)
}

func TestGetOrgAndMembership_org_not_found(t *testing.T) {
	db := testutil.NewTestDB(t)

	user := models.User{Email: "user@test.com", Name: "User"}
	require.NoError(t, db.Create(&user).Error)

	_, _, _, err := GetOrgAndMembership(db, "no-such-org", user.Email)

	assert.ErrorIs(t, err, ErrOrgNotFound)
}

func TestGetOrgAndMembership_user_not_found(t *testing.T) {
	db := testutil.NewTestDB(t)

	org := models.Organization{Slug: "org-user-missing", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)

	_, _, _, err := GetOrgAndMembership(db, org.Slug, "notareal@email.com")

	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestGetOrgAndMembership_forbidden_when_not_member(t *testing.T) {
	db := testutil.NewTestDB(t)

	org := models.Organization{Slug: "org-no-membership", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Email: "outsider@example.com", Name: "Outsider"}
	require.NoError(t, db.Create(&user).Error)

	_, _, _, err := GetOrgAndMembership(db, org.Slug, user.Email)

	assert.ErrorIs(t, err, ErrForbidden)
}

// ── sentinel errors ──────────────────────────────────────────────────────────

func TestSentinelErrors_are_distinct(t *testing.T) {
	assert.NotEqual(t, ErrOrgNotFound, ErrUserNotFound)
	assert.NotEqual(t, ErrOrgNotFound, ErrForbidden)
	assert.NotEqual(t, ErrUserNotFound, ErrForbidden)
}
