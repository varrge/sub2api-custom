<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card overflow-hidden" data-testid="invitation-stats">
        <div v-if="statsError" role="alert" class="flex flex-wrap items-center justify-between gap-3 px-5 py-4 text-sm text-red-800 dark:text-red-200 sm:px-6">
          <span><span class="font-medium">{{ t('invitations.stats.loadFailed') }}</span>{{ t('invitations.sep') }}{{ statsError }}</span>
          <button type="button" class="btn btn-secondary btn-sm" @click="loadStats">{{ t('invitations.records.retry') }}</button>
        </div>
        <dl v-else class="grid divide-y divide-gray-100 dark:divide-dark-700 sm:grid-cols-2 sm:divide-y-0 lg:grid-cols-4 lg:divide-x">
          <div class="px-5 py-4 sm:px-6">
            <dt class="text-sm text-gray-500 dark:text-dark-400">{{ t('invitations.stats.pending') }}</dt>
            <dd class="mt-1">
              <button
                type="button"
                class="text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
                data-testid="invitation-stats-pending"
                @click="openQueue"
              >
                <span class="text-xl font-semibold tabular-nums" :class="stats?.pending_count ? 'text-amber-700 dark:text-amber-400' : 'text-gray-900 dark:text-white'">{{ money(stats?.pending_amount) }}</span>
                <span class="block text-xs text-gray-500 underline-offset-2 hover:underline dark:text-dark-400">{{ t('invitations.stats.pendingCount', { count: stats?.pending_count ?? '—' }) }}</span>
              </button>
            </dd>
          </div>
          <div class="px-5 py-4 sm:px-6">
            <dt class="text-sm text-gray-500 dark:text-dark-400">{{ t('invitations.stats.approved') }}</dt>
            <dd class="mt-1">
              <span class="text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ money(stats?.approved_amount) }}</span>
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('invitations.stats.pendingCount', { count: stats?.approved_count ?? '—' }) }}</span>
            </dd>
          </div>
          <div class="px-5 py-4 sm:px-6">
            <dt class="text-sm text-gray-500 dark:text-dark-400">{{ t('invitations.stats.declined') }}</dt>
            <dd class="mt-1">
              <span class="text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ money(stats?.declined_amount) }}</span>
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('invitations.stats.pendingCount', { count: stats?.declined_count ?? '—' }) }}</span>
            </dd>
          </div>
          <div class="px-5 py-4 sm:px-6">
            <dt class="text-sm text-gray-500 dark:text-dark-400">{{ t('invitations.stats.codes') }}</dt>
            <dd class="mt-1">
              <span class="text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ stats?.total_invites ?? '—' }}</span>
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('invitations.stats.codesDetail', { used: stats?.used_invites ?? '—', active: stats?.active_invites ?? '—' }) }}</span>
            </dd>
          </div>
        </dl>
        <p
          v-if="settings && !settings.enabled"
          class="border-t border-gray-100 bg-amber-50 px-5 py-3 text-sm text-amber-900 dark:border-dark-700 dark:bg-amber-900/20 dark:text-amber-200 sm:px-6"
          data-testid="invitation-paused"
        >
          {{ t('invitations.settings.enabledOff') }}
        </p>
      </section>

      <section class="card p-5 sm:p-6">
        <InvitationTabs v-model="tab" :tabs="tabs" :label="t('invitations.adminTitle')" id-prefix="admin-invitation" />

        <div v-show="tab === 'review'" id="admin-invitation-panel-review" class="pt-5" role="tabpanel" aria-labelledby="admin-invitation-tab-review">
          <p class="mb-4 max-w-3xl text-xs text-gray-500 dark:text-dark-400">{{ t('invitations.rewards.note') }}</p>
          <InvitationRecords
            v-if="visited.has('review')"
            :key="`review-${queueKey}`"
            kind="rebates"
            admin
            :initial-status="reviewStatus"
            :user-id="filterUserId"
            :refresh-key="refreshKey"
            @user="selectUser"
            @clear-user="filterUserId = undefined"
            @review="openReview"
          />
        </div>

        <div v-show="tab === 'codes'" id="admin-invitation-panel-codes" class="pt-5" role="tabpanel" aria-labelledby="admin-invitation-tab-codes">
          <InvitationRecords
            v-if="visited.has('codes')"
            kind="codes"
            admin
            :user-id="filterUserId"
            :refresh-key="refreshKey"
            @user="selectUser"
            @clear-user="filterUserId = undefined"
            @changed="onCodeVoided"
          />
        </div>

        <div v-show="tab === 'rules'" id="admin-invitation-panel-rules" class="pt-5" role="tabpanel" aria-labelledby="admin-invitation-tab-rules">
          <div v-if="settingsLoading && !settings" class="flex justify-center py-8" role="status" :aria-label="t('common.loading')">
            <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
          </div>
          <div v-else-if="settingsError && !settings" role="alert" class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-200" data-testid="invitation-settings-load-error">
            <span><span class="font-medium">{{ t('invitations.settings.loadFailed') }}</span>{{ t('invitations.sep') }}{{ settingsError }}</span>
            <button type="button" class="btn btn-secondary btn-sm" @click="loadSettings">{{ t('invitations.records.retry') }}</button>
          </div>
          <InvitationSettingsForm v-else-if="settings" :settings="settings" @saved="onSettingsSaved" @dirty="rulesDirty = $event" />
        </div>

        <div v-show="tab === 'users'" id="admin-invitation-panel-users" class="pt-5" role="tabpanel" aria-labelledby="admin-invitation-tab-users">
          <InvitationUserLevel
            v-if="visited.has('users')"
            :user-id="selectedUserId"
            :levels="settings?.levels ?? []"
            :refresh-key="userRefreshKey"
            @select="selectedUserId = $event"
            @records="showUserRecords"
            @changed="loadStats"
          />
        </div>
      </section>
    </div>

    <InvitationReviewDialog :rebate="reviewTarget" :action="reviewAction" @close="reviewTarget = null" @changed="onReviewed" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import InvitationRecords from '@/components/invitations/InvitationRecords.vue'
import InvitationReviewDialog from '@/components/invitations/InvitationReviewDialog.vue'
import InvitationSettingsForm from '@/components/invitations/InvitationSettingsForm.vue'
import InvitationTabs from '@/components/invitations/InvitationTabs.vue'
import InvitationUserLevel from '@/components/invitations/InvitationUserLevel.vue'
import {
  invitationsAPI,
  type InvitationRebate,
  type InvitationSettings,
  type RebateStatus,
  type InvitationStats,
  type ReviewAction
} from '@/api/invitations'
import { invitationErrorMessage } from '@/utils/invitationErrors'
import { formatMoney } from '@/utils/invitationFormat'

type Tab = 'review' | 'codes' | 'rules' | 'users'

const { t } = useI18n()
const tab = ref<Tab>('review')
const visited = reactive(new Set<Tab>(['review']))
const stats = ref<InvitationStats | null>(null)
const statsError = ref('')
const settings = ref<InvitationSettings | null>(null)
const settingsLoading = ref(false)
const settingsError = ref('')
const rulesDirty = ref(false)
const refreshKey = ref(0)
const userRefreshKey = ref(0)
const queueKey = ref(0)
// The review list opens on the pending queue; a user's own history opens on every status.
const reviewStatus = ref<RebateStatus | ''>('pending')
const filterUserId = ref<number>()
const selectedUserId = ref<number>()
const reviewTarget = ref<InvitationRebate | null>(null)
const reviewAction = ref<ReviewAction>('approve')

const tabs = computed(() => [
  { key: 'review', label: t('invitations.tabs.review'), count: stats.value?.pending_count },
  { key: 'codes', label: t('invitations.tabs.codes') },
  { key: 'rules', label: t('invitations.tabs.rules'), dot: rulesDirty.value ? t('invitations.tabs.unsaved') : undefined },
  { key: 'users', label: t('invitations.tabs.users') }
])

function money(value: number | undefined): string {
  return value === undefined ? '—' : formatMoney(value)
}

async function loadStats() {
  statsError.value = ''
  try {
    stats.value = await invitationsAPI.stats()
  } catch (error) {
    statsError.value = invitationErrorMessage(error, t, t('invitations.stats.loadFailed'))
  }
}
async function loadSettings() {
  settingsLoading.value = true
  settingsError.value = ''
  try {
    settings.value = await invitationsAPI.settings()
  } catch (error) {
    settingsError.value = invitationErrorMessage(error, t, t('invitations.settings.loadFailed'))
  } finally {
    settingsLoading.value = false
  }
}

function onSettingsSaved(saved: InvitationSettings) {
  settings.value = saved
  // Level names and limits feed the selected user's overview.
  userRefreshKey.value++
}
function selectUser(id: number) {
  selectedUserId.value = id
  tab.value = 'users'
}
function showUserRecords(target: 'codes' | 'review', id: number) {
  filterUserId.value = id
  if (target === 'review') {
    reviewStatus.value = ''
    queueKey.value++
  }
  tab.value = target
}
/** Jump to the pending queue across all inviters, resetting any filters left on the review list. */
function openQueue() {
  filterUserId.value = undefined
  reviewStatus.value = 'pending'
  queueKey.value++
  tab.value = 'review'
}
function openReview(rebate: InvitationRebate, action: ReviewAction) {
  reviewAction.value = action
  reviewTarget.value = rebate
}
function onCodeVoided() {
  // The inviter got a slot back, and the code totals changed.
  userRefreshKey.value++
  void loadStats()
}
function onReviewed() {
  refreshKey.value++
  userRefreshKey.value++
  void loadStats()
}

function warnUnsaved(event: BeforeUnloadEvent) {
  if (!rulesDirty.value) return
  event.preventDefault()
  event.returnValue = ''
}
onBeforeRouteLeave(() => !rulesDirty.value || window.confirm(t('invitations.settings.leaveConfirm')))

watch(tab, (value) => visited.add(value))
onMounted(() => {
  window.addEventListener('beforeunload', warnUnsaved)
  void loadStats()
  void loadSettings()
})
onBeforeUnmount(() => window.removeEventListener('beforeunload', warnUnsaved))
</script>
