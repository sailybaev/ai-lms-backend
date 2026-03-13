package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/services"
	"gorm.io/gorm"
)

type BrandingHandler struct {
	DB *gorm.DB
}

func NewBrandingHandler(db *gorm.DB) *BrandingHandler {
	return &BrandingHandler{DB: db}
}

// GetBranding handles GET /api/org/:org/branding
func (h *BrandingHandler) GetBranding(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string))
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           org.ID,
		"name":         org.Name,
		"platformName": org.PlatformName,
		"logoUrl":      org.LogoURL,
	})
}

type UpdateBrandingRequest struct {
	PlatformName *string `json:"platformName"`
	LogoURL      *string `json:"logoUrl"`
}

// UpdateBranding handles PATCH /api/org/:org/branding
func (h *BrandingHandler) UpdateBranding(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	var req UpdateBrandingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.PlatformName != nil {
		updates["platform_name"] = *req.PlatformName
	}
	if req.LogoURL != nil {
		updates["logo_url"] = *req.LogoURL
	}

	if len(updates) > 0 {
		if err := h.DB.Model(org).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update branding"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           org.ID,
		"name":         org.Name,
		"platformName": org.PlatformName,
		"logoUrl":      org.LogoURL,
	})
}
