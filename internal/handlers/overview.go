package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
)

// Overview mirrors OverviewController::index. Admins and operators see
// unscoped aggregate monitoring counts for tasks and active incidents.
func (h *Handler) Overview(c *gin.Context) {
	p := h.principal(c)
	if !p.IsAdminOrOperator() {
		forbidden(c)
		return
	}

	now := time.Now()

	var activeTasks, overdueTasks int64
	h.DB.Model(&models.GroupTask{}).
		Where("status = ?", enums.GroupTaskOpen).
		Count(&activeTasks)
	h.DB.Model(&models.GroupTask{}).
		Where("status = ?", enums.GroupTaskOpen).
		Where("scheduled_at < ?", now).
		Count(&overdueTasks)

	activeIncidentStatuses := []string{"open", "in_progress"}

	type aggregate struct {
		Key       string
		Aggregate int64
	}

	bySeverity := map[string]int64{}
	var sevRows []aggregate
	h.DB.Model(&models.Incident{}).
		Select("severity AS `key`, count(*) AS aggregate").
		Where("status IN ?", activeIncidentStatuses).
		Group("severity").
		Scan(&sevRows)
	for _, r := range sevRows {
		bySeverity[r.Key] = r.Aggregate
	}

	byStatus := map[string]int64{}
	var statusRows []aggregate
	h.DB.Model(&models.Incident{}).
		Select("status AS `key`, count(*) AS aggregate").
		Where("status IN ?", activeIncidentStatuses).
		Group("status").
		Scan(&statusRows)
	var activeIncidents int64
	for _, r := range statusRows {
		byStatus[r.Key] = r.Aggregate
		activeIncidents += r.Aggregate
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"tasks": gin.H{
				"active":  activeTasks,
				"overdue": overdueTasks,
			},
			"incidents": gin.H{
				"active": activeIncidents,
				"by_severity": gin.H{
					"low":      bySeverity["low"],
					"medium":   bySeverity["medium"],
					"high":     bySeverity["high"],
					"critical": bySeverity["critical"],
				},
				"by_status": gin.H{
					"open":        byStatus["open"],
					"in_progress": byStatus["in_progress"],
				},
			},
		},
	})
}
