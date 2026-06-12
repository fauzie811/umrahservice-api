package models

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"

	"gorm.io/datatypes"
)

// FlexInt is an int64 that decodes from a JSON number OR a JSON string. Room
// meta stores group_hotel_id values inconsistently (e.g. [3190,"3122"]); PHP
// compares them loosely, so we must accept both. Null/empty decode to 0.
type FlexInt int64

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*f = FlexInt(v)
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexInt(n)
	return nil
}

// Int64 returns the underlying value.
func (f FlexInt) Int64() int64 { return int64(f) }

// Pilgrim maps `pilgrims`.
type Pilgrim struct {
	ID             uint64     `gorm:"primaryKey"`
	Fullname       string     `gorm:"column:fullname"`
	Title          *string    `gorm:"column:title"`
	PassportNumber *string    `gorm:"column:passport_number"`
	NationalID     *string    `gorm:"column:national_id"`
	Gender         *string    `gorm:"column:gender"`
	Birthplace     *string    `gorm:"column:birthplace"`
	Birthdate      *time.Time `gorm:"column:birthdate"`
	Phone          *string    `gorm:"column:phone"`
	Photo          *string    `gorm:"column:photo"`
}

func (Pilgrim) TableName() string { return "pilgrims" }

// GroupPilgrim maps the `group_pilgrim` pivot (with its own id).
type GroupPilgrim struct {
	ID           uint64  `gorm:"primaryKey"`
	GroupID      uint64  `gorm:"column:group_id"`
	PilgrimID    uint64  `gorm:"column:pilgrim_id"`
	BusNo        *string `gorm:"column:bus_no"`
	IsTourLeader bool    `gorm:"column:is_tour_leader"`

	Rooms []Room `gorm:"many2many:group_pilgrim_room;joinForeignKey:group_pilgrim_id;joinReferences:room_id;"`
}

func (GroupPilgrim) TableName() string { return "group_pilgrim" }

// Room maps `rooms`. `meta` holds group_hotel_ids and room_numbers.
type Room struct {
	ID       uint64         `gorm:"primaryKey"`
	GroupID  uint64         `gorm:"column:group_id"`
	Name     *string        `gorm:"column:name"`
	Number   *string        `gorm:"column:number"`
	Capacity *int           `gorm:"column:capacity"`
	Meta     datatypes.JSON `gorm:"column:meta"`
}

func (Room) TableName() string { return "rooms" }

// RoomMeta is the decoded shape of Room.Meta. ID fields use FlexInt because the
// stored JSON mixes numeric and string ids.
type RoomMeta struct {
	GroupHotelIDs []FlexInt `json:"group_hotel_ids"`
	RoomNumbers   []struct {
		GroupHotelID FlexInt `json:"group_hotel_id"`
		RoomNumber   string  `json:"room_number"`
	} `json:"room_numbers"`
}

// GroupHotelIDInts returns the group_hotel_ids as plain int64s, dropping zeros.
func (m RoomMeta) GroupHotelIDInts() []int64 {
	ids := make([]int64, 0, len(m.GroupHotelIDs))
	for _, id := range m.GroupHotelIDs {
		if id != 0 {
			ids = append(ids, int64(id))
		}
	}
	return ids
}
