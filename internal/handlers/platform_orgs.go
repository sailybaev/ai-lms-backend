package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"gorm.io/gorm"
)

type PlatformOrgsHandler struct {
	DB *gorm.DB
}

func NewPlatformOrgsHandler(db *gorm.DB) *PlatformOrgsHandler {
	return &PlatformOrgsHandler{DB: db}
}

// ListOrgs handles GET /api/admin/orgs
func (h *PlatformOrgsHandler) ListOrgs(c *gin.Context) {
	var orgs []models.Organization
	if err := h.DB.Find(&orgs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch organizations"})
		return
	}

	result := make([]gin.H, 0, len(orgs))
	for _, org := range orgs {
		var memberCount, courseCount int64
		h.DB.Model(&models.Membership{}).Where("org_id = ?", org.ID).Count(&memberCount)
		h.DB.Model(&models.Course{}).Where("org_id = ?", org.ID).Count(&courseCount)

		result = append(result, gin.H{
			"id":           org.ID,
			"slug":         org.Slug,
			"name":         org.Name,
			"platformName": org.PlatformName,
			"createdAt":    org.CreatedAt,
			"memberCount":  memberCount,
			"courseCount":  courseCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{"orgs": result})
}

type CreateOrgRequest struct {
	Slug string `json:"slug" binding:"required"`
	Name string `json:"name" binding:"required"`
}

// CreateOrg handles POST /api/admin/orgs
func (h *PlatformOrgsHandler) CreateOrg(c *gin.Context) {
	var req CreateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing models.Organization
	if h.DB.Where("slug = ?", req.Slug).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Organization with this slug already exists"})
		return
	}

	org := models.Organization{
		Slug: req.Slug,
		Name: req.Name,
	}

	if err := h.DB.Create(&org).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"org": org})
}
