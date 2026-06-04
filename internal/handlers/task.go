package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

// visibleTasksQuery mirrors TaskController::visibleTasksQuery.
func (h *Handler) visibleTasksQuery(p *auth.Principal) *gorm.DB {
	q := h.DB.Model(&models.GroupTask{})
	if p.HasRole(enums.RoleAdmin) || p.IsSuperAdmin() {
		return q
	}
	return q.Where(
		h.DB.Where("assigned_user_id = ?", p.User.ID).
			Or("assigned_role IN ?", nonEmpty(p.Roles)),
	)
}

func nonEmpty(s []string) []string {
	if len(s) == 0 {
		return []string{""}
	}
	return s
}

// TaskIndex mirrors TaskController::index.
func (h *Handler) TaskIndex(c *gin.Context) {
	p := h.principal(c)

	status := c.Query("status")
	if status != "" && status != "open" && status != "completed" && status != "obsolete" {
		validationError(c, map[string][]string{"status": {"The selected status is invalid."}})
		return
	}
	groupID := c.Query("group_id")
	limit := 100
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n >= 1 && n <= 100 {
			limit = n
		}
	}

	q := h.visibleTasksQuery(p).Preload("Group.Customer").Preload("AssignedUser")
	if status != "" {
		q = q.Where("status = ?", status)
	} else {
		q = q.Where("status = ?", enums.GroupTaskOpen)
	}
	if groupID != "" {
		q = q.Where("group_id = ?", groupID)
	}

	var tasks []models.GroupTask
	q.Order("scheduled_at").Limit(limit).Find(&tasks)

	data := make([]gin.H, 0, len(tasks))
	for i := range tasks {
		data = append(data, h.transformTask(&tasks[i]))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// TaskComplete mirrors TaskController::complete.
func (h *Handler) TaskComplete(c *gin.Context) {
	p := h.principal(c)
	taskID := c.Param("groupTask")

	var task models.GroupTask
	if err := h.visibleTasksQuery(p).Where("id = ?", taskID).First(&task).Error; err != nil {
		forbidden(c)
		return
	}

	if task.IsScheduledInFuture() {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "This task cannot be completed before its scheduled time.",
		})
		return
	}

	checklist := h.decodeChecklist(task.Checklist)
	inputs, hasChecklist := parseChecklistBools(c)
	if hasChecklist && len(checklist) > 0 {
		for i := range checklist {
			if v, ok := inputs[i]; ok {
				checklist[i]["done"] = v
			} else {
				checklist[i]["done"] = truthy(checklist[i]["done"])
			}
		}
	}
	for _, item := range checklist {
		if !truthy(item["done"]) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"message": "All checklist items must be checked before completing this task.",
			})
			return
		}
	}

	completionPhoto := task.CompletionPhoto
	if fh, err := c.FormFile("photo"); err == nil && fh != nil {
		if completionPhoto != nil && *completionPhoto != "" {
			_ = h.Storage.Delete(c.Request.Context(), *completionPhoto)
		}
		content, contentType, ext, rerr := readUpload(fh)
		if rerr == nil {
			if key, serr := h.Storage.Store(c.Request.Context(), "task_completions", ext, contentType, content); serr == nil {
				completionPhoto = &key
			}
		}
	}

	completedAt := task.CompletedAt
	if completedAt == nil {
		now := time.Now()
		completedAt = &now
	}
	note := task.CompletionNote
	if v, ok := c.GetPostForm("completion_note"); ok && strings.TrimSpace(v) != "" {
		note = &v
	}

	updates := map[string]interface{}{
		"status":           enums.GroupTaskCompleted,
		"completed_at":     completedAt,
		"completion_photo": completionPhoto,
		"checklist":        jsonArray(checklist),
		"completion_note":  note,
	}
	h.DB.Model(&models.GroupTask{}).Where("id = ?", task.ID).Updates(updates)

	h.DB.Preload("Group.Customer").Preload("AssignedUser").First(&task, task.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Task completed successfully",
		"data":    h.transformTask(&task),
	})
}

// TaskUpdateChecklist mirrors TaskController::updateChecklist.
func (h *Handler) TaskUpdateChecklist(c *gin.Context) {
	p := h.principal(c)
	taskID := c.Param("groupTask")

	inputs, present := parseChecklistBools(c)
	if !present {
		validationError(c, map[string][]string{"checklist": {"The checklist field is required."}})
		return
	}

	var task models.GroupTask
	if err := h.visibleTasksQuery(p).Where("id = ?", taskID).First(&task).Error; err != nil {
		forbidden(c)
		return
	}

	checklist := h.decodeChecklist(task.Checklist)
	for i := range checklist {
		if v, ok := inputs[i]; ok {
			checklist[i]["done"] = v
		} else {
			checklist[i]["done"] = truthy(checklist[i]["done"])
		}
	}

	h.DB.Model(&models.GroupTask{}).Where("id = ?", task.ID).Update("checklist", jsonArray(checklist))
	h.DB.Preload("Group.Customer").Preload("AssignedUser").First(&task, task.ID)
	c.JSON(http.StatusOK, gin.H{"data": h.transformTask(&task)})
}

func (h *Handler) decodeChecklist(raw interface{ MarshalJSON() ([]byte, error) }) []map[string]interface{} {
	out := []map[string]interface{}{}
	if b, err := raw.MarshalJSON(); err == nil {
		_ = unmarshalChecklist(b, &out)
	}
	return out
}

func (h *Handler) transformTask(task *models.GroupTask) gin.H {
	var assigned gin.H
	if task.AssignedUser != nil {
		assigned = gin.H{
			"id":    task.AssignedUser.ID,
			"name":  task.AssignedUser.Name,
			"phone": task.AssignedUser.Phone,
		}
	}

	var eventValue, eventLabel, teamValue, teamLabel, statusValue, statusLabel interface{}
	if task.EventType != nil {
		eventValue = *task.EventType
		eventLabel = enums.GroupTaskEventLabel(*task.EventType)
	}
	if task.TeamType != nil {
		teamValue = *task.TeamType
		teamLabel = enums.GroupTaskTeamLabel(*task.TeamType)
	}
	if task.Status != nil {
		statusValue = *task.Status
		statusLabel = enums.GroupTaskStatusLabel(*task.Status)
	}

	var photoURL *string
	if task.CompletionPhoto != nil && *task.CompletionPhoto != "" {
		url := h.Storage.URL(*task.CompletionPhoto)
		photoURL = &url
	}

	return gin.H{
		"id":                   task.ID,
		"group_id":             task.GroupID,
		"group_name":           groupName(task.Group),
		"customer_name":        groupCustomerName(task.Group),
		"title":                task.Title,
		"description":          task.Description,
		"event_type":           eventValue,
		"event_label":          eventLabel,
		"team_type":            teamValue,
		"team_label":           teamLabel,
		"status":               statusValue,
		"status_label":         statusLabel,
		"scheduled_at":         support.ISO8601(task.ScheduledAt),
		"completed_at":         support.ISO8601(task.CompletedAt),
		"completion_photo_url": photoURL,
		"checklist":            h.decodeChecklist(task.Checklist),
		"completion_note":      task.CompletionNote,
		"assigned_user":        assigned,
		"assigned_role":        task.AssignedRole,
		"is_auto_generated":    task.IsAutoGenerated,
		"meta":                 rawJSONOrEmpty(task.Meta),
	}
}

func groupName(g *models.Group) interface{} {
	if g == nil {
		return nil
	}
	return g.Name
}

func groupCustomerName(g *models.Group) interface{} {
	if g == nil {
		return nil
	}
	return g.CustomerName()
}
