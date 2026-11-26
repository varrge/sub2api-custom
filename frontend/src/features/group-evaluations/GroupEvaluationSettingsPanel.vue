<template>
  <section class="space-y-4">
    <div class="card !rounded-3xl !border-0 p-5 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('groupEvaluations.admin.title') }}</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('groupEvaluations.admin.description') }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button v-if="configs.some(dirty)" type="button" class="btn btn-ghost btn-sm" :disabled="loading || saving" @click="load()">{{ t('groupEvaluations.admin.discardReload') }}</button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || saving || configs.some(dirty)" @click="load()">{{ t('common.refresh') }}</button>
        </div>
      </div>
    </div>
    <p v-if="error" class="rounded-xl bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">{{ error }}</p>
    <div v-if="loading && !configs.length" class="card py-10 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
    <div v-else class="space-y-3">
      <p v-if="!configs.length" class="card p-5 text-sm text-gray-500">{{ t('groupEvaluations.admin.noGroups') }}</p>
      <label v-if="configs.length" class="block">
        <span class="input-label">{{ t('groupEvaluations.admin.selectGroup') }}</span>
        <select v-model="editingId" class="input max-w-lg" :disabled="saving">
          <option v-for="config in configs" :key="config.group_id" :value="config.group_id">{{ config.group_name }} · {{ config.group_platform }} · #{{ config.group_id }}{{ dirty(config) ? ' *' : '' }}</option>
        </select>
      </label>
      <p class="text-xs text-gray-500">{{ t('groupEvaluations.admin.budgetHint') }}</p>
      <article v-for="config in editingConfigs" :key="config.group_id" class="card !rounded-2xl !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700">
        <header class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <div class="min-w-0"><h3 class="truncate font-semibold text-gray-900 dark:text-white">{{ config.group_name }}</h3><p class="text-xs text-gray-500">#{{ config.group_id }} · {{ config.group_platform }} · {{ config.running ? t('groupEvaluations.running') : config.last_run_at ? formatTime(config.last_run_at) : t('groupEvaluations.neverRun') }}</p></div>
          <div class="flex flex-wrap items-center gap-2">
            <button type="button" class="btn btn-ghost btn-sm" :disabled="!config.enabled" @click="selected = config">{{ t('groupEvaluations.viewResults') }}</button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="saving || config.group_platform !== 'openai' || !config.enabled || config.running || dirty(config) || (!config.candy_enabled && !config.pelican_enabled)" :title="t('groupEvaluations.admin.savedConfigHint')" @click="run(config)">{{ t('groupEvaluations.admin.runNow') }}</button>
          </div>
        </header>
        <fieldset :disabled="saving" class="grid gap-4 px-4 py-4 sm:grid-cols-2 lg:grid-cols-4">
          <p v-if="config.group_platform !== 'openai'" class="text-sm text-amber-700 dark:text-amber-400 sm:col-span-2 lg:col-span-4">{{ t('groupEvaluations.admin.unsupported') }}</p>
          <label class="flex items-center gap-2 text-sm sm:col-span-2 lg:col-span-4"><input v-model="config.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" :disabled="config.group_platform !== 'openai' && !config.enabled" /> {{ t('groupEvaluations.admin.enabled') }}</label>
          <label class="flex items-center gap-2 text-sm"><input v-model="config.candy_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" /> {{ t('groupEvaluations.candy') }}</label>
          <label class="flex items-center gap-2 text-sm"><input v-model="config.pelican_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" /> {{ t('groupEvaluations.pelican') }}</label>
          <label class="flex items-center gap-2 text-sm"><input v-model="config.automatic" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" /> {{ t('groupEvaluations.admin.automatic') }}</label>
          <label class="block"><span class="input-label">{{ t('groupEvaluations.admin.model') }}</span><input v-model.trim="config.model" class="input" type="text" maxlength="200" /></label>
          <label class="block"><span class="input-label">{{ t('groupEvaluations.admin.effort') }}</span><select v-model="config.effort" class="input" :disabled="!config.enabled"><option value="low">low</option><option value="medium">medium</option><option value="high">high</option></select></label>
          <label class="block"><span class="input-label">{{ t('groupEvaluations.admin.runs') }}</span><input v-model.number="config.runs" class="input" type="number" min="1" max="10" step="1" :disabled="!config.enabled || !config.candy_enabled" /></label>
          <label class="block"><span class="input-label">{{ t('groupEvaluations.admin.interval') }}</span><input v-model.number="config.interval_seconds" class="input" type="number" min="10" max="86400" step="1" :disabled="!config.enabled || !config.automatic" /></label>
          <label class="block"><span class="input-label">{{ t('groupEvaluations.admin.dailyLimit') }}</span><input v-model.number="config.daily_limit" class="input" type="number" min="1" max="1000" step="1" :disabled="!config.enabled" /></label>
          <div class="flex items-end justify-end sm:col-span-2 lg:col-span-4"><button type="button" class="btn btn-primary" :disabled="saving || !dirty(config)" @click="save(config)">{{ t('groupEvaluations.admin.save') }}</button></div>
        </fieldset>
      </article>
    </div>
    <GroupEvaluationDialog :show="selected !== null" :group-id="selected?.group_id ?? null" :group-name="selected?.group_name" admin @close="selected = null" @updated="refreshRuntime" />
    <TotpStepUpDialog :controller="stepUp" />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { listEvaluationConfigs, runEvaluation, saveEvaluationConfig, type EvaluationConfig, type EvaluationConfigInput } from '@/api/groupEvaluations'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'
import { isStepUpBlocked, isStepUpCancelled, stepUpBlockReason, useStepUp } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import GroupEvaluationDialog from './GroupEvaluationDialog.vue'

const { t, locale } = useI18n()
const appStore = useAppStore()
const configs = ref<EvaluationConfig[]>([])
const editingId = ref<number | null>(null)
const editingConfigs = computed(() => configs.value.filter(config => config.group_id === editingId.value))
const selected = ref<EvaluationConfig | null>(null)
const snapshots = new Map<number, string>()
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const stepUp = useStepUp()
let controller: AbortController | null = null
let runtimeController: AbortController | null = null
let poll: ReturnType<typeof setTimeout> | undefined
let alive = true

function formatTime(value: string) { return new Date(value).toLocaleString(locale.value, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }) }
function configInput(config: EvaluationConfig): EvaluationConfigInput { return { revision: config.revision, enabled: config.enabled, candy_enabled: config.candy_enabled, pelican_enabled: config.pelican_enabled, automatic: config.automatic, interval_seconds: config.interval_seconds, model: config.model, effort: config.effort, runs: config.runs, daily_limit: config.daily_limit } }
function snapshot(config: EvaluationConfig) { return JSON.stringify(configInput(config)) }
function dirty(config: EvaluationConfig) { return snapshots.get(config.group_id) !== snapshot(config) }
function report(err: unknown) {
  if (isStepUpCancelled(err)) return
  if (isStepUpBlocked(err)) {
    error.value = stepUpBlockReason(err) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' ? t('stepUp.adminApiKeyForbidden') : t('stepUp.notEnabled')
  } else {
    error.value = extractApiErrorMessage(err, t('groupEvaluations.admin.failed'))
  }
}

async function load() {
  controller?.abort()
  const current = new AbortController()
  controller = current
  loading.value = true
  error.value = ''
  try {
    const result = await listEvaluationConfigs(current.signal)
    if (current.signal.aborted) return
    configs.value = result.items
    if (!configs.value.some(config => config.group_id === editingId.value)) editingId.value = configs.value[0]?.group_id ?? null
    snapshots.clear()
    configs.value.forEach(config => snapshots.set(config.group_id, snapshot(config)))
    clearTimeout(poll)
    if (configs.value.some(config => config.running)) poll = setTimeout(() => void refreshRuntime(), 3000)
  } catch (err) {
    if (!current.signal.aborted) report(err)
  } finally {
    if (controller === current) loading.value = false
  }
}

async function save(config: EvaluationConfig) {
  if (config.enabled && config.group_platform !== 'openai') {
    error.value = t('groupEvaluations.admin.unsupported')
    return
  }
  if (!Number.isInteger(config.interval_seconds) || config.interval_seconds < 10 || config.interval_seconds > 86400 || !Number.isInteger(config.runs) || config.runs < 1 || config.runs > 10 || !Number.isInteger(config.daily_limit) || config.daily_limit < 1 || config.daily_limit > 1000 || (config.enabled && ((!config.candy_enabled && !config.pelican_enabled) || !config.model.trim()))) {
    error.value = t('groupEvaluations.admin.invalid')
    return
  }
  saving.value = true
  error.value = ''
  const input = configInput(config)
  try {
    const updated = await stepUp.run(() => saveEvaluationConfig(config.group_id, input))
    if (!alive) return
    Object.assign(config, updated)
    snapshots.set(config.group_id, snapshot(config))
    appStore.showSuccess(t('groupEvaluations.admin.saved'))
  } catch (err) { report(err) } finally { saving.value = false }
}

async function run(config: EvaluationConfig) {
  if (dirty(config) || !config.enabled || config.running || config.group_platform !== 'openai') return
  saving.value = true
  error.value = ''
  try {
    await stepUp.run(() => runEvaluation(config.group_id))
    if (!alive) return
    config.running = true
    selected.value = config
    void refreshRuntime()
  } catch (err) { report(err) } finally { saving.value = false }
}

async function refreshRuntime() {
  if (!alive || runtimeController) return
  clearTimeout(poll)
  const current = new AbortController()
  runtimeController = current
  try {
    const result = await listEvaluationConfigs(current.signal)
    if (current.signal.aborted) return
    for (const config of configs.value) {
      const latest = result.items.find(item => item.group_id === config.group_id)
      if (latest) Object.assign(config, { running: latest.running, last_run_at: latest.last_run_at, next_run_at: latest.next_run_at })
    }
    if (result.items.some(config => config.running)) poll = setTimeout(() => void refreshRuntime(), 3000)
  } catch (err) {
    if (!current.signal.aborted) report(err)
  } finally {
    if (runtimeController === current) runtimeController = null
  }
}

onMounted(() => void load())
onBeforeUnmount(() => {
  alive = false
  controller?.abort()
  runtimeController?.abort()
  clearTimeout(poll)
  stepUp.onCancel()
})
</script>
