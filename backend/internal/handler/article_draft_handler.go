package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/middleware"
	"github.com/gbplantwiki/gbplantwiki/internal/service"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// ArticleDraftHandler exposes the authenticated editing workflow:
// drafts, page revision tokens, publishing, withdrawal and revisions.
type ArticleDraftHandler struct {
	svc    *service.ArticleDraftService
	logger *slog.Logger
}

// NewArticleDraftHandler creates an ArticleDraftHandler.
func NewArticleDraftHandler(svc *service.ArticleDraftService, logger *slog.Logger) *ArticleDraftHandler {
	return &ArticleDraftHandler{svc: svc, logger: logger}
}

// ListMine handles GET /account/articles — owner-side article list.
func (h *ArticleDraftHandler) ListMine(c *gin.Context) {
	items, err := h.svc.ListMine(middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(gin.H{"list": items}))
}

// ListDrafts handles GET /account/drafts — independent draft list.
func (h *ArticleDraftHandler) ListDrafts(c *gin.Context) {
	items, err := h.svc.ListDrafts(middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(gin.H{"list": items}))
}

// OpenDraft handles GET /account/drafts/open?article_id=:id.
// article_id omitted/0 returns an empty seed for a brand-new article.
func (h *ArticleDraftHandler) OpenDraft(c *gin.Context) {
	articleID, _ := strconv.ParseUint(c.DefaultQuery("article_id", "0"), 10, 64)
	resp, err := h.svc.Open(middleware.GetUserID(c), uint(articleID))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(resp))
}

// GetDraft handles GET /account/drafts/:id.
func (h *ArticleDraftHandler) GetDraft(c *gin.Context) {
	draftID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.GetDraft(draftID, middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(resp))
}

// SaveDraft handles PUT /account/drafts — create or save an independent draft.
func (h *ArticleDraftHandler) SaveDraft(c *gin.Context) {
	var req dto.ArticleDraftSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam+": "+err.Error()))
		return
	}
	resp, err := h.svc.Save(middleware.GetUserID(c), req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(resp))
}

// DiscardDraft handles DELETE /account/drafts/:id.
func (h *ArticleDraftHandler) DiscardDraft(c *gin.Context) {
	draftID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Discard(draftID, middleware.GetUserID(c)); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(gin.H{"discarded": true}))
}

// PublishDraft handles POST /account/articles/publish.
func (h *ArticleDraftHandler) PublishDraft(c *gin.Context) {
	var req dto.ArticlePublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam+": "+err.Error()))
		return
	}
	article, err := h.svc.Publish(middleware.GetUserID(c), req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(article))
}

// ReconcileDraft handles POST /account/drafts/:id/reconcile.
func (h *ArticleDraftHandler) ReconcileDraft(c *gin.Context) {
	draftID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.Reconcile(draftID, middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(resp))
}

// WithdrawArticle handles POST /account/articles/:id/withdraw.
func (h *ArticleDraftHandler) WithdrawArticle(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Withdraw(id, middleware.GetUserID(c)); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(gin.H{"withdrawn": true}))
}

// ListRevisions handles GET /account/articles/:id/revisions.
func (h *ArticleDraftHandler) ListRevisions(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	revs, err := h.svc.ListRevisions(id, middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(gin.H{"list": revs}))
}

// GetRevision handles GET /account/articles/:id/revisions/:no.
func (h *ArticleDraftHandler) GetRevision(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	no, err := strconv.Atoi(c.Param("no"))
	if err != nil || no < 1 {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid revision number"))
		return
	}
	rev, err := h.svc.GetRevision(id, middleware.GetUserID(c), no)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(rev))
}

// RestoreRevision handles POST /account/articles/:id/revisions/:no/restore.
func (h *ArticleDraftHandler) RestoreRevision(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	no, err := strconv.Atoi(c.Param("no"))
	if err != nil || no < 1 {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid revision number"))
		return
	}
	resp, err := h.svc.Restore(id, middleware.GetUserID(c), no)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.OK(resp))
}

// parseIDParam parses a uint route parameter, recording a 400 on failure.
func parseIDParam(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid "+name))
		return 0, false
	}
	return uint(v), true
}
