package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

// TaskMessages mirrors MessageController::taskIndex.
func (h *Handler) TaskMessages(c *gin.Context) {
	task, ok := h.findTask(c)
	if !ok {
		return
	}
	h.listMessages(c, models.MorphGroupTask, task.ID)
}

// TaskMessageStore mirrors MessageController::taskStore.
func (h *Handler) TaskMessageStore(c *gin.Context) {
	task, ok := h.findTask(c)
	if !ok {
		return
	}
	h.storeMessage(c, models.MorphGroupTask, task.ID)
}

// IncidentMessages mirrors MessageController::incidentIndex.
func (h *Handler) IncidentMessages(c *gin.Context) {
	inc, ok := h.findIncidentParam(c)
	if !ok {
		return
	}
	h.listMessages(c, models.MorphIncident, inc.ID)
}

// IncidentMessageStore mirrors MessageController::incidentStore.
func (h *Handler) IncidentMessageStore(c *gin.Context) {
	inc, ok := h.findIncidentParam(c)
	if !ok {
		return
	}
	h.storeMessage(c, models.MorphIncident, inc.ID)
}

// MessageUpdate mirrors MessageController::update.
func (h *Handler) MessageUpdate(c *gin.Context) {
	p := h.principal(c)
	var msg models.Message
	if err := h.DB.First(&msg, c.Param("message")).Error; err != nil {
		notFound(c, "")
		return
	}
	if !auth.CanUpdateMessage(p, &msg) {
		abort403(c)
		return
	}

	body := messageBody(c)
	if body == "" {
		validationError(c, map[string][]string{"body": {"The body field is required."}})
		return
	}
	h.DB.Model(&models.Message{}).Where("id = ?", msg.ID).Update("body", body)
	h.DB.Preload("User").First(&msg, msg.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Message updated successfully",
		"data":    transformMessage(&msg),
	})
}

// MessageDestroy mirrors MessageController::destroy.
func (h *Handler) MessageDestroy(c *gin.Context) {
	p := h.principal(c)
	var msg models.Message
	if err := h.DB.First(&msg, c.Param("message")).Error; err != nil {
		notFound(c, "")
		return
	}
	if !auth.CanDeleteMessage(p, &msg) {
		abort403(c)
		return
	}
	h.DB.Delete(&models.Message{}, msg.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Message deleted successfully"})
}

func (h *Handler) listMessages(c *gin.Context, morphType string, parentID uint64) {
	p := h.principal(c)
	if !auth.CanViewMessageable(h.DB, p, morphType, parentID) {
		abort403(c)
		return
	}

	var messages []models.Message
	h.DB.Preload("User").
		Where("messageable_type = ? AND messageable_id = ?", morphType, parentID).
		Order("created_at DESC").
		Find(&messages)

	data := make([]gin.H, 0, len(messages))
	for i := range messages {
		data = append(data, transformMessage(&messages[i]))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) storeMessage(c *gin.Context, morphType string, parentID uint64) {
	p := h.principal(c)
	if !auth.CanViewMessageable(h.DB, p, morphType, parentID) {
		abort403(c)
		return
	}

	body := messageBody(c)
	if body == "" {
		validationError(c, map[string][]string{"body": {"The body field is required."}})
		return
	}

	uid := p.User.ID
	msg := models.Message{
		MessageableType: morphType,
		MessageableID:   parentID,
		UserID:          &uid,
		Body:            body,
	}
	if err := h.DB.Create(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not post message."})
		return
	}
	h.DB.Preload("User").First(&msg, msg.ID)

	h.broadcastMessageSent(&msg)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Message posted successfully",
		"data":    transformMessage(&msg),
	})
}

func (h *Handler) findTask(c *gin.Context) (*models.GroupTask, bool) {
	var task models.GroupTask
	if err := h.DB.First(&task, c.Param("groupTask")).Error; err != nil {
		notFound(c, "")
		return nil, false
	}
	return &task, true
}

func (h *Handler) findIncidentParam(c *gin.Context) (*models.Incident, bool) {
	var inc models.Incident
	if err := h.DB.First(&inc, c.Param("incident")).Error; err != nil {
		notFound(c, "")
		return nil, false
	}
	return &inc, true
}

func messageBody(c *gin.Context) string {
	var m map[string]interface{}
	_ = bindJSONorForm(c, &m)
	if v, ok := stringField(m, "body"); ok {
		return v
	}
	return ""
}

func transformMessage(msg *models.Message) gin.H {
	var user gin.H
	if msg.User != nil {
		user = gin.H{"id": msg.User.ID, "name": msg.User.Name}
	}
	created := msg.CreatedAt
	updated := msg.UpdatedAt
	return gin.H{
		"id":         msg.ID,
		"body":       msg.Body,
		"user":       user,
		"created_at": support.ISO8601(&created),
		"updated_at": support.ISO8601(&updated),
	}
}
