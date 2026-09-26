package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// CareArticleRepository handles persistence of care articles.
type CareArticleRepository struct {
	db *gorm.DB
}

// NewCareArticleRepository creates a CareArticleRepository.
func NewCareArticleRepository(db *gorm.DB) *CareArticleRepository {
	return &CareArticleRepository{db: db}
}

// Create inserts an article.
func (r *CareArticleRepository) Create(a *model.CareArticle) error {
	return r.db.Create(a).Error
}

// FindByID locates an article by id.
func (r *CareArticleRepository) FindByID(id uint) (*model.CareArticle, error) {
	var a model.CareArticle
	if err := r.db.First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// Update persists an article.
func (r *CareArticleRepository) Update(a *model.CareArticle) error {
	return r.db.Save(a).Error
}

// Delete removes an article by id together with its revision archive.
func (r *CareArticleRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 先删修订留档，再删文章本身（外键约束下顺序不能反）。
		if err := tx.Where("article_id = ?", id).Delete(&model.ArticleRevision{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&model.CareArticle{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// IncrementView bumps the view count of an article.
func (r *CareArticleRepository) IncrementView(id uint) error {
	return r.db.Model(&model.CareArticle{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// List filters visible (published) articles by topic tag and keyword with pagination.
func (r *CareArticleRepository) List(topicTag, keyword string, page, pageSize int) ([]model.CareArticle, int64, error) {
	var items []model.CareArticle
	var total int64
	q := r.db.Model(&model.CareArticle{}).Where("status = ?", constants.ArticleStatusPublished)
	if topicTag != "" {
		q = q.Where("topic_tag = ?", topicTag)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("title LIKE ? OR content LIKE ?", like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("published_revision DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListLatest returns the latest published articles for the home page.
func (r *CareArticleRepository) ListLatest(limit int) ([]model.CareArticle, error) {
	var items []model.CareArticle
	if err := r.db.Where("status = ?", constants.ArticleStatusPublished).
		Order("published_revision DESC, id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListByOwner returns every article owned by the user, including drafts and
// withdrawn ones, newest activity first. Used by the author's own management view.
func (r *CareArticleRepository) ListByOwner(userID uint, page, pageSize int) ([]model.CareArticle, int64, error) {
	var items []model.CareArticle
	var total int64
	q := r.db.Model(&model.CareArticle{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// SaveDraft overwrites the independent draft columns without touching the
// published columns. Only owners call this (ownership checked in service).
func (r *CareArticleRepository) SaveDraft(a *model.CareArticle) error {
	updates := map[string]interface{}{
		"draft_title":         a.DraftTitle,
		"draft_content":       a.DraftContent,
		"draft_cover":         a.DraftCover,
		"draft_topic_tag":     a.DraftTopicTag,
		"draft_base_revision": a.DraftBaseRevision,
		"has_draft":           true,
		"draft_saved_at":      gorm.Expr("CURRENT_TIMESTAMP(3)"),
	}
	res := r.db.Model(&model.CareArticle{}).Where("id = ?", a.ID).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return r.db.First(a, a.ID).Error
}

// PublishDraft atomically turns the draft into the live snapshot.
//
// It is an optimistic compare-and-swap on published_revision: when the live
// revision no longer equals baseRevision (someone else republished in the
// meantime), ErrConflict is returned and the draft is left untouched.
func (r *CareArticleRepository) PublishDraft(id, baseRevision, operatorID uint, operatorName string) (*model.CareArticle, error) {
	var result model.CareArticle
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var a model.CareArticle
		if err := tx.First(&a, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if !a.HasDraft {
			return ErrConflict // nothing to publish
		}
		if a.PublishedRevision != baseRevision {
			return ErrConflict // stale draft: live version was republished
		}

		newRevision := a.PublishedRevision + 1
		now := time.Now()
		if err := tx.Model(&a).Updates(map[string]interface{}{
			"title":              a.DraftTitle,
			"content":            a.DraftContent,
			"cover":              a.DraftCover,
			"topic_tag":          a.DraftTopicTag,
			"status":             constants.ArticleStatusPublished,
			"published_revision": newRevision,
			"published_at":       now,
			"has_draft":          false,
			"draft_saved_at":     nil,
		}).Error; err != nil {
			return err
		}

		rev := &model.ArticleRevision{
			ArticleID:    a.ID,
			Revision:     newRevision,
			Action:       constants.RevisionActionPublish,
			Title:        a.DraftTitle,
			Content:      a.DraftContent,
			Cover:        a.DraftCover,
			TopicTag:     a.DraftTopicTag,
			OperatorID:   operatorID,
			OperatorName: operatorName,
			CreatedAt:    now,
		}
		if err := tx.Create(rev).Error; err != nil {
			return err
		}
		return tx.First(&result, id).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Offline unpublishes the article (visitors can no longer see it) while
// keeping the article row, its draft and all revisions.
func (r *CareArticleRepository) Offline(id, operatorID uint, operatorName string) (*model.CareArticle, error) {
	var out model.CareArticle
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var a model.CareArticle
		if err := tx.First(&a, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if a.Status != constants.ArticleStatusPublished {
			return ErrConflict
		}
		now := time.Now()
		if err := tx.Model(&a).Updates(map[string]interface{}{
			"status":       constants.ArticleStatusOffline,
			"published_at": nil,
		}).Error; err != nil {
			return err
		}
		if a.PublishedRevision > 0 {
			rev := &model.ArticleRevision{
				ArticleID:    a.ID,
				Revision:     a.PublishedRevision,
				Action:       constants.RevisionActionOffline,
				Title:        a.Title,
				Content:      a.Content,
				Cover:        a.Cover,
				TopicTag:     a.TopicTag,
				OperatorID:   operatorID,
				OperatorName: operatorName,
				CreatedAt:    now,
			}
			if err := tx.Create(rev).Error; err != nil {
				return err
			}
		}
		return tx.First(&out, id).Error
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// RestoreRevision copies a historical revision's content into a new draft,
// without changing the live version. The draft's base revision is the current
// live revision, so a later publish still goes through conflict detection.
func (r *CareArticleRepository) RestoreRevision(articleID, revisionID, operatorID uint, operatorName string) (*model.CareArticle, error) {
	var out model.CareArticle
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var rev model.ArticleRevision
		if err := tx.Where("id = ? AND article_id = ?", revisionID, articleID).First(&rev).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		var a model.CareArticle
		if err := tx.First(&a, articleID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if err := tx.Model(&a).Updates(map[string]interface{}{
			"draft_title":         rev.Title,
			"draft_content":       rev.Content,
			"draft_cover":         rev.Cover,
			"draft_topic_tag":     rev.TopicTag,
			"draft_base_revision": a.PublishedRevision,
			"has_draft":           true,
			"draft_saved_at":      gorm.Expr("CURRENT_TIMESTAMP(3)"),
		}).Error; err != nil {
			return err
		}
		audit := &model.ArticleRevision{
			ArticleID:    a.ID,
			Revision:     rev.Revision,
			Action:       constants.RevisionActionRestore,
			Title:        rev.Title,
			Content:      rev.Content,
			Cover:        rev.Cover,
			TopicTag:     rev.TopicTag,
			OperatorID:   operatorID,
			OperatorName: operatorName,
			CreatedAt:    time.Now(),
		}
		if err := tx.Create(audit).Error; err != nil {
			return err
		}
		return tx.First(&out, articleID).Error
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListRevisions returns the revision history of an article, newest first.
func (r *CareArticleRepository) ListRevisions(articleID uint) ([]model.ArticleRevision, error) {
	var items []model.ArticleRevision
	if err := r.db.Where("article_id = ?", articleID).
		Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// FindRevision locates one revision by id (must belong to the given article).
func (r *CareArticleRepository) FindRevision(articleID, revisionID uint) (*model.ArticleRevision, error) {
	var rev model.ArticleRevision
	if err := r.db.Where("id = ? AND article_id = ?", revisionID, articleID).First(&rev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rev, nil
}
