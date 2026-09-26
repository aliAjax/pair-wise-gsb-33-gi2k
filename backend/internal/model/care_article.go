package model

import "time"

// CareArticle is a gardening knowledge article.
//
// 正文编辑走独立草稿（Draft* 字段），线上版本只在发布时整体切换为草稿快照，
// 因此“正文最后保存时间”和“作者最后保存时间”不会互相覆盖。
//
// PublishedRevision 是当前线上页面修订号（乐观锁）：发布时客户端必须带回
// 打开时看到的修订号；线上已被别人重新发布时发布失败，本地草稿原样保留。
type CareArticle struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Title     string    `gorm:"size:255;not null" json:"title"`
	Content   string    `gorm:"type:longtext" json:"content"`
	Cover     string    `gorm:"size:255" json:"cover"`
	TopicTag  string    `gorm:"size:32;index;not null" json:"topic_tag"`
	Status    string    `gorm:"size:16;default:draft" json:"status"`
	ViewCount int       `gorm:"default:0" json:"view_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 线上页面修订号：每次发布 +1；未发布过为 0。
	PublishedRevision uint       `gorm:"not null;default:0" json:"published_revision"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`

	// 独立草稿：与线上字段（Title/Content/Cover/TopicTag）分开存储。
	DraftTitle    string `gorm:"size:255" json:"draft_title,omitempty"`
	DraftContent  string `gorm:"type:longtext" json:"draft_content,omitempty"`
	DraftCover    string `gorm:"size:255" json:"draft_cover,omitempty"`
	DraftTopicTag string `gorm:"size:32" json:"draft_topic_tag,omitempty"`
	// 草稿所依据的线上修订号，用于发布时的冲突判断。
	DraftBaseRevision uint       `gorm:"not null;default:0" json:"draft_base_revision"`
	HasDraft          bool       `gorm:"not null;default:false" json:"has_draft"`
	DraftSavedAt      *time.Time `json:"draft_saved_at,omitempty"`

	// 修订留档。
	Revisions []ArticleRevision `gorm:"foreignKey:ArticleID;references:ID" json:"-"`
}

// ArticleRevision 是文章每次发布/撤回/恢复时留下的不可变快照。
type ArticleRevision struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	ArticleID uint `gorm:"index;not null" json:"article_id"`
	// Revision 是页面修订号：发布产生 revision=PublishedRevision 的快照；
	// 撤回快照沿用发布时的修订号（可恢复回该版本）。
	Revision     uint      `gorm:"not null;index" json:"revision"`
	Action       string    `gorm:"size:16;not null" json:"action"`
	Title        string    `gorm:"size:255;not null" json:"title"`
	Content      string    `gorm:"type:longtext" json:"content"`
	Cover        string    `gorm:"size:255" json:"cover"`
	TopicTag     string    `gorm:"size:32;not null" json:"topic_tag"`
	OperatorID   uint      `gorm:"not null" json:"operator_id"`
	OperatorName string    `gorm:"size:64" json:"operator_name"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName keeps the revision history table name explicit.
func (ArticleRevision) TableName() string { return "article_revisions" }
