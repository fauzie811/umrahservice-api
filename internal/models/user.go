package models

import (
	"time"

	"gorm.io/datatypes"
)

// User maps the `users` table.
type User struct {
	ID              uint64         `gorm:"primaryKey"`
	Name            string         `gorm:"column:name"`
	Email           string         `gorm:"column:email"`
	Phone           *string        `gorm:"column:phone"`
	Photo           *string        `gorm:"column:photo"`
	Password        string         `gorm:"column:password"`
	StaffID         *uint64        `gorm:"column:staff_id"`
	LastLoginAt     *time.Time     `gorm:"column:last_login_at"`
	LastLoginIP     *string        `gorm:"column:last_login_ip"`
	Meta            datatypes.JSON `gorm:"column:meta"`
	EmailVerifiedAt *time.Time     `gorm:"column:email_verified_at"`
	RememberToken   *string        `gorm:"column:remember_token"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at"`

	Vendors []Vendor `gorm:"many2many:user_vendor;"`
}

func (User) TableName() string { return "users" }

// IsSuperAdmin mirrors User::isSuperAdmin (id == 1).
func (u *User) IsSuperAdmin() bool { return u.ID == 1 }

// Vendor maps the `vendors` table (only fields needed by the API).
type Vendor struct {
	ID          uint64 `gorm:"primaryKey"`
	CompanyName string `gorm:"column:company_name"`
}

func (Vendor) TableName() string { return "vendors" }
