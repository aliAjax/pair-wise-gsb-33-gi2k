package model

import "time"

// CareArticle is a gardening knowledge article. The row always holds the
// currently online snapshot (latest published revision); work in progress
// lives in ArticleDraft and never mutates this row directly.
type CareArticle struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Title      string    `gorm:"size:255;not null" json:"title"`
	Content    string    `gorm:"type:longtext" json:"content"`
	Cover      string    `gorm:"size:255" json:"cover"`
	TopicTag   string    `gorm:"size:32;index;not null" json:"topic_tag"`
	Status     string    `gorm:"size:16;default:published" json:"status"`
	RevisionNo int       `gorm:"not null;default:0" json:"revision_no"`
	ViewCount  int       `gorm:"default:0" json:"view_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName keeps the legacy table name after the model grew revision fields.
func (CareArticle) TableName() string { return "care_articles" }
