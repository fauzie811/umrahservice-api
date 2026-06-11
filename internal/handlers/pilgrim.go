package handlers

import (
	"encoding/base64"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
)

// LuggageTag mirrors Api\PilgrimController::byTag.
func (h *Handler) LuggageTag(c *gin.Context) {
	code := normalizeCode(c.Param("code"))

	decoded, err := base64.StdEncoding.DecodeString(code)
	if err != nil {
		notFound(c, "")
		return
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		notFound(c, "")
		return
	}
	groupID := parseUintPtr(parts[0])
	pilgrimID := parseUintPtr(parts[1])
	if groupID == nil || pilgrimID == nil {
		notFound(c, "")
		return
	}

	var gp models.GroupPilgrim
	if err := h.DB.Where("group_id = ? AND pilgrim_id = ?", *groupID, *pilgrimID).First(&gp).Error; err != nil {
		notFound(c, "")
		return
	}

	var group models.Group
	if err := h.DB.
		Preload("Customer").Preload("Mutawif").Preload("Mutawif2").Preload("Mutawif3").Preload("Muassasah").
		First(&group, *groupID).Error; err != nil {
		notFound(c, "")
		return
	}

	var pilgrim models.Pilgrim
	if err := h.DB.First(&pilgrim, *pilgrimID).Error; err != nil {
		notFound(c, "")
		return
	}

	// Rooms attached to this group_pilgrim.
	var gpWithRooms models.GroupPilgrim
	h.DB.Preload("Rooms").First(&gpWithRooms, gp.ID)
	rooms := gpWithRooms.Rooms

	handlers := h.hotelHandlersByCity(&group)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"pilgrim": gin.H{
			"fullname":        pilgrim.Fullname,
			"passport_number": pilgrim.PassportNumber,
			"photo_url":       h.photoURL(pilgrim.Photo),
			"bus_no":          gp.BusNo,
			"room_numbers":    roomNumbers(rooms),
		},
		"group": gin.H{
			"id":         group.ID,
			"group_name": group.Name,
			"muassasah":  muassasahName(&group),
		},
		"luggage_tag_color": h.luggageTagColor(&group),
		"tour_leaders":      h.tourLeaders(&group),
		"mutawifs":          mutawifContacts(&group),
		"hotels":            h.buildHotels(&group, rooms),
		"handlers": gin.H{
			"makkah":  handlerForCity(handlers, "Makkah"),
			"madinah": handlerForCity(handlers, "Madinah"),
		},
	}})
}

// normalizeCode strips a full scanned URL down to the trailing code segment.
func normalizeCode(code string) string {
	if idx := strings.LastIndex(code, "luggage-tag/"); idx >= 0 {
		code = code[idx+len("luggage-tag/"):]
	}
	return strings.Trim(code, "/ ")
}

func roomNumbers(rooms []models.Room) interface{} {
	seen := map[string]bool{}
	var nums []string
	for _, r := range rooms {
		if r.Number != nil && *r.Number != "" && !seen[*r.Number] {
			seen[*r.Number] = true
			nums = append(nums, *r.Number)
		}
	}
	if len(nums) == 0 {
		return nil
	}
	return strings.Join(nums, " / ")
}

func muassasahName(g *models.Group) interface{} {
	if g.Muassasah != nil {
		return g.Muassasah.CompanyName
	}
	return nil
}

func (h *Handler) luggageTagColor(g *models.Group) interface{} {
	var meta map[string]interface{}
	decodeJSON(g.Meta, &meta)
	val, _ := meta["luggage_tag_color"].(string)
	tint := enums.LuggageTagTint(val)
	if tint == "" {
		return nil
	}
	return tint
}

func (h *Handler) tourLeaders(g *models.Group) []gin.H {
	var contacts []models.Contact
	h.DB.Table("contacts").
		Joins("JOIN group_tour_leader gtl ON gtl.contact_id = contacts.id").
		Where("gtl.group_id = ? AND contacts.type = ?", g.ID, "tour-leader").
		Find(&contacts)

	out := make([]gin.H, 0, len(contacts))
	for _, ct := range contacts {
		out = append(out, gin.H{"name": ct.Name, "phone": ct.Phone})
	}
	return out
}

func mutawifContacts(g *models.Group) []gin.H {
	out := []gin.H{}
	for _, m := range g.Mutawifs() {
		out = append(out, gin.H{"name": m.Name, "phone": m.Phone})
	}
	return out
}

// hotelHandler is a decoded+enriched group.hotel_handlers entry.
type hotelHandler struct {
	City      string  `json:"city"`
	HandlerID *uint64 `json:"handler_id"`
	Name      string  `json:"name"`
	Phone     string  `json:"phone"`
}

func (h *Handler) hotelHandlersByCity(g *models.Group) map[string]hotelHandler {
	var raw []map[string]interface{}
	decodeJSON(g.HotelHandlers, &raw)

	// Collect handler ids to enrich names/phones.
	ids := []uint64{}
	for _, item := range raw {
		if id := toUint(item["handler_id"]); id != nil {
			ids = append(ids, *id)
		}
	}
	users := map[uint64]models.User{}
	if len(ids) > 0 {
		var list []models.User
		h.DB.Where("id IN ?", ids).Find(&list)
		for _, u := range list {
			users[u.ID] = u
		}
	}

	out := map[string]hotelHandler{}
	for _, item := range raw {
		city, _ := item["city"].(string)
		hh := hotelHandler{City: city}
		if id := toUint(item["handler_id"]); id != nil {
			hh.HandlerID = id
			if u, ok := users[*id]; ok {
				hh.Name = u.Name
				if u.Phone != nil {
					hh.Phone = *u.Phone
				}
			} else {
				hh.Name = "-"
				hh.Phone = "-"
			}
		} else {
			if n, ok := item["name"].(string); ok {
				hh.Name = n
			}
			if p, ok := item["phone"].(string); ok {
				hh.Phone = p
			}
		}
		out[city] = hh
	}
	return out
}

func handlerForCity(handlers map[string]hotelHandler, city string) interface{} {
	h, ok := handlers[city]
	if !ok || h.Name == "" {
		return nil
	}
	var phone interface{}
	if h.Phone != "" {
		phone = h.Phone
	}
	return gin.H{"name": h.Name, "phone": phone}
}

// buildHotels ports PilgrimController::buildHotels.
func (h *Handler) buildHotels(group *models.Group, rooms []models.Room) []gin.H {
	// Collect group_hotel_ids referenced by rooms' meta.
	idSet := map[int64]bool{}
	hotelRoomMap := map[int64]string{}
	for _, r := range rooms {
		var meta models.RoomMeta
		decodeJSON(r.Meta, &meta)
		for _, id := range meta.GroupHotelIDs {
			idSet[id] = true
		}
		for _, rn := range meta.RoomNumbers {
			if rn.RoomNumber != "" {
				hotelRoomMap[rn.GroupHotelID] = rn.RoomNumber
			}
		}
	}

	var ids []int64
	for id := range idSet {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		h.DB.Model(&models.GroupHotel{}).Where("group_id = ?", group.ID).Pluck("id", &ids)
	}
	if len(ids) == 0 {
		return []gin.H{}
	}

	var ghs []models.GroupHotel
	// Columns are qualified because the JOIN brings in hotels.id / hotels.group
	// ambiguity; an unqualified `id IN ?` errors out and silently drops hotels.
	if err := h.DB.Preload("Hotel").
		Select("group_hotel.*").
		Where("group_hotel.group_id = ? AND group_hotel.id IN ?", group.ID, ids).
		Joins("JOIN hotels ON hotels.id = group_hotel.hotel_id AND hotels.city IN ('Makkah','Madinah')").
		Find(&ghs).Error; err != nil {
		return []gin.H{}
	}

	// Sort by hotel city descending (Madinah before Makkah).
	sort.SliceStable(ghs, func(i, j int) bool {
		ci, cj := "", ""
		if ghs[i].Hotel != nil {
			ci = ghs[i].Hotel.City
		}
		if ghs[j].Hotel != nil {
			cj = ghs[j].Hotel.City
		}
		return ci > cj
	})

	out := make([]gin.H, 0, len(ghs))
	for i := range ghs {
		gh := &ghs[i]
		roomNum := hotelRoomMap[int64(gh.ID)]
		var city, name interface{}
		if gh.Hotel != nil {
			city = gh.Hotel.City
			name = gh.Hotel.Name
		}
		var rn interface{}
		if roomNum != "" {
			rn = roomNum
		}
		dr := gh.DateRange()
		var drv interface{}
		if dr != "" {
			drv = dr
		}
		out = append(out, gin.H{
			"city":        city,
			"name":        name,
			"date_range":  drv,
			"room_number": rn,
		})
	}
	return out
}

func toUint(v interface{}) *uint64 {
	switch t := v.(type) {
	case float64:
		u := uint64(t)
		return &u
	case string:
		return parseUintPtr(t)
	}
	return nil
}
