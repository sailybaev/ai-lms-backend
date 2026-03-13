package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/services"
	"gorm.io/gorm"
)

type GroupsHandler struct {
	DB *gorm.DB
}

func NewGroupsHandler(db *gorm.DB) *GroupsHandler {
	return &GroupsHandler{DB: db}
}

// ListGroups handles GET /api/org/:org/admin/groups?courseId=...
func (h *GroupsHandler) ListGroups(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")
	courseID := c.Query("courseId")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	query := h.DB.Where("org_id = ?", org.ID).
		Preload("Members").
		Preload("Members.User").
		Preload("AssignedTeacher").
		Preload("Course")

	if courseID != "" {
		query = query.Where("course_id = ?", courseID)
	}

	var groups []models.Group
	if err := query.Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch groups"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

type CreateGroupRequest struct {
	Name              string   `json:"name" binding:"required"`
	Description       *string  `json:"description"`
	CourseID          *string  `json:"courseId"`
	AssignedTeacherID *string  `json:"assignedTeacherId"`
	MemberIDs         []string `json:"memberIds"`
}

// CreateGroup handles POST /api/org/:org/admin/groups
func (h *GroupsHandler) CreateGroup(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group := models.Group{
		OrgID:             org.ID,
		Name:              req.Name,
		Description:       req.Description,
		CourseID:          req.CourseID,
		AssignedTeacherID: req.AssignedTeacherID,
	}

	if err := h.DB.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	for _, memberID := range req.MemberIDs {
		gm := models.GroupMember{
			GroupID: group.ID,
			UserID:  memberID,
		}
		h.DB.Create(&gm)
	}

	h.DB.Preload("Members").Preload("Members.User").Preload("AssignedTeacher").Preload("Course").First(&group, "id = ?", group.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Group created successfully",
		"group":   group,
	})
}

type UpdateGroupRequest struct {
	GroupID           string   `json:"groupId" binding:"required"`
	Name              *string  `json:"name"`
	Description       *string  `json:"description"`
	CourseID          *string  `json:"courseId"`
	AssignedTeacherID *string  `json:"assignedTeacherId"`
	MemberIDs         []string `json:"memberIds"`
}

// UpdateGroup handles PUT /api/org/:org/admin/groups
func (h *GroupsHandler) UpdateGroup(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var group models.Group
	if err := h.DB.Where("id = ? AND org_id = ?", req.GroupID, org.ID).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.CourseID != nil {
		updates["course_id"] = *req.CourseID
	}
	if req.AssignedTeacherID != nil {
		updates["assigned_teacher_id"] = *req.AssignedTeacherID
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&group).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group"})
			return
		}
	}

	if req.MemberIDs != nil {
		h.DB.Where("group_id = ?", group.ID).Delete(&models.GroupMember{})
		for _, memberID := range req.MemberIDs {
			gm := models.GroupMember{
				GroupID: group.ID,
				UserID:  memberID,
			}
			h.DB.Create(&gm)
		}
	}

	h.DB.Preload("Members").Preload("Members.User").Preload("AssignedTeacher").Preload("Course").First(&group, "id = ?", group.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Group updated successfully",
		"group":   group,
	})
}

// DeleteGroup handles DELETE /api/org/:org/admin/groups?groupId=...
func (h *GroupsHandler) DeleteGroup(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")
	groupID := c.Query("groupId")

	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "groupId query parameter is required"})
		return
	}

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "admin")
	if err != nil {
		handleOrgError(c, err)
		return
	}

	var group models.Group
	if err := h.DB.Where("id = ? AND org_id = ?", groupID, org.ID).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	h.DB.Where("group_id = ?", groupID).Delete(&models.GroupMember{})

	if err := h.DB.Delete(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Group deleted successfully"})
}
