package models

import "time"

// PersonalAccessToken maps Sanctum's `personal_access_tokens` table.
type PersonalAccessToken struct {
	ID            uint64     `gorm:"primaryKey"`
	TokenableType string     `gorm:"column:tokenable_type"`
	TokenableID   uint64     `gorm:"column:tokenable_id"`
	Name          string     `gorm:"column:name"`
	Token         string     `gorm:"column:token"` // sha256 hex of the plaintext
	Abilities     *string    `gorm:"column:abilities"`
	LastUsedAt    *time.Time `gorm:"column:last_used_at"`
	ExpiresAt     *time.Time `gorm:"column:expires_at"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (PersonalAccessToken) TableName() string { return "personal_access_tokens" }

// Tokenable morph value used by the User model.
const TokenableUser = "App\\Models\\User"

// Role maps Spatie's `roles` table.
type Role struct {
	ID        uint64 `gorm:"primaryKey"`
	Name      string `gorm:"column:name"`
	GuardName string `gorm:"column:guard_name"`
}

func (Role) TableName() string { return "roles" }

// Permission maps Spatie's `permissions` table.
type Permission struct {
	ID        uint64 `gorm:"primaryKey"`
	Name      string `gorm:"column:name"`
	GuardName string `gorm:"column:guard_name"`
}

func (Permission) TableName() string { return "permissions" }

// Notification maps Laravel's `notifications` table (uuid id).
type Notification struct {
	ID             string     `gorm:"column:id;primaryKey"`
	Type           string     `gorm:"column:type"`
	NotifiableType string     `gorm:"column:notifiable_type"`
	NotifiableID   uint64     `gorm:"column:notifiable_id"`
	Data           string     `gorm:"column:data"`
	ReadAt         *time.Time `gorm:"column:read_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (Notification) TableName() string { return "notifications" }

// Service maps `services` (joined via group_service for ServiceType filtering).
type Service struct {
	ID   uint64 `gorm:"primaryKey"`
	Type string `gorm:"column:type"`
	Name string `gorm:"column:name"`
}

func (Service) TableName() string { return "services" }
