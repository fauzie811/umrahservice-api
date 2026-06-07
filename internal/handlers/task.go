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
	if p.IsAdminOrOperator() || p.IsSuperAdmin() {
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

	q := h.visibleTasksQuery(p).Preload("Group.Customer").Preload("AssignedUser").Preload("ChecklistItems", orderBySort).Preload("ChecklistItems.CheckedByUser")
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

// TaskShow mirrors TaskController::show.
func (h *Handler) TaskShow(c *gin.Context) {
	p := h.principal(c)
	taskID := c.Param("groupTask")

	var task models.GroupTask
	err := h.visibleTasksQuery(p).
		Preload("Group.Customer").Preload("AssignedUser").Preload("ChecklistItems", orderBySort).Preload("ChecklistItems.CheckedByUser").
		Where("id = ?", taskID).First(&task).Error
	if err != nil {
		forbidden(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": h.transformTask(&task)})
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

	if h.hasIncompleteChecklist(task.ID) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "All checklist items must be checked before completing this task.",
		})
		return
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
		"completion_note":  note,
	}
	h.DB.Model(&models.GroupTask{}).Where("id = ?", task.ID).Updates(updates)

	h.DB.Preload("Group.Customer").Preload("AssignedUser").Preload("ChecklistItems", orderBySort).Preload("ChecklistItems.CheckedByUser").First(&task, task.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Task completed successfully",
		"data":    h.transformTask(&task),
	})
}

// TaskCheckItem mirrors TaskController::checkItem.
func (h *Handler) TaskCheckItem(c *gin.Context) {
	p := h.principal(c)

	task, item, ok := h.authorizeChecklistItem(c, p)
	if !ok {
		return
	}

	fh, ferr := c.FormFile("photo")
	hasPhoto := ferr == nil && fh != nil

	if item.PhotoRequired && !hasPhoto {
		validationError(c, map[string][]string{"photo": {"The photo field is required."}})
		return
	}

	photoPath := item.Photo
	if hasPhoto {
		if !isImage(fh.Filename) {
			validationError(c, map[string][]string{"photo": {"The photo field must be an image."}})
			return
		}
		content, contentType, ext, rerr := readUpload(fh)
		if rerr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not read upload."})
			return
		}
		if photoPath != nil && *photoPath != "" {
			_ = h.Storage.Delete(c.Request.Context(), *photoPath)
		}
		key, serr := h.Storage.Store(c.Request.Context(), "task_checklist_photos", ext, contentType, content)
		if serr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not store photo."})
			return
		}
		photoPath = &key
	}

	now := time.Now()
	h.DB.Model(item).Updates(map[string]interface{}{
		"photo":              photoPath,
		"done":               true,
		"done_at":            &now,
		"checked_by_user_id": p.User.ID,
	})

	h.respondWithTask(c, task)
}

// TaskUncheckItem mirrors TaskController::uncheckItem.
func (h *Handler) TaskUncheckItem(c *gin.Context) {
	p := h.principal(c)

	task, item, ok := h.authorizeChecklistItem(c, p)
	if !ok {
		return
	}

	if item.Photo != nil && *item.Photo != "" {
		_ = h.Storage.Delete(c.Request.Context(), *item.Photo)
	}

	h.DB.Model(item).Updates(map[string]interface{}{
		"photo":              nil,
		"done":               false,
		"done_at":            nil,
		"checked_by_user_id": nil,
	})

	h.respondWithTask(c, task)
}

// authorizeChecklistItem mirrors TaskController::authorizeChecklistItem. It
// returns the visible task and the resolved checklist item, or writes the
// matching error response and reports ok=false.
func (h *Handler) authorizeChecklistItem(c *gin.Context, p *auth.Principal) (*models.GroupTask, *models.GroupTaskChecklistItem, bool) {
	taskID := c.Param("groupTask")

	var task models.GroupTask
	if err := h.visibleTasksQuery(p).Where("id = ?", taskID).First(&task).Error; err != nil {
		forbidden(c)
		return nil, nil, false
	}

	var item models.GroupTaskChecklistItem
	if err := h.DB.Where("id = ?", c.Param("item")).First(&item).Error; err != nil {
		notFound(c, "Not Found")
		return nil, nil, false
	}

	if item.GroupTaskID != task.ID {
		notFound(c, "Not Found")
		return nil, nil, false
	}

	return &task, &item, true
}

func (h *Handler) respondWithTask(c *gin.Context, task *models.GroupTask) {
	h.DB.Preload("Group.Customer").Preload("AssignedUser").Preload("ChecklistItems", orderBySort).Preload("ChecklistItems.CheckedByUser").First(task, task.ID)
	c.JSON(http.StatusOK, gin.H{"data": h.transformTask(task)})
}

// hasIncompleteChecklist mirrors GroupTask::hasIncompleteChecklist.
func (h *Handler) hasIncompleteChecklist(taskID uint64) bool {
	var count int64
	h.DB.Model(&models.GroupTaskChecklistItem{}).
		Where("group_task_id = ? AND done = ?", taskID, false).
		Count(&count)
	return count > 0
}

// orderBySort orders preloaded checklist items by their sort column.
func orderBySort(db *gorm.DB) *gorm.DB {
	return db.Order("sort")
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
		"checklist":            h.transformChecklist(task.ChecklistItems),
		"completion_note":      task.CompletionNote,
		"assigned_user":        assigned,
		"assigned_role":        task.AssignedRole,
		"is_auto_generated":    task.IsAutoGenerated,
		"meta":                 rawJSONOrEmpty(task.Meta),
	}
}

func (h *Handler) transformChecklist(items []models.GroupTaskChecklistItem) []gin.H {
	out := make([]gin.H, 0, len(items))
	for i := range items {
		item := &items[i]
		var photoURL *string
		if item.Photo != nil && *item.Photo != "" {
			url := h.Storage.URL(*item.Photo)
			photoURL = &url
		}
		var checkedBy interface{}
		if item.CheckedByUser != nil {
			checkedBy = gin.H{
				"id":   item.CheckedByUser.ID,
				"name": item.CheckedByUser.Name,
			}
		}
		out = append(out, gin.H{
			"id":             item.ID,
			"label":          item.Label,
			"done":           item.Done,
			"photo_required": item.PhotoRequired,
			"photo_url":      photoURL,
			"done_at":        support.ISO8601(item.DoneAt),
			"checked_by":     checkedBy,
		})
	}
	return out
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
