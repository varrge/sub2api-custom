<template>
  <section class="card overflow-hidden" data-testid="campaign-settings">
    <header class="border-b border-gray-100 p-5 dark:border-dark-700 sm:p-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="flex items-center gap-2 text-lg font-semibold"><Icon name="gift" class="text-primary-500" />{{ t('activityCampaigns.admin.title') }}</h2>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('activityCampaigns.admin.description') }}</p>
        </div>
        <RouterLink to="/activities/double11" class="btn btn-secondary">{{ t('activityCampaigns.admin.openPage') }}</RouterLink>
      </div>
      <!-- Wave Selector -->
      <div class="mt-4 flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
        <label for="campaign-wave-select" class="text-sm font-medium text-gray-700 dark:text-dark-300">{{ t('activityCampaigns.admin.selectWave') }}</label>
        <select
          id="campaign-wave-select"
          v-model="selectedWaveId"
          class="input py-1 px-2 text-sm w-auto"
          :disabled="loading || saving || dirty"
          @change="onWaveChange"
          data-testid="admin-wave-selector"
        >
          <option :value="waveOneID">{{ t('activityCampaigns.waves.wave1') }}</option>
          <option :value="waveTwoID">{{ t('activityCampaigns.waves.wave2') }}</option>
          <option :value="waveThreeID">{{ t('activityCampaigns.waves.wave3') }}</option>
        </select>
        <span v-if="dirty" class="text-xs text-gray-500">{{ t('activityCampaigns.admin.unsavedWave') }}</span>
        <span v-if="loading" class="text-xs text-gray-500 flex items-center gap-1">
          <svg class="h-3.5 w-3.5 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          {{ t('activityCampaigns.admin.switching') }}
        </span>
      </div>
    </header>
    <div v-if="loading && !form" class="p-6 text-sm text-gray-500" role="status">{{ t('common.loading') }}</div>
    <div v-else-if="!form" class="space-y-3 p-6" role="alert">
      <p class="text-red-600">{{ error || t('activityCampaigns.admin.loadFailed') }}</p>
      <button type="button" class="btn btn-secondary" @click="load">{{ t('activityCampaigns.admin.retry') }}</button>
    </div>
    <div v-else class="space-y-6 p-5 sm:p-6">
      <div class="flex flex-wrap gap-x-5 gap-y-1 text-xs text-gray-500">
        <span>{{ t('activityCampaigns.admin.campaignId') }}: {{ form.id }}</span>
        <span>{{ t('activityCampaigns.admin.revision') }}: {{ form.revision }}</span>
      </div>
      <fieldset :disabled="saving" class="min-w-0 space-y-6">
        <div class="rounded-xl border border-primary-200 bg-primary-50/50 p-4 dark:border-primary-800 dark:bg-primary-900/10">
          <label class="flex items-center justify-between gap-4">
            <span class="font-semibold">{{ t('activityCampaigns.admin.enabled') }}</span>
            <Toggle v-model="form.enabled" data-testid="campaign-enabled" :aria-label="t('activityCampaigns.admin.enabled')" />
          </label>
          <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ t('activityCampaigns.admin.noteGlobalToggle') }}</p>
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1.5 text-sm">{{ t('activityCampaigns.admin.fieldTitle') }}
            <input v-model="form.title" class="input" maxlength="80" data-testid="campaign-title" />
          </label>
          <label class="space-y-1.5 text-sm">{{ t('activityCampaigns.admin.fieldSubtitle') }}
            <input v-model="form.subtitle" class="input" maxlength="160" />
          </label>
          <label class="space-y-1.5 text-sm sm:col-span-2">{{ t('activityCampaigns.admin.fieldDescription') }}
            <textarea v-model="form.description" class="input min-h-24" maxlength="2000" rows="3"></textarea>
          </label>
          <label class="space-y-1.5 text-sm">{{ t('activityCampaigns.admin.startsAt') }}
            <input v-model="startsLocal" type="datetime-local" class="input min-w-0" data-testid="campaign-start" />
          </label>
          <label class="space-y-1.5 text-sm">{{ t('activityCampaigns.admin.endsAt') }}
            <input v-model="endsLocal" type="datetime-local" class="input min-w-0" data-testid="campaign-end" />
          </label>
          <p class="text-xs text-gray-500 sm:col-span-2">{{ t('activityCampaigns.admin.timeHint') }} {{ t('activityCampaigns.admin.noteLocked') }}</p>
          <label class="space-y-1.5 text-sm sm:col-span-2">{{ t('activityCampaigns.admin.boxCampaignId') }}
            <input v-model.trim="form.box_campaign_id" class="input" maxlength="100" data-testid="campaign-pool" autocomplete="off" />
            <span class="block text-xs text-gray-500">{{ t('activityCampaigns.admin.boxCampaignIdHint') }}</span>
          </label>
        </div>

        <!-- Wave 1 Quiz Settings -->
        <div v-if="form.id === waveOneID" class="space-y-4">
          <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
            <h3 class="text-sm font-semibold">{{ t('activityCampaigns.admin.rewardsTitle') }}</h3>
            <p class="mt-2 text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('activityCampaigns.admin.rewardsFrozen') }}</p>
          </div>
          <div class="space-y-4">
            <div>
              <h3 class="text-base font-semibold">{{ t('activityCampaigns.admin.quizTitle') }}</h3>
              <p class="mt-1 text-xs text-gray-500">{{ t('activityCampaigns.admin.quizHint') }}</p>
            </div>
            <div v-for="(question, qi) in form.questions" :key="question.id" class="space-y-3 rounded-xl border border-gray-200 p-4 dark:border-dark-600" :data-testid="`campaign-question-${qi}`">
              <div class="flex items-center justify-between">
                <h4 class="text-sm font-semibold">{{ t('activityCampaigns.admin.question', { n: qi + 1 }) }}</h4>
                <button type="button" class="text-sm text-red-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="form.questions.length <= 3" :aria-label="`${t('activityCampaigns.admin.remove')} ${t('activityCampaigns.admin.question', { n: qi + 1 })}`" @click="form.questions.splice(qi, 1)">{{ t('activityCampaigns.admin.remove') }}</button>
              </div>
              <label class="block space-y-1 text-sm">{{ t('activityCampaigns.admin.prompt') }}
                <input v-model="question.prompt" class="input" maxlength="200" />
              </label>
              <div v-for="(_, oi) in question.options" :key="oi" class="flex items-center gap-2">
                <label class="min-w-0 flex-1 space-y-1 text-xs">{{ t('activityCampaigns.admin.option', { n: oi + 1 }) }}
                  <input v-model="question.options[oi]" class="input" maxlength="100" />
                </label>
                <button type="button" class="btn btn-secondary mt-5 px-2" :disabled="question.options.length <= 2" :aria-label="`${t('activityCampaigns.admin.remove')} ${t('activityCampaigns.admin.option', { n: oi + 1 })}`" @click="removeOption(question, oi)"><Icon name="x" size="sm" /></button>
              </div>
              <div class="flex flex-wrap items-end justify-between gap-3">
                <label class="space-y-1 text-xs">{{ t('activityCampaigns.admin.correctAnswer') }}
                  <select v-model.number="question.answer" class="input" :data-testid="`campaign-answer-${qi}`">
                    <option v-for="(_, oi) in question.options" :key="oi" :value="oi">{{ t('activityCampaigns.admin.option', { n: oi + 1 }) }}</option>
                  </select>
                </label>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="question.options.length >= 6" @click="question.options.push('')">{{ t('activityCampaigns.admin.addOption') }}</button>
              </div>
            </div>
            <button type="button" class="btn btn-secondary" :disabled="form.questions.length >= 10" @click="addQuestion">{{ t('activityCampaigns.admin.addQuestion') }}</button>
          </div>
        </div>

        <!-- Wave 2 Descriptive Block -->
        <div v-else-if="form.id === waveTwoID" class="rounded-xl border border-blue-200 bg-blue-50/50 p-4 dark:border-blue-800 dark:bg-blue-900/10">
          <h3 class="text-sm font-semibold text-blue-950 dark:text-blue-300">
            {{ t('activityCampaigns.wave2.productsTitle') }}
          </h3>
          <p class="mt-2 text-xs leading-relaxed text-blue-700 dark:text-blue-400">
            {{ t('activityCampaigns.wave2.description') }}
          </p>
          <div class="mt-3 text-xs text-gray-500 leading-relaxed">
            <strong>{{ t('activityCampaigns.admin.rewardsTitle') }}:</strong>
            <ul class="mt-1.5 list-disc pl-4 space-y-1">
              <li>Original Price 58: Purchase gets 1 low card. Group Buy of 3 paid seats gets 1 low card.</li>
              <li>Original Price 128: Purchase gets 2 low cards. Group Buy of 3 paid seats gets 1 low card.</li>
              <li>Original Price 198: Purchase gets 1 medium card. Group Buy of 3 paid seats gets 2 low cards.</li>
            </ul>
          </div>
        </div>

        <!-- Wave 3 Descriptive Block -->
        <div v-else-if="form.id === waveThreeID" class="rounded-xl border border-purple-200 bg-purple-50/50 p-4 dark:border-purple-800 dark:bg-purple-900/10">
          <h3 class="text-sm font-semibold text-purple-950 dark:text-purple-300">
            {{ t('activityCampaigns.wave3.title') }}
          </h3>
          <p class="mt-2 text-xs leading-relaxed text-purple-700 dark:text-purple-400">
            {{ t('activityCampaigns.wave3.description') }}
          </p>
          <div class="mt-3 text-xs text-gray-500 leading-relaxed">
            <strong>🐳 {{ t('activityCampaigns.wave3.clueTitle') }}:</strong>
            <ul class="mt-1.5 list-disc pl-4 space-y-1">
              <li>5 cute whales hidden at: <code>/dashboard</code>, <code>/keys</code>, <code>/usage</code>, <code>/group-buy</code>, <code>/profile</code>.</li>
              <li>Finding 3 gets 1 low card.</li>
              <li>Finding 5 gets another 1 low card.</li>
            </ul>
          </div>
        </div>
      </fieldset>
      <p v-if="error" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300" role="alert" data-testid="campaign-save-error">{{ error }}</p>
      <p v-if="success" class="text-sm text-emerald-600" role="status">{{ t('activityCampaigns.admin.saved') }}</p>
      <div class="flex flex-wrap gap-3 border-t border-gray-100 pt-5 dark:border-dark-700">
        <button type="button" class="btn btn-primary" :disabled="saving || conflict" data-testid="campaign-save" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</button>
        <button type="button" class="btn btn-secondary" :disabled="saving" data-testid="campaign-refresh" @click="load">{{ t('activityCampaigns.admin.refreshConfig') }}</button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  activityCampaignsAPI,
  type ActivityCampaignConfig,
  type ActivityQuestion,
  waveOneID,
  waveTwoID,
  waveThreeID
} from '@/api/activityCampaigns'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const selectedWaveId = ref(waveOneID)
const form = ref<ActivityCampaignConfig | null>(null)
const startsLocal = ref('')
const endsLocal = ref('')
const loading = ref(false)
const saving = ref(false)
const conflict = ref(false)
const error = ref('')
const success = ref(false)
let disposed = false
let loadGeneration = 0
const savedSnapshot = ref('')
const dirty = computed(() => !!form.value && JSON.stringify([form.value, startsLocal.value, endsLocal.value]) !== savedSnapshot.value)

const toChinaInput = (value: string) => {
  if (!value) return ''
  return new Date(Date.parse(value) + 8 * 3600_000).toISOString().slice(0, 16)
}
const fromChinaInput = (value: string) => {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return ''
  const at = new Date(`${value}:00+08:00`)
  return Number.isFinite(at.getTime()) && toChinaInput(at.toISOString()) === value ? at.toISOString() : ''
}
function apply(next: ActivityCampaignConfig) {
  form.value = structuredClone({ ...next, questions: next.questions || [] })
  startsLocal.value = toChinaInput(next.starts_at)
  endsLocal.value = toChinaInput(next.ends_at)
  savedSnapshot.value = JSON.stringify([form.value, startsLocal.value, endsLocal.value])
}
async function load() {
  if (saving.value) return
  const generation = ++loadGeneration
  const targetWave = selectedWaveId.value
  form.value = null
  loading.value = true
  error.value = ''
  success.value = false
  try {
    const result = await activityCampaignsAPI.adminGet(targetWave)
    if (disposed || generation !== loadGeneration) return
    if (result.id !== targetWave) throw new Error(t('activityCampaigns.admin.loadFailed'))
    apply(result)
    conflict.value = false
  } catch (e) {
    if (!disposed && generation === loadGeneration) {
      error.value = extractApiErrorMessage(e, t('activityCampaigns.admin.loadFailed'))
    }
  } finally {
    if (!disposed && generation === loadGeneration) {
      loading.value = false
    }
  }
}
function onWaveChange() {
  void load()
}
function addQuestion() {
  if (!form.value || form.value.questions.length >= 10) return
  form.value.questions.push({ id: `q-${crypto.randomUUID()}`, prompt: '', options: ['', ''], answer: 0 })
}
function removeOption(question: ActivityQuestion, index: number) {
  if (question.options.length <= 2) return
  question.options.splice(index, 1)
  if (question.answer === index) question.answer = 0
  else if (question.answer !== undefined && question.answer > index) question.answer--
}
function validation(c: ActivityCampaignConfig): string {
  if (!c.title.trim() || c.title.length > 80) return 'title'
  if (c.subtitle.length > 160) return 'subtitle'
  if (c.description.length > 2000) return 'description'
  if (!c.starts_at || !c.ends_at) return 'periodRequired'
  if (Date.parse(c.ends_at) <= Date.parse(c.starts_at)) return 'periodOrder'
  if (c.enabled && !c.box_campaign_id) return 'boxId'
  if (c.id === waveOneID) {
    if (c.questions.length < 3 || c.questions.length > 10) return 'questions'
    for (const q of c.questions) {
      if (!q.prompt.trim() || q.prompt.length > 200) return 'prompt'
      if (q.options.length < 2 || q.options.length > 6 || q.options.some(o => !o.trim() || o.length > 100)) return 'options'
      if (q.answer === undefined || !Number.isInteger(q.answer) || q.answer < 0 || q.answer >= q.options.length) return 'answer'
    }
  }
  return ''
}
async function save() {
  if (!form.value || loading.value || saving.value || conflict.value || form.value.id !== selectedWaveId.value) return
  const payload: ActivityCampaignConfig = JSON.parse(JSON.stringify(form.value))
  // Keep sub-minute precision when only editing copy or toggling the event.
  if (startsLocal.value !== toChinaInput(payload.starts_at)) payload.starts_at = fromChinaInput(startsLocal.value)
  if (endsLocal.value !== toChinaInput(payload.ends_at)) payload.ends_at = fromChinaInput(endsLocal.value)
  error.value = ''
  success.value = false
  const invalid = validation(payload)
  if (invalid) { error.value = t(`activityCampaigns.admin.validation.${invalid}`); return }
  saving.value = true
  try {
    const result = await activityCampaignsAPI.adminSave(payload, selectedWaveId.value)
    if (disposed) return
    apply(result)
    success.value = true
  } catch (e) {
    if (disposed) return
    conflict.value = (e as { status?: number })?.status === 409
    error.value = conflict.value ? t('activityCampaigns.admin.conflict') : extractApiErrorMessage(e, t('activityCampaigns.admin.saveFailed'))
  } finally {
    if (!disposed) saving.value = false
  }
}
onMounted(load)
onUnmounted(() => { disposed = true; loadGeneration++ })
</script>
