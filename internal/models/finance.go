package models

import (
	"time"

	"gorm.io/datatypes"
)

// UserCash maps `user_cashes`.
type UserCash struct {
	ID           uint64         `gorm:"primaryKey"`
	UserID       uint64         `gorm:"column:user_id"`
	GroupID      *uint64        `gorm:"column:group_id"`
	CategoryID   *uint64        `gorm:"column:category_id"`
	ToUserID     *uint64        `gorm:"column:to_user_id"`
	CashedAt     *time.Time     `gorm:"column:cashed_at"`
	Type         string         `gorm:"column:type"` // d|c|t
	Amount       float64        `gorm:"column:amount"`
	AmountC      float64        `gorm:"column:amount_c"`
	Currency     string         `gorm:"column:currency"`
	ExchangeRate *float64       `gorm:"column:exchange_rate"`
	Details      *string        `gorm:"column:details"`
	Attachments  datatypes.JSON `gorm:"column:attachments"`
	CreatedAt    time.Time      `gorm:"column:created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at"`

	Group    *Group        `gorm:"foreignKey:GroupID"`
	Category *CashCategory `gorm:"foreignKey:CategoryID"`
}

func (UserCash) TableName() string { return "user_cashes" }

// CashCategory maps `cash_categories` (self-referential parent/children).
type CashCategory struct {
	ID       uint64  `gorm:"primaryKey"`
	ParentID *uint64 `gorm:"column:parent_id"`
	Group    *string `gorm:"column:group"`
	Type     string  `gorm:"column:type"`
	Name     string  `gorm:"column:name"`

	Parent   *CashCategory  `gorm:"foreignKey:ParentID"`
	Children []CashCategory `gorm:"foreignKey:ParentID"`
}

func (CashCategory) TableName() string { return "cash_categories" }

// FullName mirrors CashCategory::fullName ("parent → name" or "name").
func (c *CashCategory) FullName() string {
	if c.Parent != nil {
		return c.Parent.Name + " → " + c.Name
	}
	return c.Name
}
