package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/testutil"
	"gorm.io/gorm"
)

// coursesRouter builds a Gin engine with all course routes attached to
// /api/org/:org/courses. The returned seedFn helper seeds org+user+membership.
func coursesRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := testutil.NewTestDB(t)
	h := NewCoursesHandler(db)
	r := gin.New()

	org := r.Group("/api/org/:org")
	// Inject auth context without a real JWT.
	org.Use(func(c *gin.Context) {
		c.Set("userEmail", c.GetHeader("X-Test-Email"))
		c.Next()
	})
	org.GET("/courses", h.ListCourses)
	org.POST("/courses", h.CreateCourse)
	org.PUT("/courses", h.UpdateCourse)
	org.DELETE("/courses", h.DeleteCourse)

	return r, db
}

func seedAdminWithOrg(t *testing.T, db *gorm.DB, slug string) (models.Organization, models.User) {
	t.Helper()
	org := models.Organization{Slug: slug, Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Email: slug + "@admin.com", Name: "Admin"}
	require.NoError(t, db.Create(&user).Error)
	m := models.Membership{OrgID: org.ID, UserID: user.ID, Role: models.RoleAdmin, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&m).Error)
	return org, user
}

func doRequest(r *gin.Engine, method, path, email string, body interface{}) *httptest.ResponseRecorder {
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req, _ := http.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Email", email)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ── ListCourses ──────────────────────────────────────────────────────────────

func TestListCourses_returns_empty_list_for_new_org(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "list-empty-org")

	w := doRequest(r, http.MethodGet, "/api/org/"+org.Slug+"/courses", user.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	courses := resp["courses"].([]interface{})
	assert.Len(t, courses, 0)
}

func TestListCourses_returns_courses_for_org(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "list-courses-org")
	course := models.Course{OrgID: org.ID, Title: "Go Basics", Status: models.CourseStatusActive, CreatedBy: user.ID}
	require.NoError(t, db.Create(&course).Error)

	w := doRequest(r, http.MethodGet, "/api/org/"+org.Slug+"/courses", user.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	courses := resp["courses"].([]interface{})
	assert.Len(t, courses, 1)
	c := courses[0].(map[string]interface{})
	assert.Equal(t, "Go Basics", c["title"])
}

func TestListCourses_forbidden_for_non_admin(t *testing.T) {
	r, db := coursesRouter(t)
	org := models.Organization{Slug: "list-forbidden-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	student := models.User{Email: "student@forbidden.com", Name: "Student"}
	require.NoError(t, db.Create(&student).Error)
	m := models.Membership{OrgID: org.ID, UserID: student.ID, Role: models.RoleStudent, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&m).Error)

	w := doRequest(r, http.MethodGet, "/api/org/"+org.Slug+"/courses", student.Email, nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListCourses_org_not_found_returns_404(t *testing.T) {
	r, db := coursesRouter(t)
	user := models.User{Email: "user@notfoundorg.com", Name: "User"}
	require.NoError(t, db.Create(&user).Error)

	w := doRequest(r, http.MethodGet, "/api/org/nonexistent/courses", user.Email, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── CreateCourse ─────────────────────────────────────────────────────────────

func TestCreateCourse_success(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "create-course-org")

	body := map[string]interface{}{"title": "New Course"}
	w := doRequest(r, http.MethodPost, "/api/org/"+org.Slug+"/courses", user.Email, body)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp, "course")
	course := resp["course"].(map[string]interface{})
	assert.Equal(t, "New Course", course["title"])
}

func TestCreateCourse_missing_title_returns_400(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "create-no-title-org")

	body := map[string]interface{}{"description": "No title"}
	w := doRequest(r, http.MethodPost, "/api/org/"+org.Slug+"/courses", user.Email, body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateCourse_forbidden_for_non_admin(t *testing.T) {
	r, db := coursesRouter(t)
	org := models.Organization{Slug: "create-forbidden-org", Name: "Org"}
	require.NoError(t, db.Create(&org).Error)
	teacher := models.User{Email: "teacher@forbidden.com", Name: "Teacher"}
	require.NoError(t, db.Create(&teacher).Error)
	m := models.Membership{OrgID: org.ID, UserID: teacher.ID, Role: models.RoleTeacher, Status: models.MembershipStatusActive}
	require.NoError(t, db.Create(&m).Error)

	body := map[string]interface{}{"title": "Unauthorized Course"}
	w := doRequest(r, http.MethodPost, "/api/org/"+org.Slug+"/courses", teacher.Email, body)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateCourse_with_status_sets_correct_status(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "create-status-org")

	body := map[string]interface{}{"title": "Active Course", "status": "active"}
	w := doRequest(r, http.MethodPost, "/api/org/"+org.Slug+"/courses", user.Email, body)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	course := resp["course"].(map[string]interface{})
	assert.Equal(t, "active", course["status"])
}

// ── UpdateCourse ─────────────────────────────────────────────────────────────

func TestUpdateCourse_success(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "update-course-org")
	course := models.Course{OrgID: org.ID, Title: "Old Title", Status: models.CourseStatusDraft, CreatedBy: user.ID}
	require.NoError(t, db.Create(&course).Error)

	newTitle := "New Title"
	body := map[string]interface{}{"courseId": course.ID, "title": newTitle}
	w := doRequest(r, http.MethodPut, "/api/org/"+org.Slug+"/courses", user.Email, body)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	c := resp["course"].(map[string]interface{})
	assert.Equal(t, newTitle, c["title"])
}

func TestUpdateCourse_missing_course_id_returns_400(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "update-no-id-org")

	body := map[string]interface{}{"title": "No ID"}
	w := doRequest(r, http.MethodPut, "/api/org/"+org.Slug+"/courses", user.Email, body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateCourse_course_not_found_returns_404(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "update-notfound-org")

	body := map[string]interface{}{"courseId": "00000000-0000-0000-0000-000000000000", "title": "Ghost"}
	w := doRequest(r, http.MethodPut, "/api/org/"+org.Slug+"/courses", user.Email, body)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── DeleteCourse ─────────────────────────────────────────────────────────────

func TestDeleteCourse_success(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "delete-course-org")
	course := models.Course{OrgID: org.ID, Title: "To Delete", Status: models.CourseStatusDraft, CreatedBy: user.ID}
	require.NoError(t, db.Create(&course).Error)

	w := doRequest(r, http.MethodDelete, "/api/org/"+org.Slug+"/courses?courseId="+course.ID, user.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "deleted successfully")

	// Verify it's actually gone.
	var count int64
	db.Model(&models.Course{}).Where("id = ?", course.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestDeleteCourse_missing_course_id_returns_400(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "delete-no-id-org")

	w := doRequest(r, http.MethodDelete, "/api/org/"+org.Slug+"/courses", user.Email, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "courseId")
}

func TestDeleteCourse_course_not_found_returns_404(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "delete-notfound-org")

	w := doRequest(r, http.MethodDelete, "/api/org/"+org.Slug+"/courses?courseId=00000000-0000-0000-0000-000000000000", user.Email, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteCourse_removes_enrollments(t *testing.T) {
	r, db := coursesRouter(t)
	org, user := seedAdminWithOrg(t, db, "delete-enrollments-org")
	course := models.Course{OrgID: org.ID, Title: "Enrolled Course", Status: models.CourseStatusActive, CreatedBy: user.ID}
	require.NoError(t, db.Create(&course).Error)
	student := models.User{Email: "student@del.com", Name: "Student"}
	require.NoError(t, db.Create(&student).Error)
	enrollment := models.Enrollment{OrgID: org.ID, CourseID: course.ID, UserID: student.ID, Status: models.EnrollmentStatusActive}
	require.NoError(t, db.Create(&enrollment).Error)

	w := doRequest(r, http.MethodDelete, "/api/org/"+org.Slug+"/courses?courseId="+course.ID, user.Email, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var enrollCount int64
	db.Model(&models.Enrollment{}).Where("course_id = ?", course.ID).Count(&enrollCount)
	assert.Equal(t, int64(0), enrollCount)
}
