package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// ArticleRevisionRepository stores immutable publish snapshots.
type ArticleRevisionRepository struct {
	db *gorm.DB
}

// NewArticleRevisionRepository creates an ArticleRevisionRepository.
func NewArticleRevisionRepository(db *gorm.DB) *ArticleRevisionRepository {
	return &ArticleRevisionRepository{db: db}
}

// Create appends a revision.
func (r *ArticleRevisionRepository) Create(tx *gorm.DB, rev *model.ArticleRevision) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(rev).Error
}

// FindByNumber locates one revision of an article.
func (r *ArticleRevisionRepository) FindByNumber(articleID uint, revisionNo int) (*model.ArticleRevision, error) {
	var rev model.ArticleRevision
	if err := r.db.Where("article_id = ? AND revision_no = ?", articleID, revisionNo).
		First(&rev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rev, nil
}

// ListByArticle returns all revisions of an article, newest first.
func (r *ArticleRevisionRepository) ListByArticle(articleID uint) ([]model.ArticleRevision, error) {
	var items []model.ArticleRevision
	if err := r.db.Where("article_id = ?", articleID).
		Order("revision_no DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// MaxRevisionNo returns the highest revision number of an article
// (0 when the article has no revision rows yet).
func (r *ArticleRevisionRepository) MaxRevisionNo(tx *gorm.DB, articleID uint) (int, error) {
	db := r.db
	if tx != nil {
		db = tx
	}
	var maxNo *int
	if err := db.Model(&model.ArticleRevision{}).
		Where("article_id = ?", articleID).
		Select("MAX(revision_no)").Row().Scan(&maxNo); err != nil {
		return 0, err
	}
	if maxNo == nil {
		return 0, nil
	}
	return *maxNo, nil
}

// CountByArticle returns how many revisions an article has (for backfill).
func (r *ArticleRevisionRepository) CountByArticle(articleID uint) (int64, error) {
	var n int64
	if err := r.db.Model(&model.ArticleRevision{}).
		Where("article_id = ?", articleID).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}
