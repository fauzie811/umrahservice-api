package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	"umrahservice-api/internal/models"
)

// Rooming mirrors Api\RoomingController::index — list a group's hotels and
// rooms with their per-hotel room numbers.
func (h *Handler) Rooming(c *gin.Context) {
	group, ok := h.findVisibleGroup(c, c.Param("id"))
	if !ok {
		return
	}

	var hotels []models.GroupHotel
	h.DB.Preload("Hotel").
		Where("group_id = ?", group.ID).
		Order("check_in").
		Find(&hotels)

	var rooms []models.Room
	h.DB.Where("group_id = ?", group.ID).Order("name").Find(&rooms)

	hotelData := make([]gin.H, 0, len(hotels))
	for i := range hotels {
		hotelData = append(hotelData, transformRoomingHotel(&hotels[i]))
	}

	roomData := make([]gin.H, 0, len(rooms))
	for i := range rooms {
		roomData = append(roomData, h.transformRoom(group.ID, &rooms[i]))
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"hotels": hotelData,
		"rooms":  roomData,
	}})
}

type roomingUpdateRequest struct {
	RoomNumbers []struct {
		GroupHotelID *int64  `json:"group_hotel_id"`
		RoomNumber   *string `json:"room_number"`
	} `json:"room_numbers"`
}

// RoomingUpdate mirrors Api\RoomingController::updateRoomNumbers — merge the
// supplied per-hotel room numbers into a room's meta.
func (h *Handler) RoomingUpdate(c *gin.Context) {
	group, ok := h.findVisibleGroup(c, c.Param("id"))
	if !ok {
		return
	}

	var room models.Room
	if err := h.DB.Where("group_id = ? AND id = ?", group.ID, c.Param("roomId")).First(&room).Error; err != nil {
		notFound(c, "")
		return
	}

	var req roomingUpdateRequest
	_ = c.ShouldBindJSON(&req)

	var validHotelIDs []int64
	h.DB.Model(&models.GroupHotel{}).Where("group_id = ?", group.ID).Pluck("id", &validHotelIDs)
	validSet := map[int64]bool{}
	for _, id := range validHotelIDs {
		validSet[id] = true
	}

	errs := map[string][]string{}
	for i, entry := range req.RoomNumbers {
		field := "room_numbers." + strconv.Itoa(i) + ".group_hotel_id"
		if entry.GroupHotelID == nil {
			errs[field] = []string{"The group hotel id field is required."}
		} else if !validSet[*entry.GroupHotelID] {
			errs[field] = []string{"The selected group hotel id is invalid."}
		}
	}
	if len(errs) > 0 {
		validationError(c, errs)
		return
	}

	// Decode the full meta so sibling keys (group_hotel_ids) survive the save.
	var meta map[string]interface{}
	decodeJSON(room.Meta, &meta)
	if meta == nil {
		meta = map[string]interface{}{}
	}

	// Existing numbers, preserving order, then apply the updates.
	order := []int64{}
	values := map[int64]string{}
	var current models.RoomMeta
	decodeJSON(room.Meta, &current)
	for _, rn := range current.RoomNumbers {
		ghID := rn.GroupHotelID.Int64()
		if _, seen := values[ghID]; !seen {
			order = append(order, ghID)
		}
		values[ghID] = rn.RoomNumber
	}
	for _, entry := range req.RoomNumbers {
		ghID := *entry.GroupHotelID
		val := ""
		if entry.RoomNumber != nil {
			val = strings.TrimSpace(*entry.RoomNumber)
		}
		if val == "" {
			delete(values, ghID)
			continue
		}
		if _, seen := values[ghID]; !seen {
			order = append(order, ghID)
		}
		values[ghID] = val
	}

	merged := make([]map[string]interface{}, 0, len(order))
	for _, ghID := range order {
		if val, ok := values[ghID]; ok {
			merged = append(merged, map[string]interface{}{
				"group_hotel_id": ghID,
				"room_number":    val,
			})
		}
	}
	meta["room_numbers"] = merged

	b, _ := json.Marshal(meta)
	room.Meta = datatypes.JSON(b)
	if err := h.DB.Save(&room).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not save room numbers."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Room numbers saved successfully",
		"data":    h.transformRoom(group.ID, &room),
	})
}

func transformRoomingHotel(gh *models.GroupHotel) gin.H {
	var city, name interface{}
	if gh.Hotel != nil {
		city = gh.Hotel.City
		name = gh.Hotel.Name
	}
	var dr interface{}
	if d := gh.DateRange(); d != "" {
		dr = d
	}
	return gin.H{
		"group_hotel_id": gh.ID,
		"city":           city,
		"name":           name,
		"date_range":     dr,
	}
}

func (h *Handler) transformRoom(groupID uint64, room *models.Room) gin.H {
	var meta models.RoomMeta
	decodeJSON(room.Meta, &meta)

	roomNumbers := make([]gin.H, 0, len(meta.RoomNumbers))
	for _, rn := range meta.RoomNumbers {
		roomNumbers = append(roomNumbers, gin.H{
			"group_hotel_id": rn.GroupHotelID.Int64(),
			"room_number":    rn.RoomNumber,
		})
	}

	return gin.H{
		"id":              room.ID,
		"name":            room.Name,
		"number":          room.Number,
		"capacity":        roomCapacityLabel(room.Capacity),
		"group_hotel_ids": h.assignedHotelIDs(groupID, meta),
		"pilgrims":        h.roomPilgrimNames(room.ID),
		"room_numbers":    roomNumbers,
	}
}

// assignedHotelIDs ports Room::getAssignedHotelIds — the group hotels this room
// covers, ordered by check-in. Falls back to all group hotels.
func (h *Handler) assignedHotelIDs(groupID uint64, meta models.RoomMeta) []int64 {
	q := h.DB.Model(&models.GroupHotel{}).Where("group_id = ?", groupID)
	if assigned := meta.GroupHotelIDInts(); len(assigned) > 0 {
		q = q.Where("id IN ?", assigned)
	}
	ids := []int64{}
	q.Order("check_in").Pluck("id", &ids)
	return ids
}

func (h *Handler) roomPilgrimNames(roomID uint64) []string {
	names := []string{}
	h.DB.Table("pilgrims").
		Select("pilgrims.fullname").
		Joins("JOIN group_pilgrim ON group_pilgrim.pilgrim_id = pilgrims.id").
		Joins("JOIN group_pilgrim_room ON group_pilgrim_room.group_pilgrim_id = group_pilgrim.id").
		Where("group_pilgrim_room.room_id = ?", roomID).
		Pluck("pilgrims.fullname", &names)
	return names
}

// roomCapacityLabel mirrors App\Enums\RoomCapacity::getLabel.
func roomCapacityLabel(c *int) interface{} {
	if c == nil {
		return nil
	}
	switch *c {
	case 1:
		return "Single"
	case 2:
		return "Double"
	case 3:
		return "Triple"
	case 4:
		return "Quad"
	case 5:
		return "Quint"
	}
	return nil
}
