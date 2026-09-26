package router

import (
	"github.com/gin-gonic/gin"

	"github.com/gbplantwiki/gbplantwiki/internal/config"
	"github.com/gbplantwiki/gbplantwiki/internal/handler"
	"github.com/gbplantwiki/gbplantwiki/internal/middleware"
)

func registerArticleRoutes(v1 *gin.RouterGroup, cfg *config.Config, h *handler.CareArticleHandler, editor *handler.ArticleDraftHandler, limiter *middleware.RateLimiter) {
	articles := v1.Group("/articles")
	articles.GET("", h.List)
	articles.GET("/:id", h.Get)
	auth := articles.Group("", middleware.AuthRequired(cfg))
	auth.POST("", limiter.Limit(), h.Create)
	auth.PUT("/:id", h.Update)
	auth.DELETE("/:id", h.Delete)

	// Authenticated editing workflow (drafts / revisions / withdrawal).
	// Kept under a separate /account prefix so owner-only paths never
	// collide with the public /articles/:id segment.
	account := v1.Group("/account", middleware.AuthRequired(cfg))
	account.GET("/articles", editor.ListMine)
	account.POST("/articles/publish", limiter.Limit(), editor.PublishDraft)
	account.POST("/articles/:id/withdraw", editor.WithdrawArticle)
	account.GET("/articles/:id/revisions", editor.ListRevisions)
	account.GET("/articles/:id/revisions/:no", editor.GetRevision)
	account.POST("/articles/:id/revisions/:no/restore", editor.RestoreRevision)
	account.GET("/drafts", editor.ListDrafts)
	account.GET("/drafts/open", editor.OpenDraft)
	account.GET("/drafts/:id", editor.GetDraft)
	account.PUT("/drafts", limiter.Limit(), editor.SaveDraft)
	account.POST("/drafts/:id/reconcile", editor.ReconcileDraft)
	account.DELETE("/drafts/:id", editor.DiscardDraft)
}
