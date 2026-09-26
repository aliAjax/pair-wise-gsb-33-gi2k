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

// CareArticleHandler exposes care article endpoints.
type CareArticleHandler struct {
	svc    *service.CareArticleService
	logger *slog.Logger
}

// NewCareArticleHandler creates a CareArticleHandler.
func NewCareArticleHandler(svc *service.CareArticleService, logger *slog.Logger) *CareArticleHandler {
	return &CareArticleHandler{svc: svc, logger: logger}
}

// List handles GET /articles — 默认访客只看到线上快照；
// scope=mine 时返回作者自己的全部文章（含草稿/撤回态），需要登录。
func (h *CareArticleHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	topicTag := c.Query("topic_tag")
	keyword := c.Query("keyword")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	if c.Query("scope") == "mine" {
		uid, ok := middleware.EnsureViewerID(c)
		if !ok {
			return
		}
		items, total, err := h.svc.ListMine(uid, page, pageSize)
		if err != nil {
			c.Error(err)
			return
		}
		c.JSON(http.StatusOK, dto.OK(dto.PageData{List: items, Total: total, Page: page, Size: pageSize}))
		return
	}
	items, total, err := h.svc.List(topicTag, keyword, page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(dto.PageData{List: items, Total: total, Page: page, Size: pageSize}))
}

// Get handles GET /articles/:id — 线上详情（草稿/撤回文章对访客不可见）。
func (h *CareArticleHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid article id"))
		return
	}
	a, err := h.svc.GetPublic(uint(id))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(a))
}

// Create handles POST /articles — 新文章先以独立草稿落库。
func (h *CareArticleHandler) Create(c *gin.Context) {
	var req dto.ArticleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam+": "+err.Error()))
		return
	}
	a, err := h.svc.Create(middleware.GetUserID(c), req)
	if err != nil {
		c.Error(err)
		return
	}
	// 草稿创建后直接进入编辑器视图。
	owner, err := h.svc.GetForOwner(a.ID, middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.OK(owner))
}

// OpenEdit handles GET /articles/:id/edit — 打开即独立草稿，线上/草稿分开返回。
func (h *CareArticleHandler) OpenEdit(c *gin.Context) {
	id, ok := parseArticleID(c)
	if !ok {
		return
	}
	owner, err := h.svc.GetForOwner(id, middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(owner))
}

// SaveDraft handles PUT /articles/:id/draft — 保存草稿，带页面修订号，不动线上。
func (h *CareArticleHandler) SaveDraft(c *gin.Context) {
	id, ok := parseArticleID(c)
	if !ok {
		return
	}
	var req dto.ArticleDraftSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam+": "+err.Error()))
		return
	}
	owner, err := h.svc.SaveDraft(id, middleware.GetUserID(c), req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(owner))
}

// Publish handles POST /articles/:id/publish — 草稿发布为新的线上快照（乐观锁）。
func (h *CareArticleHandler) Publish(c *gin.Context) {
	id, ok := parseArticleID(c)
	if !ok {
		return
	}
	var req dto.ArticlePublishRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam+": "+err.Error()))
			return
		}
	}
	owner, err := h.svc.Publish(id, middleware.GetUserID(c), req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(owner))
}

// Offline handles POST /articles/:id/offline — 撤回：访客不可见，内容留档。
func (h *CareArticleHandler) Offline(c *gin.Context) {
	id, ok := parseArticleID(c)
	if !ok {
		return
	}
	owner, err := h.svc.Offline(id, middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(owner))
}

// ListRevisions handles GET /articles/:id/revisions — 历史修订列表。
func (h *CareArticleHandler) ListRevisions(c *gin.Context) {
	id, ok := parseArticleID(c)
	if !ok {
		return
	}
	items, err := h.svc.ListRevisions(id, middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(items))
}

// GetRevision handles GET /articles/:id/revisions/:rid — 打开某次历史修订。
func (h *CareArticleHandler) GetRevision(c *gin.Context) {
	id, ok := parseArticleID(c)
	if !ok {
		return
	}
	revisionID, err := strconv.ParseUint(c.Param("rid"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid revision id"))
		return
	}
	rev, err := h.svc.GetRevision(id, uint(revisionID), middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(rev))
}

// RestoreRevision handles POST /articles/:id/revisions/:rid/restore — 恢复为新草稿。
func (h *CareArticleHandler) RestoreRevision(c *gin.Context) {
	id, ok := parseArticleID(c)
	if !ok {
		return
	}
	revisionID, err := strconv.ParseUint(c.Param("rid"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid revision id"))
		return
	}
	owner, err := h.svc.RestoreRevision(id, uint(revisionID), middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(owner))
}

// Delete handles DELETE /articles/:id.
func (h *CareArticleHandler) Delete(c *gin.Context) {
	id, ok := parseArticleID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(id, middleware.GetUserID(c)); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(gin.H{"deleted": true}))
}

func parseArticleID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid article id"))
		return 0, false
	}
	return uint(id), true
}
