<template>
  <div class="page" v-if="article">
    <el-page-header @back="$router.back()" content="文章详情" />
    <el-card class="article">
      <h1>{{ article.title }}</h1>
      <div class="meta">
        <el-tag type="success">{{ CareTopicTagMap[article.topic_tag] }}</el-tag>
        <el-tag type="info" effect="plain">页面修订 v{{ article.published_revision }}</el-tag>
        <span>发布于 {{ formatDateTime(article.published_at || article.created_at) }} · 阅读 {{ article.view_count }}</span>
        <FavoriteButton target-type="article" :target-id="article.id" />
        <el-button
          v-if="auth.user && auth.user.id === article.user_id"
          size="small"
          type="primary"
          plain
          @click="$router.push({ name: 'articleEdit', params: { id: article.id } })"
        >
          编辑（打开草稿）
        </el-button>
      </div>
      <el-image v-if="article.cover" :src="article.cover" fit="cover" class="cover" />
      <div class="content" v-html="renderedContent"></div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getArticle } from '@/api/article'
import FavoriteButton from '@/components/common/FavoriteButton.vue'
import { CareTopicTagMap, type CareArticle } from '@/constants/article'
import { useAuthStore } from '@/stores/authStore'
import { formatDateTime } from '@/utils/dateFormat'

const route = useRoute()
const auth = useAuthStore()
const article = ref<CareArticle | null>(null)

const renderedContent = computed(() => {
  const raw = article.value?.content || ''
  return raw.split('\n').map((p) => `<p>${p}</p>`).join('')
})

onMounted(async () => {
  article.value = await getArticle(route.params.id as string)
})
</script>

<style scoped>
.page { max-width: 900px; margin: 0 auto; }
.article { padding: 16px; }
.meta { display: flex; align-items: center; gap: 12px; color: #999; margin: 12px 0; flex-wrap: wrap; }
.cover { width: 100%; max-height: 400px; border-radius: 8px; margin: 12px 0; }
.content { line-height: 1.9; color: #333; }
</style>
