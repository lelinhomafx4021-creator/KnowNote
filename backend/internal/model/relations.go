package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Tag struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string    `gorm:"type:uuid;not null;uniqueIndex:idx_tags_user_name,priority:1" json:"-"`
	User      User      `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Name      string    `gorm:"size:50;not null;uniqueIndex:idx_tags_user_name,priority:2" json:"name"`
	Color     string    `gorm:"size:7;not null;default:'#6750A4'" json:"color"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
}

func (t *Tag) BeforeCreate(*gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}

type NotebookTag struct {
	NotebookID string   `gorm:"type:uuid;primaryKey"`
	TagID      string   `gorm:"type:uuid;primaryKey;index"`
	Notebook   Notebook `gorm:"constraint:OnDelete:CASCADE"`
	Tag        Tag      `gorm:"constraint:OnDelete:CASCADE"`
}

// MessageCitation 保存引用快照；来源或分块删除仅断开定位，不丢失历史证据。
type MessageCitation struct {
	ID            string  `gorm:"type:uuid;primaryKey"`
	MessageID     string  `gorm:"type:uuid;not null;uniqueIndex:idx_citations_message_index,priority:1"`
	Message       Message `gorm:"constraint:OnDelete:CASCADE"`
	ChunkID       *string `gorm:"type:uuid;index"`
	Chunk         *Chunk  `gorm:"constraint:OnDelete:SET NULL"`
	SourceID      *string `gorm:"type:uuid;index"`
	Source        *Source `gorm:"constraint:OnDelete:SET NULL"`
	SourceRef     string  `gorm:"not null"`
	SourceTitle   string  `gorm:"not null"`
	Index         int     `gorm:"not null;uniqueIndex:idx_citations_message_index,priority:2;check:index > 0"`
	Quote         string  `gorm:"type:text;not null"`
	Page          *int
	LocationLabel string
}

func (c *MessageCitation) BeforeCreate(*gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	return nil
}
func (c MessageCitation) View() Citation {
	return Citation{Index: c.Index, SourceID: c.SourceRef, SourceTitle: c.SourceTitle, Quote: c.Quote, Page: c.Page, LocationLabel: c.LocationLabel}
}

type LoginLog struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    *string   `gorm:"type:uuid;index" json:"-"`
	User      *User     `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Email     string    `gorm:"size:254;not null" json:"email"`
	Success   bool      `gorm:"not null" json:"success"`
	IP        string    `gorm:"size:45;not null" json:"ip"`
	UserAgent string    `gorm:"size:512;not null" json:"userAgent"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
}

func (l *LoginLog) BeforeCreate(*gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	return nil
}

// SchemaMigration 记录数据迁移版本，确保保留的旧列不会在重启时复活已删除引用。
type SchemaMigration struct {
	Version   string    `gorm:"size:100;primaryKey"`
	AppliedAt time.Time `gorm:"not null"`
}
