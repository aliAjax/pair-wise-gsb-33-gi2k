<template>
  <div class="page">
    <el-page-header @back="$router.back()" :content="isNew ? '写文章（独立草稿）' : '编辑文章（独立草稿）'" />

    <el-alert
      v-if="isNew"
      type="info"
      :closable="false"
      title="当前编辑的是独立草稿，保存不会影响线上；点“发布”后访客才能看到。"
      class="banner"
    />
    <template v-else>
      <el-alert
        v-if="view?.conflict"
        type="error"
        :closable="false"
        show-icon
        class="banner"
        title="线上已被别人重新发布，你的本地草稿已保留"
        description="直接发布会被拒绝。请先与新版本重新合并：可以查看新版本差异，或以新版本为基础继续修改。"
      >
        <el-button size="small" type="primary" @click="openMerge">去重新合并</el-button>
      </el-alert>
      <el-alert
        v-else
        type="info"
        :closable="false"
        class="banner"
        :title="`当前编辑独立草稿，不影响线上。页面修订号 v${liveRevision}；草稿最近保存于 ${formatDateTime(savedAt)}。`"
      />
    </template>

    <el-card class="editor" v-loading="loading">
      <el-form :model="form" label-width="80px">
        <el-form-item label="标题">
          <el-input v-model="form.title" maxlength="255" show-word-limit placeholder="文章标题" />
        </el-form-item>
        <el-form-item label="专题">
          <el-select v-model="form.topic_tag" placeholder="选择专题">
            <el-option v-for="(label, value) in CareTopicTagMap" :key="value" :label="label" :value="value" />
          </el-select>
        </el-form-item>
        <el-form-item label="封面">
          <ImageUploader v-model="form.cover" />
        </el-form-item>
        <el-form-item label="正文">
          <el-input v-model="form.content" type="textarea" :rows="14" placeholder="养护要点、操作步骤、注意事项……" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" plain :loading="saving" @click="saveDraft">保存草稿</el-button>
          <el-button type="success" :loading="publishing" @click="publish">发布到线上</el-button>
          <el-button v-if="!isNew && isOnline" type="warning" plain :loading="offlining" @click="offline">撤回线上版本</el-button>
          <el-button v-if="!isNew" @click="openRevisionsDrawer">历史修订</el-button>
          <span v-if="lastSavedHint" class="saved-hint">草稿已于 {{ formatDateTime(lastSavedHint) }} 保存</span>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 冲突合并：左本地草稿 / 右新版本（线上快照） -->
    <el-dialog v-model="mergeVisible" title="重新合并：线上有新版本" width="860px">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        title="为避免盖掉别人的新版本，请选择合并方式；合并后仍是草稿，确认无误再发布。"
        class="banner"
      />
      <el-row :gutter="12">
        <el-col :span="12">
          <div class="merge-head">我的本地草稿（基线 v{{ baseRevision }}）</div>
          <el-input :model-value="form.content" type="textarea" :rows="12" readonly />
        </el-col>
        <el-col :span="12">
          <div class="merge-head">线上新版本（v{{ liveRevision }}）</div>
          <el-input :model-value="liveSnapshot?.content || ''" type="textarea" :rows="12" readonly />
        </el-col>
      </el-row>
      <template #footer>
        <el-button @click="mergeVisible = false">稍后处理（保留草稿）</el-button>
        <el-button type="warning" @click="useLiveAsBase">以新版本为基础重改</el-button>
        <el-button type="primary" @click="rebaseKeepMine">我已在草稿里合并好，刷新基线</el-button>
      </template>
    </el-dialog>

    <!-- 历史修订 -->
    <el-drawer v-model="revisionsVisible" title="历史修订" size="640px">
      <el-table :data="revisions" v-loading="revisionsLoading" stripe>
        <el-table-column label="修订号" width="80">
          <template #default="{ row }">v{{ row.revision }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="revisionTagType(row.action as RevisionAction)">{{ RevisionActionMap[row.action as RevisionAction] || row.action }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="160" show-overflow-tooltip />
        <el-table-column label="操作人" width="110">
          <template #default="{ row }">{{ row.operator_name || `用户${row.operator_id}` }}</template>
        </el-table-column>
        <el-table-column label="时间" width="150">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link type="primary" @click="previewRevision(row)">查看</el-button>
            <el-button size="small" link type="warning" @click="restoreRevision(row)">恢复为草稿</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-drawer>

    <!-- 查看某次历史快照 -->
    <el-dialog v-model="previewVisible" :title="preview ? `修订 v${preview.revision}（${RevisionActionMap[preview.action]}）` : '历史修订'" width="760px">
      <template v-if="preview">
        <h3>{{ preview.title }}</h3>
        <el-tag size="small" type="success">{{ CareTopicTagMap[preview.topic_tag] }}</el-tag>
        <el-divider />
        <div class="preview-content" style="white-space: pre-wrap">{{ preview.content }}</div>
      </template>
      <template #footer>
        <el-button @click="previewVisible = false">关闭</el-button>
        <el-button type="warning" @click="preview && restoreRevision(preview)">恢复为新草稿</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import ImageUploader from '@/components/common/ImageUploader.vue'
import {
  CareTopicTagMap,
  RevisionActionMap,
  type ArticleOwnerView,
  type ArticleRevision,
  type CareTopicTag,
  type RevisionAction,
} from '@/constants/article'
import {
  createArticle,
  getArticleRevision,
  offlineArticle,
  openArticleEdit,
  publishArticle,
  restoreArticleRevision,
  saveArticleDraft,
  listArticleRevisions,
} from '@/api/article'
import { BizError } from '@/utils/request'
import { formatDateTime } from '@/utils/dateFormat'

const REVISION_CONFLICT_CODE = 40901

const route = useRoute()
const router = useRouter()

const articleId = computed(() => (route.params.id ? Number(route.params.id) : NaN))
const isNew = computed(() => Number.isNaN(articleId.value))

const loading = ref(false)
const saving = ref(false)
const publishing = ref(false)
const offlining = ref(false)

const view = ref<ArticleOwnerView | null>(null)
const form = ref({ title: '', content: '', cover: '', topic_tag: 'pruning' as CareTopicTag })
const baseRevision = ref(0)
const lastSavedHint = ref<string>('')

const liveRevision = computed(() => view.value?.published_revision ?? 0)
const savedAt = computed(() => view.value?.draft?.saved_at || '')
const isOnline = computed(() => view.value?.status === 'published')

const mergeVisible = ref(false)
const liveSnapshot = computed(() =>
  view.value
    ? { title: view.value.title, content: view.value.content, cover: view.value.cover, topic_tag: view.value.topic_tag }
    : null,
)

const revisionsVisible = ref(false)
const revisionsLoading = ref(false)
const revisions = ref<ArticleRevision[]>([])
const previewVisible = ref(false)
const preview = ref<ArticleRevision | null>(null)

onMounted(load)

async function load() {
  if (isNew.value) return
  loading.value = true
  try {
    view.value = await openArticleEdit(articleId.value)
    hydrateFormFromDraft(view.value)
  } finally {
    loading.value = false
  }
}

function hydrateFormFromDraft(v: ArticleOwnerView) {
  form.value = {
    title: v.draft.title,
    content: v.draft.content,
    cover: v.draft.cover,
    topic_tag: v.draft.topic_tag,
  }
  baseRevision.value = v.draft.base_revision
  lastSavedHint.value = v.draft.saved_at || ''
}

function validate(): boolean {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写标题')
    return false
  }
  if (!form.value.content.trim()) {
    ElMessage.warning('请填写正文')
    return false
  }
  if (!form.value.topic_tag) {
    ElMessage.warning('请选择专题')
    return false
  }
  return true
}

async function saveDraft() {
  if (!validate()) return
  saving.value = true
  try {
    if (isNew.value) {
      view.value = await createArticle({ ...form.value })
      router.replace({ name: 'articleEdit', params: { id: view.value.id } })
      hydrateFormFromDraft(view.value)
    } else {
      view.value = await saveArticleDraft(articleId.value, { ...form.value, base_revision: baseRevision.value })
      hydrateFormFromDraft(view.value)
    }
    ElMessage.success('草稿已保存，线上未受影响')
  } finally {
    saving.value = false
  }
}

async function publish() {
  if (!validate()) return
  // 新文章必须先存成草稿，发布才有留档载体。
  if (isNew.value) {
    await saveDraft()
  }
  publishing.value = true
  try {
    view.value = await publishArticle(articleId.value, baseRevision.value)
    hydrateFormFromDraft(view.value)
    ElMessage.success(`发布成功，当前页面修订号 v${view.value.published_revision}`)
  } catch (e) {
    // 线上被别人重新发布：本地草稿保留，提示重新合并，绝不盖掉新版本。
    if (e instanceof BizError && e.bizCode === REVISION_CONFLICT_CODE) {
      await load()
      mergeVisible.value = true
    }
  } finally {
    publishing.value = false
  }
}

async function offline() {
  try {
    await ElMessageBox.confirm('撤回后访客列表和详情都看不到这篇文章，内容与修订记录仍保留。确定撤回？', '撤回确认', {
      type: 'warning',
    })
  } catch {
    return
  }
  offlining.value = true
  try {
    view.value = await offlineArticle(articleId.value)
    hydrateFormFromDraft(view.value)
    ElMessage.success('已撤回，访客不可见；草稿与历史修订仍在')
  } finally {
    offlining.value = false
  }
}

function openMerge() {
  mergeVisible.value = true
}

// 直接以线上新版本为草稿基础（本地草稿被新版本覆盖前再让用户确认一次）。
async function useLiveAsBase() {
  try {
    await ElMessageBox.confirm('将用线上新版本覆盖当前编辑器内容（本地草稿仍可从历史修订找回），继续？', '确认', {
      type: 'warning',
    })
  } catch {
    return
  }
  if (liveSnapshot.value) {
    form.value = { ...liveSnapshot.value }
    baseRevision.value = liveRevision.value
  }
  mergeVisible.value = false
  await saveDraft()
}

// 作者已在编辑框手工合并好两边内容：只把基线刷到最新修订号再保存，之后即可发布。
async function rebaseKeepMine() {
  baseRevision.value = liveRevision.value
  mergeVisible.value = false
  await saveDraft()
}

async function openRevisionsDrawer() {
  revisionsVisible.value = true
  revisionsLoading.value = true
  try {
    revisions.value = await listArticleRevisions(articleId.value)
  } finally {
    revisionsLoading.value = false
  }
}

async function previewRevision(row: ArticleRevision) {
  preview.value = await getArticleRevision(articleId.value, row.id)
  previewVisible.value = true
}

async function restoreRevision(row: ArticleRevision) {
  try {
    await ElMessageBox.confirm(`将把修订 v${row.revision} 的内容恢复成一份新草稿，线上版本不变。确定恢复？`, '恢复修订', {
      type: 'warning',
    })
  } catch {
    return
  }
  view.value = await restoreArticleRevision(articleId.value, row.id)
  hydrateFormFromDraft(view.value)
  revisionsVisible.value = false
  previewVisible.value = false
  ElMessage.success('已恢复为新草稿，确认后可再次发布')
}

function revisionTagType(action: RevisionAction) {
  if (action === 'publish') return 'success'
  if (action === 'offline') return 'warning'
  return 'info'
}
</script>

<style scoped>
.page { max-width: 960px; margin: 0 auto; }
.banner { margin: 12px 0; }
.editor { margin-top: 12px; }
.saved-hint { margin-left: 12px; color: #888; font-size: 12px; }
.merge-head { font-weight: 600; margin-bottom: 6px; color: #3c8d5c; }
.preview-content { line-height: 1.9; color: #333; max-height: 50vh; overflow: auto; }
</style>
