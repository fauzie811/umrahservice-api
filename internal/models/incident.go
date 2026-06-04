package models

import "time"

// Incident maps `incidents`.
type Incident struct {
	ID              uint64     `gorm:"primaryKey"`
	GroupID         *uint64    `gorm:"column:group_id"`
	GroupTaskID     *uint64    `gorm:"column:group_task_id"`
	ReportedByID    *uint64    `gorm:"column:reported_by_id"`
	AssignedToID    *uint64    `gorm:"column:assigned_to_id"`
	OccurredAt      *time.Time `gorm:"column:occurred_at"`
	Title           string     `gorm:"column:title"`
	Category        *string    `gorm:"column:category"`
	Severity        *string    `gorm:"column:severity"`
	Status          *string    `gorm:"column:status"`
	Description     *string    `gorm:"column:description"`
	ResolutionNotes *string    `gorm:"column:resolution_notes"`
	ResolvedAt      *time.Time `gorm:"column:resolved_at"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`

	Group      *Group     `gorm:"foreignKey:GroupID"`
	GroupTask  *GroupTask `gorm:"foreignKey:GroupTaskID"`
	ReportedBy *User      `gorm:"foreignKey:ReportedByID"`
	AssignedTo *User      `gorm:"foreignKey:AssignedToID"`
}

func (Incident) TableName() string { return "incidents" }

// Message maps `messages` (polymorphic via messageable_type/id).
type Message struct {
	ID              uint64    `gorm:"primaryKey"`
	MessageableType string    `gorm:"column:messageable_type"`
	MessageableID   uint64    `gorm:"column:messageable_id"`
	UserID          *uint64   `gorm:"column:user_id"`
	Body            string    `gorm:"column:body"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`

	User *User `gorm:"foreignKey:UserID"`
}

func (Message) TableName() string { return "messages" }

// Laravel morph-map aliases stored in messageable_type / source_type
// (Relation::enforceMorphMap in AppServiceProvider).
const (
	MorphGroupTask = "group_task"
	MorphIncident  = "incident"
)
