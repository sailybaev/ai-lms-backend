package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/services"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ProfileHandler struct {
	DB        *gorm.DB
	UploadDir string
}

func NewProfileHandler(db *gorm.DB, uploadDir string) *ProfileHandler {
	return &ProfileHandler{DB: db, UploadDir: uploadDir}
}

// GetProfile handles GET /api/org/:org/profile
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, user, membership, err := services.GetOrgAndMembership(h.DB, orgSlug, userEmail.(string))
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this organization"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           user.ID,
		"name":         user.Name,
		"email":        user.Email,
		"avatarUrl":    user.AvatarURL,
		"createdAt":    user.CreatedAt,
		"lastActiveAt": user.LastActiveAt,
		"membership": gin.H{
			"id":     membership.ID,
			"role":   membership.Role,
			"status": membership.Status,
		},
		"org": gin.H{
			"id":   org.ID,
			"slug": org.Slug,
			"name": org.Name,
		},
	})
}

type UpdateProfileRequest struct {
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatarUrl"`
}

// UpdateProfile handles PATCH /api/org/:org/profile
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	_, user, _, err := services.GetOrgAndMembership(h.DB, orgSlug, userEmail.(string))
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this organization"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		if *req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Name cannot be empty"})
			return
		}
		updates["name"] = *req.Name
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}

	if len(updates) > 0 {
		if err := h.DB.Model(user).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}
	}

	h.DB.First(user, "id = ?", user.ID)
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":        user.ID,
			"name":      user.Name,
			"email":     user.Email,
			"avatarUrl": user.AvatarURL,
		},
		"success": true,
	})
}

// DeleteProfile handles DELETE /api/org/:org/profile — suspends membership
func (h *ProfileHandler) DeleteProfile(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	_, _, membership, err := services.GetOrgAndMembership(h.DB, orgSlug, userEmail.(string))
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this organization"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if err := h.DB.Model(membership).Update("status", models.MembershipStatusSuspended).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to suspend membership"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Membership suspended successfully",
	})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=6"`
}

// ChangePassword handles PATCH /api/org/:org/profile/password
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	_, user, _, err := services.GetOrgAndMembership(h.DB, orgSlug, userEmail.(string))
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this organization"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if user.PasswordHash == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No password set for this account"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	if err := h.DB.Model(user).Update("password_hash", string(hash)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}
