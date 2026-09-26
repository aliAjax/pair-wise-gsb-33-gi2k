import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listArticles } from '@/api/article'
import type { CareArticle } from '@/constants/article'

export const useArticleStore = defineStore('article', () => {
  const articles = ref<CareArticle[]>([])
  const total = ref(0)

  async function load(params: { page?: number; page_size?: number; topic_tag?: string; keyword?: string } = {}) {
    const res = await listArticles(params)
    // 访客列表只映射线上快照
    articles.value = res.list as CareArticle[]
    total.value = res.total
  }

  return { articles, total, load }
})
