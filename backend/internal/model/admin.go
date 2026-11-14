package model

import "time"

type SystemSetting struct {
	Key       string    `gorm:"size:100;primaryKey" json:"-"`
	Value     string    `gorm:"type:text;not null" json:"-"`
	IsSecret  bool      `gorm:"not null;default:false" json:"-"`
	UpdatedBy *string   `gorm:"type:uuid;index" json:"-"`
	User      *User     `gorm:"foreignKey:UpdatedBy;constraint:OnDelete:SET NULL" json:"-"`
	UpdatedAt time.Time `gorm:"not null" json:"-"`
}

type AuditLog struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    *string   `gorm:"type:uuid;index" json:"userId"`
	User      *User     `gorm:"constraint:OnDelete:SET NULL" json:"-"`
	Action    string    `gorm:"size:100;not null" json:"action"`
	Target    string    `gorm:"size:200;not null" json:"target"`
	Detail    string    `gorm:"type:text;not null" json:"detail"`
	CreatedAt time.Time `gorm:"not null;index" json:"createdAt"`
}
