package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/services"
	"gorm.io/gorm"
)

type CoursesHandler struct {
	DB *gorm.DB
}

func NewCoursesHandler(db *gorm.DB) *CoursesHandler {
	return &CoursesHandler{DB: db}
}

// ListCourses handles GET /api/org/:org/courses
func (h *CoursesHandler) ListCourses(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var courses []models.Course
	if err := h.DB.Where("org_id = ?", org.ID).
		Preload("Instructors").
		Preload("Instructors.User").
		Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch courses"})
		return
	}

	type InstructorInfo struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		Email     string  `json:"email"`
		AvatarURL *string `json:"avatarUrl"`
	}

	type CourseResponse struct {
		ID              string           `json:"id"`
		Title           string           `json:"title"`
		Description     *string          `json:"description"`
		ThumbnailURL    *string          `json:"thumbnailUrl"`
		Status          string           `json:"status"`
		CreatedBy       string           `json:"createdBy"`
		Instructors     []InstructorInfo `json:"instructors"`
		EnrollmentCount int64            `json:"enrollmentCount"`
		CreatedAt       interface{}      `json:"createdAt"`
	}

	result := make([]CourseResponse, 0, len(courses))
	for _, course := range courses {
		var count int64
		h.DB.Model(&models.Enrollment{}).Where("course_id = ? AND status = 'active'", course.ID).Count(&count)

		instructors := make([]InstructorInfo, 0)
		for _, ci := range course.Instructors {
			if ci.User != nil {
				instructors = append(instructors, InstructorInfo{
					ID:        ci.User.ID,
					Name:      ci.User.Name,
					Email:     ci.User.Email,
					AvatarURL: ci.User.AvatarURL,
				})
			}
		}

		result = append(result, CourseResponse{
			ID:              course.ID,
			Title:           course.Title,
			Description:     course.Description,
			ThumbnailURL:    course.ThumbnailURL,
			Status:          string(course.Status),
			CreatedBy:       course.CreatedBy,
			Instructors:     instructors,
			EnrollmentCount: count,
			CreatedAt:       course.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"courses": result})
}

type CreateCourseRequest struct {
	Title         string   `json:"title" binding:"required"`
	Description   *string  `json:"description"`
	ThumbnailURL  *string  `json:"thumbnailUrl"`
	Status        *string  `json:"status"`
	InstructorIDs []string `json:"instructorIds"`
}

// CreateCourse handles POST /api/org/:org/courses
func (h *CoursesHandler) CreateCourse(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, user, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := models.CourseStatusDraft
	if req.Status != nil {
		status = models.CourseStatus(*req.Status)
	}

	course := models.Course{
		OrgID:        org.ID,
		Title:        req.Title,
		Description:  req.Description,
		ThumbnailURL: req.ThumbnailURL,
		Status:       status,
		CreatedBy:    user.ID,
	}

	if err := h.DB.Create(&course).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create course"})
		return
	}

	// Add instructors
	for _, instructorID := range req.InstructorIDs {
		ci := models.CourseInstructor{
			CourseID: course.ID,
			UserID:   instructorID,
		}
		h.DB.Create(&ci)
	}

	h.DB.Preload("Instructors").Preload("Instructors.User").First(&course, "id = ?", course.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Course created successfully",
		"course":  course,
	})
}

type UpdateCourseRequest struct {
	CourseID      string   `json:"courseId" binding:"required"`
	Title         *string  `json:"title"`
	Description   *string  `json:"description"`
	ThumbnailURL  *string  `json:"thumbnailUrl"`
	Status        *string  `json:"status"`
	InstructorIDs []string `json:"instructorIds"`
}

// UpdateCourse handles PUT /api/org/:org/courses
func (h *CoursesHandler) UpdateCourse(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req UpdateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var course models.Course
	if err := h.DB.Where("id = ? AND org_id = ?", req.CourseID, org.ID).First(&course).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Course not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ThumbnailURL != nil {
		updates["thumbnail_url"] = *req.ThumbnailURL
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&course).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update course"})
			return
		}
	}

	// Update instructors if provided
	if req.InstructorIDs != nil {
		h.DB.Where("course_id = ?", course.ID).Delete(&models.CourseInstructor{})
		for _, instructorID := range req.InstructorIDs {
			ci := models.CourseInstructor{
				CourseID: course.ID,
				UserID:   instructorID,
			}
			h.DB.Create(&ci)
		}
	}

	h.DB.Preload("Instructors").Preload("Instructors.User").First(&course, "id = ?", course.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Course updated successfully",
		"course":  course,
	})
}

// DeleteCourse handles DELETE /api/org/:org/courses?courseId=...
func (h *CoursesHandler) DeleteCourse(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")
	courseID := c.Query("courseId")

	if courseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "courseId query parameter is required"})
		return
	}

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var course models.Course
	if err := h.DB.Where("id = ? AND org_id = ?", courseID, org.ID).First(&course).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Course not found"})
		return
	}

	// Delete related records first
	h.DB.Where("course_id = ?", courseID).Delete(&models.CourseInstructor{})
	h.DB.Where("course_id = ?", courseID).Delete(&models.Enrollment{})

	if err := h.DB.Delete(&course).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete course"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Course deleted successfully"})
}

// handleOrgError is a helper to respond with the right status based on service errors
func handleOrgError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrOrgNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}
	if errors.Is(err, services.ErrUserNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}
	if errors.Is(err, services.ErrForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
}
