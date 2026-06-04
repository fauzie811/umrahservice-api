package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

var incidentCategories = map[string]bool{
	"general": true, "hotel": true, "flight": true, "transport": true,
	"pilgrim": true, "finance": true, "medical": true, "document": true,
}
var incidentSeverities = map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
var incidentStatuses = map[string]bool{"open": true, "in_progress": true, "resolved": true, "closed": true}

// IncidentIndex mirrors IncidentController::index.
func (h *Handler) IncidentIndex(c *gin.Context) {
	p := h.principal(c)
	if !p.CanViewAnyIncident() {
		abort403(c)
		return
	}

	groupID := c.Query("group_id")
	status := c.Query("status")
	severity := c.Query("severity")
	category := c.Query("category")

	errs := map[string][]string{}
	if status != "" && !incidentStatuses[status] {
		errs["status"] = []string{"The selected status is invalid."}
	}
	if severity != "" && !incidentSeverities[severity] {
		errs["severity"] = []string{"The selected severity is invalid."}
	}
	if category != "" && !incidentCategories[category] {
		errs["category"] = []string{"The selected category is invalid."}
	}
	if len(errs) > 0 {
		validationError(c, errs)
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n >= 1 && n <= 100 {
			limit = n
		}
	}

	q := h.DB.Model(&models.Incident{}).
		Preload("Group.Customer").Preload("ReportedBy").Preload("AssignedTo").Preload("GroupTask")
	if groupID != "" {
		q = q.Where("group_id = ?", groupID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}

	var incidents []models.Incident
	q.Order("occurred_at DESC").Limit(limit).Find(&incidents)

	data := make([]gin.H, 0, len(incidents))
	for i := range incidents {
		if auth.CanViewIncident(h.DB, p, &incidents[i]) {
			data = append(data, h.transformIncident(&incidents[i]))
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

type incidentStoreRequest struct {
	GroupID     *uint64 `json:"group_id" form:"group_id"`
	GroupTaskID *uint64 `json:"group_task_id" form:"group_task_id"`
	Title       string  `json:"title" form:"title"`
	Description string  `json:"description" form:"description"`
	Category    string  `json:"category" form:"category"`
	Severity    string  `json:"severity" form:"severity"`
	OccurredAt  string  `json:"occurred_at" form:"occurred_at"`
}

// IncidentStore mirrors IncidentController::store.
func (h *Handler) IncidentStore(c *gin.Context) {
	p := h.principal(c)
	if !p.CanCreateIncident() {
		abort403(c)
		return
	}

	var req incidentStoreRequest
	_ = c.ShouldBind(&req)

	errs := map[string][]string{}
	if req.GroupID == nil {
		errs["group_id"] = []string{"The group id field is required."}
	}
	if req.Title == "" {
		errs["title"] = []string{"The title field is required."}
	}
	if req.Description == "" {
		errs["description"] = []string{"The description field is required."}
	}
	if !incidentCategories[req.Category] {
		errs["category"] = []string{"The selected category is invalid."}
	}
	if !incidentSeverities[req.Severity] {
		errs["severity"] = []string{"The selected severity is invalid."}
	}
	if len(errs) > 0 {
		validationError(c, errs)
		return
	}

	inc := models.Incident{
		GroupID:     req.GroupID,
		GroupTaskID: req.GroupTaskID,
		Title:       req.Title,
		Category:    &req.Category,
		Severity:    &req.Severity,
	}
	if req.Description != "" {
		inc.Description = &req.Description
	}
	if t := parseDate(req.OccurredAt); t != nil {
		inc.OccurredAt = t
	}

	if err := h.DB.Create(&inc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not create incident."})
		return
	}
	h.loadIncident(&inc)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Incident reported successfully",
		"data":    h.transformIncident(&inc),
	})
}

// IncidentShow mirrors IncidentController::show.
func (h *Handler) IncidentShow(c *gin.Context) {
	p := h.principal(c)
	inc, ok := h.findIncident(c)
	if !ok {
		return
	}
	if !auth.CanViewIncident(h.DB, p, inc) {
		abort403(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.transformIncident(inc)})
}

// IncidentUpdate mirrors IncidentController::update.
func (h *Handler) IncidentUpdate(c *gin.Context) {
	p := h.principal(c)
	inc, ok := h.findIncident(c)
	if !ok {
		return
	}
	if !auth.CanUpdateIncident(h.DB, p, inc) {
		abort403(c)
		return
	}

	var body map[string]interface{}
	_ = bindJSONorForm(c, &body)

	updates := map[string]interface{}{}
	errs := map[string][]string{}

	if v, ok := stringField(body, "title"); ok {
		if v == "" {
			errs["title"] = []string{"The title field must be a string."}
		} else {
			updates["title"] = v
		}
	}
	if v, ok := stringField(body, "description"); ok {
		updates["description"] = v
	}
	if v, ok := stringField(body, "category"); ok {
		if !incidentCategories[v] {
			errs["category"] = []string{"The selected category is invalid."}
		} else {
			updates["category"] = v
		}
	}
	if v, ok := stringField(body, "severity"); ok {
		if !incidentSeverities[v] {
			errs["severity"] = []string{"The selected severity is invalid."}
		} else {
			updates["severity"] = v
		}
	}
	if v, ok := stringField(body, "occurred_at"); ok {
		if t := parseDate(v); t != nil {
			updates["occurred_at"] = t
		}
	}

	if p.HasRole(enums.RoleAdmin) || p.IsSuperAdmin() {
		if v, ok := stringField(body, "status"); ok {
			if !incidentStatuses[v] {
				errs["status"] = []string{"The selected status is invalid."}
			} else {
				updates["status"] = v
			}
		}
		if _, present := body["assigned_to_id"]; present {
			updates["assigned_to_id"] = body["assigned_to_id"]
		}
		if v, ok := stringField(body, "resolution_notes"); ok {
			updates["resolution_notes"] = v
		}
	}

	if len(errs) > 0 {
		validationError(c, errs)
		return
	}
	if len(updates) > 0 {
		h.DB.Model(&models.Incident{}).Where("id = ?", inc.ID).Updates(updates)
	}
	h.DB.First(inc, inc.ID)
	h.loadIncident(inc)

	c.JSON(http.StatusOK, gin.H{
		"message": "Incident updated successfully",
		"data":    h.transformIncident(inc),
	})
}

// IncidentDestroy mirrors IncidentController::destroy.
func (h *Handler) IncidentDestroy(c *gin.Context) {
	p := h.principal(c)
	inc, ok := h.findIncident(c)
	if !ok {
		return
	}
	if !p.CanDeleteIncident() {
		abort403(c)
		return
	}
	h.DB.Delete(&models.Incident{}, inc.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Incident deleted successfully"})
}

func (h *Handler) findIncident(c *gin.Context) (*models.Incident, bool) {
	var inc models.Incident
	if err := h.DB.Preload("Group").First(&inc, c.Param("incident")).Error; err != nil {
		notFound(c, "")
		return nil, false
	}
	return &inc, true
}

func (h *Handler) loadIncident(inc *models.Incident) {
	h.DB.Preload("Group.Customer").Preload("ReportedBy").Preload("AssignedTo").Preload("GroupTask").First(inc, inc.ID)
}

func (h *Handler) transformIncident(inc *models.Incident) gin.H {
	var category, categoryLabel, severity, severityLabel, status, statusLabel interface{}
	if inc.Category != nil {
		category = *inc.Category
		categoryLabel = enums.IncidentCategoryLabel(*inc.Category)
	}
	if inc.Severity != nil {
		severity = *inc.Severity
		severityLabel = enums.IncidentSeverityLabel(*inc.Severity)
	}
	if inc.Status != nil {
		status = *inc.Status
		statusLabel = enums.IncidentStatusLabel(*inc.Status)
	}

	var reportedBy, assignedTo gin.H
	if inc.ReportedBy != nil {
		reportedBy = gin.H{"id": inc.ReportedBy.ID, "name": inc.ReportedBy.Name}
	}
	if inc.AssignedTo != nil {
		assignedTo = gin.H{"id": inc.AssignedTo.ID, "name": inc.AssignedTo.Name}
	}
	var taskTitle interface{}
	if inc.GroupTask != nil {
		taskTitle = inc.GroupTask.Title
	}

	created := inc.CreatedAt
	return gin.H{
		"id":               inc.ID,
		"group_id":         inc.GroupID,
		"group_name":       groupName(inc.Group),
		"customer_name":    groupCustomerName(inc.Group),
		"group_task_id":    inc.GroupTaskID,
		"group_task_title": taskTitle,
		"title":            inc.Title,
		"description":      inc.Description,
		"category":         category,
		"category_label":   categoryLabel,
		"severity":         severity,
		"severity_label":   severityLabel,
		"status":           status,
		"status_label":     statusLabel,
		"resolution_notes": inc.ResolutionNotes,
		"occurred_at":      support.ISO8601(inc.OccurredAt),
		"resolved_at":      support.ISO8601(inc.ResolvedAt),
		"reported_by":      reportedBy,
		"assigned_to":      assignedTo,
		"created_at":       support.ISO8601(&created),
	}
}
