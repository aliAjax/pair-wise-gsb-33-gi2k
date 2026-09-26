export type CareTopicTag = 'fertilizing' | 'pruning' | 'repotting' | 'pest_control' | 'propagation'

export type ArticleStatus = 'published' | 'draft' | 'withdrawn'

export const CareTopicTagMap: Record<CareTopicTag, string> = {
  fertilizing: '施肥',
  pruning: '修剪',
  repotting: '换盆',
  pest_control: '病虫害',
  propagation: '繁殖',
}

export const ArticleStatusMap: Record<ArticleStatus, string> = {
  published: '已发布',
  draft: '未发布',
  withdrawn: '已撤回',
}

export const TOPIC_TAGS = Object.keys(CareTopicTagMap) as CareTopicTag[]

export interface CareArticle {
  id: number
  user_id: number
  title: string
  content: string
  cover: string
  topic_tag: CareTopicTag
  status: ArticleStatus | string
  revision_no: number
  view_count: number
  created_at: string
  updated_at: string
}

// 独立草稿（编辑器打开的对象，与线上版本分离）。
export interface ArticleDraft {
  draft_id: number
  article_id: number
  title: string
  content: string
  cover: string
  topic_tag: CareTopicTag
  // 页面修订号：草稿基于的线上修订。
  base_revision_no: number
  saved_at: string
  article_status: ArticleStatus | string
  online_revision_no: number
  online_updated_at: string
  // 线上已被别人重新发布、需要重新合并。
  conflict: boolean
}

export interface ArticleDraftSavePayload {
  draft_id?: number
  article_id?: number
  title: string
  content: string
  cover?: string
  topic_tag: CareTopicTag
  base_revision_no?: number
}

export interface ArticleDraftItem {
  draft_id: number
  article_id: number
  title: string
  cover: string
  topic_tag: CareTopicTag
  base_revision_no: number
  saved_at: string
  article_status: string
  online_revision_no: number
  conflict: boolean
}

export interface ArticleEditorMeta {
  id: number
  title: string
  cover: string
  topic_tag: CareTopicTag
  status: ArticleStatus | string
  revision_no: number
  view_count: number
  created_at: string
  updated_at: string
  draft_id: number
  draft_saved_at: string
  base_revision_no: number
  has_draft: boolean
  draft_conflict: boolean
}

export interface ArticleRevision {
  id: number
  revision_no: number
  user_id: number
  title: string
  content: string
  cover: string
  topic_tag: CareTopicTag
  summary: string
  created_at: string
}

// 40901 冲突时后端返回的结构化 details。
export interface RevisionConflictDetails {
  draft_id: number
  article_id: number
  base_revision_no: number
  online_revision_no: number
  online_title: string
  online_updated_at: string
}

export const CODE_REVISION_STALE = 40901
