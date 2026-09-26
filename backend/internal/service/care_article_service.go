package service

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// CareArticleService implements care article business logic.
//
// 编辑模型：线上快照（title/content/...）与独立草稿（draft_*）分离。
// 保存只写草稿；发布走 published_revision 乐观比较，线上被他人重新发布时
// 拒绝发布并保留草稿；历史修订可恢复为新草稿；撤回只改可见状态，内容留档。
type CareArticleService struct {
	repo     *repository.CareArticleRepository
	userRepo *repository.UserRepository
	logger   *slog.Logger
}

// NewCareArticleService creates a CareArticleService.
func NewCareArticleService(repo *repository.CareArticleRepository, userRepo *repository.UserRepository, logger *slog.Logger) *CareArticleService {
	return &CareArticleService{repo: repo, userRepo: userRepo, logger: logger}
}

// Create stores a brand new article as an independent draft (nothing is live yet).
func (s *CareArticleService) Create(userID uint, req dto.ArticleCreateRequest) (*model.CareArticle, error) {
	if !constants.IsValidTopicTag(req.TopicTag) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("CareArticle[topic_tag=%s] create failed: invalid topic tag", req.TopicTag))
	}
	a := &model.CareArticle{
		UserID:        userID,
		Status:        constants.ArticleStatusDraft,
		HasDraft:      true,
		DraftTitle:    req.Title,
		DraftContent:  req.Content,
		DraftCover:    req.Cover,
		DraftTopicTag: req.TopicTag,
	}
	if err := s.repo.Create(a); err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogArticleCreateFailed, req.Title), "error", err)
		return nil, fmt.Errorf("care article create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleCreateSuccess, req.Title), "id", a.ID, "user_id", userID)
	return a, nil
}

// GetPublic returns the live snapshot for visitors. Drafts and offline articles are invisible.
func (s *CareArticleService) GetPublic(id uint) (*dto.ArticlePublicView, error) {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return nil, s.wrapNotFound(err, id, "care article get")
	}
	if a.Status != constants.ArticleStatusPublished {
		return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("CareArticle[id=%d] not found", id))
	}
	if err := s.repo.IncrementView(id); err == nil {
		a.ViewCount++
		s.logger.Info(fmt.Sprintf(constants.LogArticleViewIncremented, id), "id", id)
	}
	v := toPublicView(a)
	return &v, nil
}

// GetForOwner returns the live snapshot together with the independent draft.
// When no draft exists the draft payload is seeded from the live snapshot so the
// editor opens on a copy instead of mutating the online version.
func (s *CareArticleService) GetForOwner(id, userID uint) (*dto.ArticleOwnerView, error) {
	a, err := s.loadOwned(id, userID)
	if err != nil {
		return nil, err
	}
	return buildOwnerView(a), nil
}

// SaveDraft writes the independent draft only; the live snapshot is never touched.
// The draft keeps the page revision the editor was opened on, which drives
// conflict detection at publish time.
func (s *CareArticleService) SaveDraft(id, userID uint, req dto.ArticleDraftSaveRequest) (*dto.ArticleOwnerView, error) {
	if !constants.IsValidTopicTag(req.TopicTag) {
		return nil, util.NewAppError(422, constants.CodeValidationError, "invalid topic tag")
	}
	a, err := s.loadOwned(id, userID)
	if err != nil {
		return nil, err
	}
	a.DraftTitle = req.Title
	a.DraftContent = req.Content
	a.DraftCover = req.Cover
	a.DraftTopicTag = req.TopicTag
	a.DraftBaseRevision = req.BaseRevision
	if err := s.repo.SaveDraft(a); err != nil {
		return nil, fmt.Errorf("care article save draft: %w", err)
	}
	s.logger.Info("care article draft saved", "id", id, "user_id", userID, "base_revision", req.BaseRevision)
	return buildOwnerView(a), nil
}

// Publish turns the draft into the live snapshot. It is an optimistic
// compare-and-swap on the page revision: if the live version was republished by
// someone else after the draft was forked, it fails with 409 and the local
// draft is retained for re-merge instead of overwriting the new version.
func (s *CareArticleService) Publish(id, userID uint, req dto.ArticlePublishRequest) (*dto.ArticleOwnerView, error) {
	a, err := s.loadOwned(id, userID)
	if err != nil {
		return nil, err
	}
	if !a.HasDraft {
		return nil, util.NewAppError(422, constants.CodeValidationError, "没有可发布的草稿，请先保存草稿")
	}
	base := a.DraftBaseRevision
	if req.BaseRevision != nil {
		base = *req.BaseRevision
	}
	if base != a.PublishedRevision {
		// 草稿保留不动，仅提示重新合并，不能盖掉新版本。
		return nil, util.NewAppError(409, constants.CodeRevisionConflict, constants.MsgRevisionConflict)
	}
	operator := s.operatorName(userID)
	published, err := s.repo.PublishDraft(id, base, userID, operator)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, util.NewAppError(409, constants.CodeRevisionConflict, constants.MsgRevisionConflict)
		}
		return nil, fmt.Errorf("care article publish: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleUpdateSuccess, id),
		"id", id, "revision", published.PublishedRevision, "user_id", userID)
	return buildOwnerView(published), nil
}

// Offline withdraws the live snapshot: visitors can no longer see the article,
// while the row, its draft and its revision history remain.
func (s *CareArticleService) Offline(id, userID uint) (*dto.ArticleOwnerView, error) {
	if _, err := s.loadOwned(id, userID); err != nil {
		return nil, err
	}
	out, err := s.repo.Offline(id, userID, s.operatorName(userID))
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, util.NewAppError(422, constants.CodeValidationError, "文章当前不在线上，无法撤回")
		}
		return nil, fmt.Errorf("care article offline: %w", err)
	}
	s.logger.Info("care article offlined", "id", id, "user_id", userID)
	return buildOwnerView(out), nil
}

// ListRevisions returns the revision archive (owner only).
func (s *CareArticleService) ListRevisions(id, userID uint) ([]dto.ArticleRevisionView, error) {
	if _, err := s.loadOwned(id, userID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListRevisions(id)
	if err != nil {
		return nil, fmt.Errorf("care article revisions: %w", err)
	}
	views := make([]dto.ArticleRevisionView, 0, len(items))
	for i := range items {
		views = append(views, toRevisionView(&items[i]))
	}
	return views, nil
}

// GetRevision opens one historical snapshot (owner only).
func (s *CareArticleService) GetRevision(id, revisionID, userID uint) (*dto.ArticleRevisionView, error) {
	if _, err := s.loadOwned(id, userID); err != nil {
		return nil, err
	}
	rev, err := s.repo.FindRevision(id, revisionID)
	if err != nil {
		return nil, s.wrapNotFound(err, revisionID, "care article revision get")
	}
	v := toRevisionView(rev)
	return &v, nil
}

// RestoreRevision copies a historical snapshot into a new draft without
// changing the live version.
func (s *CareArticleService) RestoreRevision(id, revisionID, userID uint) (*dto.ArticleOwnerView, error) {
	if _, err := s.loadOwned(id, userID); err != nil {
		return nil, err
	}
	out, err := s.repo.RestoreRevision(id, revisionID, userID, s.operatorName(userID))
	if err != nil {
		return nil, s.wrapNotFound(err, revisionID, "care article revision restore")
	}
	s.logger.Info("care article revision restored into draft",
		"article_id", id, "revision_id", revisionID, "user_id", userID)
	return buildOwnerView(out), nil
}

// Delete removes an article (and its revision archive), verifying ownership.
func (s *CareArticleService) Delete(id, userID uint) error {
	if _, err := s.loadOwned(id, userID); err != nil {
		return err
	}
	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("care article delete: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleDeleteSuccess, id), "id", id)
	return nil
}

// List filters visible articles by topic tag and keyword.
func (s *CareArticleService) List(topicTag, keyword string, page, pageSize int) ([]dto.ArticlePublicView, int64, error) {
	items, total, err := s.repo.List(topicTag, keyword, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("care article list: %w", err)
	}
	views := make([]dto.ArticlePublicView, 0, len(items))
	for i := range items {
		views = append(views, toPublicView(&items[i]))
	}
	return views, total, nil
}

// ListMine lists the caller's own articles, including drafts and withdrawn ones.
// 未发布内容只对作者本人可见，访客接口永远走不到这里。
func (s *CareArticleService) ListMine(userID uint, page, pageSize int) ([]dto.ArticleOwnerView, int64, error) {
	items, total, err := s.repo.ListByOwner(userID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("care article list mine: %w", err)
	}
	views := make([]dto.ArticleOwnerView, 0, len(items))
	for i := range items {
		views = append(views, *buildOwnerView(&items[i]))
	}
	return views, total, nil
}

// ListLatest returns the newest published articles for the home page.
func (s *CareArticleService) ListLatest(limit int) ([]dto.ArticlePublicView, error) {
	items, err := s.repo.ListLatest(limit)
	if err != nil {
		return nil, fmt.Errorf("care article latest: %w", err)
	}
	views := make([]dto.ArticlePublicView, 0, len(items))
	for i := range items {
		views = append(views, toPublicView(&items[i]))
	}
	return views, nil
}

// loadOwned fetches an article and verifies the caller is its author.
func (s *CareArticleService) loadOwned(id, userID uint) (*model.CareArticle, error) {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return nil, s.wrapNotFound(err, id, "care article find")
	}
	if a.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("CareArticle[id=%d] access denied: user_id=%d is not owner", id, userID))
	}
	return a, nil
}

func (s *CareArticleService) operatorName(userID uint) string {
	u, err := s.userRepo.FindByID(userID)
	if err != nil || u == nil {
		return ""
	}
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.Username
}

func (s *CareArticleService) wrapNotFound(err error, id uint, prefix string) error {
	if errors.Is(err, repository.ErrNotFound) {
		return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("CareArticle[id=%d] not found", id))
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

// toPublicView projects only the live snapshot; draft columns never leave this boundary.
func toPublicView(a *model.CareArticle) dto.ArticlePublicView {
	return dto.ArticlePublicView{
		ID:                a.ID,
		UserID:            a.UserID,
		Title:             a.Title,
		Content:           a.Content,
		Cover:             a.Cover,
		TopicTag:          a.TopicTag,
		Status:            a.Status,
		ViewCount:         a.ViewCount,
		PublishedRevision: a.PublishedRevision,
		CreatedAt:         a.CreatedAt,
		UpdatedAt:         a.UpdatedAt,
		PublishedAt:       a.PublishedAt,
	}
}

// buildOwnerView assembles the editor payload. With no saved draft, the draft
// payload is seeded from the live snapshot (editor edits a copy).
func buildOwnerView(a *model.CareArticle) *dto.ArticleOwnerView {
	v := &dto.ArticleOwnerView{ArticlePublicView: toPublicView(a)}
	if a.HasDraft {
		v.Draft = dto.ArticleDraftView{
			Title:        a.DraftTitle,
			Content:      a.DraftContent,
			Cover:        a.DraftCover,
			TopicTag:     a.DraftTopicTag,
			BaseRevision: a.DraftBaseRevision,
			HasDraft:     true,
			SavedAt:      a.DraftSavedAt,
		}
	} else {
		v.Draft = dto.ArticleDraftView{
			Title:        a.Title,
			Content:      a.Content,
			Cover:        a.Cover,
			TopicTag:     a.TopicTag,
			BaseRevision: a.PublishedRevision,
			HasDraft:     false,
		}
	}
	// 草稿基线落后于线上修订号：线上已被重新发布，需要重新合并。
	v.Conflict = a.HasDraft && a.DraftBaseRevision != a.PublishedRevision
	return v
}

func toRevisionView(r *model.ArticleRevision) dto.ArticleRevisionView {
	return dto.ArticleRevisionView{
		ID:           r.ID,
		Revision:     r.Revision,
		Action:       r.Action,
		Title:        r.Title,
		Content:      r.Content,
		Cover:        r.Cover,
		TopicTag:     r.TopicTag,
		OperatorID:   r.OperatorID,
		OperatorName: r.OperatorName,
		CreatedAt:    r.CreatedAt,
	}
}
