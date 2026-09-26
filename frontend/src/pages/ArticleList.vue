<template>
  <div class="page">
    <div class="head-row">
      <h1>养护知识文章</h1>
      <el-button v-if="auth.isLoggedIn" type="primary" @click="$router.push({ name: 'articleNew' })">写文章（草稿）</el-button>
    </div>

    <el-radio-group v-if="auth.isLoggedIn" v-model="scope" class="scope-tabs" @change="onScopeChange">
      <el-radio-button value="public">线上文章</el-radio-button>
      <el-radio-button value="mine">我的文章（含草稿/已撤回）</el-radio-button>
    </el-radio-group>

    <template v-if="scope === 'public'">
      <div class="topic-tabs">
        <el-radio-group v-model="topicTag" @change="onTopicChange">
          <el-radio-button value="">全部</el-radio-button>
          <el-radio-button v-for="(label, value) in CareTopicTagMap" :key="value" :value="value">{{ label }}</el-radio-button>
        </el-radio-group>
      </div>
      <SearchFilter @search="onSearch" @reset="onReset" />
    </template>

    <el-row :gutter="16">
      <el-col v-for="a in items" :key="a.id" :xs="12" :sm="8" :md="6">
        <CareArticleCard
          :article="a"
          :owner-view="scope === 'mine'"
          @click="onOpen(a)"
        />
      </el-col>
    </el-row>
    <el-empty v-if="!items.length" :description="scope === 'mine' ? '还没有文章，去写第一篇吧' : '暂无文章'" />
    <el-pagination v-if="total > 0" layout="prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="onPage" class="pager" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import CareArticleCard from '@/components/common/CareArticleCard.vue'
import SearchFilter from '@/components/common/SearchFilter.vue'
import { listArticles } from '@/api/article'
import { CareTopicTagMap, type ArticleOwnerView, type CareArticle } from '@/constants/article'
import { useAuthStore } from '@/stores/authStore'

const auth = useAuthStore()
const router = useRouter()

const scope = ref<'public' | 'mine'>('public')
const topicTag = ref('')
const keyword = ref('')
const page = ref(1)
const pageSize = 8

// 线上视图元素是 CareArticle；我的视图元素是 ArticleOwnerView（带 draft/conflict）。
const items = ref<Array<CareArticle | ArticleOwnerView>>([])
const total = ref(0)

onMounted(load)

async function load() {
  if (scope.value === 'mine') {
    const res = await listArticles({ page: page.value, page_size: pageSize, scope: 'mine' })
    items.value = res.list
    total.value = res.total
    return
  }
  const res = await listArticles({ page: page.value, page_size: pageSize, topic_tag: topicTag.value, keyword: keyword.value })
  items.value = res.list
  total.value = res.total
}

function onOpen(a: CareArticle | ArticleOwnerView) {
  if (scope.value === 'mine') {
    router.push({ name: 'articleEdit', params: { id: a.id } })
  } else {
    router.push(`/articles/${a.id}`)
  }
}
function onScopeChange() {
  page.value = 1
  load()
}
function onTopicChange() {
  page.value = 1
  load()
}
function onSearch(kw: string) {
  keyword.value = kw
  page.value = 1
  load()
}
function onReset() {
  topicTag.value = ''
  keyword.value = ''
  page.value = 1
  load()
}
function onPage(p: number) {
  page.value = p
  load()
}
</script>

<style scoped>
.page { max-width: 1200px; margin: 0 auto; }
.head-row { display: flex; align-items: center; justify-content: space-between; }
.scope-tabs { margin-bottom: 12px; }
.topic-tabs { margin: 12px 0; }
.pager { margin-top: 20px; justify-content: center; }
</style>
