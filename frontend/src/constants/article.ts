export type CareTopicTag = 'fertilizing' | 'pruning' | 'repotting' | 'pest_control' | 'propagation'

export const CareTopicTagMap: Record<CareTopicTag, string> = {
  fertilizing: '施肥',
  pruning: '修剪',
  repotting: '换盆',
  pest_control: '病虫害',
  propagation: '繁殖',
}

export const TOPIC_TAGS = Object.keys(CareTopicTagMap) as CareTopicTag[]

// 文章生命周期状态：draft 从未发布 / published 线上 / offline 已撤回（访客不可见）
export type ArticleStatus = 'draft' | 'published' | 'offline'

// 访客与列表读到的线上快照
export interface CareArticle {
  id: number
  user_id: number
  title: string
  content: string
  cover: string
  topic_tag: CareTopicTag
  status: ArticleStatus | string
  view_count: number
  published_revision: number
  published_at?: string
  created_at: string
  updated_at: string
}

// 独立草稿：与线上快照分开存储，base_revision 是草稿所依据的页面修订号
export interface ArticleDraft {
  title: string
  content: string
  cover: string
  topic_tag: CareTopicTag
  base_revision: number
  has_draft: boolean
  saved_at?: string
}

// 作者打开编辑器时拿到的线上 + 草稿组合视图。
// conflict=true 表示草稿基线落后于线上修订号（被别人重新发布过），需重新合并。
export interface ArticleOwnerView {
  id: number
  user_id: number
  title: string
  content: string
  cover: string
  topic_tag: CareTopicTag
  status: ArticleStatus | string
  view_count: number
  published_revision: number
  published_at?: string
  created_at: string
  updated_at: string
  draft: ArticleDraft
  conflict: boolean
}

// 历史修订快照
export type RevisionAction = 'publish' | 'offline' | 'restore'

export const RevisionActionMap: Record<RevisionAction, string> = {
  publish: '发布',
  offline: '撤回',
  restore: '恢复',
}

export interface ArticleRevision {
  id: number
  revision: number
  action: RevisionAction
  title: string
  content: string
  cover: string
  topic_tag: CareTopicTag
  operator_id: number
  operator_name: string
  created_at: string
}

export interface ArticleDraftPayload {
  title: string
  content: string
  cover?: string
  topic_tag: CareTopicTag
  base_revision: number
}
