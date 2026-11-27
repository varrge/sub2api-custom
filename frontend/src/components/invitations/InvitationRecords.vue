<template>
  <section :data-testid="`invitation-records-${kind}`">
    <form class="flex flex-wrap items-center gap-3" role="search" @submit.prevent="submitSearch">
      <div class="relative min-w-0 flex-1 basis-56">
        <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        <input
          v-model="query"
          type="search"
          class="input pl-9"
          maxlength="200"
          :placeholder="searchLabel"
          :aria-label="searchLabel"
          data-testid="invitation-records-search"
        />
      </div>
      <div class="w-40">
        <Select v-model="status" :options="statusOptions" :aria-label="t('invitations.records.status')" />
      </div>
      <span v-if="admin && userId" class="badge badge-primary gap-1.5 py-1" data-testid="invitation-records-user">
        {{ t('invitations.records.onlyUser', { id: userId }) }}
        <button type="button" class="rounded-full hover:text-primary-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:text-primary-200" :aria-label="t('invitations.records.clearFilters')" @click="emit('clearUser')">
          <Icon name="x" size="xs" />
        </button>
      </span>
      <button type="button" class="btn btn-secondary btn-icon" :disabled="loading" :aria-label="t('common.refresh')" :title="t('common.refresh')" @click="load">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
      </button>
    </form>

    <div v-if="loadError" role="alert" class="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-200">
      <span><span class="font-medium">{{ t('invitations.records.loadFailed') }}</span>{{ t('invitations.sep') }}{{ loadError }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('invitations.records.retry') }}</button>
    </div>

    <div class="mt-4">
      <DataTable :columns="columns" :data="records" :loading="loading" row-key="id" :sticky-first-column="false">
        <template #empty>
          <div class="flex flex-col items-center gap-2 py-2" data-testid="invitation-records-empty">
            <Icon name="inbox" size="xl" class="text-gray-300 dark:text-dark-600" />
            <p class="font-medium text-gray-900 dark:text-gray-100">{{ emptyTitle }}</p>
            <p v-if="!filtered" class="text-sm text-gray-500 dark:text-dark-400">{{ t(kind === 'codes' ? 'invitations.records.emptyCodesHint' : 'invitations.records.emptyRebatesHint') }}</p>
            <button v-else type="button" class="btn btn-secondary btn-sm mt-1" @click="clearFilters">{{ t('invitations.records.clearFilters') }}</button>
          </div>
        </template>
        <template #cell-code="{ row }">
          <code class="block break-all font-mono text-xs text-gray-900 dark:text-gray-100" :class="admin ? 'max-w-[9.5rem] whitespace-normal' : ''">{{ row.code }}</code>
          <span v-if="row.rebate_eligible === false" class="badge badge-primary mt-1">{{ t('invitations.enterprise.noRebate') }}</span>
        </template>
        <template #cell-inviter="{ row }">
          <button type="button" class="max-w-[9rem] whitespace-normal 2xl:max-w-[16rem] text-left text-primary-700 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-primary-300" :title="t('invitations.records.viewUser', { id: row.inviter_id })" @click="emit('user', row.inviter_id)">
            <span class="break-all">{{ row.inviter_email }}</span>
            <span class="block text-xs text-gray-500 dark:text-dark-400">#{{ row.inviter_id }}</span>
          </button>
        </template>
        <template #cell-invitee="{ row }">
          <div :class="admin ? 'max-w-[9rem] whitespace-normal 2xl:max-w-[16rem]' : ''">
            <template v-if="row.invitee_email">
              <span class="break-all">{{ row.invitee_email }}</span>
              <span v-if="admin && row.invitee_id" class="text-xs text-gray-500 dark:text-dark-400"> #{{ row.invitee_id }}</span>
            </template>
            <span v-else class="text-gray-400 dark:text-dark-500">{{ t('invitations.records.notUsed') }}</span>
            <!-- Rebates name the code that brought the invitee in; codes show when it was spent. -->
            <code v-if="kind === 'rebates'" class="mt-0.5 block break-all font-mono text-xs text-gray-500 dark:text-dark-400">{{ row.invite_code }}</code>
          </div>
          <span v-if="kind === 'codes' && row.used_at" class="mt-0.5 block whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">{{ formatDateTimeToMinute(row.used_at) }}</span>
        </template>
        <template #cell-status="{ row }">
          <InvitationStatusCell :status="row.status" :reviewed-at="row.reviewed_at" :auto-approved="row.auto_approved" :reason="row.review_reason" />
        </template>
        <template #cell-source="{ row }">
          <span class="whitespace-nowrap font-medium tabular-nums text-gray-900 dark:text-white">{{ formatMoney(row.first_amount, row.source_currency) }} <span class="text-xs font-normal text-gray-500 dark:text-dark-400">{{ row.source_currency }}</span></span>
          <span class="block whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">{{ t(`invitations.sources.${row.source_type}`) }}<template v-if="admin && row.source_id"> #{{ row.source_id }}</template></span>
          <span class="block whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">{{ formatDateTimeToMinute(row.source_occurred_at) }}</span>
          <span v-if="concernOf(row)" class="mt-1 block max-w-[11rem] whitespace-normal text-xs font-medium" :class="concernOf(row) === 'refund' ? 'text-red-700 dark:text-red-300' : 'text-amber-700 dark:text-amber-300'" :data-testid="`rebate-source-concern-${row.id}`">
            {{ t(concernOf(row) === 'refund' ? 'invitations.records.sourceRefund' : 'invitations.records.sourceIncomplete', { status: row.source_status }) }}
          </span>
        </template>
        <template #cell-rebate_amount="{ row }">
          <span class="whitespace-nowrap font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatMoney(row.rebate_amount) }}</span>
          <span class="block whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">{{ row.level_name }}, {{ formatRate(row.rebate_rate) }}</span>
          <!-- The admin table has one column more than fits, so the status lives under the amount it decides. -->
          <div v-if="admin" class="mt-2">
            <InvitationStatusCell :status="row.status" :reviewed-at="row.reviewed_at" :auto-approved="row.auto_approved" :reason="row.review_reason" />
          </div>
        </template>
        <template #cell-created_at="{ row }">
          <span class="whitespace-nowrap">{{ formatDateTimeToMinute(row.created_at) }}</span>
          <span class="block whitespace-nowrap text-xs text-gray-500 dark:text-dark-400" :title="formatDateTimeToMinute(row.expires_at)">{{ t('invitations.issue.expires', { date: formatDateOnly(row.expires_at) }) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div v-if="kind === 'codes' && row.status === 'unused'" class="flex flex-wrap gap-2">
            <template v-if="!admin">
              <button type="button" class="btn btn-secondary btn-sm" @click="copyToClipboard(row.code, t('invitations.issue.codeCopied'))">{{ t('invitations.issue.copyCode') }}</button>
              <button type="button" class="btn btn-secondary btn-sm" @click="copyToClipboard(registrationInvitationLink(row.code), t('invitations.issue.linkCopied'))">{{ t('invitations.issue.copyLink') }}</button>
            </template>
            <button v-else type="button" class="btn btn-secondary btn-icon btn-sm" :aria-label="t('invitations.issue.copyCode')" :title="t('invitations.issue.copyCode')" :data-testid="`invite-copy-${row.id}`" @click="copyToClipboard(row.code, t('invitations.issue.codeCopied'))">
              <Icon name="copy" size="sm" />
            </button>
            <button v-if="admin" type="button" class="btn btn-secondary btn-sm text-red-700 dark:text-red-300" :data-testid="`invite-disable-${row.id}`" @click="disableTarget = row">{{ t('invitations.records.disable') }}</button>
          </div>
          <div v-else-if="kind === 'rebates' && admin" class="flex flex-wrap gap-2">
            <template v-if="row.status === 'pending'">
              <button type="button" class="btn btn-primary btn-sm" :data-testid="`rebate-approve-${row.id}`" @click="emit('review', row, 'approve')">{{ t('invitations.review.approve') }}</button>
              <button type="button" class="btn btn-secondary btn-sm" :data-testid="`rebate-reject-${row.id}`" @click="emit('review', row, 'reject')">{{ t('invitations.review.reject') }}</button>
            </template>
            <button v-else-if="row.status === 'approved'" type="button" class="btn btn-secondary btn-sm" :data-testid="`rebate-revoke-${row.id}`" @click="emit('review', row, 'revoke')">{{ t('invitations.review.revoke') }}</button>
          </div>
        </template>
      </DataTable>
    </div>
    <Pagination v-if="total > 0" class="mt-4" :page="page" :page-size="pageSize" :total="total" @update:page="page = $event" @update:page-size="changePageSize" />

    <ConfirmDialog
      v-if="admin && kind === 'codes'"
      :show="!!disableTarget"
      :title="t('invitations.records.disableTitle')"
      :message="t('invitations.records.disableBody', { email: disableTarget?.inviter_email ?? '' })"
      :confirm-text="t('invitations.records.disableConfirm')"
      danger
      @confirm="disableCode"
      @cancel="disableTarget = null"
    >
      <code v-if="disableTarget" class="block break-all rounded-lg bg-gray-50 px-3 py-2 font-mono text-xs text-gray-900 dark:bg-dark-900 dark:text-gray-100">{{ disableTarget.code }}</code>
    </ConfirmDialog>
  </section>
  <TotpStepUpDialog :controller="stepUp" />
</template>

<script setup lang="ts">
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { isStepUpCancelled, useStepUp } from '@/composables/useStepUp'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import InvitationStatusCell from '@/components/invitations/InvitationStatusCell.vue'
import type { Column } from '@/components/common/types'
import {
  invitationsAPI,
  type InvitationCode,
  type InvitationListParams,
  type InvitationRebate,
  type InviteStatus,
  type RebateStatus,
  type ReviewAction
} from '@/api/invitations'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { formatDateOnly, formatDateTimeToMinute } from '@/utils/format'
import { invitationErrorMessage } from '@/utils/invitationErrors'
import { formatMoney, formatRate, sourceConcern, type SourceConcern } from '@/utils/invitationFormat'
import { registrationInvitationLink } from '@/utils/registrationInvitation'

type Status = InviteStatus | RebateStatus | ''

const props = withDefaults(defineProps<{
  kind: 'codes' | 'rebates'
  admin?: boolean
  /** Admin only: restrict the list to one inviter. */
  userId?: number
  /** Bump to reload after something outside this list changed its data. */
  refreshKey?: number
  /** Status selected when the list first opens, e.g. the pending review queue. */
  initialStatus?: Status
}>(), { admin: false, userId: undefined, refreshKey: 0, initialStatus: '' })

const emit = defineEmits<{
  review: [rebate: InvitationRebate, action: ReviewAction]
  user: [id: number]
  clearUser: []
  /** A code was voided here, so totals shown elsewhere are stale. */
  changed: []
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const appStore = useAppStore()
const stepUp = useStepUp()
const disableTarget = ref<InvitationCode | null>(null)
const disabling = ref(false)
const records = ref<(InvitationCode | InvitationRebate)[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const status = ref<Status>(props.initialStatus)
const query = ref('')
const appliedQuery = ref('')
const loading = ref(false)
const loadError = ref('')
let requestVersion = 0
let searchTimer: ReturnType<typeof setTimeout> | undefined

const statuses = computed<Status[]>(() => props.kind === 'codes' ? ['unused', 'used', 'expired', 'disabled'] : ['pending', 'approved', 'rejected', 'revoked'])
const statusOptions = computed(() => [
  { value: '', label: t('invitations.records.allStatuses') },
  ...statuses.value.map((value) => ({ value, label: t(`invitations.statuses.${value}`) }))
])
const searchLabel = computed(() => t(props.admin ? 'invitations.records.adminSearch' : 'invitations.records.search'))
const filtered = computed(() => !!status.value || !!appliedQuery.value || (props.admin && !!props.userId))
const emptyTitle = computed(() => {
  if (filtered.value) return t('invitations.records.emptyFiltered')
  return t(props.kind === 'codes' ? 'invitations.records.emptyCodes' : 'invitations.records.emptyRebates')
})
const columns = computed<Column[]>(() => {
  const inviter = props.admin ? ['inviter'] : []
  const keys = props.kind === 'codes'
    ? ['code', ...inviter, 'invitee', 'status', 'created_at', 'actions']
    : [...inviter, 'invitee', 'source', 'rebate_amount', ...(props.admin ? ['actions'] : ['status'])]
  return keys.map((key) => ({ key, label: t(`invitations.columns.${key}`) }))
})

function concernOf(row: InvitationCode | InvitationRebate): SourceConcern {
  return props.admin && 'source_type' in row ? sourceConcern(row.source_type, row.source_status) : ''
}

async function disableCode() {
  const target = disableTarget.value
  if (!target || disabling.value) return
  disabling.value = true
  try {
    await stepUp.run(() => invitationsAPI.disableCode(target.id))
    appStore.showSuccess(t('invitations.records.disabled'))
    emit('changed')
  } catch (error) {
    if (isStepUpCancelled(error)) return
    // Most often the code was used or expired a moment ago; the reload below shows its real state.
    appStore.showError(`${t('invitations.records.disableFailed')}${t('invitations.sep')}${invitationErrorMessage(error, t, t('invitations.records.disableFailed'))}`)
  } finally {
    disabling.value = false
    disableTarget.value = null
    void load()
  }
}

async function load() {
  const version = ++requestVersion
  loading.value = true
  loadError.value = ''
  try {
    const params: InvitationListParams = { page: page.value, page_size: pageSize.value }
    if (props.admin && props.userId) params.user_id = props.userId
    if (status.value) params.status = status.value
    if (appliedQuery.value) params.q = appliedQuery.value
    const result = await invitationsAPI[props.kind](params, props.admin)
    if (version !== requestVersion) return
    records.value = result.items || []
    total.value = result.total
    // The last row of a page can disappear after a review; step back instead of showing an empty page.
    if (!records.value.length && total.value > 0 && page.value > 1) page.value = Math.max(1, Math.ceil(total.value / pageSize.value))
  } catch (error) {
    if (version !== requestVersion) return
    records.value = []
    total.value = 0
    loadError.value = invitationErrorMessage(error, t, t('invitations.records.loadFailed'))
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

function reloadFromFirstPage() {
  if (page.value !== 1) page.value = 1
  else void load()
}
function submitSearch() {
  clearTimeout(searchTimer)
  const next = query.value.trim()
  if (next === appliedQuery.value) return
  appliedQuery.value = next
}
function changePageSize(value: number) {
  pageSize.value = value
  reloadFromFirstPage()
}
function clearFilters() {
  clearTimeout(searchTimer)
  query.value = ''
  appliedQuery.value = ''
  status.value = ''
  if (props.admin && props.userId) emit('clearUser')
}

watch(query, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(submitSearch, 350)
})
watch([() => props.userId, status, appliedQuery], reloadFromFirstPage)
watch([page, () => props.refreshKey], () => void load(), { immediate: true })
onBeforeUnmount(() => clearTimeout(searchTimer))

defineExpose({ reload: load, records })
</script>
