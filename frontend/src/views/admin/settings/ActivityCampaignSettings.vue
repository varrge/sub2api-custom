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
    </header>
    <div v-if="loading" class="p-6 text-sm text-gray-500" role="status">{{ t('common.loading') }}</div>
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
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { activityCampaignsAPI, type ActivityCampaignConfig, type ActivityQuestion } from '@/api/activityCampaigns'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
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

const toChinaInput = (value: string) => new Date(Date.parse(value) + 8 * 3600_000).toISOString().slice(0, 16)
const fromChinaInput = (value: string) => {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return ''
  const at = new Date(`${value}:00+08:00`)
  return Number.isFinite(at.getTime()) && toChinaInput(at.toISOString()) === value ? at.toISOString() : ''
}
function apply(next: ActivityCampaignConfig) {
  form.value = structuredClone(next)
  startsLocal.value = toChinaInput(next.starts_at)
  endsLocal.value = toChinaInput(next.ends_at)
}
async function load() {
  if (saving.value) return
  const generation = ++loadGeneration
  loading.value = true
  error.value = ''
  success.value = false
  try {
    const result = await activityCampaignsAPI.adminGet()
    if (disposed || generation !== loadGeneration) return
    apply(result)
    conflict.value = false
  } catch (e) {
    if (!disposed && generation === loadGeneration) error.value = extractApiErrorMessage(e, t('activityCampaigns.admin.loadFailed'))
  } finally {
    if (!disposed && generation === loadGeneration) loading.value = false
  }
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
  if (c.questions.length < 3 || c.questions.length > 10) return 'questions'
  for (const q of c.questions) {
    if (!q.prompt.trim() || q.prompt.length > 200) return 'prompt'
    if (q.options.length < 2 || q.options.length > 6 || q.options.some(o => !o.trim() || o.length > 100)) return 'options'
    if (q.answer === undefined || !Number.isInteger(q.answer) || q.answer < 0 || q.answer >= q.options.length) return 'answer'
  }
  return ''
}
async function save() {
  if (!form.value || saving.value || conflict.value) return
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
    const result = await activityCampaignsAPI.adminSave(payload)
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
