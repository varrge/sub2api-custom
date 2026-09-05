<template>
  <div class="space-y-6">
    <!-- 页头(独立形态下展示标题;后台形态 AppHeader 已有页面标题) -->
    <div v-if="!embedded">
      <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-3xl">{{ t('modelPlaza.title') }}</h1>
      <p class="mt-1.5 text-sm text-gray-500 dark:text-dark-400">{{ t('modelPlaza.description') }}</p>
    </div>

    <!-- 全局价格说明(管理员配置,Markdown) -->
    <div
      v-if="descriptionHtml"
      class="plaza-description rounded-2xl border border-gray-100 bg-white px-5 py-4 text-sm shadow-card dark:border-dark-700/50 dark:bg-dark-800/50"
      v-html="descriptionHtml"
    ></div>

    <!-- 未登录提示 -->
    <p
      v-if="!isAuthenticated"
      class="flex items-center gap-1.5 text-xs text-gray-400 dark:text-dark-500"
    >
      <Icon name="infoCircle" size="xs" class="h-3.5 w-3.5" />
      {{ t('modelPlaza.anonymousHint') }}
    </p>

    <!-- 加载/错误/空 -->
    <div v-if="loading" class="flex min-h-[240px] items-center justify-center">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-600/25 border-t-primary-600 dark:border-primary-400/25 dark:border-t-primary-400"></div>
    </div>
    <div
      v-else-if="error"
      class="rounded-2xl border border-red-200 bg-red-50 px-5 py-8 text-center text-sm text-red-600 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
    >
      {{ t('modelPlaza.loadFailed') }}
    </div>
    <template v-else>
      <!-- 筛选栏: 搜索框 + 品牌 Pill 胶囊 -->
      <div class="rounded-2xl border border-gray-100 bg-white p-4 shadow-card dark:border-dark-700/50 dark:bg-dark-800/50">
        <div class="flex flex-col gap-3.5">
          <!-- 搜索输入框 -->
          <div class="relative max-w-md">
            <Icon
              name="search"
              size="xs"
              class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400 dark:text-dark-400"
            />
            <input
              v-model="searchQuery"
              type="search"
              :aria-label="t('modelPlaza.searchPlaceholder')"
              :placeholder="t('modelPlaza.searchPlaceholder')"
              class="w-full rounded-xl border border-gray-200 bg-gray-50/50 py-2 pl-9 pr-8 text-sm text-gray-900 placeholder-gray-400 transition focus:border-primary-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-dark-700 dark:bg-dark-900/50 dark:text-white dark:placeholder-dark-400 dark:focus:border-primary-400 dark:focus:bg-dark-900"
            />
            <button
              v-if="searchQuery"
              type="button"
              :aria-label="t('modelPlaza.clearSearch')"
              class="absolute right-2.5 top-1/2 -translate-y-1/2 rounded-md p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-dark-200"
              @click="searchQuery = ''"
            >
              <Icon name="x" size="xs" class="h-3.5 w-3.5" />
            </button>
          </div>

          <!-- 品牌 Pills 筛选 -->
          <div class="flex flex-wrap items-center gap-2">
            <!-- 全部 -->
            <button
              type="button"
              :aria-pressed="selectedBrand === 'all'"
              class="inline-flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
              :class="selectedBrand === 'all'
                ? 'bg-primary-600 text-white shadow-sm dark:bg-primary-500'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-dark-200 dark:hover:bg-dark-600'"
              @click="selectedBrand = 'all'"
            >
              {{ t('modelPlaza.brands.all') }}
            </button>

            <!-- 动态品牌 -->
            <button
              v-for="brand in availableBrands"
              :key="brand.id"
              type="button"
              :aria-pressed="selectedBrand === brand.id"
              class="inline-flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
              :class="selectedBrand === brand.id
                ? 'bg-primary-600 text-white shadow-sm dark:bg-primary-500'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-dark-200 dark:hover:bg-dark-600'"
              @click="selectedBrand = brand.id"
            >
              <PlatformIcon :platform="brand.platform" size="xs" class="h-3.5 w-3.5" />
              <span>{{ brand.name }}</span>
            </button>
          </div>
        </div>
      </div>

      <!-- 模型计数 -->
      <div class="flex items-center justify-between text-xs text-gray-500 dark:text-dark-400">
        <span>{{ t('modelPlaza.modelCount', { count: filteredModels.length }) }}</span>
      </div>

      <!-- 模型卡片网格: 1/2/3 列响应式 -->
      <div
        v-if="filteredModels.length > 0"
        class="grid grid-cols-1 items-start gap-5 md:grid-cols-2 xl:grid-cols-3"
      >
        <PlazaModelCard
          v-for="model in filteredModels"
          :key="model.id"
          :model="model"
          @open-group-detail="handleOpenGroupDetail"
        />
      </div>

      <!-- 空状态 -->
      <div
        v-else
        class="rounded-2xl border border-dashed border-gray-300 px-5 py-12 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400"
      >
        {{ searchActive ? t('modelPlaza.noSearchResult') : t('modelPlaza.empty') }}
      </div>
    </template>

    <!-- 弹窗展示分组详情 (包含窄化的该模型定价表) -->
    <BaseDialog
      :show="showDetailDialog"
      :title="dialogTitle"
      width="extra-wide"
      @close="showDetailDialog = false"
    >
      <div v-if="activeGroupDetail" class="mt-2">
        <PlazaGroupSection :group="activeGroupDetail" />
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlazaGroupSection from './PlazaGroupSection.vue'
import PlazaModelCard from './PlazaModelCard.vue'
import type { ModelPlazaGroup, ModelPlazaResponse } from '@/api/modelPlaza'
import { useAuthStore } from '@/stores/auth'
import { useTemporaryRateNow } from '@/utils/temporary-rate'
import { aggregatePlazaModels, type AggregatedPlazaModel, type PlazaBrandInfo, type GroupModelVariant } from './plaza-models'

const props = defineProps<{
  response: ModelPlazaResponse | null
  loading: boolean
  error?: boolean
  /** 后台内嵌形态(AppLayout 内):隐藏页头。 */
  embedded?: boolean
}>()

const { t } = useI18n()
const authStore = useAuthStore()
const temporaryRateNow = useTemporaryRateNow()
const isAuthenticated = computed(() => authStore.isAuthenticated)

const selectedBrand = ref<string>('all')
const searchQuery = ref('')

const searchActive = computed(() => searchQuery.value.trim() !== '' || selectedBrand.value !== 'all')

const descriptionHtml = computed(() => {
  const md = props.response?.description?.trim()
  if (!md) return ''
  return DOMPurify.sanitize(marked.parse(md) as string)
})

const allAggregatedModels = computed(() => {
  const groups = props.response?.groups ?? []
  return aggregatePlazaModels(groups, temporaryRateNow.value)
})

const availableBrands = computed<PlazaBrandInfo[]>(() => {
  const map = new Map<string, PlazaBrandInfo>()
  for (const m of allAggregatedModels.value) {
    if (!map.has(m.brand.id)) {
      map.set(m.brand.id, m.brand)
    }
  }
  return Array.from(map.values()).sort((a, b) => a.name.localeCompare(b.name))
})

const filteredModels = computed<AggregatedPlazaModel[]>(() => {
  let list = allAggregatedModels.value
  if (selectedBrand.value !== 'all') {
    list = list.filter((m) => m.brand.id === selectedBrand.value)
  }
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter((m) => m.name.toLowerCase().includes(q))
  }
  return list
})

// 详情弹窗
const showDetailDialog = ref(false)
const activeGroupDetail = ref<ModelPlazaGroup | null>(null)
const targetModelName = ref<string>('')

const dialogTitle = computed(() => {
  if (!activeGroupDetail.value) return t('modelPlaza.groupDetail')
  return `${activeGroupDetail.value.name} - ${targetModelName.value || t('modelPlaza.pricingDetail')}`
})

function handleOpenGroupDetail({ group, model }: GroupModelVariant) {
  targetModelName.value = model.name
  activeGroupDetail.value = {
    ...group,
    models: [model]
  }
  showDetailDialog.value = true
}
</script>

<style scoped>
.plaza-description {
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.plaza-description :deep(h1),
.plaza-description :deep(h2),
.plaza-description :deep(h3) {
  margin-top: 0.75rem;
  margin-bottom: 0.5rem;
  font-weight: 600;
  color: rgb(17 24 39);
}

:global(.dark) .plaza-description :deep(h1),
:global(.dark) .plaza-description :deep(h2),
:global(.dark) .plaza-description :deep(h3) {
  color: rgb(255 255 255);
}

.plaza-description :deep(p) {
  margin-bottom: 0.5rem;
  color: rgb(55 65 81);
}

:global(.dark) .plaza-description :deep(p) {
  color: rgb(229 231 235);
}

.plaza-description :deep(a) {
  color: rgb(37 99 235);
  text-decoration: underline;
  text-underline-offset: 4px;
}

.plaza-description :deep(ul) {
  margin-bottom: 0.5rem;
  list-style-type: disc;
  padding-left: 1.25rem;
}

.plaza-description :deep(ol) {
  margin-bottom: 0.5rem;
  list-style-type: decimal;
  padding-left: 1.25rem;
}

.plaza-description :deep(li) {
  margin-bottom: 0.125rem;
}

.plaza-description :deep(code) {
  border-radius: 0.25rem;
  background-color: rgb(243 244 246);
  padding: 0.125rem 0.375rem;
  font-family: monospace;
  font-size: 0.75rem;
}

:global(.dark) .plaza-description :deep(code) {
  background-color: rgb(31 41 55);
}

.plaza-description :deep(blockquote) {
  margin-top: 0.5rem;
  margin-bottom: 0.5rem;
  border-left-width: 4px;
  border-color: rgb(209 213 219);
  padding-left: 0.75rem;
  color: rgb(75 85 99);
}

:global(.dark) .plaza-description :deep(blockquote) {
  border-color: rgb(75 85 99);
  color: rgb(156 163 175);
}
</style>
