<template>
  <BaseDialog :show="show" :title="`${detail?.group.group_name || groupName} · ${t('groupEvaluations.title')}`" width="wide" @close="emit('close')">
    <div class="space-y-5">
      <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('groupEvaluations.description') }}</p>
      <p v-if="error" class="rounded-xl bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">{{ error }}</p>
      <p v-if="loading && !detail" class="py-10 text-center text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
      <template v-if="detail">
        <div class="flex flex-wrap items-center justify-between gap-2 rounded-xl bg-gray-50 px-4 py-3 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
          <span>{{ detail.group.automatic ? t('groupEvaluations.intervalSummary', { seconds: detail.group.interval_seconds }) : t('groupEvaluations.manualOnly') }}</span>
          <span v-if="detail.group.running" role="status">{{ t('groupEvaluations.running') }}</span>
          <span v-else-if="detail.group.automatic && detail.group.next_run_at">{{ t('groupEvaluations.nextRun', { time: formatTime(detail.group.next_run_at) }) }}</span>
        </div>
        <div v-if="kinds.length > 1" class="tabs inline-flex" role="tablist" :aria-label="t('groupEvaluations.tests')">
          <button v-for="kind in kinds" :key="kind" type="button" role="tab" class="tab" :class="activeKind === kind ? 'tab-active' : ''" :aria-selected="activeKind === kind" @click="activeKind = kind">
            {{ t(`groupEvaluations.${kind}`) }}
          </button>
        </div>
        <h4 v-else-if="kinds.length" class="font-semibold">{{ t(`groupEvaluations.${kinds[0]}`) }}</h4>
        <p v-if="!kinds.length" class="py-8 text-center text-sm text-gray-500">{{ t('groupEvaluations.unavailable') }}</p>
        <p v-else-if="!records.length" class="py-10 text-center text-sm text-gray-500">{{ t('groupEvaluations.noResults') }}</p>
        <template v-else>
          <div v-if="activeKind === 'candy'" class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-gray-100 p-4 dark:border-dark-700">
            <div>
              <p class="text-xs text-gray-500">{{ t('groupEvaluations.accuracy', { count: evaluated }) }}</p>
              <strong class="text-3xl text-gray-900 dark:text-white">{{ evaluated ? `${Math.round(passed / evaluated * 100)}%` : '—' }}</strong>
              <p class="text-xs text-gray-500">{{ t('groupEvaluations.passedCount', { passed, count: evaluated }) }}</p>
              <p class="mt-1 text-xs text-gray-500">{{ t('groupEvaluations.errorCount', { count: failed }) }}</p>
            </div>
            <p class="max-w-sm text-xs text-gray-500">{{ t('groupEvaluations.referenceHint') }}</p>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h4 class="font-semibold text-gray-900 dark:text-white">{{ t('groupEvaluations.recent', { kind: t(`groupEvaluations.${activeKind}`) }) }}</h4>
            <span class="text-xs text-gray-500">{{ t('groupEvaluations.recordCount', { count: records.length }) }}</span>
          </div>
          <div v-if="activeKind === 'candy'" class="overflow-x-auto rounded-xl border border-gray-100 dark:border-dark-700">
            <table class="w-full text-left text-sm">
              <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr>
                <th class="px-3 py-2">{{ t('groupEvaluations.timeModel') }}</th>
                <th class="px-3 py-2">{{ t('groupEvaluations.answer') }}</th>
                <th class="px-3 py-2">{{ t('groupEvaluations.result') }}</th>
              </tr></thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="record in records" :key="record.id" data-testid="candy-record">
                  <td class="px-3 py-3"><time>{{ formatTime(record.created_at) }}</time><small class="mt-1 block text-gray-500">{{ record.model }} · {{ record.effort }}</small><small class="mt-1 block text-gray-500">{{ record.prompt_version }}</small></td>
                  <td class="max-w-xs whitespace-pre-wrap break-words px-3 py-3"><code>{{ record.answer || '—' }}</code><small v-if="record.expected_answer" class="mt-1 block text-gray-500">{{ t('groupEvaluations.referenceAnswer') }} {{ record.expected_answer }}</small></td>
                  <td class="px-3 py-3"><span :class="record.status === 'passed' ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'">{{ t(`groupEvaluations.status.${record.status}`) }}</span><small v-if="record.error" class="mt-1 block text-gray-500">{{ record.error }}</small></td>
                </tr>
              </tbody>
            </table>
          </div>
          <template v-else>
            <div class="max-h-64 space-y-1 overflow-y-auto" :aria-label="t('groupEvaluations.pelicanHistory')">
              <button v-for="record in records" :key="record.id" type="button" data-testid="pelican-record" class="flex w-full items-center justify-between gap-3 rounded-xl border p-3 text-left text-sm" :class="selectedPelican?.id === record.id ? 'border-primary-400 bg-primary-50 dark:bg-primary-900/20' : 'border-gray-100 dark:border-dark-700'" :aria-pressed="selectedPelican?.id === record.id" @click="selectedPelicanId = record.id">
                <span>{{ formatTime(record.created_at) }}<small class="mt-1 block text-gray-500">{{ record.model }} · {{ record.effort }}</small></span>
                <span>{{ t(`groupEvaluations.status.${record.status}`) }}</span>
              </button>
            </div>
            <template v-if="selectedPelican">
              <p class="text-xs text-gray-500">{{ t('groupEvaluations.selectedRecord', { time: formatTime(selectedPelican.created_at) }) }}</p>
              <p v-if="selectedPelican.error" class="text-sm text-amber-700 dark:text-amber-400" role="status">{{ selectedPelican.error }}</p>
              <iframe v-if="preview" :key="selectedPelican.id" :srcdoc="preview" sandbox="" referrerpolicy="no-referrer" :title="t('groupEvaluations.previewTitle')" class="h-96 w-full rounded-xl border border-gray-200 bg-white dark:border-dark-700" />
              <p v-else class="text-sm text-gray-500">{{ t('groupEvaluations.noPreview') }}</p>
              <p class="text-xs text-gray-500">{{ t('groupEvaluations.staticPreview') }}</p>
            </template>
          </template>
        </template>
        <p class="rounded-xl bg-blue-50 p-3 text-xs leading-relaxed text-blue-800 dark:bg-blue-900/20 dark:text-blue-200">{{ t('groupEvaluations.diagnosticNotice') }}</p>
      </template>
    </div>
    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-between gap-3">
        <span class="text-xs text-gray-500">{{ t('groupEvaluations.readOnly') }}</span>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="load()">{{ t('common.refresh') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { getEvaluationDetail, type EvaluationDetail, type EvaluationKind } from '@/api/groupEvaluations'
import { recentEvaluationRecords } from './evaluationHistory'
import { buildSafePreview } from './safePreview'

const props = withDefaults(defineProps<{ show: boolean; groupId: number | null; groupName?: string; admin?: boolean }>(), { groupName: '', admin: false })
const emit = defineEmits<{ close: []; updated: [] }>()
const { t, locale } = useI18n()
const detail = ref<EvaluationDetail | null>(null)
const loading = ref(false)
const error = ref('')
const activeKind = ref<EvaluationKind>('candy')
const selectedPelicanId = ref<number | null>(null)
let controller: AbortController | null = null
let poll: ReturnType<typeof setTimeout> | undefined

const kinds = computed<EvaluationKind[]>(() => {
  const group = detail.value?.group
  if (!group?.enabled) return []
  return (['candy', 'pelican'] as const).filter(kind => group[`${kind}_enabled`])
})
const records = computed(() => kinds.value.includes(activeKind.value) ? recentEvaluationRecords(detail.value?.records[activeKind.value]) : [])
const passed = computed(() => records.value.filter(record => record.status === 'passed').length)
const evaluated = computed(() => records.value.filter(record => record.status === 'passed' || record.status === 'mismatch').length)
const failed = computed(() => records.value.filter(record => record.status === 'error').length)
const selectedPelican = computed(() => records.value.find(record => record.id === selectedPelicanId.value) || records.value[0])
const preview = computed(() => activeKind.value === 'pelican' && selectedPelican.value?.html ? buildSafePreview(selectedPelican.value.html) : '')

function formatTime(value: string) {
  return new Date(value).toLocaleString(locale.value, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
}

function stop() {
  controller?.abort()
  controller = null
  clearTimeout(poll)
  loading.value = false
}

async function load() {
  if (!props.show || props.groupId === null) return
  stop()
  const current = new AbortController()
  controller = current
  loading.value = true
  error.value = ''
  try {
    const result = await getEvaluationDetail(props.groupId, props.admin, current.signal)
    if (current.signal.aborted) return
    detail.value = result
    if (!kinds.value.includes(activeKind.value)) activeKind.value = kinds.value[0] || 'candy'
    emit('updated')
    if (result.group.running) poll = setTimeout(() => void load(), 2500)
  } catch {
    if (!current.signal.aborted) {
      detail.value = null
      error.value = t('groupEvaluations.loadFailed')
    }
  } finally {
    if (controller === current) loading.value = false
  }
}

watch(() => [props.show, props.groupId, props.admin], () => {
  stop()
  detail.value = null
  selectedPelicanId.value = null
  activeKind.value = 'candy'
  if (props.show) void load()
}, { immediate: true })
onBeforeUnmount(stop)
</script>
