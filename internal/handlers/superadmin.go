package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type SuperadminHandler struct {
	DB *gorm.DB
}

func NewSuperadminHandler(db *gorm.DB) *SuperadminHandler {
	return &SuperadminHandler{DB: db}
}

// GetStats handles GET /api/superadmin/stats
func (h *SuperadminHandler) GetStats(c *gin.Context) {
	var totalOrgs, totalUsers, totalSuperAdmins, activeOrgs int64

	h.DB.Model(&models.Organization{}).Count(&totalOrgs)
	h.DB.Model(&models.User{}).Count(&totalUsers)
	h.DB.Model(&models.User{}).Where("is_super_admin = true").Count(&totalSuperAdmins)

	// Active orgs: orgs that have at least one active membership
	h.DB.Model(&models.Organization{}).
		Joins("JOIN memberships ON memberships.org_id = organizations.id").
		Where("memberships.status = 'active'").
		Distinct("organizations.id").
		Count(&activeOrgs)

	c.JSON(http.StatusOK, gin.H{
		"totalOrganizations": totalOrgs,
		"totalUsers":         totalUsers,
		"totalSuperAdmins":   totalSuperAdmins,
		"activeOrganizations": activeOrgs,
	})
}

// ListUsers handles GET /api/superadmin/users
func (h *SuperadminHandler) ListUsers(c *gin.Context) {
	type UserRow struct {
		models.User
		MembershipCount int64
	}

	var users []models.User
	if err := h.DB.Preload("Memberships").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	result := make([]gin.H, 0, len(users))
	for _, u := range users {
		result = append(result, gin.H{
			"id":              u.ID,
			"email":           u.Email,
			"name":            u.Name,
			"isSuperAdmin":    u.IsSuperAdmin,
			"createdAt":       u.CreatedAt,
			"membershipCount": len(u.Memberships),
		})
	}

	c.JSON(http.StatusOK, gin.H{"users": result})
}

// ListAdmins handles GET /api/superadmin/admins
func (h *SuperadminHandler) ListAdmins(c *gin.Context) {
	var admins []models.User
	if err := h.DB.Where("is_super_admin = true").Find(&admins).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch admins"})
		return
	}

	result := make([]gin.H, 0, len(admins))
	for _, a := range admins {
		result = append(result, gin.H{
			"id":           a.ID,
			"email":        a.Email,
			"name":         a.Name,
			"isSuperAdmin": a.IsSuperAdmin,
			"createdAt":    a.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"admins": result})
}

type CreateAdminRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

// CreateAdmin handles POST /api/superadmin/admins
func (h *SuperadminHandler) CreateAdmin(c *gin.Context) {
	var req CreateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing models.User
	if h.DB.Where("email = ?", req.Email).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	hashStr := string(hash)

	admin := models.User{
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: &hashStr,
		IsSuperAdmin: true,
	}

	if err := h.DB.Create(&admin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create admin"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"admin": gin.H{
			"id":           admin.ID,
			"email":        admin.Email,
			"name":         admin.Name,
			"isSuperAdmin": admin.IsSuperAdmin,
			"createdAt":    admin.CreatedAt,
		},
	})
}

type PatchAdminRequest struct {
	IsSuperAdmin bool `json:"isSuperAdmin"`
}

// PatchAdmin handles PATCH /api/superadmin/admins/:id
func (h *SuperadminHandler) PatchAdmin(c *gin.Context) {
	id := c.Param("id")

	var req PatchAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := h.DB.Model(&user).Update("is_super_admin", req.IsSuperAdmin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": gin.H{
		"id":           user.ID,
		"email":        user.Email,
		"name":         user.Name,
		"isSuperAdmin": req.IsSuperAdmin,
	}})
}

// GetOrganization handles GET /api/superadmin/organizations/:id
func (h *SuperadminHandler) GetOrganization(c *gin.Context) {
	id := c.Param("id")

	var org models.Organization
	if err := h.DB.Preload("Domains").First(&org, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	var memberCount, courseCount int64
	h.DB.Model(&models.Membership{}).Where("org_id = ?", org.ID).Count(&memberCount)
	h.DB.Model(&models.Course{}).Where("org_id = ?", org.ID).Count(&courseCount)

	c.JSON(http.StatusOK, gin.H{
		"organization": gin.H{
			"id":           org.ID,
			"slug":         org.Slug,
			"name":         org.Name,
			"platformName": org.PlatformName,
			"logoUrl":      org.LogoURL,
			"createdAt":    org.CreatedAt,
			"memberCount":  memberCount,
			"courseCount":  courseCount,
			"domains":      org.Domains,
		},
	})
}

// GetOrgMembers handles GET /api/superadmin/organizations/:id/members
func (h *SuperadminHandler) GetOrgMembers(c *gin.Context) {
	orgID := c.Param("id")

	var memberships []models.Membership
	if err := h.DB.Where("org_id = ?", orgID).Preload("User").Find(&memberships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch members"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"members": memberships})
}

type CreateOrgMemberRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

// CreateOrgMember handles POST /api/superadmin/organizations/:id/members
func (h *SuperadminHandler) CreateOrgMember(c *gin.Context) {
	orgID := c.Param("id")

	var org models.Organization
	if err := h.DB.First(&org, "id = ?", orgID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	var req CreateOrgMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var targetUser models.User
	userExists := h.DB.Where("email = ?", req.Email).First(&targetUser).Error == nil

	if !userExists {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		hashStr := string(hash)
		targetUser = models.User{
			Email:        req.Email,
			Name:         req.Name,
			PasswordHash: &hashStr,
		}
		if err := h.DB.Create(&targetUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	}

	var existing models.Membership
	if h.DB.Where("org_id = ? AND user_id = ?", orgID, targetUser.ID).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a member"})
		return
	}

	membership := models.Membership{
		OrgID:  orgID,
		UserID: targetUser.ID,
		Role:   models.Role(req.Role),
		Status: models.MembershipStatusActive,
	}
	if err := h.DB.Create(&membership).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create membership"})
		return
	}

	membership.User = &targetUser
	c.JSON(http.StatusCreated, gin.H{"membership": membership})
}

type PatchOrgMemberRequest struct {
	Role string `json:"role" binding:"required"`
}

// PatchOrgMember handles PATCH /api/superadmin/organizations/:id/members/:membershipId
func (h *SuperadminHandler) PatchOrgMember(c *gin.Context) {
	orgID := c.Param("id")
	membershipID := c.Param("membershipId")

	var req PatchOrgMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var membership models.Membership
	if err := h.DB.Where("id = ? AND org_id = ?", membershipID, orgID).Preload("User").First(&membership).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Membership not found"})
		return
	}

	if err := h.DB.Model(&membership).Update("role", req.Role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update membership"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"membership": membership})
}

// DeleteOrgMember handles DELETE /api/superadmin/organizations/:id/members/:membershipId
func (h *SuperadminHandler) DeleteOrgMember(c *gin.Context) {
	orgID := c.Param("id")
	membershipID := c.Param("membershipId")

	var membership models.Membership
	if err := h.DB.Where("id = ? AND org_id = ?", membershipID, orgID).First(&membership).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Membership not found"})
		return
	}

	if err := h.DB.Delete(&membership).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete membership"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
