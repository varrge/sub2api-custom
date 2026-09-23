<template>
  <section class="gb-panel space-y-4 p-4 sm:p-5">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h2 class="font-semibold text-gray-900 dark:text-white">
        {{ t('groupBuy.ruleManagement') }}
      </h2>
      <span v-if="dirty" class="gb-pill gb-pill-gray">{{ t('groupBuy.rulesUnsaved') }}</span>
    </div>
    <p v-if="error" class="gb-notice gb-notice-error p-3" role="alert">
      {{ error }}
      <button type="button" class="ml-2 underline" @click="load">{{ t('groupBuy.retry') }}</button>
    </p>
    <div v-if="loading && !state" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>
    <template v-else-if="state">
      <div class="gb-strip space-y-2 p-3 text-sm text-gray-700 dark:text-gray-300">
        <p class="font-medium">
          {{ t('groupBuy.rulesPublished') }}
          <span v-if="state.published_at" class="text-xs text-gray-500 dark:text-gray-400">
            · {{ t('groupBuy.rulesPublishedAt', { time: exactDate(state.published_at) }) }}
          </span>
        </p>
        <p v-if="!publishedActive.length" class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('groupBuy.rulesNoPublished') }}
        </p>
        <ul v-else class="space-y-1">
          <li
            v-for="doc in publishedActive"
            :key="doc.id"
            class="flex min-w-0 items-baseline gap-2"
          >
            <span class="min-w-0 [overflow-wrap:anywhere]">{{ doc.title }}</span>
            <button
              type="button"
              class="gb-accent shrink-0 text-xs hover:underline"
              @click="preview(doc)"
            >
              {{ t('groupBuy.rulePreview') }}
            </button>
          </li>
        </ul>
      </div>

      <div class="grid gap-4 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]">
        <div class="min-w-0 space-y-2">
          <p class="gb-label">{{ t('groupBuy.rulesDraftSection') }}</p>
          <p v-if="!documents.length" class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('groupBuy.rulesEmptyDraft') }}
          </p>
          <div
            v-for="(doc, index) in documents"
            :key="doc.id"
            class="gb-sort-row flex w-full min-w-0 items-center gap-2 p-2 text-sm"
            :class="selectedId === doc.id ? 'border-primary-500/50 ring-1 ring-primary-500/30' : ''"
          >
            <button
              type="button"
              class="min-w-0 flex-1 text-left"
              @click="selectedId = doc.id"
            >
              <span class="[overflow-wrap:anywhere]">
                {{ doc.title.trim() || t('groupBuy.ruleUntitled') }}
              </span>
              <span class="mt-1 flex flex-wrap gap-1">
                <span v-if="!doc.active" class="gb-pill gb-pill-gray">
                  {{ t('groupBuy.ruleInactive') }}
                </span>
                <span v-else-if="isNew(doc)" class="gb-pill gb-pill-sky">
                  {{ t('groupBuy.ruleNew') }}
                </span>
                <span v-else-if="isChanged(doc)" class="gb-pill gb-pill-teal">
                  {{ t('groupBuy.ruleChanged') }}
                </span>
              </span>
            </button>
            <span class="flex shrink-0 flex-col">
              <button
                type="button"
                class="px-1 text-xs leading-4 text-gray-500 hover:text-gray-800 disabled:opacity-30 dark:hover:text-gray-200"
                :aria-label="t('groupBuy.up')"
                :disabled="index === 0 || saving || loading"
                @click="move(index, -1)"
              >
                ▲
              </button>
              <button
                type="button"
                class="px-1 text-xs leading-4 text-gray-500 hover:text-gray-800 disabled:opacity-30 dark:hover:text-gray-200"
                :aria-label="t('groupBuy.down')"
                :disabled="index === documents.length - 1 || saving || loading"
                @click="move(index, 1)"
              >
                ▼
              </button>
            </span>
          </div>
          <button
            type="button"
            class="btn btn-secondary w-full"
            :disabled="documents.length >= 20 || saving || loading"
            @click="addDocument"
          >
            {{ t('groupBuy.ruleAdd') }}
          </button>
        </div>

        <div v-if="selected" class="min-w-0 space-y-3">
          <label class="block text-sm">
            {{ t('groupBuy.ruleTitle') }}
            <input
              v-model="selected.title"
              class="input mt-1 w-full"
              maxlength="120"
              :disabled="saving || loading"
              :placeholder="t('groupBuy.ruleTitle')"
            />
          </label>
          <label class="block text-sm">
            {{ t('groupBuy.ruleContent') }}
            <textarea
              v-model="selected.content"
              class="input mt-1 w-full font-mono text-xs leading-5"
              rows="12"
              maxlength="50000"
              :disabled="saving || loading"
              :placeholder="t('groupBuy.ruleContent')"
            />
          </label>
          <div class="flex flex-wrap items-center gap-3">
            <label class="flex items-center gap-2 text-sm">
              <input v-model="selected.active" type="checkbox" :disabled="saving || loading" />
              {{ t('groupBuy.ruleActive') }}
            </label>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="saving || loading"
              @click="preview(selected)"
            >
              {{ t('groupBuy.rulePreview') }}
            </button>
            <button
              type="button"
              class="btn btn-danger btn-sm ml-auto"
              :disabled="saving || loading"
              @click="removeDocument(selectedIndex)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </div>
      </div>

      <p v-if="validationError && dirty" class="gb-notice gb-notice-warn p-3" role="alert">
        {{ validationError }}
      </p>
      <div class="flex flex-wrap items-center gap-3">
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="saving || loading || !dirty || !!validationError"
          @click="saveDraft"
        >
          {{ saving ? t('common.processing') : t('groupBuy.rulesSaveDraft') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="saving || loading || dirty"
          :title="dirty ? t('groupBuy.rulesUnsaved') : undefined"
          @click="confirmingPublish = true"
        >
          {{ t('groupBuy.rulesPublish') }}
        </button>
      </div>
    </template>

    <BaseDialog
      :show="confirmingPublish"
      :title="t('groupBuy.rulesPublish')"
      width="narrow"
      @close="confirmingPublish = false"
    >
      <p class="text-sm leading-6 text-gray-700 dark:text-gray-300">
        {{ t('groupBuy.rulesPublishConfirm', { count: publishChangedCount }) }}
      </p>
      <template #footer>
        <button class="btn btn-secondary" :disabled="saving" @click="confirmingPublish = false">
          {{ t('common.cancel') }}
        </button>
        <button class="btn btn-primary" :disabled="saving" @click="doPublish">
          {{ saving ? t('common.processing') : t('groupBuy.rulesPublish') }}
        </button>
      </template>
    </BaseDialog>

    <RulesReadDialog
      :document="previewSource"
      @close="previewSource = null"
      @confirm="previewSource = null"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import RulesReadDialog from './RulesReadDialog.vue'
import { useAdminMonthCardRules } from './useMonthCardRules'
import { useAppStore } from '@/stores/app'
import { exactDate } from './model'
import type { MonthCardRuleDraft, PublishedMonthCardRule } from '@/types/monthCardRules'
import './glass.css'

const { t } = useI18n()
const app = useAppStore()
const { state, loading, saving, error, load, save, publish } = useAdminMonthCardRules()

const documents = ref<MonthCardRuleDraft[]>([])
const selectedId = ref('')
const confirmingPublish = ref(false)
const previewSource = ref<{ id: string; title: string; content: string; version: string; read_at?: string | null } | null>(null)

const clone = (docs: MonthCardRuleDraft[]) => docs.map((doc) => ({ ...doc }))
const normalize = (docs: MonthCardRuleDraft[]) =>
  docs.map((doc) => ({ id: doc.id, title: doc.title, content: doc.content, active: !!doc.active }))

// 保存/发布成功后 composable 刷新 state，本地草稿随之重置
watch(
  state,
  (value) => {
    documents.value = value ? clone(value.documents) : []
    if (!documents.value.some((doc) => doc.id === selectedId.value)) {
      selectedId.value = documents.value[0]?.id ?? ''
    }
  },
  { immediate: true }
)

const dirty = computed(
  () =>
    !!state.value &&
    JSON.stringify(normalize(documents.value)) !== JSON.stringify(normalize(state.value.documents))
)
const selectedIndex = computed(() => documents.value.findIndex((doc) => doc.id === selectedId.value))
const selected = computed(() => (selectedIndex.value >= 0 ? documents.value[selectedIndex.value] : null))
const publishedActive = computed(() => state.value?.published_documents ?? [])

const validationError = computed(() => {
  const docs = documents.value
  if (docs.length > 20) return t('groupBuy.rulesInvalid')
  if (!docs.some((doc) => doc.active)) return t('groupBuy.rulesInvalid')
  for (const doc of docs) {
    if (!doc.title.trim() || !doc.content.trim()) return t('groupBuy.rulesInvalid')
    if (doc.title.length > 120 || doc.content.length > 50000) return t('groupBuy.rulesInvalid')
  }
  return ''
})

function publishedVersion(id: string) {
  return state.value?.published_documents.find((doc) => doc.id === id) ?? null
}
function isNew(doc: MonthCardRuleDraft) {
  return !publishedVersion(doc.id)
}
function isChanged(doc: MonthCardRuleDraft) {
  const published = publishedVersion(doc.id)
  return !!published && (published.title !== doc.title || published.content !== doc.content)
}

// 以服务端已保存草稿为准统计需要用户重读的篇数；仅排序变化不触发重读
const publishChangedCount = computed(() => {
  if (!state.value) return 0
  return state.value.documents.filter(
    (doc) => doc.active && (isNew(doc) || isChanged(doc))
  ).length
})

function addDocument() {
  if (documents.value.length >= 20 || saving.value || loading.value) return
  const doc: MonthCardRuleDraft = {
    id: crypto.randomUUID(),
    title: '',
    content: '',
    active: true
  }
  documents.value.push(doc)
  selectedId.value = doc.id
}
function move(index: number, direction: -1 | 1) {
  if (saving.value || loading.value) return
  const target = index + direction
  if (target < 0 || target >= documents.value.length) return
  const [item] = documents.value.splice(index, 1)
  documents.value.splice(target, 0, item)
}
function removeDocument(index: number) {
  if (index < 0 || saving.value || loading.value) return
  documents.value.splice(index, 1)
  if (!documents.value.some((doc) => doc.id === selectedId.value)) {
    selectedId.value = documents.value[0]?.id ?? ''
  }
}
function preview(doc: MonthCardRuleDraft | PublishedMonthCardRule) {
  previewSource.value = {
    id: doc.id,
    title: doc.title.trim() || t('groupBuy.ruleUntitled'),
    content: doc.content,
    version: 'version' in doc ? doc.version : 'draft',
    read_at: null
  }
}
async function saveDraft() {
  if (saving.value || loading.value || !dirty.value || validationError.value) return
  if (await save(clone(documents.value))) app.showSuccess(t('groupBuy.rulesSaved'))
}
async function doPublish() {
  if (saving.value || dirty.value) return
  if (await publish()) {
    confirmingPublish.value = false
    app.showSuccess(t('groupBuy.rulesPublishedSuccess'))
  }
}
onMounted(load)
</script>
