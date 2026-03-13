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
	"gorm.io/gorm"
)

func studentsRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := testutil.NewTestDB(t)
	h := NewStudentsHandler(db)
	r := gin.New()
	org := r.Group("/api/org/:org")
	org.Use(func(c *gin.Context) {
		c.Set("userEmail", c.GetHeader("X-Test-Email"))
		c.Next()
	})
	org.GET("/students", h.ListStudents)
	org.POST("/students", h.CreateStudent)
	org.PUT("/students", h.UpdateStudent)
	org.PATCH("/students", h.PatchStudentStatus)
	org.DELETE("/students", h.DeleteStudent)
	return r, db
}

// ── ListStudents ──────────────────────────────────────────────────────────────

func TestListStudents_empty_for_new_org(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "list-students-empty")

	w := doRequest(r, http.MethodGet, "/api/org/"+org.Slug+"/students", admin.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Len(t, resp["students"].([]interface{}), 0)
}

func TestListStudents_returns_only_students(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "list-students-populated")

	// Add a student.
	student := models.User{Email: "student1@list.com", Name: "Student 1"}
	require.NoError(t, db.Create(&student).Error)
	ms := models.Membership{OrgID: org.ID, UserID: student.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&ms).Error)

	// Add a teacher — should NOT appear in student list.
	teacher := models.User{Email: "teacher1@list.com", Name: "Teacher 1"}
	require.NoError(t, db.Create(&teacher).Error)
	mt := models.Membership{OrgID: org.ID, UserID: teacher.ID, Role: models.RoleTeacher, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&mt).Error)

	w := doRequest(r, http.MethodGet, "/api/org/"+org.Slug+"/students", admin.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	students := resp["students"].([]interface{})
	assert.Len(t, students, 1)
	s := students[0].(map[string]interface{})
	assert.Equal(t, "student1@list.com", s["email"])
}

func TestListStudents_accessible_by_teacher(t *testing.T) {
	r, db := studentsRouter(t)
	org := models.Organization{Slug: "list-teacher-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	teacher := models.User{Email: "teacher@list.com", Name: "Teacher"}
	require.NoError(t, db.Create(&teacher).Error)
	mt := models.Membership{OrgID: org.ID, UserID: teacher.ID, Role: models.RoleTeacher, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&mt).Error)

	w := doRequest(r, http.MethodGet, "/api/org/"+org.Slug+"/students", teacher.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListStudents_forbidden_for_student(t *testing.T) {
	r, db := studentsRouter(t)
	org := models.Organization{Slug: "list-student-forbidden", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	student := models.User{Email: "student@forbidden2.com", Name: "Student"}
	require.NoError(t, db.Create(&student).Error)
	ms := models.Membership{OrgID: org.ID, UserID: student.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&ms).Error)

	w := doRequest(r, http.MethodGet, "/api/org/"+org.Slug+"/students", student.Email, nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── CreateStudent ─────────────────────────────────────────────────────────────

func TestCreateStudent_success_new_user(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "create-student-new")

	body := map[string]interface{}{
		"name":     "New Student",
		"email":    "newstudent@create.com",
		"password": "pass123",
	}
	w := doRequest(r, http.MethodPost, "/api/org/"+org.Slug+"/students", admin.Email, body)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	student := resp["student"].(map[string]interface{})
	assert.Equal(t, "newstudent@create.com", student["email"])
}

func TestCreateStudent_adds_existing_user_to_org(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "create-student-existing")

	// Pre-create user (e.g. already on another org).
	existing := models.User{Email: "existing@create.com", Name: "Existing"}
	require.NoError(t, db.Create(&existing).Error)

	body := map[string]interface{}{"name": "Existing", "email": "existing@create.com"}
	w := doRequest(r, http.MethodPost, "/api/org/"+org.Slug+"/students", admin.Email, body)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateStudent_conflict_when_already_member(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "create-student-conflict")

	student := models.User{Email: "alreadymember@create.com", Name: "Member"}
	require.NoError(t, db.Create(&student).Error)
	ms := models.Membership{OrgID: org.ID, UserID: student.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&ms).Error)

	body := map[string]interface{}{"name": "Member", "email": "alreadymember@create.com"}
	w := doRequest(r, http.MethodPost, "/api/org/"+org.Slug+"/students", admin.Email, body)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "already a member")
}

func TestCreateStudent_missing_required_fields_returns_400(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "create-student-missing")

	cases := []map[string]interface{}{
		{"email": "a@b.com"},           // missing name
		{"name": "No Email"},           // missing email
		{"name": "Bad", "email": "x"},  // invalid email
	}
	for _, body := range cases {
		w := doRequest(r, http.MethodPost, "/api/org/"+org.Slug+"/students", admin.Email, body)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func TestCreateStudent_without_password_still_works(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "create-student-nopass")

	body := map[string]interface{}{"name": "No Password", "email": "nopass@create.com"}
	w := doRequest(r, http.MethodPost, "/api/org/"+org.Slug+"/students", admin.Email, body)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// ── PatchStudentStatus ────────────────────────────────────────────────────────

func TestPatchStudentStatus_success(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "patch-student-status")

	student := models.User{Email: "patch@status.com", Name: "Patch Student"}
	require.NoError(t, db.Create(&student).Error)
	ms := models.Membership{OrgID: org.ID, UserID: student.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&ms).Error)

	body := map[string]interface{}{"membershipId": ms.ID, "status": "suspended"}
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/students", admin.Email, body)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "suspended")
}

func TestPatchStudentStatus_membership_not_found_returns_404(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "patch-notfound-status")

	body := map[string]interface{}{"membershipId": "00000000-0000-0000-0000-000000000000", "status": "active"}
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/students", admin.Email, body)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPatchStudentStatus_missing_fields_returns_400(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "patch-missing-status")

	body := map[string]interface{}{"membershipId": "some-id"} // missing status
	w := doRequest(r, http.MethodPatch, "/api/org/"+org.Slug+"/students", admin.Email, body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── DeleteStudent ─────────────────────────────────────────────────────────────

func TestDeleteStudent_success(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "delete-student-ok")

	student := models.User{Email: "delete@student.com", Name: "Delete Me"}
	require.NoError(t, db.Create(&student).Error)
	ms := models.Membership{OrgID: org.ID, UserID: student.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&ms).Error)

	w := doRequest(r, http.MethodDelete, "/api/org/"+org.Slug+"/students?membershipId="+ms.ID, admin.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "removed successfully")
}

func TestDeleteStudent_missing_membership_id_returns_400(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "delete-student-no-id")

	w := doRequest(r, http.MethodDelete, "/api/org/"+org.Slug+"/students", admin.Email, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "membershipId")
}

func TestDeleteStudent_not_found_returns_404(t *testing.T) {
	r, db := studentsRouter(t)
	org, admin := seedAdminWithOrg(t, db, "delete-student-notfound")

	w := doRequest(r, http.MethodDelete, "/api/org/"+org.Slug+"/students?membershipId=00000000-0000-0000-0000-000000000000", admin.Email, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
