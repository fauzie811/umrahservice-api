package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

// BaggageItemTypes mirrors BaggageController::itemTypes — active types for pickers.
func (h *Handler) BaggageItemTypes(c *gin.Context) {
	var types []models.BaggageItemType
	h.DB.Where("is_active = ?", true).
		Order("sort").Order("name").
		Find(&types)

	data := make([]gin.H, 0, len(types))
	for i := range types {
		t := &types[i]
		data = append(data, gin.H{"id": t.ID, "name": t.Name, "sort": t.Sort})
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// BaggageIndex mirrors BaggageController::index — list counts for a group.
func (h *Handler) BaggageIndex(c *gin.Context) {
	group, ok := h.findVisibleGroup(c, c.Param("id"))
	if !ok {
		return
	}

	checkpoint := c.Query("checkpoint")
	if checkpoint != "" && !enums.IsBaggageCheckpoint(checkpoint) {
		validationError(c, map[string][]string{"checkpoint": {"The selected checkpoint is invalid."}})
		return
	}

	q := h.DB.Model(&models.GroupBaggageCount{}).
		Preload("ItemType").Preload("CountedBy").
		Where("group_id = ?", group.ID)
	if checkpoint != "" {
		q = q.Where("checkpoint = ?", checkpoint)
	}

	var counts []models.GroupBaggageCount
	q.Order("checkpoint").Find(&counts)

	data := make([]gin.H, 0, len(counts))
	for i := range counts {
		data = append(data, transformBaggageCount(&counts[i]))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

type baggageStoreRequest struct {
	Checkpoint        string  `json:"checkpoint" form:"checkpoint"`
	BaggageItemTypeID *uint64 `json:"baggage_item_type_id" form:"baggage_item_type_id"`
	City              *string `json:"city" form:"city"`
	ExpectedCount     *int    `json:"expected_count" form:"expected_count"`
	Count             *int    `json:"count" form:"count"`
	Note              *string `json:"note" form:"note"`
	CountedAt         string  `json:"counted_at" form:"counted_at"`
}

// BaggageStore mirrors BaggageController::store — upsert a count at a checkpoint.
func (h *Handler) BaggageStore(c *gin.Context) {
	p := h.principal(c)
	group, ok := h.findVisibleGroup(c, c.Param("id"))
	if !ok {
		return
	}

	var req baggageStoreRequest
	_ = c.ShouldBind(&req)

	errs := map[string][]string{}
	if req.Checkpoint == "" {
		errs["checkpoint"] = []string{"The checkpoint field is required."}
	} else if !enums.IsBaggageCheckpoint(req.Checkpoint) {
		errs["checkpoint"] = []string{"The selected checkpoint is invalid."}
	}
	if req.BaggageItemTypeID == nil {
		errs["baggage_item_type_id"] = []string{"The baggage item type id field is required."}
	} else {
		var n int64
		h.DB.Model(&models.BaggageItemType{}).Where("id = ?", *req.BaggageItemTypeID).Count(&n)
		if n == 0 {
			errs["baggage_item_type_id"] = []string{"The selected baggage item type id is invalid."}
		}
	}
	if req.Count == nil {
		errs["count"] = []string{"The count field is required."}
	} else if *req.Count < 0 {
		errs["count"] = []string{"The count field must be at least 0."}
	}
	if req.ExpectedCount != nil && *req.ExpectedCount < 0 {
		errs["expected_count"] = []string{"The expected count field must be at least 0."}
	}
	if len(errs) > 0 {
		validationError(c, errs)
		return
	}

	city := ""
	if req.City != nil {
		city = *req.City
	}
	countedAt := parseDate(req.CountedAt)
	if countedAt == nil {
		now := time.Now()
		countedAt = &now
	}
	expected := 0
	if req.ExpectedCount != nil {
		expected = *req.ExpectedCount
	}

	var count models.GroupBaggageCount
	err := h.DB.Where(&models.GroupBaggageCount{
		GroupID:           group.ID,
		Checkpoint:        req.Checkpoint,
		City:              city,
		BaggageItemTypeID: *req.BaggageItemTypeID,
	}).First(&count).Error
	created := err == gorm.ErrRecordNotFound

	count.GroupID = group.ID
	count.Checkpoint = req.Checkpoint
	count.City = city
	count.BaggageItemTypeID = *req.BaggageItemTypeID
	count.ExpectedCount = expected
	count.Count = *req.Count
	count.Note = req.Note
	count.CountedAt = countedAt
	count.CountedByID = &p.User.ID

	if err := h.DB.Save(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not save baggage count."})
		return
	}

	h.DB.Preload("ItemType").Preload("CountedBy").First(&count, count.ID)

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{
		"message": "Baggage count saved successfully",
		"data":    transformBaggageCount(&count),
	})
}

// BaggageDestroy mirrors BaggageController::destroy.
func (h *Handler) BaggageDestroy(c *gin.Context) {
	var count models.GroupBaggageCount
	if err := h.DB.First(&count, c.Param("baggageCount")).Error; err != nil {
		notFound(c, "")
		return
	}
	if _, ok := h.findVisibleGroup(c, itoa(count.GroupID)); !ok {
		return
	}
	h.DB.Delete(&models.GroupBaggageCount{}, count.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Baggage count deleted successfully"})
}

// findVisibleGroup loads a group within the auth user's baggage scope (mirrors
// BaggageController::visibleGroupQuery + findOrFail). Responds 404 when missing.
func (h *Handler) findVisibleGroup(c *gin.Context, id string) (*models.Group, bool) {
	p := h.principal(c)
	q := applyGroupRoleScopes(h.DB.Model(&models.Group{}), p, true)
	var group models.Group
	if err := q.Where("groups.id = ?", id).First(&group).Error; err != nil {
		notFound(c, "")
		return nil, false
	}
	return &group, true
}

func transformBaggageCount(count *models.GroupBaggageCount) gin.H {
	var city interface{}
	if count.City != "" {
		city = count.City
	}
	var itemType, countedBy gin.H
	if count.ItemType != nil {
		itemType = gin.H{"id": count.ItemType.ID, "name": count.ItemType.Name}
	}
	if count.CountedBy != nil {
		countedBy = gin.H{"id": count.CountedBy.ID, "name": count.CountedBy.Name}
	}
	updated := count.UpdatedAt
	return gin.H{
		"id":               count.ID,
		"group_id":         count.GroupID,
		"checkpoint":       count.Checkpoint,
		"checkpoint_label": enums.BaggageCheckpointLabel(count.Checkpoint),
		"city":             city,
		"item_type":        itemType,
		"expected_count":   count.ExpectedCount,
		"count":            count.Count,
		"variance":         count.Count - count.ExpectedCount,
		"note":             count.Note,
		"counted_at":       support.ISO8601(count.CountedAt),
		"counted_by":       countedBy,
		"updated_at":       support.ISO8601(&updated),
	}
}
