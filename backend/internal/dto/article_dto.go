package dto

import "time"

// ArticleCreateRequest is the legacy payload kept for backward
// compatibility; new clients save drafts instead.
type ArticleCreateRequest struct {
	Title    string `json:"title" binding:"required,max=255"`
	Content  string `json:"content" binding:"required"`
	Cover    string `json:"cover" binding:"omitempty,max=255"`
	TopicTag string `json:"topic_tag" binding:"required"`
}

// ArticleDraftSaveRequest is the payload for creating/saving a draft.
// For a brand-new article draft_id and article_id are empty.
// BaseRevisionNo is the page revision token; it is only honored on the
// first save against an existing article.
type ArticleDraftSaveRequest struct {
	DraftID        uint   `json:"draft_id"`
	ArticleID      uint   `json:"article_id"`
	Title          string `json:"title" binding:"required,max=255"`
	Content        string `json:"content" binding:"required"`
	Cover          string `json:"cover" binding:"omitempty,max=255"`
	TopicTag       string `json:"topic_tag" binding:"required"`
	BaseRevisionNo int    `json:"base_revision_no"`
}

// ArticlePublishRequest publishes a draft. BaseRevisionNo is the page
// revision token: the server rejects the publish when the online article
// has advanced past it, and keeps the draft untouched.
type ArticlePublishRequest struct {
	DraftID        uint   `json:"draft_id" binding:"required"`
	BaseRevisionNo int    `json:"base_revision_no"`
	Summary        string `json:"summary" binding:"omitempty,max=255"`
}

// ArticleRestoreRequest opens a historical revision as a new draft.
type ArticleRestoreRequest struct {
	RevisionNo int `json:"revision_no" binding:"required"`
}

// ArticleDraftResponse is what the editor opens. It embeds either the
// existing draft or a freshly seeded copy of the online article.
type ArticleDraftResponse struct {
	DraftID          uint      `json:"draft_id"`
	ArticleID        uint      `json:"article_id"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	Cover            string    `json:"cover"`
	TopicTag         string    `json:"topic_tag"`
	BaseRevisionNo   int       `json:"base_revision_no"`
	SavedAt          time.Time `json:"saved_at"`
	ArticleStatus    string    `json:"article_status"`
	OnlineRevisionNo int       `json:"online_revision_no"`
	OnlineUpdatedAt  time.Time `json:"online_updated_at"`
	// Conflict is true when the draft is based on an older revision than
	// the online article; the editor must offer re-merging before publish.
	Conflict bool `json:"conflict"`
}

// ArticleDraftItem is one row of the owner's draft list.
type ArticleDraftItem struct {
	DraftID          uint      `json:"draft_id"`
	ArticleID        uint      `json:"article_id"`
	Title            string    `json:"title"`
	Cover            string    `json:"cover"`
	TopicTag         string    `json:"topic_tag"`
	BaseRevisionNo   int       `json:"base_revision_no"`
	SavedAt          time.Time `json:"saved_at"`
	ArticleStatus    string    `json:"article_status"`
	OnlineRevisionNo int       `json:"online_revision_no"`
	Conflict         bool      `json:"conflict"`
}

// ArticleRevisionResponse is a single historical revision.
type ArticleRevisionResponse struct {
	ID         uint      `json:"id"`
	RevisionNo int       `json:"revision_no"`
	UserID     uint      `json:"user_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Cover      string    `json:"cover"`
	TopicTag   string    `json:"topic_tag"`
	Summary    string    `json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
}

// ArticleEditorMeta is the owner-side view of an article plus its draft.
type ArticleEditorMeta struct {
	ID             uint      `json:"id"`
	Title          string    `json:"title"`
	Cover          string    `json:"cover"`
	TopicTag       string    `json:"topic_tag"`
	Status         string    `json:"status"`
	RevisionNo     int       `json:"revision_no"`
	ViewCount      int       `json:"view_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DraftID        uint      `json:"draft_id"`
	DraftSavedAt   time.Time `json:"draft_saved_at"`
	BaseRevisionNo int       `json:"base_revision_no"`
	HasDraft       bool      `json:"has_draft"`
	DraftConflict  bool      `json:"draft_conflict"`
}

// RevisionConflictDetails is returned with code 40901 when publishing
// collides with a newer online revision. The draft is preserved server
// side; these fields let the page prompt for a re-merge.
type RevisionConflictDetails struct {
	DraftID          uint      `json:"draft_id"`
	ArticleID        uint      `json:"article_id"`
	BaseRevisionNo   int       `json:"base_revision_no"`
	OnlineRevisionNo int       `json:"online_revision_no"`
	OnlineTitle      string    `json:"online_title"`
	OnlineUpdatedAt  time.Time `json:"online_updated_at"`
}
