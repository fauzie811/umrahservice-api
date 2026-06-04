package models

import "time"

// BaggageItemType maps `baggage_item_types` (master picker list).
type BaggageItemType struct {
	ID       uint64 `gorm:"primaryKey"`
	Name     string `gorm:"column:name"`
	Sort     *int   `gorm:"column:sort"`
	IsActive bool   `gorm:"column:is_active"`
}

func (BaggageItemType) TableName() string { return "baggage_item_types" }

// GroupBaggageCount maps `group_baggage_counts`.
type GroupBaggageCount struct {
	ID                uint64     `gorm:"primaryKey"`
	GroupID           uint64     `gorm:"column:group_id"`
	BaggageItemTypeID uint64     `gorm:"column:baggage_item_type_id"`
	Checkpoint        string     `gorm:"column:checkpoint"`
	City              string     `gorm:"column:city"`
	ExpectedCount     int        `gorm:"column:expected_count"`
	Count             int        `gorm:"column:count"`
	Note              *string    `gorm:"column:note"`
	CountedAt         *time.Time `gorm:"column:counted_at"`
	CountedByID       *uint64    `gorm:"column:counted_by_id"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`

	ItemType  *BaggageItemType `gorm:"foreignKey:BaggageItemTypeID"`
	CountedBy *User            `gorm:"foreignKey:CountedByID"`
}

func (GroupBaggageCount) TableName() string { return "group_baggage_counts" }
