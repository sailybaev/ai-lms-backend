package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"gorm.io/gorm"
)

type OrgResolveHandler struct {
	DB *gorm.DB
}

func NewOrgResolveHandler(db *gorm.DB) *OrgResolveHandler {
	return &OrgResolveHandler{DB: db}
}

// Resolve handles GET /api/org/resolve?host=...
// Tries exact domain match in org_domains, then subdomain fallback.
func (h *OrgResolveHandler) Resolve(c *gin.Context) {
	host := c.Query("host")
	if host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host query parameter is required"})
		return
	}

	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Try exact domain match first
	var domain models.OrganizationDomain
	if err := h.DB.Where("domain = ?", host).First(&domain).Error; err == nil {
		var org models.Organization
		if err := h.DB.Where("id = ?", domain.OrgID).First(&org).Error; err == nil {
			c.JSON(http.StatusOK, gin.H{"orgSlug": org.Slug})
			return
		}
	}

	// Try subdomain fallback: extract first part of host as slug
	// e.g. "acme.example.com" -> "acme"
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		potentialSlug := parts[0]
		var org models.Organization
		if err := h.DB.Where("slug = ?", potentialSlug).First(&org).Error; err == nil {
			c.JSON(http.StatusOK, gin.H{"orgSlug": org.Slug})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found for host"})
}
