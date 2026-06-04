package models

import (
	"time"

	"gorm.io/datatypes"
)

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

// RoomMeta is the decoded shape of Room.Meta.
type RoomMeta struct {
	GroupHotelIDs []int64 `json:"group_hotel_ids"`
	RoomNumbers   []struct {
		GroupHotelID int64  `json:"group_hotel_id"`
		RoomNumber   string `json:"room_number"`
	} `json:"room_numbers"`
}
