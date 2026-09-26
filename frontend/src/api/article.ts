import request from '@/utils/request'
import type { PageData } from '@/types/api'
import {
  type ArticleDraft,
  type ArticleDraftItem,
  type ArticleDraftSavePayload,
  type ArticleEditorMeta,
  type ArticleRevision,
  type CareArticle,
  type CareTopicTag,
} from '@/constants/article'

/* ---------------- 公开接口：只读线上已发布快照 ---------------- */

export function listArticles(params: { page?: number; page_size?: number; topic_tag?: string; keyword?: string }) {
  return request.get<never, PageData<CareArticle>>('/articles', { params })
}

export function getArticle(id: number | string) {
  return request.get<never, CareArticle>(`/articles/${id}`)
}

/* ---------------- 兼容接口（直接发布，仍带修订留档） ---------------- */

export function createArticle(payload: { title: string; content: string; cover?: string; topic_tag: CareTopicTag }) {
  return request.post<never, CareArticle>('/articles', payload)
}

export function updateArticle(
  id: number,
  payload: { title: string; content: string; cover?: string; topic_tag: CareTopicTag },
) {
  return request.put<never, CareArticle>(`/articles/${id}`, payload)
}

export function deleteArticle(id: number) {
  return request.delete<never, { deleted: boolean }>(`/articles/${id}`)
}

/* ---------------- 编辑工作流：独立草稿 + 修订号 + 发布/撤回 ---------------- */

// 打开编辑器：返回已有草稿，或由线上版本播种的新草稿（尚未落库）。
export function openDraft(articleId?: number) {
  return request.get<never, ArticleDraft>('/account/drafts/open', {
    params: articleId ? { article_id: articleId } : {},
  })
}

export function getDraft(draftId: number) {
  return request.get<never, ArticleDraft>(`/account/drafts/${draftId}`)
}

// 保存草稿（新建或更新）。
export function saveDraft(payload: ArticleDraftSavePayload) {
  return request.put<never, ArticleDraft>('/account/drafts', payload)
}

export function discardDraft(draftId: number) {
  return request.delete<never, { discarded: boolean }>(`/account/drafts/${draftId}`)
}

// 发布草稿：携带页面修订号，过期时后端返回 409/40901 并保留草稿。
export function publishDraft(payload: { draft_id: number; base_revision_no: number; summary?: string }) {
  return request.post<never, CareArticle>('/account/articles/publish', payload)
}

// 合并完线上新版本后，把草稿重新基于当前线上修订。
export function reconcileDraft(draftId: number) {
  return request.post<never, ArticleDraft>(`/account/drafts/${draftId}/reconcile`)
}

// 撤回：访客不可见，内容、修订与未发布草稿保留。
export function withdrawArticle(id: number) {
  return request.post<never, { withdrawn: boolean }>(`/account/articles/${id}/withdraw`)
}

export function listMyArticles() {
  return request.get<never, { list: ArticleEditorMeta[] }>('/account/articles')
}

export function listMyDrafts() {
  return request.get<never, { list: ArticleDraftItem[] }>('/account/drafts')
}

/* ---------------- 历史修订 ---------------- */

export function listRevisions(articleId: number) {
  return request.get<never, { list: ArticleRevision[] }>(`/account/articles/${articleId}/revisions`)
}

export function getRevision(articleId: number, revisionNo: number) {
  return request.get<never, ArticleRevision>(`/account/articles/${articleId}/revisions/${revisionNo}`)
}

// 把历史修订打开成一份新草稿（不覆盖任何现有草稿）。
export function restoreRevision(articleId: number, revisionNo: number) {
  return request.post<never, ArticleDraft>(`/account/articles/${articleId}/revisions/${revisionNo}/restore`)
}
