package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// errStalePublish is the in-transaction sentinel for an optimistic-lock
// failure; it is translated into a 409 AppError by Publish.
var errStalePublish = errors.New("online revision advanced during publish")

// lockingClause returns a SELECT ... FOR UPDATE row lock (MySQL/InnoDB).
func lockingClause() clause.Expression {
	return clause.Locking{Strength: "UPDATE"}
}

// ArticleDraftService owns the editing workflow: independent drafts,
// page revision tokens, optimistic publish, withdrawal and revision
// restore. It is the only place allowed to turn draft content into a new
// online revision.
type ArticleDraftService struct {
	db           *gorm.DB
	articleRepo  *repository.CareArticleRepository
	draftRepo    *repository.ArticleDraftRepository
	revisionRepo *repository.ArticleRevisionRepository
	logger       *slog.Logger
}

// NewArticleDraftService creates an ArticleDraftService.
func NewArticleDraftService(db *gorm.DB, articleRepo *repository.CareArticleRepository, draftRepo *repository.ArticleDraftRepository, revisionRepo *repository.ArticleRevisionRepository, logger *slog.Logger) *ArticleDraftService {
	return &ArticleDraftService{db: db, articleRepo: articleRepo, draftRepo: draftRepo, revisionRepo: revisionRepo, logger: logger}
}

// Open returns what the editor page needs for an article: the existing
// independent draft if there is one, otherwise a seed copied from the
// online snapshot. The seed is not persisted before the first save.
// articleID 0 returns an empty seed for a brand-new article.
func (s *ArticleDraftService) Open(userID, articleID uint) (*dto.ArticleDraftResponse, error) {
	if articleID == 0 {
		return &dto.ArticleDraftResponse{}, nil
	}
	article, err := s.loadOwnedArticle(articleID, userID)
	if err != nil {
		return nil, err
	}
	draft, err := s.draftRepo.FindByArticle(articleID, userID)
	switch {
	case err == nil:
		return s.buildDraftResponse(draft, article), nil
	case errors.Is(err, repository.ErrNotFound):
		// Seed from the online row; no row persisted until the first save.
		return &dto.ArticleDraftResponse{
			ArticleID:        article.ID,
			Title:            article.Title,
			Content:          article.Content,
			Cover:            article.Cover,
			TopicTag:         article.TopicTag,
			BaseRevisionNo:   article.RevisionNo,
			ArticleStatus:    article.Status,
			OnlineRevisionNo: article.RevisionNo,
			OnlineUpdatedAt:  article.UpdatedAt,
		}, nil
	default:
		return nil, fmt.Errorf("article draft open: %w", err)
	}
}

// GetDraft returns a persisted draft together with online comparison meta.
func (s *ArticleDraftService) GetDraft(draftID, userID uint) (*dto.ArticleDraftResponse, error) {
	draft, err := s.draftRepo.FindByID(draftID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("ArticleDraft[id=%d] not found", draftID))
		}
		return nil, fmt.Errorf("article draft get: %w", err)
	}
	if draft.ArticleID == nil {
		return s.buildDraftResponse(draft, nil), nil
	}
	article, err := s.articleRepo.FindByID(*draft.ArticleID)
	if err != nil {
		return nil, fmt.Errorf("article draft get article: %w", err)
	}
	return s.buildDraftResponse(draft, article), nil
}

// Save stores editor content into the independent draft. The page
// revision token (base_revision_no) is only honored when the draft is
// first created; afterwards it is frozen server side and moves solely
// through an explicit Reconcile after a conflict.
func (s *ArticleDraftService) Save(userID uint, req dto.ArticleDraftSaveRequest) (*dto.ArticleDraftResponse, error) {
	if !constants.IsValidTopicTag(req.TopicTag) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("ArticleDraft[topic_tag=%s] save failed: invalid topic tag", req.TopicTag))
	}

	// Updating an existing draft never changes its revision base.
	if req.DraftID != 0 {
		draft, err := s.draftRepo.FindByID(req.DraftID, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("ArticleDraft[id=%d] not found", req.DraftID))
			}
			return nil, fmt.Errorf("article draft save find: %w", err)
		}
		draft.Title, draft.Content, draft.Cover, draft.TopicTag = req.Title, req.Content, req.Cover, req.TopicTag
		draft.SavedAt = time.Now()
		if err := s.draftRepo.Update(draft, false); err != nil {
			return nil, fmt.Errorf("article draft save: %w", err)
		}
		s.logger.Info(fmt.Sprintf(constants.LogArticleDraftSaved, articleRef(draft.ArticleID), draft.ID, userID))
		return s.respondDraft(draft)
	}

	// First save of a brand-new article draft.
	if req.ArticleID == 0 {
		draft := &model.ArticleDraft{
			ArticleID: nil, UserID: userID,
			Title: req.Title, Content: req.Content, Cover: req.Cover, TopicTag: req.TopicTag,
			BaseRevisionNo: 0, SavedAt: time.Now(),
		}
		if err := s.draftRepo.Create(draft); err != nil {
			return nil, fmt.Errorf("article draft create: %w", err)
		}
		s.logger.Info(fmt.Sprintf(constants.LogArticleDraftSaved, 0, draft.ID, userID))
		return s.buildDraftResponse(draft, nil), nil
	}

	// First save of a draft for an existing article: the page token must
	// still match the online revision, otherwise someone published while
	// the editor was open and we must not silently fork from stale content.
	article, err := s.loadOwnedArticle(req.ArticleID, userID)
	if err != nil {
		return nil, err
	}
	if req.BaseRevisionNo != article.RevisionNo {
		return nil, s.staleDraftError(&model.ArticleDraft{ArticleID: &article.ID, UserID: userID, BaseRevisionNo: req.BaseRevisionNo}, article)
	}
	if existing, err := s.draftRepo.FindByArticle(req.ArticleID, userID); err == nil {
		// Another session opened the same draft: save into it, keep base.
		existing.Title, existing.Content, existing.Cover, existing.TopicTag = req.Title, req.Content, req.Cover, req.TopicTag
		existing.SavedAt = time.Now()
		if err := s.draftRepo.Update(existing, false); err != nil {
			return nil, fmt.Errorf("article draft save: %w", err)
		}
		s.logger.Info(fmt.Sprintf(constants.LogArticleDraftSaved, req.ArticleID, existing.ID, userID))
		return s.buildDraftResponse(existing, article), nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("article draft save lookup: %w", err)
	}

	draft := &model.ArticleDraft{
		ArticleID: &article.ID, UserID: userID,
		Title: req.Title, Content: req.Content, Cover: req.Cover, TopicTag: req.TopicTag,
		BaseRevisionNo: article.RevisionNo, SavedAt: time.Now(),
	}
	if err := s.draftRepo.Create(draft); err != nil {
		return nil, fmt.Errorf("article draft create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleDraftSaved, req.ArticleID, draft.ID, userID))
	return s.buildDraftResponse(draft, article), nil
}

// Discard drops an independent draft without touching the article.
func (s *ArticleDraftService) Discard(draftID, userID uint) error {
	if err := s.draftRepo.Delete(draftID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("ArticleDraft[id=%d] not found", draftID))
		}
		return fmt.Errorf("article draft discard: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleDraftDiscarded, draftID, userID))
	return nil
}

// Publish turns a draft into the next online revision. It is guarded by
// an optimistic compare on the page revision token: when the online
// article has been republished since the draft was opened/re-based, the
// publish is rejected with code 40901 and the draft is kept intact.
func (s *ArticleDraftService) Publish(userID uint, req dto.ArticlePublishRequest) (*model.CareArticle, error) {
	draft, err := s.draftRepo.FindByID(req.DraftID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("ArticleDraft[id=%d] not found", req.DraftID))
		}
		return nil, fmt.Errorf("article publish draft find: %w", err)
	}
	if !constants.IsValidTopicTag(draft.TopicTag) {
		return nil, util.NewAppError(422, constants.CodeValidationError, "invalid topic tag")
	}

	// Brand-new article: create online row + revision 1, then drop draft.
	if draft.ArticleID == nil {
		article := &model.CareArticle{
			UserID: userID, Title: draft.Title, Content: draft.Content,
			Cover: draft.Cover, TopicTag: draft.TopicTag,
			Status: constants.ArticleStatusPublished, RevisionNo: 1,
		}
		err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(article).Error; err != nil {
				return err
			}
			return s.revisionRepo.Create(tx, snapshotOf(article, 1, req.Summary))
		})
		if err != nil {
			return nil, fmt.Errorf("article publish create: %w", err)
		}
		// Remove only this exact draft (article_id NULL is shared by every
		// brand-new draft, so DeleteByArticle must not be used here).
		if err := s.draftRepo.Delete(draft.ID, userID); err != nil {
			return nil, fmt.Errorf("article publish draft cleanup: %w", err)
		}
		s.logger.Info(fmt.Sprintf(constants.LogArticlePublished, article.ID, 1, userID))
		return article, nil
	}

	articleID := *draft.ArticleID
	article, err := s.loadOwnedArticle(articleID, userID)
	if err != nil {
		return nil, err
	}

	var published *model.CareArticle
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Row lock + recheck closes the race with a concurrent publish.
		var current model.CareArticle
		if err := tx.Clauses(lockingClause()).Where("id = ?", articleID).First(&current).Error; err != nil {
			return err
		}
		if current.RevisionNo != draft.BaseRevisionNo {
			return errStalePublish
		}
		next := current.RevisionNo + 1
		rev := &model.ArticleRevision{
			ArticleID: current.ID, RevisionNo: next, UserID: userID,
			Title: draft.Title, Content: draft.Content, Cover: draft.Cover,
			TopicTag: draft.TopicTag, Summary: req.Summary,
		}
		if err := s.revisionRepo.Create(tx, rev); err != nil {
			return err
		}
		current.Title, current.Content, current.Cover, current.TopicTag = draft.Title, draft.Content, draft.Cover, draft.TopicTag
		current.RevisionNo = next
		// Publishing an edit also brings a withdrawn article back online.
		current.Status = constants.ArticleStatusPublished
		if err := tx.Save(&current).Error; err != nil {
			return err
		}
		if err := s.draftRepo.DeleteByArticle(tx, current.ID, userID); err != nil {
			return err
		}
		published = &current
		return nil
	})
	if err != nil {
		if errors.Is(err, errStalePublish) {
			fresh, findErr := s.articleRepo.FindByID(articleID)
			s.logger.Warn(fmt.Sprintf(constants.LogArticlePublishStale, articleID, draft.BaseRevisionNo, draft.BaseRevisionNo, userID))
			if findErr == nil {
				return nil, s.staleDraftError(draft, fresh)
			}
			return nil, s.staleDraftError(draft, article)
		}
		return nil, fmt.Errorf("article publish: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticlePublished, published.ID, published.RevisionNo, userID))
	return published, nil
}

// Reconcile re-bases a draft onto the current online revision after the
// editor has merged the newer published content into the draft. Draft
// content is preserved; only the revision token advances.
func (s *ArticleDraftService) Reconcile(draftID, userID uint) (*dto.ArticleDraftResponse, error) {
	draft, err := s.draftRepo.FindByID(draftID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("ArticleDraft[id=%d] not found", draftID))
		}
		return nil, fmt.Errorf("article reconcile draft find: %w", err)
	}
	if draft.ArticleID == nil {
		return nil, util.NewAppError(400, constants.CodeBadRequest, "new article draft has no online revision to merge")
	}
	article, err := s.loadOwnedArticle(*draft.ArticleID, userID)
	if err != nil {
		return nil, err
	}
	draft.BaseRevisionNo = article.RevisionNo
	draft.SavedAt = time.Now()
	if err := s.draftRepo.Update(draft, true); err != nil {
		return nil, fmt.Errorf("article reconcile: %w", err)
	}
	return s.buildDraftResponse(draft, article), nil
}

// ListDrafts returns every independent draft of the user, annotated with
// the online article it belongs to (if any).
func (s *ArticleDraftService) ListDrafts(userID uint) ([]dto.ArticleDraftItem, error) {
	drafts, err := s.draftRepo.ListByOwner(userID)
	if err != nil {
		return nil, fmt.Errorf("article draft list: %w", err)
	}
	items := make([]dto.ArticleDraftItem, 0, len(drafts))
	for _, d := range drafts {
		item := dto.ArticleDraftItem{
			DraftID: d.ID, Title: d.Title, Cover: d.Cover, TopicTag: d.TopicTag,
			BaseRevisionNo: d.BaseRevisionNo, SavedAt: d.SavedAt,
		}
		if d.ArticleID != nil {
			item.ArticleID = *d.ArticleID
			if a, err := s.articleRepo.FindByID(*d.ArticleID); err == nil {
				item.ArticleStatus = a.Status
				item.OnlineRevisionNo = a.RevisionNo
				item.Conflict = a.RevisionNo != d.BaseRevisionNo
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// ListMine returns owner-side metadata for all articles of the user,
// merged with each article's pending draft state.
func (s *ArticleDraftService) ListMine(userID uint) ([]dto.ArticleEditorMeta, error) {
	items, err := s.articleRepo.ListByOwner(userID)
	if err != nil {
		return nil, fmt.Errorf("care article list mine: %w", err)
	}
	drafts, err := s.draftRepo.ListByOwner(userID)
	if err != nil {
		return nil, fmt.Errorf("care article list mine drafts: %w", err)
	}
	draftByArticle := make(map[uint]model.ArticleDraft, len(drafts))
	for _, d := range drafts {
		if d.ArticleID != nil {
			draftByArticle[*d.ArticleID] = d
		}
	}
	metas := make([]dto.ArticleEditorMeta, 0, len(items))
	for _, a := range items {
		meta := dto.ArticleEditorMeta{
			ID: a.ID, Title: a.Title, Cover: a.Cover, TopicTag: a.TopicTag,
			Status: a.Status, RevisionNo: a.RevisionNo, ViewCount: a.ViewCount,
			CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
		}
		if d, ok := draftByArticle[a.ID]; ok {
			meta.HasDraft = true
			meta.DraftID = d.ID
			meta.DraftSavedAt = d.SavedAt
			meta.BaseRevisionNo = d.BaseRevisionNo
			meta.DraftConflict = d.BaseRevisionNo != a.RevisionNo
		}
		metas = append(metas, meta)
	}
	return metas, nil
}

// Withdraw takes a published article offline. Visitors stop seeing it
// immediately while the content, revisions and any unpublished draft are
// retained. Idempotent for already-withdrawn articles.
func (s *ArticleDraftService) Withdraw(id, userID uint) error {
	exist, err := s.articleRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("CareArticle[id=%d] not found", id))
		}
		return fmt.Errorf("care article withdraw find: %w", err)
	}
	if exist.UserID != userID {
		return util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("CareArticle[id=%d] withdraw failed: user_id=%d is not owner", id, userID))
	}
	if exist.Status == constants.ArticleStatusWithdrawn {
		return nil
	}
	if err := s.articleRepo.UpdateColumns(id, map[string]interface{}{"status": constants.ArticleStatusWithdrawn}); err != nil {
		return fmt.Errorf("care article withdraw: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleWithdrawn, id, exist.RevisionNo, userID))
	return nil
}

// ListRevisions returns the revision history of an owned article.
func (s *ArticleDraftService) ListRevisions(articleID, userID uint) ([]dto.ArticleRevisionResponse, error) {
	if _, err := s.loadOwnedArticle(articleID, userID); err != nil {
		return nil, err
	}
	revs, err := s.revisionRepo.ListByArticle(articleID)
	if err != nil {
		return nil, fmt.Errorf("article revision list: %w", err)
	}
	out := make([]dto.ArticleRevisionResponse, 0, len(revs))
	for _, r := range revs {
		out = append(out, toRevisionResponse(r))
	}
	return out, nil
}

// GetRevision opens a single historical revision of an owned article.
func (s *ArticleDraftService) GetRevision(articleID, userID uint, revisionNo int) (*dto.ArticleRevisionResponse, error) {
	if _, err := s.loadOwnedArticle(articleID, userID); err != nil {
		return nil, err
	}
	rev, err := s.revisionRepo.FindByNumber(articleID, revisionNo)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound,
				fmt.Sprintf("ArticleRevision[article=%d revision=%d] not found", articleID, revisionNo))
		}
		return nil, fmt.Errorf("article revision get: %w", err)
	}
	resp := toRevisionResponse(*rev)
	return &resp, nil
}

// Restore opens a historical revision as a NEW draft, leaving history
// untouched. An existing unpublished draft blocks the operation so it can
// never be overwritten; it must be published or discarded first.
func (s *ArticleDraftService) Restore(articleID, userID uint, revisionNo int) (*dto.ArticleDraftResponse, error) {
	article, err := s.loadOwnedArticle(articleID, userID)
	if err != nil {
		return nil, err
	}
	rev, err := s.revisionRepo.FindByNumber(articleID, revisionNo)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound,
				fmt.Sprintf("ArticleRevision[article=%d revision=%d] not found", articleID, revisionNo))
		}
		return nil, fmt.Errorf("article revision restore find: %w", err)
	}
	if existing, err := s.draftRepo.FindByArticle(articleID, userID); err == nil {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("an unpublished draft (id=%d) already exists, discard or publish it before restoring a revision", existing.ID))
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("article revision restore lookup: %w", err)
	}
	draft := &model.ArticleDraft{
		ArticleID: &article.ID, UserID: userID,
		Title: rev.Title, Content: rev.Content, Cover: rev.Cover, TopicTag: rev.TopicTag,
		BaseRevisionNo: article.RevisionNo, SavedAt: time.Now(),
	}
	if err := s.draftRepo.Create(draft); err != nil {
		return nil, fmt.Errorf("article revision restore create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleRevisionRestored, articleID, revisionNo, draft.ID, userID))
	return s.buildDraftResponse(draft, article), nil
}

func (s *ArticleDraftService) loadOwnedArticle(articleID, userID uint) (*model.CareArticle, error) {
	article, err := s.articleRepo.FindByID(articleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("CareArticle[id=%d] not found", articleID))
		}
		return nil, fmt.Errorf("article load: %w", err)
	}
	if article.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("CareArticle[id=%d] access denied: user_id=%d is not owner", articleID, userID))
	}
	return article, nil
}

func (s *ArticleDraftService) respondDraft(draft *model.ArticleDraft) (*dto.ArticleDraftResponse, error) {
	if draft.ArticleID == nil {
		return s.buildDraftResponse(draft, nil), nil
	}
	article, err := s.articleRepo.FindByID(*draft.ArticleID)
	if err != nil {
		return nil, fmt.Errorf("article draft reload: %w", err)
	}
	return s.buildDraftResponse(draft, article), nil
}

func (s *ArticleDraftService) buildDraftResponse(d *model.ArticleDraft, article *model.CareArticle) *dto.ArticleDraftResponse {
	resp := &dto.ArticleDraftResponse{
		DraftID:        d.ID,
		Title:          d.Title,
		Content:        d.Content,
		Cover:          d.Cover,
		TopicTag:       d.TopicTag,
		BaseRevisionNo: d.BaseRevisionNo,
		SavedAt:        d.SavedAt,
	}
	if d.ArticleID != nil {
		resp.ArticleID = *d.ArticleID
	}
	if article != nil {
		resp.ArticleStatus = article.Status
		resp.OnlineRevisionNo = article.RevisionNo
		resp.OnlineUpdatedAt = article.UpdatedAt
		resp.Conflict = article.RevisionNo != d.BaseRevisionNo
	}
	return resp
}

func (s *ArticleDraftService) staleDraftError(draft *model.ArticleDraft, article *model.CareArticle) error {
	s.logger.Warn(fmt.Sprintf(constants.LogArticlePublishStale,
		article.ID, draft.BaseRevisionNo, article.RevisionNo, draft.UserID))
	return util.NewAppError(409, constants.CodeRevisionStale, constants.MsgRevisionStale).
		WithDetails(dto.RevisionConflictDetails{
			DraftID:          draft.ID,
			ArticleID:        article.ID,
			BaseRevisionNo:   draft.BaseRevisionNo,
			OnlineRevisionNo: article.RevisionNo,
			OnlineTitle:      article.Title,
			OnlineUpdatedAt:  article.UpdatedAt,
		})
}

func articleRef(id *uint) uint {
	if id == nil {
		return 0
	}
	return *id
}

func toRevisionResponse(r model.ArticleRevision) dto.ArticleRevisionResponse {
	return dto.ArticleRevisionResponse{
		ID: r.ID, RevisionNo: r.RevisionNo, UserID: r.UserID,
		Title: r.Title, Content: r.Content, Cover: r.Cover, TopicTag: r.TopicTag,
		Summary: r.Summary, CreatedAt: r.CreatedAt,
	}
}
