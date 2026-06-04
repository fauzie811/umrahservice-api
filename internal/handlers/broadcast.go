package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

// BroadcastAuth mirrors the /broadcasting/auth endpoint (Broadcast::auth) for
// token-authenticated clients, replicating routes/channels.php authorization.
func (h *Handler) BroadcastAuth(c *gin.Context) {
	p := h.principal(c)
	socketID := c.PostForm("socket_id")
	channelName := c.PostForm("channel_name")
	if channelName == "" {
		channelName = c.Query("channel_name")
	}
	if socketID == "" {
		socketID = c.Query("socket_id")
	}

	// Strip the private-/presence- prefix to resolve the logical channel.
	logical := channelName
	switch {
	case strings.HasPrefix(channelName, "private-"):
		logical = strings.TrimPrefix(channelName, "private-")
	case strings.HasPrefix(channelName, "presence-"):
		logical = strings.TrimPrefix(channelName, "presence-")
	}

	if !h.authorizeChannel(p, logical) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Forbidden"})
		return
	}

	// All authorized channels here are private (no channel_data).
	c.JSON(http.StatusOK, h.Broadcaster.AuthResponse(socketID, channelName, ""))
}

// authorizeChannel ports the callbacks in routes/channels.php.
func (h *Handler) authorizeChannel(p *auth.Principal, logical string) bool {
	// App.Models.User.{id}
	if strings.HasPrefix(logical, "App.Models.User.") {
		idStr := strings.TrimPrefix(logical, "App.Models.User.")
		id, err := strconv.ParseUint(idStr, 10, 64)
		return err == nil && p.User.ID == id
	}

	// messages.{type}.{id}
	if strings.HasPrefix(logical, "messages.") {
		rest := strings.TrimPrefix(logical, "messages.")
		idx := strings.LastIndex(rest, ".")
		if idx < 0 {
			return false
		}
		morphType := rest[:idx]
		id, err := strconv.ParseUint(rest[idx+1:], 10, 64)
		if err != nil {
			return false
		}
		return auth.CanViewMessageable(h.DB, p, morphType, id)
	}

	return false
}

// broadcastMessageSent mirrors Message model's MessageSent::dispatch on create.
func (h *Handler) broadcastMessageSent(msg *models.Message) {
	if h.Broadcaster == nil {
		return
	}
	channel := "private-messages." + msg.MessageableType + "." + strconv.FormatUint(msg.MessageableID, 10)

	var user interface{}
	if msg.User != nil {
		user = gin.H{"id": msg.User.ID, "name": msg.User.Name}
	}
	created := msg.CreatedAt
	payload := gin.H{
		"id":         msg.ID,
		"body":       msg.Body,
		"user":       user,
		"created_at": support.ISO8601(&created),
	}
	// Best-effort: ignore errors so a missing Reverb server doesn't fail the request.
	_ = h.Broadcaster.Trigger(context.Background(), channel, "MessageSent", payload)
}
