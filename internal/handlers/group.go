package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

// applyGroupRoleScopes ports the role-based whereHas filters used by the group
// queries. includeCheckIn toggles the Runner branch (index only).
func applyGroupRoleScopes(q *gorm.DB, p *auth.Principal, includeCheckIn bool) *gorm.DB {
	if includeCheckIn && p.HasExactRoles(enums.RoleRunner) {
		q = q.Where(`EXISTS (
			SELECT 1 FROM group_service gs
			JOIN services s ON s.id = gs.service_id
			WHERE gs.group_id = groups.id AND s.type IN ?
		)`, []string{enums.ServiceHotel, enums.ServiceHandling})
	}
	if p.HasExactRoles(enums.RoleAirportHandler) {
		if len(p.VendorIDs) == 0 {
			q = q.Where("1 = 0")
		} else {
			q = q.Where(`EXISTS (
				SELECT 1 FROM group_flights gf
				WHERE gf.group_id = groups.id AND gf.handler_id IN ?
			)`, p.VendorIDs)
		}
	}
	if p.HasExactRoles(enums.RoleMutawif) {
		uid := p.User.ID
		q = q.Where("mutawif_id = ? OR mutawif_2_id = ? OR mutawif_3_id = ?", uid, uid, uid)
	}
	return q
}

// groupInScope mirrors GroupController::getBaseQuery()->findOrFail($id): the
// group must be visible to the principal under the current-period, status and
// role scopes. Returns false (caller should 404) when the group is out of
// scope, closing the IDOR on the file endpoints.
func (h *Handler) groupInScope(c *gin.Context, p *auth.Principal, groupID uint64) bool {
	periodID := support.CurrentPeriodID(h.DB)
	q := h.DB.Model(&models.Group{}).
		Scopes(models.CurrentPeriod(periodID, true)).
		Where("status = ?", resolveGroupStatusFilter(c, p))
	q = applyGroupRoleScopes(q, p, true)

	var count int64
	q.Where("groups.id = ?", groupID).Count(&count)
	return count > 0
}

// validGroupStatuses mirrors Umrahservice\Groups\Enums\GroupStatus.
var validGroupStatuses = map[string]bool{
	"draft": true, "pending": true, "confirmed": true, "cancelled": true,
}

// resolveGroupStatusFilter mirrors GroupController::resolveStatusFilter. Only
// Admin/Finance may override the default via the `status` query param; every
// other role is locked to confirmed groups.
func resolveGroupStatusFilter(c *gin.Context, p *auth.Principal) string {
	if p.HasRole(enums.RoleAdmin) || p.HasRole(enums.RoleFinance) || p.IsSuperAdmin() {
		if requested := c.Query("status"); validGroupStatuses[requested] {
			return requested
		}
	}
	return "confirmed"
}

// GroupIndex mirrors Api\GroupController::index.
func (h *Handler) GroupIndex(c *gin.Context) {
	p := h.principal(c)
	periodID := support.CurrentPeriodID(h.DB)

	q := h.DB.Model(&models.Group{}).
		Preload("Customer").
		Scopes(models.CurrentPeriod(periodID, true)).
		Where("status = ?", resolveGroupStatusFilter(c, p))
	q = applyGroupRoleScopes(q, p, true)

	var groups []models.Group
	q.Order("arrival_date").Find(&groups)

	data := make([]gin.H, 0, len(groups))
	for i := range groups {
		g := &groups[i]
		data = append(data, gin.H{
			"id":            g.ID,
			"group_name":    g.Name,
			"customer_name": g.CustomerName(),
			"pax_adults":    g.PaxAdults,
			"pax_children":  g.PaxChildren,
			"pax_infants":   g.PaxInfants,
			"arrival_date":  formatDateOnly(g.ArrivalDate),
			"progress":      g.Progress,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// GroupShow mirrors Api\GroupController::show.
func (h *Handler) GroupShow(c *gin.Context) {
	p := h.principal(c)
	id := c.Param("id")
	periodID := support.CurrentPeriodID(h.DB)

	q := h.DB.Model(&models.Group{}).
		Preload("Customer").
		Preload("Mutawif").
		Preload("Mutawif2").
		Preload("Mutawif3").
		Preload("Services").
		Scopes(models.CurrentPeriod(periodID, true), models.Confirmed)
	// show does NOT apply the Runner branch.
	q = applyGroupRoleScopes(q, p, false)

	var group models.Group
	if err := q.Where("groups.id = ?", id).First(&group).Error; err != nil {
		notFound(c, "")
		return
	}

	pdfName, pdfData := h.pifData(&group)

	hotelHandlers := h.hotelHandlersByCity(&group)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":               group.ID,
		"group_name":       group.Name,
		"customer_name":    group.CustomerName(),
		"pax_adults":       group.PaxAdults,
		"pax_children":     group.PaxChildren,
		"pax_infants":      group.PaxInfants,
		"arrival_date":     formatDateOnly(group.ArrivalDate),
		"departure_date":   formatDateOnly(group.DepartureDate),
		"progress":         group.Progress,
		"services":         groupServices(&group),
		"tour_leaders":     h.tourLeaders(&group),
		"mutawifs":         mutawifContacts(&group),
		"airport_jeddah":   h.airportArrivalHandlers(group.ID),
		"check_in_makkah":  handlerForCity(hotelHandlers, "Makkah"),
		"check_in_madinah": handlerForCity(hotelHandlers, "Madinah"),
		"pdf_name":         pdfName,
		"pdf_data":         pdfData,
	}})
}

// airportArrivalHandlers returns distinct airport-handler vendors on the group's
// arrival flights (the "Tim Airport Jeddah" team).
func (h *Handler) airportArrivalHandlers(groupID uint64) []gin.H {
	var vendors []models.Vendor
	h.DB.Table("vendors").
		Joins("JOIN group_flights gf ON gf.handler_id = vendors.id").
		Where("gf.group_id = ? AND gf.type = ?", groupID, "arrival").
		Distinct().Find(&vendors)
	out := make([]gin.H, 0, len(vendors))
	for _, v := range vendors {
		out = append(out, gin.H{"name": v.CompanyName, "phone": v.ContactPhone})
	}
	return out
}

// GroupFiles mirrors Api\GroupController::files. Split out from GroupShow so
// clients can lazy-load the file list.
func (h *Handler) GroupFiles(c *gin.Context) {
	groupID := parseUintPtr(c.Param("id"))
	if groupID == nil {
		notFound(c, "")
		return
	}
	if !h.groupInScope(c, h.principal(c), *groupID) {
		notFound(c, "")
		return
	}
	files := h.buildGroupFiles(*groupID)
	c.JSON(http.StatusOK, gin.H{"data": files})
}

// groupFile is one item in the group files list.
type groupFile struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (h *Handler) buildGroupFiles(groupID uint64) []groupFile {
	var rows []models.GroupFile
	h.DB.Where("group_id = ?", groupID).Find(&rows)

	files := make([]groupFile, 0, len(rows))
	for _, f := range rows {
		files = append(files, groupFile{
			ID:   f.ID,
			Name: f.Name,
			URL:  h.Storage.FileURL(f.File),
		})
	}
	return files
}

// GroupStoreFile mirrors Api\GroupController::storeFile.
func (h *Handler) GroupStoreFile(c *gin.Context) {
	p := h.principal(c)
	if !p.Can("groups.updateData") && !p.IsSuperAdmin() {
		forbidden(c)
		return
	}
	groupID := parseUintPtr(c.Param("id"))
	if groupID == nil {
		notFound(c, "")
		return
	}
	if !h.groupInScope(c, p, *groupID) {
		notFound(c, "")
		return
	}

	name := c.PostForm("name")
	fh, ferr := c.FormFile("file")
	errs := map[string][]string{}
	if name == "" {
		errs["name"] = []string{"The name field is required."}
	}
	if ferr != nil || fh == nil {
		errs["file"] = []string{"The file field is required."}
	}
	if len(errs) > 0 {
		validationError(c, errs)
		return
	}

	content, contentType, ext, err := readUpload(fh)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not read upload."})
		return
	}
	key, err := h.Storage.Store(c.Request.Context(), "group_data", ext, contentType, content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not store file."})
		return
	}

	row := models.GroupFile{GroupID: *groupID, Name: name, File: key}
	h.DB.Create(&row)

	c.JSON(http.StatusCreated, gin.H{"message": "File uploaded successfully"})
}

// GroupUpdateFile mirrors Api\GroupController::updateFile.
func (h *Handler) GroupUpdateFile(c *gin.Context) {
	p := h.principal(c)
	if !p.Can("groups.updateData") && !p.IsSuperAdmin() {
		forbidden(c)
		return
	}
	groupID := parseUintPtr(c.Param("id"))
	fileID, _ := atoiParam(c.Param("fileId"))
	if groupID == nil {
		notFound(c, "")
		return
	}
	if !h.groupInScope(c, p, *groupID) {
		notFound(c, "")
		return
	}

	var row models.GroupFile
	if err := h.DB.Where("group_id = ? AND id = ?", *groupID, fileID).First(&row).Error; err != nil {
		notFound(c, "")
		return
	}

	if name := c.PostForm("name"); name != "" {
		row.Name = name
	}

	if fh, ferr := c.FormFile("file"); ferr == nil && fh != nil {
		_ = h.Storage.Delete(c.Request.Context(), row.File)
		content, contentType, ext, err := readUpload(fh)
		if err == nil {
			if key, err := h.Storage.Store(c.Request.Context(), "group_data", ext, contentType, content); err == nil {
				row.File = key
			}
		}
	}

	h.DB.Save(&row)
	c.JSON(http.StatusOK, gin.H{"message": "File updated successfully"})
}

// GroupDeleteFile mirrors Api\GroupController::deleteFile.
func (h *Handler) GroupDeleteFile(c *gin.Context) {
	p := h.principal(c)
	if !p.Can("groups.updateData") && !p.IsSuperAdmin() {
		forbidden(c)
		return
	}
	groupID := parseUintPtr(c.Param("id"))
	fileID, _ := atoiParam(c.Param("fileId"))
	if groupID == nil {
		notFound(c, "")
		return
	}
	if !h.groupInScope(c, p, *groupID) {
		notFound(c, "")
		return
	}

	var row models.GroupFile
	if err := h.DB.Where("group_id = ? AND id = ?", *groupID, fileID).First(&row).Error; err != nil {
		notFound(c, "")
		return
	}

	_ = h.Storage.Delete(c.Request.Context(), row.File)
	h.DB.Delete(&models.GroupFile{}, row.ID)

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}
