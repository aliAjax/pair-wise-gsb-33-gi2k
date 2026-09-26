package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// ArticleDraftRepository persists independent editing drafts.
type ArticleDraftRepository struct {
	db *gorm.DB
}

// NewArticleDraftRepository creates an ArticleDraftRepository.
func NewArticleDraftRepository(db *gorm.DB) *ArticleDraftRepository {
	return &ArticleDraftRepository{db: db}
}

// Create inserts a new draft.
func (r *ArticleDraftRepository) Create(d *model.ArticleDraft) error {
	return r.db.Create(d).Error
}

// Update mutates editable draft fields and (optionally) the base revision
// when the draft is rebased onto a newer online revision.
func (r *ArticleDraftRepository) Update(d *model.ArticleDraft, rebase bool) error {
	values := map[string]interface{}{
		"title":     d.Title,
		"content":   d.Content,
		"cover":     d.Cover,
		"topic_tag": d.TopicTag,
		"saved_at":  d.SavedAt,
	}
	if rebase {
		values["base_revision_no"] = d.BaseRevisionNo
	}
	res := r.db.Model(&model.ArticleDraft{}).Where("id = ? AND user_id = ?", d.ID, d.UserID).
		UpdateColumns(values)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindByID locates a draft by id.
func (r *ArticleDraftRepository) FindByID(id, userID uint) (*model.ArticleDraft, error) {
	var d model.ArticleDraft
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

// FindByArticle locates the draft of an article for a user.
func (r *ArticleDraftRepository) FindByArticle(articleID, userID uint) (*model.ArticleDraft, error) {
	var d model.ArticleDraft
	if err := r.db.Where("article_id = ? AND user_id = ?", articleID, userID).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

// ListByOwner returns all drafts of a user, newest saved first.
func (r *ArticleDraftRepository) ListByOwner(userID uint) ([]model.ArticleDraft, error) {
	var items []model.ArticleDraft
	if err := r.db.Where("user_id = ?", userID).Order("updated_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Delete removes a draft owned by the user.
func (r *ArticleDraftRepository) Delete(id, userID uint) error {
	res := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.ArticleDraft{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByArticle removes the draft of an article (e.g. after publish).
func (r *ArticleDraftRepository) DeleteByArticle(tx *gorm.DB, articleID, userID uint) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Where("article_id = ? AND user_id = ?", articleID, userID).
		Delete(&model.ArticleDraft{}).Error
}
