package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/services"
	"gorm.io/gorm"
)

type AIChatHandler struct {
	DB        *gorm.DB
	AIBaseURL string
}

func NewAIChatHandler(db *gorm.DB, aiBaseURL string) *AIChatHandler {
	return &AIChatHandler{DB: db, AIBaseURL: aiBaseURL}
}

// CreateSession handles POST /api/org/:org/ai/sessions
func (h *AIChatHandler) CreateSession(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, user, _, err := services.GetOrgAndMembership(h.DB, orgSlug, userEmail.(string))
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

	var req struct {
		CourseID *string `json:"courseId"`
		LessonID *string `json:"lessonId"`
		Title    *string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session := models.AIChatSession{
		OrgID:    org.ID,
		UserID:   user.ID,
		CourseID: req.CourseID,
		LessonID: req.LessonID,
		Title:    req.Title,
	}

	if err := h.DB.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"session": session})
}

// ListSessions handles GET /api/org/:org/ai/sessions
func (h *AIChatHandler) ListSessions(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, user, _, err := services.GetOrgAndMembership(h.DB, orgSlug, userEmail.(string))
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

	var sessions []models.AIChatSession
	if err := h.DB.Where("org_id = ? AND user_id = ?", org.ID, user.ID).
		Order("created_at DESC").
		Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// GetSession handles GET /api/org/:org/ai/sessions/:sessionId
func (h *AIChatHandler) GetSession(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")
	sessionID := c.Param("sessionId")

	org, user, _, err := services.GetOrgAndMembership(h.DB, orgSlug, userEmail.(string))
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

	var session models.AIChatSession
	if err := h.DB.Where("id = ? AND org_id = ? AND user_id = ?", sessionID, org.ID, user.ID).
		Preload("Messages").
		First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"session": session})
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OllamaResponse struct {
	Message OllamaMessage `json:"message"`
}

// SendMessage handles POST /api/org/:org/ai/sessions/:sessionId/messages
func (h *AIChatHandler) SendMessage(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")
	sessionID := c.Param("sessionId")

	org, user, _, err := services.GetOrgAndMembership(h.DB, orgSlug, userEmail.(string))
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

	// Verify session belongs to user and org
	var session models.AIChatSession
	if err := h.DB.Where("id = ? AND org_id = ? AND user_id = ?", sessionID, org.ID, user.ID).
		Preload("Messages").
		First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save user message
	userMsg := models.AIMessage{
		SessionID: session.ID,
		Sender:    models.SenderUser,
		Content:   req.Content,
		CreatedAt: time.Now(),
	}
	if err := h.DB.Create(&userMsg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save message"})
		return
	}

	// Build message history for AI
	ollamaMessages := make([]OllamaMessage, 0, len(session.Messages)+1)
	for _, m := range session.Messages {
		role := "user"
		if m.Sender == models.SenderAssistant {
			role = "assistant"
		} else if m.Sender == models.SenderSystem {
			role = "system"
		}
		ollamaMessages = append(ollamaMessages, OllamaMessage{
			Role:    role,
			Content: m.Content,
		})
	}
	ollamaMessages = append(ollamaMessages, OllamaMessage{
		Role:    "user",
		Content: req.Content,
	})

	// Call Ollama AI
	ollamaReq := OllamaRequest{
		Model:    "llama3",
		Messages: ollamaMessages,
		Stream:   false,
	}

	reqBody, _ := json.Marshal(ollamaReq)
	aiURL := fmt.Sprintf("%s/api/chat", h.AIBaseURL)
	httpResp, err := http.Post(aiURL, "application/json", bytes.NewReader(reqBody))

	assistantContent := "I'm unable to respond right now. Please try again later."
	if err == nil {
		defer httpResp.Body.Close()
		respBody, readErr := io.ReadAll(httpResp.Body)
		if readErr == nil {
			var ollamaResp OllamaResponse
			if jsonErr := json.Unmarshal(respBody, &ollamaResp); jsonErr == nil && ollamaResp.Message.Content != "" {
				assistantContent = ollamaResp.Message.Content
			}
		}
	}

	// Save assistant response
	assistantMsg := models.AIMessage{
		SessionID: session.ID,
		Sender:    models.SenderAssistant,
		Content:   assistantContent,
		CreatedAt: time.Now(),
	}
	if err := h.DB.Create(&assistantMsg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save assistant response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": assistantMsg})
}
