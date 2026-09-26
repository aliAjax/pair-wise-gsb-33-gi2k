<template>
  <div class="page">
    <div class="head">
      <h1>我的文章与草稿</h1>
      <el-button type="primary" @click="$router.push({ name: 'articleEditorNew' })">写新文章</el-button>
    </div>

    <el-card>
      <template #header>未发布草稿（独立于线上版本）</template>
      <el-table :data="drafts" empty-text="暂无草稿" v-loading="loading">
        <el-table-column label="标题">
          <template #default="{ row }">
            <el-link type="primary" @click="openDraftRow(row)">{{ row.title || '（未命名）' }}</el-link>
            <el-tag v-if="row.conflict" type="danger" size="small" class="conflict-tag">线上有新版本，需合并</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="主题" width="90">
          <template #default="{ row }">{{ CareTopicTagMap[row.topic_tag as CareTopicTag] }}</template>
        </el-table-column>
        <el-table-column label="基准修订" width="100">
          <template #default="{ row }">
            <span v-if="row.article_id">v{{ row.base_revision_no }}</span>
            <span v-else>新文章</span>
          </template>
        </el-table-column>
        <el-table-column label="保存时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.saved_at) }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card class="articles-card">
      <template #header>已发布 / 已撤回</template>
      <el-table :data="articles" v-loading="loading" row-key="id">
        <el-table-column label="标题">
          <template #default="{ row }">
            <el-link type="primary" @click="openArticle(row)">{{ row.title }}</el-link>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)">{{ ArticleStatusMap[row.status as ArticleStatus] || row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="修订" width="80">
          <template #default="{ row }">v{{ row.revision_no }}</template>
        </el-table-column>
        <el-table-column label="阅读" prop="view_count" width="80" />
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="300">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="editArticle(row)">
              {{ row.has_draft ? '继续草稿' : '编辑' }}
            </el-button>
            <el-button link type="primary" size="small" @click="$router.push({ name: 'articleDetail', params: { id: row.id } })">查看</el-button>
            <el-button
              v-if="row.status === 'published'"
              link
              type="warning"
              size="small"
              @click="onWithdraw(row)"
            >撤回</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listMyArticles, listMyDrafts, withdrawArticle } from '@/api/article'
import {
  ArticleStatusMap,
  CareTopicTagMap,
  type ArticleDraftItem,
  type ArticleEditorMeta,
  type ArticleStatus,
  type CareTopicTag,
} from '@/constants/article'
import { formatDateTime } from '@/utils/dateFormat'

const router = useRouter()
const loading = ref(false)
const drafts = ref<ArticleDraftItem[]>([])
const articles = ref<ArticleEditorMeta[]>([])

onMounted(load)

async function load() {
  loading.value = true
  try {
    const [d, a] = await Promise.all([listMyDrafts(), listMyArticles()])
    drafts.value = d.list
    articles.value = a.list
  } finally {
    loading.value = false
  }
}

function openDraftRow(row: ArticleDraftItem) {
  if (row.article_id) {
    router.push({ name: 'articleEditor', params: { id: row.article_id }, query: { draft_id: row.draft_id } })
  } else {
    router.push({ name: 'articleEditorNew', query: { draft_id: row.draft_id } })
  }
}

function editArticle(row: ArticleEditorMeta) {
  const query = row.has_draft ? { draft_id: row.draft_id } : {}
  router.push({ name: 'articleEditor', params: { id: row.id }, query })
}
function openArticle(row: ArticleEditorMeta) {
  if (row.status === 'published') {
    router.push({ name: 'articleDetail', params: { id: row.id } })
  } else {
    editArticle(row)
  }
}

async function onWithdraw(row: ArticleEditorMeta) {
  await ElMessageBox.confirm(`撤回后访客将看不到《${row.title}》，内容与修订仍保留，可随时再编辑发布。`, '确认撤回', {
    type: 'warning',
  })
  await withdrawArticle(row.id)
  ElMessage.success('已撤回，访客端不可见')
  load()
}

function statusTagType(status: string) {
  if (status === 'published') return 'success'
  if (status === 'withdrawn') return 'info'
  return 'warning'
}
</script>

<style scoped>
.page { max-width: 1100px; margin: 0 auto; }
.head { display: flex; justify-content: space-between; align-items: center; }
.articles-card { margin-top: 16px; }
.conflict-tag { margin-left: 8px; }
</style>
