<template>
  <div class="page" v-if="article">
    <el-page-header @back="$router.back()" content="文章详情" />
    <el-card class="article">
      <h1>{{ article.title }}</h1>
      <div class="meta">
        <el-tag type="success">{{ CareTopicTagMap[article.topic_tag] }}</el-tag>
        <span>
          发布于 {{ formatDateTime(article.created_at) }} ·
          修订 v{{ article.revision_no }} ·
          更新于 {{ formatDateTime(article.updated_at) }} ·
          阅读 {{ article.view_count }}
        </span>
        <FavoriteButton target-type="article" :target-id="article.id" />
        <template v-if="isOwner">
          <el-button size="small" type="primary" plain @click="goEdit">编辑（独立草稿）</el-button>
          <el-button size="small" type="warning" plain @click="onWithdraw">撤回</el-button>
        </template>
      </div>
      <el-image v-if="article.cover" :src="article.cover" fit="cover" class="cover" />
      <div class="content" v-html="renderedContent"></div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getArticle, withdrawArticle } from '@/api/article'
import FavoriteButton from '@/components/common/FavoriteButton.vue'
import { CareTopicTagMap, type CareArticle } from '@/constants/article'
import { useAuthStore } from '@/stores/authStore'
import { formatDateTime } from '@/utils/dateFormat'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const article = ref<CareArticle | null>(null)

const renderedContent = computed(() => {
  const raw = article.value?.content || ''
  return raw.split('\n').map((p) => `<p>${p}</p>`).join('')
})

const isOwner = computed(() => Boolean(article.value && auth.user?.id === article.value.user_id))

onMounted(async () => {
  if (auth.token && !auth.user) {
    await auth.fetchProfile().catch(() => undefined)
  }
  article.value = await getArticle(route.params.id as string)
})

function goEdit() {
  router.push({ name: 'articleEditor', params: { id: route.params.id } })
}

async function onWithdraw() {
  if (!article.value) return
  await ElMessageBox.confirm('撤回后访客将看不到本文，内容与修订仍保留。', '确认撤回', { type: 'warning' })
  await withdrawArticle(article.value.id)
  ElMessage.success('已撤回')
  router.push({ name: 'articleManage' })
}
</script>

<style scoped>
.page { max-width: 900px; margin: 0 auto; }
.article { padding: 16px; }
.meta { display: flex; align-items: center; gap: 12px; color: #999; margin: 12px 0; flex-wrap: wrap; }
.cover { width: 100%; max-height: 400px; border-radius: 8px; margin: 12px 0; }
.content { line-height: 1.9; color: #333; }
</style>
