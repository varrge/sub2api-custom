<template>
  <div class="mt-3 text-sm leading-6 text-gray-700 dark:text-gray-300">
    <div v-if="loading" class="flex items-center gap-2 text-gray-500 dark:text-gray-400">
      <span>{{ t('common.loading') }}</span>
      <button type="button" class="gb-rules-link" @click="emit('reload')">
        {{ t('groupBuy.retry') }}
      </button>
    </div>
    <div v-else-if="error" class="flex flex-wrap items-center gap-2" role="alert">
      <span class="text-red-600 dark:text-red-400">{{ error }}</span>
      <button type="button" class="gb-rules-link" @click="emit('reload')">
        {{ t('groupBuy.retry') }}
      </button>
    </div>
    <template v-else-if="documents.length">
      <div class="flex flex-wrap items-baseline gap-x-1">
        <label class="inline-flex items-center gap-1.5">
          <input
            type="checkbox"
            :checked="modelValue"
            :disabled="checkboxDisabled"
            :aria-describedby="hintId"
            @change="onToggle"
          />
          <span>{{ t('groupBuy.rulesConsent') }}</span>
        </label>
        <template v-for="(doc, index) in documents" :key="doc.id">
          <span v-if="index" aria-hidden="true">、</span>
          <button
            type="button"
            class="gb-rules-link inline font-medium"
            @click="openDocument(doc)"
          >
            <span class="[overflow-wrap:anywhere]">《{{ doc.title }}》</span>
            <span
              v-if="!doc.read_at"
              class="mb-1 ml-0.5 inline-block h-1.5 w-1.5 rounded-full bg-red-500 align-super"
              aria-hidden="true"
            />
            <span v-if="!doc.read_at" class="sr-only">（{{ t('groupBuy.ruleUnread') }}）</span>
          </button>
        </template>
      </div>
      <p
        :id="hintId"
        class="mt-1 text-xs"
        :class="hasUnread ? 'text-amber-600 dark:text-amber-400' : 'text-green-600 dark:text-green-400'"
      >
        {{ hasUnread ? t('groupBuy.rulesReadHint') : t('groupBuy.rulesReadyHint') }}
      </p>
    </template>
    <RulesReadDialog
      :document="activeDocument"
      :confirming="!!activeDocument && readingId === activeDocument.id"
      @close="closeDialog"
      @confirm="confirmRead"
    />
  </div>
</template>

<script lang="ts">
let consentCounter = 0
</script>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import RulesReadDialog from './RulesReadDialog.vue'
import type { PublishedMonthCardRule } from '@/types/monthCardRules'

const props = withDefaults(
  defineProps<{
    documents: PublishedMonthCardRule[]
    loading: boolean
    error: string
    modelValue: boolean
    disabled?: boolean
    readingId?: string
  }>(),
  { disabled: false, readingId: '' }
)
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  read: [document: PublishedMonthCardRule]
  reload: []
}>()
const { t } = useI18n()

const hintId = `rules-consent-hint-${++consentCounter}`
const activeId = ref('')
const pendingReadId = ref('')

const hasUnread = computed(() => props.documents.some((doc) => !doc.read_at))
const checkboxDisabled = computed(
  () =>
    props.disabled ||
    props.loading ||
    !!props.error ||
    !props.documents.length ||
    hasUnread.value
)
const activeDocument = computed(
  () => props.documents.find((doc) => doc.id === activeId.value) ?? null
)

function onToggle(event: Event) {
  if (checkboxDisabled.value) return
  emit('update:modelValue', (event.target as HTMLInputElement).checked)
}

function openDocument(doc: PublishedMonthCardRule) {
  activeId.value = doc.id
}

function closeDialog() {
  activeId.value = ''
  pendingReadId.value = ''
}

function confirmRead(doc: PublishedMonthCardRule) {
  if (doc.read_at) {
    closeDialog()
    return
  }
  if (props.readingId) return
  pendingReadId.value = doc.id
  emit('read', doc)
}

// 关闭弹窗只在服务端确认已读（read_at 更新）之后；失败保留红点与弹窗
watch(
  () => props.documents,
  (documents) => {
    if (pendingReadId.value) {
      const doc = documents.find((item) => item.id === pendingReadId.value)
      if (doc?.read_at) closeDialog()
    }
    if (activeId.value && !documents.some((item) => item.id === activeId.value)) {
      closeDialog()
    }
  }
)

// 已读请求结束但未更新 read_at：视为失败，解除等待状态，弹窗保持打开
watch(
  () => props.readingId,
  (now, previous) => {
    if (previous && !now && pendingReadId.value) {
      const doc = props.documents.find((item) => item.id === pendingReadId.value)
      if (!doc?.read_at) pendingReadId.value = ''
    }
  }
)
</script>

<style scoped>
.gb-rules-link {
  color: rgb(22 163 74);
  text-decoration: underline;
  text-underline-offset: 3px;
  transition: color 0.15s ease;
}
.gb-rules-link:hover:not(:disabled) {
  color: rgb(21 128 61);
}
.gb-rules-link:focus-visible {
  outline: 2px solid rgb(22 163 74 / 0.5);
  outline-offset: 2px;
  border-radius: 0.25rem;
}
.dark .gb-rules-link {
  color: rgb(74 222 128);
}
.dark .gb-rules-link:hover:not(:disabled) {
  color: rgb(134 239 172);
}
@media (prefers-reduced-motion: reduce) {
  .gb-rules-link {
    transition: none;
  }
}
</style>
