package router

import (
	"github.com/gin-gonic/gin"

	"github.com/gbplantwiki/gbplantwiki/internal/config"
	"github.com/gbplantwiki/gbplantwiki/internal/handler"
	"github.com/gbplantwiki/gbplantwiki/internal/middleware"
)

func registerArticleRoutes(v1 *gin.RouterGroup, cfg *config.Config, h *handler.CareArticleHandler, limiter *middleware.RateLimiter) {
	articles := v1.Group("/articles")
	// 访客接口：只暴露线上快照；带 token 且 scope=mine 时可看自己的草稿/撤回文章。
	articles.GET("", middleware.OptionalAuth(cfg), h.List)
	articles.GET("/:id", h.Get)

	// 作者接口：独立草稿、发布（修订号乐观锁）、撤回、历史修订。
	auth := articles.Group("", middleware.AuthRequired(cfg))
	auth.POST("", limiter.Limit(), h.Create)
	auth.GET("/:id/edit", h.OpenEdit)
	auth.PUT("/:id/draft", h.SaveDraft)
	auth.POST("/:id/publish", h.Publish)
	auth.POST("/:id/offline", h.Offline)
	auth.GET("/:id/revisions", h.ListRevisions)
	auth.GET("/:id/revisions/:rid", h.GetRevision)
	auth.POST("/:id/revisions/:rid/restore", h.RestoreRevision)
	auth.DELETE("/:id", h.Delete)
}
