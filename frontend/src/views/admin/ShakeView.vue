<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card p-5 sm:p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('shakeAdmin.campaignsTitle') }}</h2>
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('shakeAdmin.campaignsHint') }}</p>
          </div>
          <button class="btn btn-primary" data-testid="shake-new" @click="openEditor(null)">
            <Icon name="plus" size="sm" />{{ t('shakeAdmin.newCampaign') }}
          </button>
        </div>
        <p v-if="listError" role="alert" class="mt-4 text-sm text-red-600">{{ listError }}</p>
        <p v-else-if="!loading && campaigns.length === 0" class="mt-4 text-sm text-gray-500">{{ t('shakeAdmin.noCampaigns') }}</p>
        <ul class="mt-4 divide-y divide-gray-100 dark:divide-dark-700">
          <li v-for="c in campaigns" :key="c.id" class="flex flex-wrap items-center justify-between gap-3 py-3" :data-testid="`shake-campaign-${c.id}`">
            <button type="button" class="min-w-0 text-left" @click="select(c.id)">
              <span class="font-medium text-gray-900 dark:text-white">{{ c.config.name }}</span>
              <span :class="['ml-2 rounded-full px-2 py-0.5 text-xs', statusClass(c.status)]">{{ t(`shakeAdmin.status_${c.status}`) }}</span>
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ fmt(c.config.starts_at) }} — {{ fmt(c.config.ends_at) }} · v{{ c.config_version }} · <code class="select-all">{{ c.id }}</code></span>
            </button>
            <div class="flex flex-wrap gap-2">
              <button v-if="c.status !== 'ended'" class="btn btn-secondary btn-sm" @click="openEditor(c)">{{ t('shakeAdmin.edit') }}</button>
              <button v-if="c.stored_status === 'draft'" class="btn btn-primary btn-sm" :data-testid="`shake-activate-${c.id}`" @click="act(() => api.activateCampaign(c.id), 'activated')">{{ t('shakeAdmin.activate') }}</button>
              <button v-if="c.stored_status === 'draft'" class="btn btn-secondary btn-sm" @click="removeDraft(c)">{{ t('shakeAdmin.delete') }}</button>
              <button v-if="c.stored_status === 'active'" class="btn btn-secondary btn-sm" @click="endTarget = c">{{ t('shakeAdmin.end') }}</button>
            </div>
          </li>
        </ul>
      </section>

      <section v-if="selected" class="card space-y-5 p-5 sm:p-6" data-testid="shake-detail">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ selected.campaign.config.name }}</h2>
          <button v-if="selected.campaign.status === 'active'" class="btn btn-secondary btn-sm" @click="act(() => api.syncCampaign(selected!.campaign.id), 'synced')">
            <Icon name="refresh" size="sm" />{{ t('shakeAdmin.syncNow') }}
          </button>
        </div>
        <dl class="grid gap-3 text-sm sm:grid-cols-4">
          <div><dt class="text-xs text-gray-500">{{ t('shakeAdmin.statUsers') }}</dt><dd class="font-semibold">{{ selected.stats.users }}</dd></div>
          <div><dt class="text-xs text-gray-500">{{ t('shakeAdmin.statDraws') }}</dt><dd class="font-semibold">{{ selected.stats.draws }}</dd></div>
          <div><dt class="text-xs text-gray-500">{{ t('shakeAdmin.statPaid') }}</dt><dd class="font-semibold">${{ selected.stats.balance_paid.toFixed(2) }}</dd></div>
          <div>
            <dt class="text-xs text-gray-500">{{ t('shakeAdmin.statCards') }}</dt>
            <dd v-for="row in selected.stats.cards" :key="row.tier" class="text-xs">
              {{ t(`luckyBox.tier${cap(row.tier)}`) }}：{{ t('shakeAdmin.cardCounts', row) }}
            </dd>
          </div>
        </dl>

        <form v-if="selected.campaign.status !== 'ended' && selected.campaign.stored_status !== 'draft'" class="grid gap-2 rounded-lg bg-gray-50 p-3 dark:bg-dark-800 sm:grid-cols-6" data-testid="shake-grant" @submit.prevent="grant">
          <h3 class="text-sm font-semibold sm:col-span-6">{{ t('shakeAdmin.grantTitle') }}</h3>
          <input v-model.number="grantForm.user_id" type="number" min="1" class="input" :placeholder="t('shakeAdmin.userId')" required />
          <select v-model="grantForm.card_tier" class="input">
            <option v-for="tier in tiers" :key="tier" :value="tier">{{ t(`luckyBox.tier${cap(tier)}`) }}</option>
          </select>
          <input v-model.number="grantForm.quantity" type="number" min="1" max="100" class="input" required />
          <input v-model.trim="grantForm.reason" class="input sm:col-span-2" minlength="2" maxlength="500" :placeholder="t('shakeAdmin.reason')" required />
          <button type="submit" class="btn btn-primary">{{ t('shakeAdmin.grant') }}</button>
        </form>

        <div>
          <div class="mb-2 flex flex-wrap items-center gap-2">
            <h3 class="text-sm font-semibold">{{ t('shakeAdmin.drawsTitle') }}</h3>
            <input v-model.number="drawUser" type="number" min="1" class="input w-36" :placeholder="t('shakeAdmin.userId')" @change="loadDraws" />
            <button v-if="drawUser" class="btn btn-secondary btn-sm" @click="lookupUser">{{ t('shakeAdmin.lookupUser') }}</button>
          </div>
          <p v-if="userCards" class="mb-2 text-xs text-gray-600 dark:text-dark-300" data-testid="shake-user-cards">
            {{ userCards.email }} · {{ t('shakeAdmin.userCards', { low: userCards.by_tier.low, medium: userCards.by_tier.medium, high: userCards.by_tier.high }) }}
          </p>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[640px] text-sm">
              <thead class="text-left text-xs text-gray-500"><tr>
                <th class="py-1">#</th><th>{{ t('shakeAdmin.user') }}</th><th>{{ t('shakeAdmin.cardTier') }}</th><th>{{ t('shakeAdmin.prize') }}</th><th>{{ t('shakeAdmin.status') }}</th><th>{{ t('shakeAdmin.time') }}</th><th></th>
              </tr></thead>
              <tbody>
                <tr v-for="d in draws" :key="d.id" class="border-t border-gray-100 dark:border-dark-700">
                  <td class="py-1">{{ d.id }}</td>
                  <td>{{ d.email || d.userId }}</td>
                  <td>{{ t(`luckyBox.tier${cap(d.cardTier)}`) }}</td>
                  <td>{{ d.prize?.name }}<template v-if="d.prize?.type === 'balance'"> (${{ d.prize.amount }})</template></td>
                  <td>{{ d.status === 'voided' ? t('luckyBox.statusVoided') : t('luckyBox.statusDelivered') }}</td>
                  <td>{{ fmt(d.createdAt) }}</td>
                  <td><button v-if="d.status === 'delivered'" class="btn btn-secondary btn-sm" @click="voidTarget = d">{{ t('shakeAdmin.void') }}</button></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section class="card space-y-3 p-5 sm:p-6" data-testid="shake-reward-jobs">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('shakeAdmin.jobsTitle') }}</h2>
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('shakeAdmin.jobsHint') }}</p>
          </div>
          <div class="flex gap-2">
            <select v-model="jobCampaign" class="input" :aria-label="t('shakeAdmin.jobTarget')">
              <option value="">{{ t('shakeAdmin.jobTarget') }}</option>
              <option v-for="c in campaigns.filter((x) => x.status !== 'ended')" :key="c.id" :value="c.id">{{ c.config.name }}</option>
            </select>
            <button class="btn btn-primary" :disabled="!jobCampaign" @click="act(() => api.createRewardJob(jobCampaign), 'jobCreated')">{{ t('shakeAdmin.bindJob') }}</button>
          </div>
        </div>
        <p v-if="jobs.length === 0" class="text-sm text-gray-500">{{ t('shakeAdmin.noJobs') }}</p>
        <div v-for="job in jobs" :key="job.id" class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span>
              {{ campaignName(job.campaign_id) }} ← {{ fmt(job.source_starts_at) }} — {{ fmt(job.source_ends_at) }}
              · {{ t(`shakeAdmin.period_${job.period}`) }}
              · {{ t('shakeAdmin.jobProgress', { days: job.completed_days, recipients: job.recipients, cards: job.cards_granted }) }}
            </span>
            <span class="flex gap-2">
              <button class="btn btn-secondary btn-sm" @click="act(() => api.retryRewardJob(job.id), 'jobRetried')">{{ t('shakeAdmin.retry') }}</button>
              <button class="btn btn-secondary btn-sm" @click="act(() => api.refreshRewardPolicy(job.id), 'jobRefreshed')">{{ t('shakeAdmin.refreshPolicy') }}</button>
              <button class="btn btn-secondary btn-sm" @click="act(() => api.setRewardJobEnabled(job.id, !job.enabled), 'saved')">{{ job.enabled ? t('shakeAdmin.pause') : t('shakeAdmin.resume') }}</button>
            </span>
          </div>
          <p v-if="job.last_error" class="mt-1 text-xs text-red-600 dark:text-red-400">{{ job.last_error }}</p>
        </div>
      </section>
    </div>

    <BaseDialog :show="editorOpen" :title="editing ? t('shakeAdmin.editTitle') : t('shakeAdmin.newCampaign')" width="extra-wide" @close="editorOpen = false">
      <ShakeCampaignEditor
        v-if="editorOpen"
        :key="editing?.id ?? 'new'"
        :initial="editing?.config"
        :start-locked="editing?.stored_status === 'active'"
        :saving="saving"
        @save="saveConfig"
        @cancel="editorOpen = false"
      />
    </BaseDialog>

    <BaseDialog :show="!!endTarget" :title="t('shakeAdmin.endTitle')" width="narrow" @close="endTarget = null">
      <p class="mb-3 text-sm text-gray-600 dark:text-dark-300">{{ t('shakeAdmin.endWarning') }}</p>
      <input v-model.trim="reason" class="input" maxlength="500" :placeholder="t('shakeAdmin.reason')" data-testid="shake-end-reason" />
      <template #footer>
        <button class="btn btn-secondary" @click="endTarget = null">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="reason.length < 2" @click="confirmEnd">{{ t('shakeAdmin.end') }}</button>
      </template>
    </BaseDialog>

    <BaseDialog :show="!!voidTarget" :title="t('shakeAdmin.voidTitle')" width="narrow" @close="voidTarget = null">
      <p class="mb-3 text-sm text-gray-600 dark:text-dark-300">{{ t('shakeAdmin.voidWarning') }}</p>
      <input v-model.trim="reason" class="input" maxlength="500" :placeholder="t('shakeAdmin.reason')" />
      <template #footer>
        <button class="btn btn-secondary" @click="voidTarget = null">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="reason.length < 2" @click="confirmVoid">{{ t('shakeAdmin.void') }}</button>
      </template>
    </BaseDialog>

    <TotpStepUpDialog :controller="stepUp" />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import ShakeCampaignEditor from '@/components/admin/shake/ShakeCampaignEditor.vue'
import * as api from '@/api/admin/shake'
import type { ShakeCampaign, ShakeConfig, ShakeRewardJob, ShakeStats, ShakeUserCards } from '@/api/admin/shake'
import type { BoxCardTier, BoxDraw } from '@/api/luckyBox'
import { useAppStore } from '@/stores/app'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatBeijingDateTimeToMinute } from '@/utils/beijingDateTime'

const { t } = useI18n()
const appStore = useAppStore()
const stepUp = useStepUp()
const tiers: BoxCardTier[] = ['low', 'medium', 'high']
const cap = (v: string) => v.charAt(0).toUpperCase() + v.slice(1)
const fmt = (v?: string | null) => (v ? formatBeijingDateTimeToMinute(v) : '—')

const loading = ref(false)
const listError = ref('')
const campaigns = ref<ShakeCampaign[]>([])
const selected = ref<{ campaign: ShakeCampaign; stats: ShakeStats } | null>(null)
const draws = ref<BoxDraw[]>([])
const drawUser = ref<number | null>(null)
const userCards = ref<ShakeUserCards | null>(null)
const jobs = ref<ShakeRewardJob[]>([])
const jobCampaign = ref('')
const editorOpen = ref(false)
const editing = ref<ShakeCampaign | null>(null)
const saving = ref(false)
const endTarget = ref<ShakeCampaign | null>(null)
const voidTarget = ref<BoxDraw | null>(null)
const reason = ref('')
const grantForm = reactive({ user_id: null as number | null, card_tier: 'low' as BoxCardTier, quantity: 1, reason: '' })

const statusClass = (s: string) =>
  ({ active: 'bg-emerald-100 text-emerald-700', scheduled: 'bg-cyan-100 text-cyan-700', draft: 'bg-gray-100 text-gray-600', ended: 'bg-gray-200 text-gray-500' })[s] ?? ''
const campaignName = (id: string) => campaigns.value.find((c) => c.id === id)?.config.name ?? id

const showError = (e: unknown) => {
  if (isStepUpCancelled(e)) return
  if (isStepUpBlocked(e)) {
    appStore.showError(stepUpBlockReason(e) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' ? t('stepUp.adminApiKeyForbidden') : t('stepUp.notEnabled'))
    return
  }
  appStore.showError(extractApiErrorMessage(e, t('shakeAdmin.failed')))
}

const load = async () => {
  loading.value = true
  listError.value = ''
  try {
    const [items, jobItems] = await Promise.all([api.listCampaigns(), api.listRewardJobs()])
    campaigns.value = items
    jobs.value = jobItems
    if (selected.value) await select(selected.value.campaign.id)
  } catch (e) {
    listError.value = extractApiErrorMessage(e, t('shakeAdmin.failed'))
  } finally {
    loading.value = false
  }
}

const select = async (id: string) => {
  try {
    selected.value = await api.getCampaign(id)
    userCards.value = null
    await loadDraws()
  } catch (e) {
    showError(e)
  }
}

const loadDraws = async () => {
  if (!selected.value) return
  try {
    draws.value = await api.listDraws({ campaign_id: selected.value.campaign.id, user_id: drawUser.value || undefined })
  } catch (e) {
    showError(e)
  }
}

const lookupUser = async () => {
  if (!selected.value || !drawUser.value) return
  try {
    userCards.value = await api.getUserCards(selected.value.campaign.id, drawUser.value)
  } catch (e) {
    showError(e)
  }
}

const act = async (action: () => Promise<unknown>, done: string) => {
  try {
    await stepUp.run(action)
    appStore.showSuccess(t(`shakeAdmin.done_${done}`))
    await load()
  } catch (e) {
    showError(e)
  }
}

const openEditor = (c: ShakeCampaign | null) => {
  editing.value = c
  editorOpen.value = true
}

const saveConfig = async (config: ShakeConfig) => {
  saving.value = true
  try {
    const target = editing.value
    await stepUp.run(() => (target ? api.saveCampaign(target.id, target.config_version, config) : api.createCampaign(config)))
    appStore.showSuccess(t('shakeAdmin.done_saved'))
    editorOpen.value = false
    await load()
  } catch (e) {
    showError(e)
  } finally {
    saving.value = false
  }
}

const removeDraft = async (c: ShakeCampaign) => {
  if (!window.confirm(t('shakeAdmin.deleteConfirm', { name: c.config.name }))) return
  await act(() => api.deleteDraft(c.id), 'deleted')
  if (selected.value?.campaign.id === c.id) selected.value = null
}

const confirmEnd = async () => {
  const target = endTarget.value
  if (!target) return
  await act(() => api.endCampaign(target.id, reason.value), 'ended')
  endTarget.value = null
  reason.value = ''
}

const confirmVoid = async () => {
  const target = voidTarget.value
  if (!target) return
  await act(() => api.voidDraw(target.id, reason.value), 'voided')
  voidTarget.value = null
  reason.value = ''
}

const grant = async () => {
  const target = selected.value
  if (!target || !grantForm.user_id) return
  const payload = { user_id: grantForm.user_id, card_tier: grantForm.card_tier, quantity: grantForm.quantity, reason: grantForm.reason }
  await act(() => api.grantCards(target.campaign.id, payload), 'granted')
  grantForm.reason = ''
}

onMounted(load)
</script>
