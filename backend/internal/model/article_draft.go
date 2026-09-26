package model

import "time"

// ArticleDraft is the independent editing copy of an article. There is at
// most one draft per (article, user); a draft for a brand-new article has
// ArticleID = NULL until it is published (MySQL allows multiple NULLs in
// the unique index, so a user may have several in-progress new drafts).
//
// BaseRevisionNo records the online revision the draft was opened against.
// It is deliberately frozen while the draft is edited: publishing fails
// when the online article has advanced past it, so a concurrent
// republication can never be silently overwritten.
type ArticleDraft struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ArticleID      *uint     `gorm:"index:idx_draft_article_user,unique" json:"article_id"`
	UserID         uint      `gorm:"index:idx_draft_article_user,unique;not null" json:"user_id"`
	Title          string    `gorm:"size:255;not null" json:"title"`
	Content        string    `gorm:"type:longtext" json:"content"`
	Cover          string    `gorm:"size:255" json:"cover"`
	TopicTag       string    `gorm:"size:32;not null" json:"topic_tag"`
	BaseRevisionNo int       `gorm:"not null;default:0" json:"base_revision_no"`
	SavedAt        time.Time `json:"saved_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TableName pins the draft table name.
func (ArticleDraft) TableName() string { return "article_drafts" }
