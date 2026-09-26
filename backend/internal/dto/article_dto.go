package dto

import "time"

// ArticleCreateRequest is the payload for creating an article.
// 新文章先落为独立草稿，不直接覆盖线上（此时还没有线上版本）。
type ArticleCreateRequest struct {
	Title    string `json:"title" binding:"required,max=255"`
	Content  string `json:"content" binding:"required"`
	Cover    string `json:"cover" binding:"omitempty,max=255"`
	TopicTag string `json:"topic_tag" binding:"required"`
}

// ArticleDraftSaveRequest saves the independent draft.
// BaseRevision 是编辑开始时页面上的修订号，原样存回草稿，发布时用于冲突判断。
type ArticleDraftSaveRequest struct {
	Title        string `json:"title" binding:"required,max=255"`
	Content      string `json:"content" binding:"required"`
	Cover        string `json:"cover" binding:"omitempty,max=255"`
	TopicTag     string `json:"topic_tag" binding:"required"`
	BaseRevision uint   `json:"base_revision"`
}

// ArticlePublishRequest publishes the draft.
// BaseRevision 必须等于当前线上修订号；线上已被别人重新发布时返回 409，
// 草稿不会被覆盖。
type ArticlePublishRequest struct {
	BaseRevision *uint `json:"base_revision"`
}

// ArticlePublicView is what visitors and lists see: 只有线上快照，不含草稿。
type ArticlePublicView struct {
	ID                uint       `json:"id"`
	UserID            uint       `json:"user_id"`
	Title             string     `json:"title"`
	Content           string     `json:"content"`
	Cover             string     `json:"cover"`
	TopicTag          string     `json:"topic_tag"`
	Status            string     `json:"status"`
	ViewCount         int        `json:"view_count"`
	PublishedRevision uint       `json:"published_revision"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
}

// ArticleDraftView is the editor-side draft payload.
type ArticleDraftView struct {
	Title        string     `json:"title"`
	Content      string     `json:"content"`
	Cover        string     `json:"cover"`
	TopicTag     string     `json:"topic_tag"`
	BaseRevision uint       `json:"base_revision"`
	HasDraft     bool       `json:"has_draft"`
	SavedAt      *time.Time `json:"saved_at,omitempty"`
}

// ArticleOwnerView is returned to the author when opening an article to edit.
// 线上快照与独立草稿分开返回；Conflict 表示草稿基线已落后于线上，
// 前端需要先引导重新合并再发布。
type ArticleOwnerView struct {
	ArticlePublicView
	Draft    ArticleDraftView `json:"draft"`
	Conflict bool             `json:"conflict"`
}

// ArticleRevisionView is one immutable history snapshot.
type ArticleRevisionView struct {
	ID           uint      `json:"id"`
	Revision     uint      `json:"revision"`
	Action       string    `json:"action"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	Cover        string    `json:"cover"`
	TopicTag     string    `json:"topic_tag"`
	OperatorID   uint      `json:"operator_id"`
	OperatorName string    `json:"operator_name"`
	CreatedAt    time.Time `json:"created_at"`
}
