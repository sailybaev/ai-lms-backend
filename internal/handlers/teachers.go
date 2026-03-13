package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/services"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type TeachersHandler struct {
	DB *gorm.DB
}

func NewTeachersHandler(db *gorm.DB) *TeachersHandler {
	return &TeachersHandler{DB: db}
}

// ListTeachers handles GET /api/org/:org/teachers
func (h *TeachersHandler) ListTeachers(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var memberships []models.Membership
	if err := h.DB.Where("org_id = ? AND role = ?", org.ID, models.RoleTeacher).
		Preload("User").
		Find(&memberships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch teachers"})
		return
	}

	teachers := make([]gin.H, 0, len(memberships))
	for _, m := range memberships {
		if m.User == nil {
			continue
		}
		teachers = append(teachers, gin.H{
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

	c.JSON(http.StatusOK, gin.H{"teachers": teachers})
}

type CreateTeacherRequest struct {
	Name     string  `json:"name" binding:"required"`
	Email    string  `json:"email" binding:"required,email"`
	Password *string `json:"password"`
}

// CreateTeacher handles POST /api/org/:org/teachers
func (h *TeachersHandler) CreateTeacher(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req CreateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

	var existingMembership models.Membership
	if h.DB.Where("org_id = ? AND user_id = ?", org.ID, targetUser.ID).First(&existingMembership).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this organization"})
		return
	}

	membership := models.Membership{
		OrgID:  org.ID,
		UserID: targetUser.ID,
		Role:   models.RoleTeacher,
		Status: models.MembershipStatusActive,
	}
	if err := h.DB.Create(&membership).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create membership"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Teacher created successfully",
		"teacher": gin.H{
			"id":           targetUser.ID,
			"name":         targetUser.Name,
			"email":        targetUser.Email,
			"avatarUrl":    targetUser.AvatarURL,
			"status":       membership.Status,
			"membershipId": membership.ID,
			"joinedAt":     membership.CreatedAt,
		},
	})
}

type UpdateTeacherRequest struct {
	UserID      string  `json:"userId" binding:"required"`
	Name        string  `json:"name" binding:"required"`
	Email       string  `json:"email" binding:"required,email"`
	AvatarURL   *string `json:"avatarUrl"`
	NewPassword *string `json:"newPassword"`
}

// UpdateTeacher handles PUT /api/org/:org/teachers
func (h *TeachersHandler) UpdateTeacher(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req UpdateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var membership models.Membership
	if err := h.DB.Where("org_id = ? AND user_id = ? AND role = ?", org.ID, req.UserID, models.RoleTeacher).First(&membership).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found in organization"})
		return
	}

	var teacher models.User
	if err := h.DB.First(&teacher, "id = ?", req.UserID).Error; err != nil {
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

	if err := h.DB.Model(&teacher).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update teacher"})
		return
	}

	h.DB.First(&teacher, "id = ?", teacher.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Teacher updated successfully",
		"teacher": gin.H{
			"id":           teacher.ID,
			"name":         teacher.Name,
			"email":        teacher.Email,
			"avatarUrl":    teacher.AvatarURL,
			"status":       membership.Status,
			"membershipId": membership.ID,
		},
	})
}

type PatchTeacherStatusRequest struct {
	MembershipID string `json:"membershipId" binding:"required"`
	Status       string `json:"status" binding:"required"`
}

// PatchTeacherStatus handles PATCH /api/org/:org/teachers
func (h *TeachersHandler) PatchTeacherStatus(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req PatchTeacherStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var membership models.Membership
	if err := h.DB.Where("id = ? AND org_id = ? AND role = ?", req.MembershipID, org.ID, models.RoleTeacher).
		Preload("User").First(&membership).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Teacher membership not found"})
		return
	}

	if err := h.DB.Model(&membership).Update("status", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Teacher status updated",
		"teacher": gin.H{
			"membershipId": membership.ID,
			"status":       req.Status,
			"user":         membership.User,
		},
	})
}

// DeleteTeacher handles DELETE /api/org/:org/teachers?membershipId=...
func (h *TeachersHandler) DeleteTeacher(c *gin.Context) {
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
	if err := h.DB.Where("id = ? AND org_id = ? AND role = ?", membershipID, org.ID, models.RoleTeacher).First(&membership).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Teacher membership not found"})
		return
	}

	if err := h.DB.Delete(&membership).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove teacher"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Teacher removed successfully"})
}
