package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/handlers"
)

// New builds the Gin engine mirroring routes/api.php (mounted under /api).
func New(db *gorm.DB, h *handlers.Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := r.Group("/api")

	api.POST("/login", h.Login)
	api.POST("/logout", auth.Middleware(db), h.Logout)

	authed := api.Group("")
	authed.Use(auth.Middleware(db))
	{
		authed.POST("/broadcasting/auth", h.BroadcastAuth)

		authed.GET("/user", h.CurrentUser)
		authed.POST("/user/profile", h.ProfileUpdate)

		authed.GET("/wallet/balance", h.Balance)
		authed.GET("/wallet/transactions", h.Transactions)
		authed.GET("/wallet/recipients", h.Recipients)
		authed.GET("/wallet/categories", h.Categories)
		authed.POST("/wallet/transactions", h.WalletStore)

		authed.GET("/groups", h.GroupIndex)
		authed.GET("/groups/:id", h.GroupShow)
		authed.POST("/groups/:id/files", h.GroupStoreFile)
		authed.POST("/groups/:id/files/:fileId", h.GroupUpdateFile)
		authed.DELETE("/groups/:id/files/:fileId", h.GroupDeleteFile)

		authed.GET("/baggage-item-types", h.BaggageItemTypes)
		authed.GET("/groups/:id/baggage", h.BaggageIndex)
		authed.POST("/groups/:id/baggage", h.BaggageStore)
		authed.DELETE("/baggage/:baggageCount", h.BaggageDestroy)

		authed.GET("/luggage-tag/*code", h.LuggageTag)

		authed.GET("/schedules", h.Schedule)

		authed.GET("/tasks", h.TaskIndex)
		authed.POST("/tasks/:groupTask/complete", h.TaskComplete)
		authed.PATCH("/tasks/:groupTask/checklist", h.TaskUpdateChecklist)
		authed.GET("/tasks/:groupTask/messages", h.TaskMessages)
		authed.POST("/tasks/:groupTask/messages", h.TaskMessageStore)

		authed.GET("/incidents", h.IncidentIndex)
		authed.POST("/incidents", h.IncidentStore)
		authed.GET("/incidents/:incident", h.IncidentShow)
		authed.PATCH("/incidents/:incident", h.IncidentUpdate)
		authed.DELETE("/incidents/:incident", h.IncidentDestroy)
		authed.GET("/incidents/:incident/messages", h.IncidentMessages)
		authed.POST("/incidents/:incident/messages", h.IncidentMessageStore)

		authed.PATCH("/messages/:message", h.MessageUpdate)
		authed.DELETE("/messages/:message", h.MessageDestroy)

		authed.GET("/notifications", h.Notifications)
		authed.GET("/notifications/unread-count", h.UnreadCount)
		authed.POST("/notifications/read-all", h.MarkAllAsRead)
		authed.POST("/notifications/:id/read", h.MarkAsRead)
		authed.DELETE("/notifications/:id", h.DeleteNotification)
	}

	return r
}
