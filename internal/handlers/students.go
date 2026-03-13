package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/services"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type StudentsHandler struct {
	DB *gorm.DB
}

func NewStudentsHandler(db *gorm.DB) *StudentsHandler {
	return &StudentsHandler{DB: db}
}

type StudentResponse struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Email        string      `json:"email"`
	AvatarURL    *string     `json:"avatarUrl"`
	Status       string      `json:"status"`
	MembershipID string      `json:"membershipId"`
	JoinedAt     interface{} `json:"joinedAt"`
	LastActiveAt *interface{} `json:"lastActiveAt"`
}

// ListStudents handles GET /api/org/:org/students
func (h *StudentsHandler) ListStudents(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin", "teacher")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var memberships []models.Membership
	if err := h.DB.Where("org_id = ? AND role = ?", org.ID, models.RoleStudent).
		Preload("User").
		Find(&memberships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch students"})
		return
	}

	students := make([]gin.H, 0, len(memberships))
	for _, m := range memberships {
		if m.User == nil {
			continue
		}
		students = append(students, gin.H{
			"id":           m.User.ID,
			"name":         m.User.Name,
			"email":        m.User.Email,
			"avatarUrl":    m.User.AvatarURL,
			"status":       m.Status,
			"membershipId": m.ID,
			"joinedAt":     m.CreatedAt,
			"lastActiveAt": m.User.LastActiveAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"students": students})
}

type CreateStudentRequest struct {
	Name     string  `json:"name" binding:"required"`
	Email    string  `json:"email" binding:"required,email"`
	Password *string `json:"password"`
}

// CreateStudent handles POST /api/org/:org/students
func (h *StudentsHandler) CreateStudent(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req CreateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var existingUser models.User
	userExists := h.DB.Where("email = ?", req.Email).First(&existingUser).Error == nil

	var targetUser models.User
	if userExists {
		targetUser = existingUser
	} else {
		targetUser = models.User{
			Email: req.Email,
			Name:  req.Name,
		}
		if req.Password != nil && *req.Password != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), 10)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
				return
			}
			hashStr := string(hash)
			targetUser.PasswordHash = &hashStr
		}
		if err := h.DB.Create(&targetUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	}

	// Check if membership already exists
	var existingMembership models.Membership
	if h.DB.Where("org_id = ? AND user_id = ?", org.ID, targetUser.ID).First(&existingMembership).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this organization"})
		return
	}

	membership := models.Membership{
		OrgID:  org.ID,
		UserID: targetUser.ID,
		Role:   models.RoleStudent,
		Status: models.MembershipStatusActive,
	}
	if err := h.DB.Create(&membership).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create membership"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Student created successfully",
		"student": gin.H{
			"id":           targetUser.ID,
			"name":         targetUser.Name,
			"email":        targetUser.Email,
			"avatarUrl":    targetUser.AvatarURL,
			"status":       membership.Status,
			"membershipId": membership.ID,
			"joinedAt":     membership.CreatedAt,
			"lastActiveAt": targetUser.LastActiveAt,
		},
	})
}

type UpdateStudentRequest struct {
	UserID      string  `json:"userId" binding:"required"`
	Name        string  `json:"name" binding:"required"`
	Email       string  `json:"email" binding:"required,email"`
	AvatarURL   *string `json:"avatarUrl"`
	NewPassword *string `json:"newPassword"`
}

// UpdateStudent handles PUT /api/org/:org/students
func (h *StudentsHandler) UpdateStudent(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req UpdateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify user is a student in this org
	var membership models.Membership
	if err := h.DB.Where("org_id = ? AND user_id = ? AND role = ?", org.ID, req.UserID, models.RoleStudent).First(&membership).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not found in organization"})
		return
	}

	var student models.User
	if err := h.DB.First(&student, "id = ?", req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	updates := map[string]interface{}{
		"name":  req.Name,
		"email": req.Email,
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	if req.NewPassword != nil && *req.NewPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.NewPassword), 10)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		updates["password_hash"] = string(hash)
	}

	if err := h.DB.Model(&student).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update student"})
		return
	}

	h.DB.First(&student, "id = ?", student.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Student updated successfully",
		"student": gin.H{
			"id":           student.ID,
			"name":         student.Name,
			"email":        student.Email,
			"avatarUrl":    student.AvatarURL,
			"status":       membership.Status,
			"membershipId": membership.ID,
		},
	})
}

type PatchStudentStatusRequest struct {
	MembershipID string `json:"membershipId" binding:"required"`
	Status       string `json:"status" binding:"required"`
}

// PatchStudentStatus handles PATCH /api/org/:org/students
func (h *StudentsHandler) PatchStudentStatus(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req PatchStudentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var membership models.Membership
	if err := h.DB.Where("id = ? AND org_id = ? AND role = ?", req.MembershipID, org.ID, models.RoleStudent).
		Preload("User").First(&membership).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student membership not found"})
		return
	}

	if err := h.DB.Model(&membership).Update("status", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Student status updated",
		"student": gin.H{
			"membershipId": membership.ID,
			"status":       req.Status,
			"user":         membership.User,
		},
	})
}

// DeleteStudent handles DELETE /api/org/:org/students?membershipId=...
func (h *StudentsHandler) DeleteStudent(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")
	membershipID := c.Query("membershipId")

	if membershipID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "membershipId query parameter is required"})
		return
	}

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var membership models.Membership
	if err := h.DB.Where("id = ? AND org_id = ? AND role = ?", membershipID, org.ID, models.RoleStudent).First(&membership).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student membership not found"})
		return
	}

	if err := h.DB.Delete(&membership).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove student"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Student removed successfully"})
}
