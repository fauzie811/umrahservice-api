package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

const notifiableUser = "user"

// Notifications mirrors NotificationController::index (paginate 15).
func (h *Handler) Notifications(c *gin.Context) {
	userID := h.principal(c).User.ID

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	const perPage = 15

	base := h.DB.Model(&models.Notification{}).
		Where("notifiable_type = ? AND notifiable_id = ?", notifiableUser, userID)

	var total int64
	base.Count(&total)

	var rows []models.Notification
	base.Order("created_at DESC").
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(&rows)

	data := make([]gin.H, 0, len(rows))
	for _, n := range rows {
		created := n.CreatedAt
		data = append(data, gin.H{
			"id":         n.ID,
			"data":       rawJSONString(n.Data),
			"read_at":    support.ISO(n.ReadAt),
			"created_at": support.ISO(&created),
		})
	}

	lastPage := int((total + perPage - 1) / perPage)
	if lastPage < 1 {
		lastPage = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"data":         data,
		"unread_count": h.unreadCount(userID),
		"meta": gin.H{
			"current_page": page,
			"last_page":    lastPage,
			"per_page":     perPage,
			"total":        total,
		},
	})
}

// UnreadCount mirrors NotificationController::unreadCount.
func (h *Handler) UnreadCount(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"count": h.unreadCount(h.principal(c).User.ID)})
}

// MarkAllAsRead mirrors NotificationController::markAllAsRead.
func (h *Handler) MarkAllAsRead(c *gin.Context) {
	userID := h.principal(c).User.ID
	now := time.Now()
	h.DB.Model(&models.Notification{}).
		Where("notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL", notifiableUser, userID).
		Update("read_at", now)

	c.JSON(http.StatusOK, gin.H{
		"message":      "All notifications marked as read.",
		"unread_count": 0,
	})
}

// MarkAsRead mirrors NotificationController::markAsRead.
func (h *Handler) MarkAsRead(c *gin.Context) {
	userID := h.principal(c).User.ID
	id := c.Param("id")

	var n models.Notification
	if err := h.DB.Where("id = ? AND notifiable_type = ? AND notifiable_id = ?", id, notifiableUser, userID).First(&n).Error; err != nil {
		notFound(c, "Notification not found.")
		return
	}
	if n.ReadAt == nil {
		now := time.Now()
		h.DB.Model(&models.Notification{}).Where("id = ?", n.ID).Update("read_at", now)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Notification marked as read.",
		"unread_count": h.unreadCount(userID),
	})
}

// DeleteNotification mirrors NotificationController::destroy.
func (h *Handler) DeleteNotification(c *gin.Context) {
	userID := h.principal(c).User.ID
	id := c.Param("id")

	var n models.Notification
	if err := h.DB.Where("id = ? AND notifiable_type = ? AND notifiable_id = ?", id, notifiableUser, userID).First(&n).Error; err != nil {
		notFound(c, "Notification not found.")
		return
	}
	h.DB.Delete(&models.Notification{}, "id = ?", n.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":      "Notification deleted.",
		"unread_count": h.unreadCount(userID),
	})
}

func (h *Handler) unreadCount(userID uint64) int64 {
	var count int64
	h.DB.Model(&models.Notification{}).
		Where("notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL", notifiableUser, userID).
		Count(&count)
	return count
}
