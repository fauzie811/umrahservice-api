package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

// applyGroupRoleScopes ports the role-based whereHas filters used by the group
// queries. includeCheckIn toggles the CheckInOutTeam branch (index only).
func applyGroupRoleScopes(q *gorm.DB, p *auth.Principal, includeCheckIn bool) *gorm.DB {
	if includeCheckIn && p.HasExactRoles(enums.RoleCheckInOutTeam) {
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
		Scopes(models.CurrentPeriod(periodID, true), models.Confirmed)
	// show does NOT apply the CheckInOutTeam branch.
	q = applyGroupRoleScopes(q, p, false)

	var group models.Group
	if err := q.Where("groups.id = ?", id).First(&group).Error; err != nil {
		notFound(c, "")
		return
	}

	files := h.buildGroupFiles(group.ID)
	pdfName, pdfData := h.pifData(&group)

	mutawifNames := []string{}
	for _, m := range group.Mutawifs() {
		mutawifNames = append(mutawifNames, m.Name)
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":            group.ID,
		"group_name":    group.Name,
		"customer_name": group.CustomerName(),
		"pax_adults":    group.PaxAdults,
		"pax_children":  group.PaxChildren,
		"pax_infants":   group.PaxInfants,
		"arrival_date":  formatDateOnly(group.ArrivalDate),
		"progress":      group.Progress,
		"mutawifs":      mutawifNames,
		"files":         files,
		"pdf_name":      pdfName,
		"pdf_data":      pdfData,
	}})
}

// fileEntry is one item in the group files list.
type groupFile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (h *Handler) buildGroupFiles(groupID uint64) []groupFile {
	var data models.GroupData
	if err := h.DB.Where("group_id = ?", groupID).First(&data).Error; err != nil {
		return []groupFile{}
	}

	files := []groupFile{}
	add := func(id int, name string, url *string) {
		if url != nil && *url != "" {
			files = append(files, groupFile{ID: id, Name: name, URL: *url})
		}
	}
	add(1, "Visa", data.Visa)
	add(2, "Ticket", data.Ticket)
	add(3, "Roomlist", data.Roomlist)
	add(4, "Manifest", data.Manifest)

	var dynamic []struct {
		Name string `json:"name"`
		File string `json:"file"`
	}
	decodeJSON(data.Files, &dynamic)
	idx := 0
	for _, f := range dynamic {
		if f.File == "" {
			continue
		}
		files = append(files, groupFile{
			ID:   10 + idx,
			Name: f.Name,
			URL:  h.Storage.URL(f.File),
		})
		idx++
	}

	return files
}

// dynamicFile is a stored {name, file} entry in group_data.files.
type dynamicFile struct {
	Name string `json:"name"`
	File string `json:"file"`
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

	var data models.GroupData
	h.DB.Where("group_id = ?", *groupID).FirstOrCreate(&data, models.GroupData{GroupID: *groupID})

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

	files := h.filteredDynamicFiles(data.Files)
	files = append(files, dynamicFile{Name: name, File: key})
	data.Files = jsonArray(files)
	h.DB.Save(&data)

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

	var data models.GroupData
	if err := h.DB.Where("group_id = ?", *groupID).First(&data).Error; err != nil {
		notFound(c, "")
		return
	}

	fh, _ := c.FormFile("file")
	name := c.PostForm("name")

	if col, ok := fixedFileColumn(fileID); ok {
		if fh != nil {
			if old := fixedFileValue(&data, col); old != nil && *old != "" {
				_ = h.Storage.Delete(c.Request.Context(), *old)
			}
			content, contentType, ext, err := readUpload(fh)
			if err == nil {
				key, err := h.Storage.Store(c.Request.Context(), "group_data", ext, contentType, content)
				if err == nil {
					setFixedFile(&data, col, &key)
				}
			}
		}
		h.DB.Save(&data)
		c.JSON(http.StatusOK, gin.H{"message": "File updated successfully"})
		return
	}

	if fileID >= 10 {
		index := fileID - 10
		files := h.filteredDynamicFiles(data.Files)
		if index >= 0 && index < len(files) {
			if name != "" {
				files[index].Name = name
			}
			if fh != nil {
				_ = h.Storage.Delete(c.Request.Context(), files[index].File)
				content, contentType, ext, err := readUpload(fh)
				if err == nil {
					if key, err := h.Storage.Store(c.Request.Context(), "group_data", ext, contentType, content); err == nil {
						files[index].File = key
					}
				}
			}
			data.Files = jsonArray(files)
			h.DB.Save(&data)
			c.JSON(http.StatusOK, gin.H{"message": "File updated successfully"})
			return
		}
	}

	notFound(c, "File not found")
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

	var data models.GroupData
	if err := h.DB.Where("group_id = ?", *groupID).First(&data).Error; err != nil {
		notFound(c, "")
		return
	}

	if col, ok := fixedFileColumn(fileID); ok {
		if old := fixedFileValue(&data, col); old != nil && *old != "" {
			_ = h.Storage.Delete(c.Request.Context(), *old)
			setFixedFile(&data, col, nil)
			h.DB.Save(&data)
		}
		c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
		return
	}

	if fileID >= 10 {
		index := fileID - 10
		files := h.filteredDynamicFiles(data.Files)
		if index >= 0 && index < len(files) {
			if files[index].File != "" {
				_ = h.Storage.Delete(c.Request.Context(), files[index].File)
			}
			files = append(files[:index], files[index+1:]...)
			data.Files = jsonArray(files)
			h.DB.Save(&data)
			c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
			return
		}
	}

	notFound(c, "File not found")
}

func (h *Handler) filteredDynamicFiles(raw datatypes.JSON) []dynamicFile {
	var files []dynamicFile
	decodeJSON(raw, &files)
	out := make([]dynamicFile, 0, len(files))
	for _, f := range files {
		if f.File != "" {
			out = append(out, f)
		}
	}
	return out
}

func fixedFileColumn(fileID int) (string, bool) {
	switch fileID {
	case 1:
		return "visa", true
	case 2:
		return "ticket", true
	case 3:
		return "roomlist", true
	case 4:
		return "manifest", true
	}
	return "", false
}

func fixedFileValue(d *models.GroupData, col string) *string {
	switch col {
	case "visa":
		return d.Visa
	case "ticket":
		return d.Ticket
	case "roomlist":
		return d.Roomlist
	case "manifest":
		return d.Manifest
	}
	return nil
}

func setFixedFile(d *models.GroupData, col string, v *string) {
	switch col {
	case "visa":
		d.Visa = v
	case "ticket":
		d.Ticket = v
	case "roomlist":
		d.Roomlist = v
	case "manifest":
		d.Manifest = v
	}
}
