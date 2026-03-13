package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type PlatformUsersHandler struct {
	DB *gorm.DB
}

func NewPlatformUsersHandler(db *gorm.DB) *PlatformUsersHandler {
	return &PlatformUsersHandler{DB: db}
}

// ListUsers handles GET /api/admin/users?role=&status=&search=&orgId=
func (h *PlatformUsersHandler) ListUsers(c *gin.Context) {
	role := c.Query("role")
	status := c.Query("status")
	search := c.Query("search")
	orgID := c.Query("orgId")

	query := h.DB.Model(&models.User{}).Preload("Memberships").Preload("Memberships.Org")

	if search != "" {
		query = query.Where("users.name ILIKE ? OR users.email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if orgID != "" || role != "" || status != "" {
		query = query.Joins("JOIN memberships ON memberships.user_id = users.id")
		if orgID != "" {
			query = query.Where("memberships.org_id = ?", orgID)
		}
		if role != "" {
			query = query.Where("memberships.role = ?", role)
		}
		if status != "" {
			query = query.Where("memberships.status = ?", status)
		}
		query = query.Distinct("users.id")
	}

	var users []models.User
	if err := query.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	result := make([]gin.H, 0, len(users))
	for _, u := range users {
		result = append(result, gin.H{
			"id":           u.ID,
			"email":        u.Email,
			"name":         u.Name,
			"avatarUrl":    u.AvatarURL,
			"isSuperAdmin": u.IsSuperAdmin,
			"createdAt":    u.CreatedAt,
			"lastActiveAt": u.LastActiveAt,
			"memberships":  u.Memberships,
		})
	}

	c.JSON(http.StatusOK, result)
}

type CreatePlatformUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required"`
	OrgID string `json:"orgId" binding:"required"`
}

// CreateUser handles POST /api/admin/users
func (h *PlatformUsersHandler) CreateUser(c *gin.Context) {
	var req CreatePlatformUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check org exists
	var org models.Organization
	if err := h.DB.First(&org, "id = ?", req.OrgID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	var targetUser models.User
	userExists := h.DB.Where("email = ?", req.Email).First(&targetUser).Error == nil

	if !userExists {
		targetUser = models.User{
			Email: req.Email,
			Name:  req.Name,
		}
		if err := h.DB.Create(&targetUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	}

	// Check if already a member
	var existingMembership models.Membership
	if h.DB.Where("org_id = ? AND user_id = ?", req.OrgID, targetUser.ID).First(&existingMembership).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this organization"})
		return
	}

	membership := models.Membership{
		OrgID:  req.OrgID,
		UserID: targetUser.ID,
		Role:   models.Role(req.Role),
		Status: models.MembershipStatusActive,
	}
	if err := h.DB.Create(&membership).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create membership"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           targetUser.ID,
		"email":        targetUser.Email,
		"name":         targetUser.Name,
		"avatarUrl":    targetUser.AvatarURL,
		"isSuperAdmin": targetUser.IsSuperAdmin,
		"createdAt":    targetUser.CreatedAt,
		"membership":   membership,
	})
}

// GetUser handles GET /api/admin/users/:id
func (h *PlatformUsersHandler) GetUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if err := h.DB.Preload("Memberships").Preload("Memberships.Org").First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           user.ID,
		"email":        user.Email,
		"name":         user.Name,
		"avatarUrl":    user.AvatarURL,
		"isSuperAdmin": user.IsSuperAdmin,
		"createdAt":    user.CreatedAt,
		"lastActiveAt": user.LastActiveAt,
		"memberships":  user.Memberships,
	})
}

type UpdatePlatformUserRequest struct {
	Name      *string `json:"name"`
	Email     *string `json:"email"`
	AvatarURL *string `json:"avatarUrl"`
}

// UpdateUser handles PATCH /api/admin/users/:id
func (h *PlatformUsersHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if err := h.DB.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req UpdatePlatformUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&user).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}
	}

	h.DB.First(&user, "id = ?", user.ID)
	c.JSON(http.StatusOK, gin.H{
		"id":           user.ID,
		"email":        user.Email,
		"name":         user.Name,
		"avatarUrl":    user.AvatarURL,
		"isSuperAdmin": user.IsSuperAdmin,
		"createdAt":    user.CreatedAt,
	})
}

// DeleteUser handles DELETE /api/admin/users/:id
func (h *PlatformUsersHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	// Hash password for checking
	_ = bcrypt.DefaultCost

	var user models.User
	if err := h.DB.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Delete memberships first
	h.DB.Where("user_id = ?", id).Delete(&models.Membership{})

	if err := h.DB.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}
