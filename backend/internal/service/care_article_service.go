package service

import (
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// CareArticleService implements care article read/publish-snapshot logic.
// All editing goes through ArticleDraftService; this service never mutates
// online content without appending a revision snapshot.
type CareArticleService struct {
	db           *gorm.DB
	repo         *repository.CareArticleRepository
	revisionRepo *repository.ArticleRevisionRepository
	logger       *slog.Logger
}

// NewCareArticleService creates a CareArticleService.
func NewCareArticleService(db *gorm.DB, repo *repository.CareArticleRepository, revisionRepo *repository.ArticleRevisionRepository, logger *slog.Logger) *CareArticleService {
	return &CareArticleService{db: db, repo: repo, revisionRepo: revisionRepo, logger: logger}
}

// Create immediately publishes an article owned by the current user
// (legacy endpoint). A revision-1 snapshot is recorded so even direct
// publishes enter the revision archive.
func (s *CareArticleService) Create(userID uint, a *model.CareArticle) (*model.CareArticle, error) {
	if !constants.IsValidTopicTag(a.TopicTag) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("CareArticle[topic_tag=%s] create failed: invalid topic tag", a.TopicTag))
	}
	a.UserID = userID
	a.Status = constants.ArticleStatusPublished
	a.RevisionNo = 1
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(a).Error; err != nil {
			return err
		}
		return s.revisionRepo.Create(tx, snapshotOf(a, 1, ""))
	})
	if err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogArticleCreateFailed, a.Title), "error", err)
		return nil, fmt.Errorf("care article create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleCreateSuccess, a.Title), "id", a.ID, "user_id", userID)
	return a, nil
}

// Get returns a published article and increments its view count.
// Withdrawn or never-published articles are invisible to visitors.
func (s *CareArticleService) Get(id uint) (*model.CareArticle, error) {
	a, err := s.repo.FindPublishedByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("CareArticle[id=%d] not found", id))
		}
		return nil, fmt.Errorf("care article get: %w", err)
	}
	if err := s.repo.IncrementView(id); err == nil {
		a.ViewCount++
		s.logger.Info(fmt.Sprintf(constants.LogArticleViewIncremented, id), "id", id)
	}
	return a, nil
}

// Update is the legacy direct-edit endpoint. It keeps the old contract
// (returns the updated article) but never clobbers history: the current
// online state is snapshotted as a new revision first. New clients publish
// drafts via ArticleDraftService.Publish instead.
func (s *CareArticleService) Update(id, userID uint, a *model.CareArticle) (*model.CareArticle, error) {
	exist, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("care article update find: %w", err)
	}
	if exist.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("CareArticle[id=%d] update failed: user_id=%d is not owner", id, userID))
	}
	if a.Title != "" {
		exist.Title = a.Title
	}
	if a.Content != "" {
		exist.Content = a.Content
	}
	if a.Cover != "" {
		exist.Cover = a.Cover
	}
	if a.TopicTag != "" {
		if !constants.IsValidTopicTag(a.TopicTag) {
			return nil, util.NewAppError(422, constants.CodeValidationError, "invalid topic tag")
		}
		exist.TopicTag = a.TopicTag
	}
	next := exist.RevisionNo + 1
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.revisionRepo.Create(tx, snapshotOf(exist, next, "")); err != nil {
			return err
		}
		exist.RevisionNo = next
		exist.Status = constants.ArticleStatusPublished
		return tx.Save(exist).Error
	})
	if err != nil {
		return nil, fmt.Errorf("care article update: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleUpdateSuccess, id), "id", id, "revision_no", next)
	return exist, nil
}

// Delete removes an article together with its drafts and revisions,
// verifying ownership.
func (s *CareArticleService) Delete(id, userID uint) error {
	exist, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("care article delete find: %w", err)
	}
	if exist.UserID != userID {
		return util.NewAppError(403, constants.CodeForbidden, fmt.Sprintf("CareArticle[id=%d] delete failed: not owner", id))
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.CareArticle{}, id).Error; err != nil {
			return err
		}
		if err := tx.Where("article_id = ?", id).Delete(&model.ArticleRevision{}).Error; err != nil {
			return err
		}
		return tx.Where("article_id = ? AND user_id = ?", id, userID).Delete(&model.ArticleDraft{}).Error
	})
	if err != nil {
		return fmt.Errorf("care article delete: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleDeleteSuccess, id), "id", id)
	return nil
}

// List filters published articles by topic tag and keyword.
func (s *CareArticleService) List(topicTag, keyword string, page, pageSize int) ([]model.CareArticle, int64, error) {
	items, total, err := s.repo.List(topicTag, keyword, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("care article list: %w", err)
	}
	return items, total, nil
}

// ListLatest returns the newest published articles for the home page.
func (s *CareArticleService) ListLatest(limit int) ([]model.CareArticle, error) {
	items, err := s.repo.ListLatest(limit)
	if err != nil {
		return nil, fmt.Errorf("care article latest: %w", err)
	}
	return items, nil
}

// snapshotOf builds an immutable revision row from an article state.
func snapshotOf(a *model.CareArticle, revisionNo int, summary string) *model.ArticleRevision {
	return &model.ArticleRevision{
		ArticleID:  a.ID,
		RevisionNo: revisionNo,
		UserID:     a.UserID,
		Title:      a.Title,
		Content:    a.Content,
		Cover:      a.Cover,
		TopicTag:   a.TopicTag,
		Summary:    summary,
	}
}
