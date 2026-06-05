package models

import "time"

// GroupTaskChecklistItem maps `group_task_checklist_items`.
type GroupTaskChecklistItem struct {
	ID            uint64     `gorm:"primaryKey"`
	GroupTaskID   uint64     `gorm:"column:group_task_id"`
	Label         string     `gorm:"column:label"`
	Done          bool       `gorm:"column:done"`
	PhotoRequired bool       `gorm:"column:photo_required"`
	Photo         *string    `gorm:"column:photo"`
	DoneAt        *time.Time `gorm:"column:done_at"`
	Sort          int        `gorm:"column:sort"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (GroupTaskChecklistItem) TableName() string { return "group_task_checklist_items" }
