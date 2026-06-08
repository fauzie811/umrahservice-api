package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
)

// IncidentProgressIndex mirrors IncidentProgressEntryController::index.
func (h *Handler) IncidentProgressIndex(c *gin.Context) {
	p := h.principal(c)
	inc, ok := h.findIncident(c)
	if !ok {
		return
	}
	if !auth.CanViewIncident(h.DB, p, inc) {
		abort403(c)
		return
	}

	var entries []models.IncidentProgressEntry
	h.DB.
		Where("incident_id = ?", inc.ID).
		Preload("User").
		Order("created_at DESC").Order("id DESC").
		Find(&entries)

	data := make([]gin.H, 0, len(entries))
	for i := range entries {
		data = append(data, transformProgressEntry(&entries[i]))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// IncidentProgressStore mirrors IncidentProgressEntryController::store.
func (h *Handler) IncidentProgressStore(c *gin.Context) {
	p := h.principal(c)
	inc, ok := h.findIncident(c)
	if !ok {
		return
	}
	if !auth.CanCreateIncidentProgress(h.DB, p, inc) {
		abort403(c)
		return
	}

	var body map[string]interface{}
	_ = bindJSONorForm(c, &body)

	isAdmin := p.IsSuperAdmin() || p.HasRole(enums.RoleAdmin)

	errs := map[string][]string{}

	var note string
	if v, present := stringField(body, "note"); present {
		note = strings.TrimSpace(v)
	}

	var targetStatus string
	if isAdmin {
		if v, present := stringField(body, "to_status"); present && v != "" {
			if !incidentStatuses[v] {
				errs["to_status"] = []string{"The selected to status is invalid."}
			} else {
				targetStatus = v
			}
		}
	}

	if len(errs) > 0 {
		validationError(c, errs)
		return
	}

	statusChanges := targetStatus != "" && (inc.Status == nil || *inc.Status != targetStatus)
	hasNote := note != ""

	if !statusChanges && !hasNote {
		validationError(c, map[string][]string{
			"note": {"A note is required when not changing status."},
		})
		return
	}

	var entry *models.IncidentProgressEntry
	if statusChanges {
		// Update status, then record transition (mirrors the model's updated hook).
		h.DB.Model(&models.Incident{}).Where("id = ?", inc.ID).
			Update("status", targetStatus)
		var notePtr *string
		if hasNote {
			notePtr = &note
		}
		entry = h.recordIncidentStatusChange(inc.ID, inc.Status, targetStatus, &p.User.ID, notePtr)
	} else {
		entry = &models.IncidentProgressEntry{
			IncidentID: inc.ID,
			UserID:     &p.User.ID,
			Note:       &note,
		}
		h.DB.Create(entry)
	}

	h.DB.Preload("User").First(entry, entry.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Progress entry added",
		"data":    transformProgressEntry(entry),
	})
}

// IncidentProgressDestroy mirrors IncidentProgressEntryController::destroy
// (admin-only delete).
func (h *Handler) IncidentProgressDestroy(c *gin.Context) {
	p := h.principal(c)
	inc, ok := h.findIncident(c)
	if !ok {
		return
	}

	var entry models.IncidentProgressEntry
	if err := h.DB.First(&entry, c.Param("entry")).Error; err != nil {
		notFound(c, "")
		return
	}
	if entry.IncidentID != inc.ID {
		notFound(c, "")
		return
	}
	if !p.CanDeleteIncidentProgress() {
		abort403(c)
		return
	}

	h.DB.Delete(&models.IncidentProgressEntry{}, entry.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Progress entry deleted"})
}
