package constants

// Article lifecycle statuses.
const (
	ArticleStatusDraft     = "draft"
	ArticleStatusPublished = "published"
	ArticleStatusOffline   = "offline" // 已撤回：访客不可见，但未发布内容（草稿/修订）仍保留
)

// ArticleRevisionAction marks how a revision snapshot was produced.
const (
	RevisionActionPublish = "publish" // 发布（草稿成为新的线上快照）
	RevisionActionOffline = "offline" // 撤回（线上快照撤下，内容留档）
	RevisionActionRestore = "restore" // 从历史修订恢复（内容写入新草稿，不动线上）
)
