<template>
  <div class="page">
    <el-page-header @back="$router.back()" :content="editorTitle" />

    <!-- 线上已被重新发布：保留本地草稿，先合并再发布 -->
    <el-alert
      v-if="draft.conflict"
      class="conflict-banner"
      type="warning"
      :closable="false"
      show-icon
    >
      <template #title>
        线上已有新修订（第 {{ draft.online_revision_no }} 版，{{ formatDateTime(conflictDetails?.online_updated_at || draft.online_updated_at) }}），
        你的草稿基于第 {{ draft.base_revision_no }} 版。请先对照新版本合并修改，再重新发布；当前草稿不会被覆盖。
      </template>
      <div class="conflict-actions">
        <el-button size="small" @click="showOnline = !showOnline">
          {{ showOnline ? '收起线上新版本' : '查看线上新版本' }}
        </el-button>
        <el-button size="small" type="primary" :loading="reconciling" @click="onReconcile">
          已合并完，以线上第 {{ draft.online_revision_no }} 版为基准继续
        </el-button>
      </div>
    </el-alert>

    <el-row :gutter="16">
      <el-col :xs="24" :md="16">
        <el-card v-loading="loading">
          <el-form :model="form" label-position="top">
            <el-form-item label="标题" required>
              <el-input v-model="form.title" maxlength="255" show-word-limit placeholder="请输入文章标题" />
            </el-form-item>
            <el-form-item label="养护主题" required>
              <el-select v-model="form.topic_tag" placeholder="选择主题">
                <el-option v-for="(label, value) in CareTopicTagMap" :key="value" :value="value" :label="label" />
              </el-select>
            </el-form-item>
            <el-form-item label="封面图">
              <ImageUploader v-model="form.cover" />
            </el-form-item>
            <el-form-item label="正文" required>
              <el-input v-model="form.content" type="textarea" :rows="14" placeholder="支持换行分段" />
            </el-form-item>
            <el-form-item v-if="!isNew" label="发布备注（可选）">
              <el-input v-model="summary" maxlength="255" placeholder="本次修订说明，例如：补充夏季遮阴要点" />
            </el-form-item>
          </el-form>

          <template #footer>
            <div class="editor-footer">
              <div class="meta" v-if="draft.draft_id">
                草稿第 {{ draft.draft_id }} 号 · 基准修订 v{{ draft.base_revision_no }}
                <span v-if="draft.saved_at"> · 上次保存 {{ formatDateTime(draft.saved_at) }}</span>
              </div>
              <div class="actions">
                <el-button v-if="draft.draft_id" @click="onDiscard">放弃草稿</el-button>
                <el-button type="success" plain :loading="saving" @click="onSave">保存草稿</el-button>
                <el-button type="primary" :loading="publishing" @click="onPublish">发布</el-button>
              </div>
            </div>
          </template>
        </el-card>
      </el-col>

      <el-col :xs="24" :md="8" v-if="!isNew">
        <el-card class="side-card">
          <template #header>
            <div class="side-head">
              <span>历史修订</span>
              <el-button link type="primary" @click="loadRevisions">刷新</el-button>
            </div>
          </template>
          <el-timeline v-loading="revisionsLoading">
            <el-timeline-item
              v-for="r in revisions"
              :key="r.id"
              :timestamp="formatDateTime(r.created_at)"
              :type="r.revision_no === currentRevisionNo ? 'primary' : undefined"
            >
              <div class="rev-item">
                <strong>v{{ r.revision_no }}</strong>
                <span class="rev-title">{{ r.title }}</span>
                <div v-if="r.summary" class="rev-summary">{{ r.summary }}</div>
                <div class="rev-ops">
                  <el-button link type="primary" size="small" @click="previewRevision(r)">查看</el-button>
                  <el-button link type="warning" size="small" @click="onRestore(r)">恢复为新草稿</el-button>
                </div>
              </div>
            </el-timeline-item>
          </el-timeline>
          <el-empty v-if="!revisionsLoading && !revisions.length" description="暂无修订" :image-size="60" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 线上新版本对照 -->
    <el-drawer v-model="showOnline" title="线上当前版本（只读）" size="40%">
      <div v-if="onlineVersion">
        <h3>{{ onlineVersion.title }}</h3>
        <el-tag type="success">{{ CareTopicTagMap[onlineVersion.topic_tag] }}</el-tag>
        <el-image v-if="onlineVersion.cover" :src="onlineVersion.cover" fit="cover" class="online-cover" />
        <div class="online-content">{{ onlineVersion.content }}</div>
      </div>
    </el-drawer>

    <!-- 历史修订只读预览 -->
    <el-drawer v-model="showRevision" :title="`修订 v${previewingRevision?.revision_no ?? ''}`" size="40%">
      <div v-if="previewingRevision">
        <h3>{{ previewingRevision.title }}</h3>
        <el-tag type="success">{{ CareTopicTagMap[previewingRevision.topic_tag] }}</el-tag>
        <div v-if="previewingRevision.summary" class="rev-summary">备注：{{ previewingRevision.summary }}</div>
        <el-image v-if="previewingRevision.cover" :src="previewingRevision.cover" fit="cover" class="online-cover" />
        <div class="online-content">{{ previewingRevision.content }}</div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import ImageUploader from '@/components/common/ImageUploader.vue'
import {
  getArticle,
  getDraft,
  discardDraft,
  openDraft,
  publishDraft,
  reconcileDraft,
  restoreRevision,
  saveDraft,
  listRevisions,
} from '@/api/article'
import {
  CareTopicTagMap,
  type ArticleDraft,
  type ArticleRevision,
  type CareArticle,
  type CareTopicTag,
  type RevisionConflictDetails,
} from '@/constants/article'
import { ApiBusinessError } from '@/utils/request'
import { formatDateTime } from '@/utils/dateFormat'

const route = useRoute()
const router = useRouter()

const articleId = computed(() => (route.name === 'articleEditor' ? Number(route.params.id) || 0 : 0))
const draftIdQuery = computed(() => (route.query.draft_id ? Number(route.query.draft_id) : 0))
const isNew = computed(() => articleId.value === 0)

const loading = ref(true)
const saving = ref(false)
const publishing = ref(false)
const reconciling = ref(false)
const revisionsLoading = ref(false)

const draft = ref<ArticleDraft>(emptyDraft())
const conflictDetails = ref<RevisionConflictDetails | null>(null)
const form = reactive({ title: '', content: '', cover: '', topic_tag: '' as CareTopicTag | '' })
const summary = ref('')

const revisions = ref<ArticleRevision[]>([])
const showOnline = ref(false)
const onlineVersion = ref<CareArticle | null>(null)
const showRevision = ref(false)
const previewingRevision = ref<ArticleRevision | null>(null)

const currentRevisionNo = computed(() => draft.value.online_revision_no)
const editorTitle = computed(() => (isNew.value ? '写文章（独立草稿）' : `编辑文章 #${articleId.value}`))

function emptyDraft(): ArticleDraft {
  return {
    draft_id: 0,
    article_id: 0,
    title: '',
    content: '',
    cover: '',
    topic_tag: '' as CareTopicTag,
    base_revision_no: 0,
    saved_at: '',
    article_status: '',
    online_revision_no: 0,
    online_updated_at: '',
    conflict: false,
  }
}

function hydrate(d: ArticleDraft) {
  draft.value = d
  form.title = d.title
  form.content = d.content
  form.cover = d.cover
  form.topic_tag = d.topic_tag
}

onMounted(init)

async function init() {
  loading.value = true
  try {
    let d: ArticleDraft
    if (draftIdQuery.value) {
      // 从“我的草稿”进入：精确取那份草稿（可能是尚未发布的新文章草稿）。
      d = await getDraft(draftIdQuery.value)
    } else {
      // 从文章“编辑”进入：返回已有草稿或由线上版本播种的新草稿。
      d = await openDraft(articleId.value || undefined)
    }
    hydrate(d)
    if (!isNew.value) {
      loadRevisions()
    }
  } finally {
    loading.value = false
  }
}

async function loadRevisions() {
  if (isNew.value) return
  revisionsLoading.value = true
  try {
    const res = await listRevisions(articleId.value)
    revisions.value = res.list
  } finally {
    revisionsLoading.value = false
  }
}

function validate(): boolean {
  if (!form.title.trim()) {
    ElMessage.warning('请填写标题')
    return false
  }
  if (!form.topic_tag) {
    ElMessage.warning('请选择养护主题')
    return false
  }
  if (!form.content.trim()) {
    ElMessage.warning('正文不能为空')
    return false
  }
  return true
}

async function onSave() {
  if (!validate()) return
  saving.value = true
  try {
    const saved = await saveDraft({
      draft_id: draft.value.draft_id || undefined,
      article_id: articleId.value || undefined,
      title: form.title,
      content: form.content,
      cover: form.cover,
      topic_tag: form.topic_tag as CareTopicTag,
      base_revision_no: draft.value.base_revision_no,
    })
    hydrate(saved)
    if (isNew.value && saved.article_id === 0 && saved.draft_id) {
      // 保持在新草稿页，URL 用 draft_id 标记，防止刷新丢失
      router.replace({ name: 'articleEditorNew', query: { draft_id: saved.draft_id } })
    }
    ElMessage.success('草稿已保存（线上版本未改动）')
  } catch (e) {
    handlePossibleConflict(e)
  } finally {
    saving.value = false
  }
}

async function onPublish() {
  if (!validate()) return
  // 发布前先保存，保证草稿内容是最新的
  if (!draft.value.draft_id || isDirty()) {
    await onSave()
    if (!draft.value.draft_id) return
  }
  publishing.value = true
  try {
    const article = await publishDraft({
      draft_id: draft.value.draft_id,
      base_revision_no: draft.value.base_revision_no,
      summary: summary.value,
    })
    ElMessage.success(`发布成功，当前为第 ${article.revision_no} 版`)
    router.replace({ name: 'articleDetail', params: { id: article.id } })
  } catch (e) {
    handlePossibleConflict(e)
  } finally {
    publishing.value = false
  }
}

function isDirty(): boolean {
  return (
    form.title !== draft.value.title ||
    form.content !== draft.value.content ||
    form.cover !== draft.value.cover ||
    form.topic_tag !== draft.value.topic_tag
  )
}

function handlePossibleConflict(e: unknown) {
  if (e instanceof ApiBusinessError && e.code === 40901) {
    conflictDetails.value = (e.details as RevisionConflictDetails) ?? null
    // 刷新草稿状态（draft 仍在）并拉取线上新版本
    refreshConflictView()
    ElMessage.warning('线上刚被重新发布，草稿已保留，请先合并新版本')
    return
  }
  // 其他错误已由拦截器提示
}

async function refreshConflictView() {
  try {
    const fresh = await openDraft(articleId.value || undefined)
    // openDraft 可能返回的是已存在草稿
    hydrate(fresh)
    if (articleId.value) {
      onlineVersion.value = await getArticle(articleId.value)
    }
  } catch {
    /* ignore */
  }
}

async function onReconcile() {
  if (!draft.value.draft_id) return
  reconciling.value = true
  try {
    const rebased = await reconcileDraft(draft.value.draft_id)
    hydrate(rebased)
    conflictDetails.value = null
    showOnline.value = false
    ElMessage.success(`已重新基于线上第 ${rebased.base_revision_no} 版，可以发布了`)
  } finally {
    reconciling.value = false
  }
}

async function onDiscard() {
  if (!draft.value.draft_id) return
  await ElMessageBox.confirm('放弃这份草稿？线上版本与历史修订不受影响。', '确认', { type: 'warning' })
  await discardDraft(draft.value.draft_id)
  ElMessage.success('草稿已放弃')
  if (isNew.value) {
    router.replace({ name: 'articles' })
  } else {
    router.replace({ name: 'articleDetail', params: { id: articleId.value } })
  }
}

function previewRevision(r: ArticleRevision) {
  previewingRevision.value = r
  showRevision.value = true
}

async function onRestore(r: ArticleRevision) {
  try {
    await ElMessageBox.confirm(
      `将把 v${r.revision_no} 打开为一份新的独立草稿，历史修订保持不变。继续？`,
      '恢复为新草稿',
      { type: 'info' },
    )
  } catch {
    return
  }
  const restored = await restoreRevision(articleId.value, r.revision_no)
  hydrate(restored)
  ElMessage.success(`v${r.revision_no} 已恢复为新草稿，确认内容后再发布`)
  loadRevisions()
}
</script>

<style scoped>
.page { max-width: 1100px; margin: 0 auto; }
.conflict-banner { margin: 12px 0; }
.conflict-actions { margin-top: 8px; display: flex; gap: 8px; }
.side-card { margin-top: 12px; }
.side-head { display: flex; justify-content: space-between; align-items: center; }
.editor-footer { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
.editor-footer .meta { color: #909399; font-size: 12px; }
.editor-footer .actions { display: flex; gap: 8px; }
.rev-item { line-height: 1.6; }
.rev-title { margin-left: 8px; }
.rev-summary { color: #909399; font-size: 12px; margin: 4px 0; }
.rev-ops { display: flex; gap: 4px; }
.online-cover, .online-content { display: block; margin-top: 12px; }
.online-cover { width: 100%; border-radius: 8px; }
.online-content { white-space: pre-wrap; line-height: 1.9; color: #333; }
</style>
