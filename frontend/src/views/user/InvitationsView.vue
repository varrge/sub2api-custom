<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading && !overview" class="flex justify-center py-12" role="status" :aria-label="t('common.loading')">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-if="loadError" role="alert" class="card flex flex-wrap items-center justify-between gap-3 border-red-200 p-5 text-sm text-red-800 dark:border-red-900/50 dark:text-red-200" data-testid="invitation-load-error">
        <span>{{ loadError }}</span>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="refresh">{{ t('invitations.records.retry') }}</button>
      </div>

      <template v-if="overview">
        <p v-if="!rebatesEnabled" class="rounded-xl bg-primary-50 px-5 py-4 text-sm text-primary-900 dark:bg-primary-900/20 dark:text-primary-200" data-testid="invitation-enterprise-note">
          {{ t('invitations.enterprise.note') }}
        </p>
        <div class="grid gap-6 lg:grid-cols-5">
          <section class="card p-5 sm:p-6 lg:col-span-3" data-testid="invitation-issue">
            <div
              v-if="pendingRequest"
              role="status"
              class="mb-5 rounded-xl border border-amber-300 bg-amber-50 p-4 dark:border-amber-800/60 dark:bg-amber-900/20"
              data-testid="invitation-pending"
            >
              <p class="text-sm font-semibold text-amber-900 dark:text-amber-200">{{ t('invitations.issue.pendingTitle') }}</p>
              <p class="mt-1 text-sm text-amber-900/90 dark:text-amber-200/90">{{ t('invitations.issue.pendingBody') }}</p>
              <div class="mt-3 flex flex-wrap gap-2">
                <button type="button" class="btn btn-primary btn-sm" :disabled="applying" data-testid="confirm-invitation" @click="apply">
                  {{ applying ? t('invitations.issue.confirming') : t('invitations.issue.confirm') }}
                </button>
                <button type="button" class="btn btn-ghost btn-sm" :disabled="applying" data-testid="discard-invitation" @click="discardPending">
                  {{ t('invitations.issue.discard') }}
                </button>
              </div>
            </div>

            <InvitationTicket v-if="latest" :invite="latest" />
            <div v-else class="rounded-2xl border border-dashed border-gray-300 px-5 py-8 text-center dark:border-dark-600" data-testid="invitation-no-code">
              <p class="font-medium text-gray-900 dark:text-white">{{ t('invitations.issue.none') }}</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('invitations.issue.noneHint') }}</p>
            </div>

            <div class="mt-5 flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
              <p class="min-w-0 flex-1 basis-64 text-sm" :class="blocked ? 'text-amber-800 dark:text-amber-300' : 'text-gray-600 dark:text-dark-300'" data-testid="invitation-issue-note">
                {{ issueNote }}
              </p>
              <button
                v-if="!pendingRequest"
                type="button"
                class="btn btn-primary"
                data-testid="apply-invitation"
                :disabled="applying || blocked"
                @click="apply"
              >
                <Icon name="plus" size="sm" />
                <span>{{ applying ? t('invitations.issue.applying') : t('invitations.issue.apply') }}</span>
              </button>
            </div>
            <p v-if="applyError" role="alert" class="mt-3 text-sm text-red-700 dark:text-red-300" data-testid="invitation-apply-error">
              <span class="font-medium">{{ t('invitations.issue.failed') }}</span>{{ t('invitations.sep') }}{{ applyError }}
            </p>

            <div class="mt-6 border-t border-gray-100 pt-5 dark:border-dark-700">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('invitations.steps.title') }}</h2>
              <ol class="mt-3 space-y-2.5 text-sm text-gray-700 dark:text-gray-300">
                <li v-for="(step, index) in steps" :key="index" class="flex gap-3">
                  <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-100 text-xs font-semibold text-primary-900 dark:bg-primary-900/40 dark:text-primary-200" aria-hidden="true">{{ index + 1 }}</span>
                  <span>{{ step }}</span>
                </li>
              </ol>
              <p v-if="autoNote" class="mt-3 text-sm text-gray-600 dark:text-dark-300" data-testid="invitation-auto-note">{{ autoNote }}</p>
            </div>
          </section>

          <InvitationOverviewPanel class="lg:col-span-2" :overview="overview" :show-rebates="rebatesEnabled" />
        </div>

        <section v-if="rebatesEnabled" class="card overflow-hidden" data-testid="invitation-rewards">
          <dl class="grid divide-y divide-gray-100 dark:divide-dark-700 sm:grid-cols-3 sm:divide-x sm:divide-y-0">
            <div v-for="item in rewards" :key="item.key" class="px-5 py-4 sm:px-6">
              <dt class="text-sm text-gray-500 dark:text-dark-400">{{ t(`invitations.rewards.${item.key}`) }}</dt>
              <dd class="mt-1 text-xl font-semibold tabular-nums" :class="item.tone">{{ formatMoney(item.amount) }}</dd>
            </div>
          </dl>
          <p class="border-t border-gray-100 px-5 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400 sm:px-6">{{ t('invitations.rewards.note') }}</p>
        </section>
      </template>

      <section v-if="overview || !loadError" class="card p-5 sm:p-6">
        <InvitationTabs v-model="tab" :tabs="tabs" :label="t('invitations.title')" id-prefix="invitation" />
        <div v-for="kind in kinds" v-show="tab === kind" :id="`invitation-panel-${kind}`" :key="kind" class="pt-5" role="tabpanel" :aria-labelledby="`invitation-tab-${kind}`">
          <InvitationRecords v-if="visited.has(kind)" :kind="kind" :refresh-key="refreshKey" />
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import InvitationOverviewPanel from '@/components/invitations/InvitationOverview.vue'
import InvitationRecords from '@/components/invitations/InvitationRecords.vue'
import InvitationTabs from '@/components/invitations/InvitationTabs.vue'
import InvitationTicket from '@/components/invitations/InvitationTicket.vue'
import { invitationsAPI, type InvitationCode, type InvitationOverview } from '@/api/invitations'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { beginApply, clearPendingApply, isUncertainApplyError, loadPendingApply } from '@/utils/invitationApply'
import { invitationErrorMessage } from '@/utils/invitationErrors'
import { formatMoney, formatRate, waitingPeriod } from '@/utils/invitationFormat'

type Kind = 'codes' | 'rebates'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const overview = ref<InvitationOverview | null>(null)
const rebatesEnabled = computed(() => !authStore.isEnterprise && overview.value?.rebate_enabled !== false)
const kinds = computed<Kind[]>(() => rebatesEnabled.value ? ['codes', 'rebates'] : ['codes'])
const latest = ref<InvitationCode | null>(null)
const loading = ref(false)
const applying = ref(false)
const refreshKey = ref(0)
const loadError = ref('')
const applyError = ref('')
const pendingRequest = ref('')
const tab = ref<Kind>('codes')
const visited = reactive(new Set<Kind>(['codes']))
let loadVersion = 0

// Idempotency keys are stored per account so a shared browser never replays another user's request.
const storageOwner = computed(() => authStore.user?.id ?? 0)

const lifetimeFull = computed(() => {
  const o = overview.value
  return !!o && o.level.lifetime_invite_limit > 0 && o.lifetime_used >= o.level.lifetime_invite_limit
})
const unusedFull = computed(() => {
  const o = overview.value
  return !!o && o.level.unused_invite_limit > 0 && o.unused_count >= o.level.unused_invite_limit
})
const blocked = computed(() => !overview.value?.enabled || lifetimeFull.value || unusedFull.value)
const issueNote = computed(() => {
  const o = overview.value
  if (!o) return ''
  if (!o.enabled) return t(rebatesEnabled.value ? 'invitations.issue.paused' : 'invitations.enterprise.paused')
  if (lifetimeFull.value) return t('invitations.quota.lifetimeFull')
  if (unusedFull.value) return t('invitations.quota.unusedFull')
  return t('invitations.issue.validity', { days: o.validity_days })
})
const rewards = computed(() => {
  const s = overview.value?.summary
  return [
    { key: 'pending', amount: s?.pending ?? 0, tone: 'text-amber-700 dark:text-amber-400' },
    { key: 'paid', amount: s?.paid ?? 0, tone: 'text-emerald-700 dark:text-emerald-400' },
    { key: 'declined', amount: s?.rejected_or_revoked ?? 0, tone: 'text-gray-500 dark:text-dark-400' }
  ]
})
const steps = computed(() => {
  const items = [t('invitations.steps.one'), t('invitations.steps.two')]
  if (rebatesEnabled.value) items.push(t('invitations.steps.three', { rate: formatRate(overview.value?.level.rebate_rate ?? 0) }))
  return items
})
const autoNote = computed(() => {
  if (!rebatesEnabled.value) return ''
  const hours = overview.value?.auto_approve_hours ?? 0
  if (hours <= 0) return ''
  const { unit, count } = waitingPeriod(hours)
  return t(unit === 'days' ? 'invitations.steps.autoDays' : 'invitations.steps.autoHours', { count })
})
const tabs = computed(() => kinds.value.map((key) => ({ key, label: t(`invitations.tabs.${key}`) })))

async function loadOverview() {
  const version = ++loadVersion
  loading.value = true
  loadError.value = ''
  try {
    // The newest unused code is what the user came to share; the full history lives in the tab below.
    const [nextOverview, unused] = await Promise.all([
      invitationsAPI.overview(),
      invitationsAPI.codes({ page: 1, page_size: 1, status: 'unused' })
    ])
    if (version !== loadVersion) return
    overview.value = nextOverview
    latest.value = unused.items?.[0] ?? null
  } catch (error) {
    if (version === loadVersion) loadError.value = invitationErrorMessage(error, t, t('invitations.records.loadFailed'))
  } finally {
    if (version === loadVersion) loading.value = false
  }
}

async function refresh() {
  refreshKey.value++
  await loadOverview()
}

async function apply() {
  if (applying.value) return
  // A pending request must stay confirmable even when the quota now looks full:
  // the earlier attempt may be the very code that filled it.
  if (!pendingRequest.value && blocked.value) return
  const owner = storageOwner.value
  const requestId = pendingRequest.value || beginApply(owner)
  applying.value = true
  applyError.value = ''
  try {
    const invite = await invitationsAPI.apply(requestId)
    clearPendingApply(owner)
    pendingRequest.value = ''
    latest.value = invite.status === 'unused' ? invite : latest.value
    appStore.showSuccess(t('invitations.issue.applied'))
    await refresh()
  } catch (error) {
    if (isUncertainApplyError(error)) {
      pendingRequest.value = requestId
      applyError.value = t('invitations.issue.uncertain')
    } else {
      // A business rejection is final for this request id: the server checks for an existing request first.
      clearPendingApply(owner)
      pendingRequest.value = ''
      applyError.value = invitationErrorMessage(error, t, t('invitations.issue.failed'))
      void loadOverview()
    }
  } finally {
    applying.value = false
  }
}

function discardPending() {
  clearPendingApply(storageOwner.value)
  pendingRequest.value = ''
  applyError.value = ''
  void refresh()
}

watch(tab, (value) => visited.add(value))
watch(rebatesEnabled, (enabled) => { if (!enabled) tab.value = 'codes' })
watch(storageOwner, (owner) => { pendingRequest.value = loadPendingApply(owner) }, { immediate: true })
onMounted(loadOverview)
</script>
