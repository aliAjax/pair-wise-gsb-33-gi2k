<template>
  <el-card class="article-card" shadow="hover" @click="emit('click')">
    <el-image v-if="article.cover" :src="article.cover" fit="cover" class="cover" lazy>
      <template #error><div class="img-placeholder">📖</div></template>
    </el-image>
    <div class="body">
      <div class="tags">
        <el-tag size="small" type="success">{{ CareTopicTagMap[article.topic_tag] }}</el-tag>
        <template v-if="ownerView">
          <el-tag size="small" :type="statusTag.type">{{ statusTag.text }}</el-tag>
          <el-tag v-if="(article as ArticleOwnerView).conflict" size="small" type="danger">待合并</el-tag>
        </template>
        <el-tag v-else size="small" type="info">v{{ article.published_revision }}</el-tag>
      </div>
      <div class="title">{{ displayTitle }}</div>
      <div class="meta">{{ formatDate(article.created_at) }} · 阅读 {{ article.view_count }}</div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  CareTopicTagMap,
  type ArticleOwnerView,
  type CareArticle,
} from '@/constants/article'
import { formatDate } from '@/utils/dateFormat'

const props = defineProps<{ article: CareArticle | ArticleOwnerView; ownerView?: boolean }>()
const emit = defineEmits<{ (e: 'click'): void }>()

// 我的视图里未发布的文章以草稿标题展示，避免“线上为空标题”的困惑。
const displayTitle = computed(() => {
  if (props.ownerView) {
    const a = props.article as ArticleOwnerView
    if (a.status !== 'published') return a.draft?.title || a.title
  }
  return props.article.title
})

const statusTag = computed(() => {
  const a = props.article as ArticleOwnerView
  if (a.status === 'published') {
    return { type: 'success' as const, text: '线上' }
  }
  if (a.status === 'offline') {
    return { type: 'warning' as const, text: '已撤回' }
  }
  return { type: 'info' as const, text: a.draft?.has_draft ? '草稿' : '未发布' }
})
</script>

<style scoped>
.article-card { cursor: pointer; }
.cover { width: 100%; height: 140px; border-radius: 6px; }
.img-placeholder { height: 140px; display: flex; align-items: center; justify-content: center; font-size: 40px; background: #f5f0e8; }
.body { padding-top: 10px; }
.tags { display: flex; gap: 6px; flex-wrap: wrap; }
.title { font-weight: 700; margin-top: 6px; line-height: 1.4; }
.meta { color: #888; font-size: 12px; margin-top: 6px; }
</style>
