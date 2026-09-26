package model

import "time"

// ArticleRevision is an immutable snapshot of an article at the moment it
// was published. Each successful publish appends one row; the row in
// care_articles mirrors the newest revision. Old revisions can be opened
// and restored into a new draft but are never edited in place.
type ArticleRevision struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ArticleID  uint      `gorm:"index:idx_revision_article_no,unique;not null" json:"article_id"`
	RevisionNo int       `gorm:"index:idx_revision_article_no,unique;not null" json:"revision_no"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Title      string    `gorm:"size:255;not null" json:"title"`
	Content    string    `gorm:"type:longtext" json:"content"`
	Cover      string    `gorm:"size:255" json:"cover"`
	TopicTag   string    `gorm:"size:32;not null" json:"topic_tag"`
	Summary    string    `gorm:"size:255" json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName pins the revision table name.
func (ArticleRevision) TableName() string { return "article_revisions" }
