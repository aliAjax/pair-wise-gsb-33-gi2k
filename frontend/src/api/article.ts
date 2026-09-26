import request from '@/utils/request'
import type { CareTopicTag } from '@/constants/article'
import type {
  ArticleDraftPayload,
  ArticleOwnerView,
  ArticleRevision,
  CareArticle,
} from '@/constants/article'
import type { PageData } from '@/types/api'

export function listArticles(params: {
  page?: number
  page_size?: number
  topic_tag?: string
  keyword?: string
  scope?: 'mine'
}) {
  return request.get<never, PageData<CareArticle | ArticleOwnerView>>('/articles', { params })
}

export function getArticle(id: number | string) {
  return request.get<never, CareArticle>(`/articles/${id}`)
}

// 新文章先以独立草稿创建，返回编辑器视图（线上 + 草稿）。
export function createArticle(payload: { title: string; content: string; cover?: string; topic_tag: CareTopicTag }) {
  return request.post<never, ArticleOwnerView>('/articles', payload)
}

// 打开编辑：进入独立草稿，线上快照与草稿一起返回。
export function openArticleEdit(id: number | string) {
  return request.get<never, ArticleOwnerView>(`/articles/${id}/edit`)
}

// 保存草稿：带上页面修订号，绝不触碰线上。
export function saveArticleDraft(id: number | string, payload: ArticleDraftPayload) {
  return request.put<never, ArticleOwnerView>(`/articles/${id}/draft`, payload)
}

// 发布草稿：服务端做修订号乐观比较，线上被他人重新发布时返回 409。
export function publishArticle(id: number | string, baseRevision?: number) {
  return request.post<never, ArticleOwnerView>(`/articles/${id}/publish`,
    baseRevision === undefined ? {} : { base_revision: baseRevision })
}

// 撤回：访客不可见，草稿与修订留档保留。
export function offlineArticle(id: number | string) {
  return request.post<never, ArticleOwnerView>(`/articles/${id}/offline`)
}

export function listArticleRevisions(id: number | string) {
  return request.get<never, ArticleRevision[]>(`/articles/${id}/revisions`)
}

export function getArticleRevision(id: number | string, revisionId: number | string) {
  return request.get<never, ArticleRevision>(`/articles/${id}/revisions/${revisionId}`)
}

// 把某次历史修订恢复成新草稿，不动线上。
export function restoreArticleRevision(id: number | string, revisionId: number | string) {
  return request.post<never, ArticleOwnerView>(`/articles/${id}/revisions/${revisionId}/restore`)
}

export function deleteArticle(id: number) {
  return request.delete<never, { deleted: boolean }>(`/articles/${id}`)
}
