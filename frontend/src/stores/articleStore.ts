import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { listArticles, listMyDrafts } from '@/api/article'
import type { ArticleDraftItem, CareArticle } from '@/constants/article'

export const useArticleStore = defineStore('article', () => {
  const articles = ref<CareArticle[]>([])
  const total = ref(0)

  // Owner-side independent drafts, cached for the management page.
  const myDrafts = ref<ArticleDraftItem[]>([])

  async function load(params: { page?: number; page_size?: number; topic_tag?: string; keyword?: string } = {}) {
    const res = await listArticles(params)
    articles.value = res.list
    total.value = res.total
  }

  async function loadMyDrafts() {
    const res = await listMyDrafts()
    myDrafts.value = res.list
  }

  const conflictingDraftCount = computed(() => myDrafts.value.filter((d) => d.conflict).length)

  return { articles, total, myDrafts, load, loadMyDrafts, conflictingDraftCount }
})
