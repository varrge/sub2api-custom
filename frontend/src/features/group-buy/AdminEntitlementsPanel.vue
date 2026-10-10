<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('groupBuy.entitlementsTitle') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('groupBuy.entitlementsHint') }}</p>
      </div>
      <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadList">{{ t('common.refresh') }}</button>
    </div>
    <div class="gb-tabs" role="tablist" :aria-label="t('groupBuy.cards')">
      <button v-for="value in ['teams', 'solo'] as const" :key="value" type="button" role="tab"
        class="gb-tab" :class="{ 'gb-tab-active': scope === value }" :aria-selected="scope === value" @click="scope = value">
        {{ t(value === 'teams' ? 'groupBuy.teamEntitlements' : 'groupBuy.soloEntitlements') }}
      </button>
    </div>
    <form class="flex flex-wrap items-end gap-3" @submit.prevent="applySearch">
      <label class="min-w-48 flex-1 text-sm text-gray-600 dark:text-gray-300">
        {{ t('common.search') }}
        <input v-model="searchInput" class="input mt-1 w-full" maxlength="200" :placeholder="t('groupBuy.entitlementSearch')" />
      </label>
      <label class="text-sm text-gray-600 dark:text-gray-300">
        {{ t('groupBuy.group') }}
        <select v-model.number="groupId" class="input mt-1 block w-full">
          <option :value="0">{{ t('groupBuy.allGroups') }}</option>
          <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
        </select>
      </label>
      <label class="text-sm text-gray-600 dark:text-gray-300">
        {{ t('groupBuy.entitlementValidity') }}
        <select v-model="validity" class="input mt-1 block w-full">
          <option value="active">{{ t('groupBuy.effectiveEntitlements') }}</option>
          <option value="expired">{{ t('groupBuy.ineffectiveEntitlements') }}</option>
          <option value="all">{{ t('groupBuy.allEntitlements') }}</option>
        </select>
      </label>
      <label class="text-sm text-gray-600 dark:text-gray-300">
        {{ t('groupBuy.entitlementPageSize') }}
        <select v-model.number="pageSize" class="input mt-1 block w-full">
          <option v-for="size in [20, 50, 100]" :key="size" :value="size">{{ size }}</option>
        </select>
      </label>
      <button type="submit" class="btn btn-primary">{{ t('common.search') }}</button>
    </form>
    <div v-if="error" class="gb-notice gb-notice-error flex items-center justify-between gap-3 p-3" role="alert">
      <span>{{ error }}</span><button type="button" class="underline" @click="loadList">{{ t('groupBuy.retry') }}</button>
    </div>
    <p v-if="quotaSuccess" role="status" class="gb-notice gb-notice-info p-3">{{ quotaSuccess }}</p>
    <template v-if="!error">
      <AdminEntitlementTeamsTable v-if="scope === 'teams'" :items="teams" :loading="loading" @inspect="inspectTeam" />
      <AdminEntitlementMembersTable v-else :items="soloCards" :loading="loading" :selected-ids="soloSelectedIds" :selectable-ids="soloSelectableIds" :disabled="mutationSaving"
        @inspect-user="inspectUser" @toggle-select="toggleSelection('solo', $event)" @toggle-all="toggleAll('solo')" @clear-selection="soloSelectedIds = []"
        @adjust-quota="openQuota([$event])" @adjust-selected="openSelectedQuotas('solo')" @reset-usage="openReset([$event])" @reset-selected="openSelectedReset('solo')" />
      <Pagination v-if="total > 0" :total="total" :page="page" :page-size="pageSize" :show-page-size-selector="false"
        @update:page="page = $event" />
    </template>

    <BaseDialog :show="!!selectedTeam" :title="`${t('groupBuy.teamCode')} ${selectedTeam?.code ?? ''}`" width="full" :close-on-escape="!quotaCards.length && !resetCards.length" @close="closeTeam">
      <div v-if="selectedTeam" class="space-y-4">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ selectedTeam.product.name }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ selectedTeam.product.group_name }} · {{ cny(selectedTeam.product.price_cny) }} · {{ t(`groupBuy.${selectedTeam.status}`) }}</p>
          </div>
          <button type="button" class="btn btn-secondary" :disabled="membersLoading" @click="loadMembers">{{ t('common.refresh') }}</button>
        </div>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('groupBuy.memberUsageHint') }}</p>
        <div v-if="membersError" class="gb-notice gb-notice-error p-3" role="alert">
          {{ membersError }} <button type="button" class="underline" @click="loadMembers">{{ t('groupBuy.retry') }}</button>
        </div>
        <template v-else>
          <p v-if="quotaSuccess" role="status" class="gb-notice gb-notice-info p-3">{{ quotaSuccess }}</p>
          <AdminEntitlementMembersTable :items="members" :loading="membersLoading" :selected-ids="memberSelectedIds" :selectable-ids="memberSelectableIds" :disabled="mutationSaving"
            @inspect-user="inspectUser" @toggle-select="toggleSelection('members', $event)" @toggle-all="toggleAll('members')" @clear-selection="memberSelectedIds = []"
            @adjust-quota="openQuota([$event])" @adjust-selected="openSelectedQuotas('members')" @reset-usage="openReset([$event])" @reset-selected="openSelectedReset('members')" />
          <Pagination v-if="memberTotal > 0" :total="memberTotal" :page="memberPage" :page-size="20" :show-page-size-selector="false" @update:page="memberPage = $event" />
        </template>
        <div v-if="selectedUserId" class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-600">
          <div class="flex justify-between gap-2">
            <h3 class="font-semibold">{{ t('groupBuy.userAllocationTitle', { id: selectedUserId }) }}</h3>
            <button type="button" class="btn btn-secondary btn-sm" @click="closeUser">{{ t('common.close') }}</button>
          </div>
          <p v-if="userLoading">{{ t('common.loading') }}</p>
          <p v-else-if="userError" class="gb-notice gb-notice-error p-3" role="alert">{{ userError }}</p>
          <AllocationTable v-else :allocations="allocations" :cards="userCards" />
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="!!selectedUserId && !selectedTeam" :title="t('groupBuy.userAllocationTitle', { id: selectedUserId })" width="extra-wide" @close="closeUser">
      <p v-if="userLoading">{{ t('common.loading') }}</p>
      <p v-else-if="userError" class="gb-notice gb-notice-error p-3" role="alert">{{ userError }}</p>
      <AllocationTable v-else :allocations="allocations" :cards="userCards" />
    </BaseDialog>
    <AdminQuotaAdjustmentDialog :show="!!quotaCards.length" :cards="quotaCards" :saving="quotaSaving" :error="quotaError"
      v-model:draft="quotaDraft" @close="closeQuota" @save="saveQuota" />
    <AdminUsageResetDialog :show="!!resetCards.length" :cards="resetCards" :saving="resetSaving" :error="resetError"
      v-model:draft="resetDraft" @close="closeReset" @save="saveReset" />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import type { UsageResetDraft, QuotaAdjustmentDraft, QuotaAdjustmentRequest, AdminEntitlementQuery, AdminMonthCard, AdminTeamEntitlement, ChargeAllocation, EntitlementValidity, MonthCard } from '@/types/groupBuy'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import AdminEntitlementTeamsTable from './AdminEntitlementTeamsTable.vue'
import AdminEntitlementMembersTable from './AdminEntitlementMembersTable.vue'
import AllocationTable from './AllocationTable.vue'
import AdminQuotaAdjustmentDialog from './AdminQuotaAdjustmentDialog.vue'
import AdminUsageResetDialog from './AdminUsageResetDialog.vue'
import { cny } from './model'
import { extractApiErrorMessage } from '@/utils/apiError'

defineProps<{ groups: { id: number; name: string }[] }>()
const { t } = useI18n()
const scope = ref<'teams' | 'solo'>('teams')
const validity = ref<EntitlementValidity>('active')
const groupId = ref(0)
const searchInput = ref('')
const search = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const teams = ref<AdminTeamEntitlement[]>([])
const soloCards = ref<AdminMonthCard[]>([])
const loading = ref(false)
const error = ref('')
const soloSelectedIds = ref<number[]>([])
const memberSelectedIds = ref<number[]>([])
const quotaSuccess = ref('')
let listController: AbortController | undefined
let listRequest = 0

function query(): AdminEntitlementQuery {
  return { page: page.value, page_size: pageSize.value, validity: validity.value, search: search.value || undefined, group_id: groupId.value || undefined }
}
async function loadList() {
  soloSelectedIds.value = []
  const request = ++listRequest
  listController?.abort()
  listController = new AbortController()
  loading.value = true
  error.value = ''
  teams.value = []
  soloCards.value = []
  total.value = 0
  try {
    const result = scope.value === 'teams'
      ? await adminGroupBuyAPI.entitlementTeams(query(), listController.signal)
      : await adminGroupBuyAPI.soloEntitlements(query(), listController.signal)
    if (request !== listRequest) return
    if (scope.value === 'teams') teams.value = result.items as AdminTeamEntitlement[]
    else soloCards.value = result.items as AdminMonthCard[]
    total.value = result.total
    // A refund or expiry can remove the final item on a page.
    if (page.value > 1 && !result.items.length) page.value = Math.max(1, Math.ceil(result.total / pageSize.value))
  } catch (err) {
    if (request === listRequest) error.value = extractApiErrorMessage(err, t('groupBuy.loadFailed'))
  } finally {
    if (request === listRequest) loading.value = false
  }
}
function applySearch() {
  const value = searchInput.value.trim()
  if (value === search.value && page.value === 1) void loadList()
  else { search.value = value; page.value = 1 }
}
watch([scope, validity, groupId, search, pageSize], () => { page.value = 1 }, { flush: 'sync' })
watch([scope, validity, groupId, search, pageSize, page], loadList, { immediate: true })

const selectedTeam = ref<AdminTeamEntitlement | null>(null)
const members = ref<AdminMonthCard[]>([])
const memberTotal = ref(0)
const memberPage = ref(1)
const membersLoading = ref(false)
const membersError = ref('')
let membersController: AbortController | undefined
let membersRequest = 0
function inspectTeam(team: AdminTeamEntitlement) { closeUser(); selectedTeam.value = team; memberPage.value = 1 }
function closeTeam() { if (quotaCards.value.length || resetCards.value.length) return; memberSelectedIds.value = []; closeUser(); selectedTeam.value = null; membersController?.abort(); ++membersRequest }
async function loadMembers() {
  memberSelectedIds.value = []
  if (!selectedTeam.value) return
  const request = ++membersRequest
  membersController?.abort()
  membersController = new AbortController()
  membersLoading.value = true
  membersError.value = ''
  members.value = []
  memberTotal.value = 0
  try {
    const result = await adminGroupBuyAPI.entitlementTeamCards(selectedTeam.value.code,
      { page: memberPage.value, page_size: 20, validity: 'all' }, membersController.signal)
    if (request !== membersRequest) return
    members.value = result.items
    memberTotal.value = result.total
  } catch (err) {
    if (request === membersRequest) membersError.value = extractApiErrorMessage(err, t('groupBuy.loadFailed'))
  } finally {
    if (request === membersRequest) membersLoading.value = false
  }
}
watch([selectedTeam, memberPage], loadMembers)

const selectedUserId = ref<number | null>(null)
const userCards = ref<MonthCard[]>([])
const allocations = ref<ChargeAllocation[]>([])
const userLoading = ref(false)
const userError = ref('')
let userRequest = 0
function closeUser() { selectedUserId.value = null; ++userRequest }
async function inspectUser(userId: number) {
  const request = ++userRequest
  selectedUserId.value = userId
  userLoading.value = true
  userError.value = ''
  userCards.value = []
  allocations.value = []
  try {
    const [cards, charges] = await Promise.all([adminGroupBuyAPI.cards(userId), adminGroupBuyAPI.allocations(userId)])
    if (request !== userRequest) return
    userCards.value = cards
    allocations.value = charges
  } catch (err) {
    if (request === userRequest) userError.value = extractApiErrorMessage(err, t('groupBuy.loadFailed'))
  } finally {
    if (request === userRequest) userLoading.value = false
  }
}
let disposed = false
onBeforeUnmount(() => { disposed = true; ++listRequest; ++membersRequest; ++userRequest; listController?.abort(); membersController?.abort() })
function canAdjust(card: AdminMonthCard) {
  return (card.status === 'active' || card.status === 'frozen') && Date.parse(card.starts_at) <= Date.now() && Date.parse(card.expires_at) > Date.now()
}
const soloSelectableIds = computed(() => soloCards.value.filter(canAdjust).map(card => card.id))
const memberSelectableIds = computed(() => members.value.filter(canAdjust).map(card => card.id))
function selection(scope: 'solo' | 'members') {
  return scope === 'solo'
    ? { ids: soloSelectedIds, eligible: soloSelectableIds, cards: soloCards }
    : { ids: memberSelectedIds, eligible: memberSelectableIds, cards: members }
}
function toggleSelection(scope: 'solo' | 'members', id: number) {
  const state = selection(scope)
  if (mutationSaving.value || !state.eligible.value.includes(id)) return
  state.ids.value = state.ids.value.includes(id) ? state.ids.value.filter(value => value !== id) : [...state.ids.value, id]
}
function toggleAll(scope: 'solo' | 'members') {
  if (mutationSaving.value) return
  const state = selection(scope)
  state.ids.value = state.eligible.value.every(id => state.ids.value.includes(id)) ? [] : [...state.eligible.value]
}
function openSelectedQuotas(scope: 'solo' | 'members') {
  const state = selection(scope)
  openQuota(state.cards.value.filter(card => state.ids.value.includes(card.id)))
}
const quotaCards = ref<AdminMonthCard[]>([])
const quotaSaving = ref(false)
const quotaError = ref('')
const quotaDraft = ref<QuotaAdjustmentDraft>({ setTotal: false, setWeekly: false, total: '', weekly: '' })
function openQuota(cards: AdminMonthCard[]) {
  if (mutationSaving.value || resetCards.value.length || !cards.length || cards.some(card => !canAdjust(card))) return
  quotaCards.value = cards.map(card => ({ ...card }))
  quotaError.value = ''
  quotaSuccess.value = ''
  quotaDraft.value = { setTotal: false, setWeekly: false,
    total: cards.length === 1 ? cards[0].total_quota_usd.toFixed(8).replace(/\.?0+$/, '') : '',
    weekly: cards.length === 1 ? cards[0].weekly_quota_usd.toFixed(8).replace(/\.?0+$/, '') : '' }
}
function closeQuota() { if (!quotaSaving.value) { quotaCards.value = []; quotaError.value = '' } }
async function saveQuota() {
  if (quotaSaving.value || !quotaCards.value.length) return
  quotaError.value = ''
  const draft = quotaDraft.value
  if (!draft.setTotal && !draft.setWeekly) { quotaError.value = t('groupBuy.quotaChooseLimit'); return }
  const request: QuotaAdjustmentRequest = { card_ids: quotaCards.value.map(card => card.id) }
  for (const [enabled, raw, key] of [
    [draft.setTotal, draft.total, 'total_quota_usd'],
    [draft.setWeekly, draft.weekly, 'weekly_quota_usd']
  ] as const) {
    if (!enabled) continue
    const value = raw.trim()
    if (!/^\d+(\.\d{1,8})?$/.test(value) || Number(value) <= 0 || Number(value) > 1_000_000_000) {
      quotaError.value = t('groupBuy.quotaInvalidAmount'); return
    }
    request[key] = value
  }
  for (const card of quotaCards.value) {
    const total = request.total_quota_usd === undefined ? card.total_quota_usd : Number(request.total_quota_usd)
    const weekly = request.weekly_quota_usd === undefined ? card.weekly_quota_usd : Number(request.weekly_quota_usd)
    if (total < card.total_used_usd || weekly < card.weekly_used_usd || weekly > total) {
      quotaError.value = t('groupBuy.quotaInvalidLimits', { code: card.code }); return
    }
  }
  quotaSaving.value = true
  try {
    const result = await adminGroupBuyAPI.adjustQuotas(request)
    if (disposed) return
    quotaCards.value = []
    quotaSuccess.value = t('groupBuy.quotaSaved', { count: result.updated_count })
    // Reload cards, team aggregates, and any open customer view after committing.
    await Promise.all([loadList(), loadMembers(), selectedUserId.value ? inspectUser(selectedUserId.value) : Promise.resolve()])
  } catch (err) {
    if (!disposed) quotaError.value = extractApiErrorMessage(err, t('groupBuy.quotaSaveFailed'))
  } finally {
    if (!disposed) quotaSaving.value = false
  }
}
const resetCards = ref<AdminMonthCard[]>([])
const resetSaving = ref(false)
const resetError = ref('')
const resetDraft = ref<UsageResetDraft>({ resetTotal: false, resetWeekly: false })
const mutationSaving = computed(() => quotaSaving.value || resetSaving.value)
function openSelectedReset(scope: 'solo' | 'members') {
  const state = selection(scope)
  openReset(state.cards.value.filter(card => state.ids.value.includes(card.id)))
}
function openReset(cards: AdminMonthCard[]) {
  if (mutationSaving.value || quotaCards.value.length || !cards.length || cards.some(card => !canAdjust(card))) return
  resetCards.value = cards.map(card => ({ ...card }))
  resetError.value = ''
  quotaSuccess.value = ''
  resetDraft.value = { resetTotal: false, resetWeekly: false }
}
function closeReset() { if (!resetSaving.value) { resetCards.value = []; resetError.value = '' } }
async function saveReset() {
  if (mutationSaving.value || !resetCards.value.length) return
  resetError.value = ''
  const { resetTotal, resetWeekly } = resetDraft.value
  if (!resetTotal && !resetWeekly) { resetError.value = t('groupBuy.usageResetChoose'); return }
  resetSaving.value = true
  try {
    const result = await adminGroupBuyAPI.resetUsage({ card_ids: resetCards.value.map(card => card.id), reset_total: resetTotal, reset_weekly: resetWeekly })
    if (disposed) return
    resetCards.value = []
    quotaSuccess.value = t('groupBuy.usageResetSaved', { count: result.updated_count })
    await Promise.all([loadList(), loadMembers(), selectedUserId.value ? inspectUser(selectedUserId.value) : Promise.resolve()])
  } catch (err) {
    if (!disposed) resetError.value = extractApiErrorMessage(err, t('groupBuy.usageResetFailed'))
  } finally {
    if (!disposed) resetSaving.value = false
  }
}
defineExpose({ refresh: loadList })
</script>
