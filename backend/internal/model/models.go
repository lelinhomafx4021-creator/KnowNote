package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Base struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
	UpdatedAt time.Time `gorm:"not null" json:"updatedAt"`
}

func (b *Base) BeforeCreate(*gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	return nil
}

type User struct {
	Base
	Email        string `gorm:"size:254;not null;uniqueIndex" json:"email"`
	PasswordHash string `gorm:"not null" json:"-"`
	Nickname     string `gorm:"size:100;not null" json:"nickname"`
	AvatarURL    string `json:"avatarUrl,omitempty"`
}

type UserSettings struct {
	Base
	UserID      string `gorm:"type:uuid;not null;uniqueIndex" json:"-"`
	User        User   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Model       string `gorm:"size:200;not null" json:"model"`
	AnswerStyle string `gorm:"size:20;not null;default:concise" json:"answerStyle"`
	Language    string `gorm:"size:20;not null;default:zh-CN" json:"language"`
}

type Notebook struct {
	Base
	UserID      string `gorm:"type:uuid;not null;index" json:"-"`
	User        User   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Title       string `gorm:"size:200;not null" json:"title"`
	Emoji       string `gorm:"size:32;not null" json:"emoji"`
	Color       int    `gorm:"not null;check:color >= 0 AND color <= 7" json:"color"`
	Description string `gorm:"not null;default:''" json:"description,omitempty"`
}

// NotebookOwner 在数据库层约束子资源必须与笔记本属于同一用户。
type NotebookOwner struct {
	UserID     string   `gorm:"type:uuid;not null;index" json:"-"`
	NotebookID string   `gorm:"type:uuid;not null;index" json:"notebookId"`
	Notebook   Notebook `gorm:"foreignKey:NotebookID,UserID;references:ID,UserID;constraint:OnDelete:CASCADE" json:"-"`
}

type Source struct {
	Base
	NotebookOwner
	Type      string   `gorm:"size:20;not null" json:"type"`
	Title     string   `gorm:"size:500;not null" json:"title"`
	Status    string   `gorm:"size:20;not null;default:processing;index" json:"status"`
	Progress  int      `gorm:"not null;default:0" json:"progress"`
	Size      int64    `gorm:"not null;default:0;check:size >= 0" json:"size"`
	URL       string   `json:"url,omitempty"`
	FilePath  string   `json:"-"`
	Content   string   `gorm:"type:text" json:"content,omitempty"`
	Summary   string   `gorm:"type:text" json:"summary,omitempty"`
	Topics    []string `gorm:"serializer:json;type:jsonb;not null;default:'[]'" json:"topics"`
	WordCount int      `gorm:"not null;default:0" json:"wordCount"`
	Error     string   `gorm:"type:text" json:"error,omitempty"`
}

// Vector 保留 pgvector 的读写能力，仅补充可配置维度的迁移类型。
type Vector struct{ pgvector.Vector }

func NewVector(values []float32) Vector { return Vector{pgvector.NewVector(values)} }
func (Vector) GormDataType() string     { return "vector" }
func (Vector) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	dim, ok := db.Get("embedding_dimension")
	if !ok {
		dim = 1536
	}
	return fmt.Sprintf("vector(%d)", dim)
}

type Chunk struct {
	Base
	NotebookOwner
	SourceID   string `gorm:"type:uuid;not null;uniqueIndex:idx_chunks_source_position,priority:1" json:"sourceId"`
	Source     Source `gorm:"foreignKey:SourceID,NotebookID,UserID;references:ID,NotebookID,UserID;constraint:OnDelete:CASCADE" json:"-"`
	ChunkIndex int    `gorm:"not null;uniqueIndex:idx_chunks_source_position,priority:2" json:"chunkIndex"`
	Content    string `gorm:"type:text;not null" json:"content"`
	TokenCount int    `gorm:"not null;default:0" json:"tokenCount"`
	Embedding  Vector `gorm:"not null" json:"-"`
}

type Citation struct {
	Index       int    `json:"index"`
	SourceID    string `json:"sourceId"`
	SourceTitle string `json:"sourceTitle"`
	Quote       string `json:"quote"`
}

type Message struct {
	Base
	NotebookOwner
	Role      string     `gorm:"size:20;not null" json:"role"`
	Content   string     `gorm:"type:text;not null" json:"content"`
	Citations []Citation `gorm:"serializer:json;type:jsonb;not null;default:'[]'" json:"citations,omitempty"`
	Feedback  *string    `gorm:"size:10" json:"feedback,omitempty"`
}

type Note struct {
	Base
	NotebookOwner
	Title   string `gorm:"size:500;not null" json:"title"`
	Content string `gorm:"type:text;not null" json:"content"`
	Origin  string `gorm:"size:20;not null;default:manual" json:"origin"`
}

type MindMapNode struct {
	ID       string        `json:"id"`
	Label    string        `json:"label"`
	Children []MindMapNode `json:"children,omitempty"`
}

type Artifact struct {
	Base
	NotebookOwner
	Kind    string       `gorm:"size:30;not null" json:"kind"`
	Title   string       `gorm:"size:500;not null" json:"title"`
	Content string       `gorm:"type:text" json:"content,omitempty"`
	Mindmap *MindMapNode `gorm:"serializer:json;type:jsonb" json:"mindmap,omitempty"`
}

type AudioOverview struct {
	Base
	NotebookOwner
	Status      string `gorm:"size:20;not null;default:idle" json:"status"`
	Title       string `gorm:"size:500" json:"title,omitempty"`
	DurationSec int    `gorm:"not null;default:0" json:"durationSec,omitempty"`
	URL         string `json:"url,omitempty"`
	FilePath    string `json:"-"`
	Transcript  string `gorm:"type:text" json:"transcript,omitempty"`
	Error       string `gorm:"type:text" json:"error,omitempty"`
}

type NotebookGuide struct {
	Base
	NotebookOwner
	Summary            string   `gorm:"type:text;not null;default:''" json:"summary"`
	SuggestedQuestions []string `gorm:"serializer:json;type:jsonb;not null;default:'[]'" json:"suggestedQuestions"`
	Stale              bool     `gorm:"not null;default:true" json:"-"`
}

func AllModels() []any {
	return []any{&User{}, &UserSettings{}, &Notebook{}, &Source{}, &Chunk{}, &Message{}, &Note{}, &Artifact{}, &AudioOverview{}, &NotebookGuide{}}
}
