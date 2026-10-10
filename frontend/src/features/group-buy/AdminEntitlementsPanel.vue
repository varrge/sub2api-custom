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
      <span>{{ error }}</span><button type="button" class="underline" @click="loadList">{{ t('common.retry') }}</button>
    </div>
    <template v-else>
      <AdminEntitlementTeamsTable v-if="scope === 'teams'" :items="teams" :loading="loading" @inspect="inspectTeam" />
      <AdminEntitlementMembersTable v-else :items="soloCards" :loading="loading" @inspect-user="inspectUser" />
      <Pagination v-if="total > 0" :total="total" :page="page" :page-size="pageSize" :show-page-size-selector="false"
        @update:page="page = $event" />
    </template>

    <BaseDialog :show="!!selectedTeam" :title="`${t('groupBuy.teamCode')} ${selectedTeam?.code ?? ''}`" width="full" @close="closeTeam">
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
          {{ membersError }} <button type="button" class="underline" @click="loadMembers">{{ t('common.retry') }}</button>
        </div>
        <template v-else>
          <AdminEntitlementMembersTable :items="members" :loading="membersLoading" @inspect-user="inspectUser" />
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
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import type { AdminEntitlementQuery, AdminMonthCard, AdminTeamEntitlement, ChargeAllocation, EntitlementValidity, MonthCard } from '@/types/groupBuy'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import AdminEntitlementTeamsTable from './AdminEntitlementTeamsTable.vue'
import AdminEntitlementMembersTable from './AdminEntitlementMembersTable.vue'
import AllocationTable from './AllocationTable.vue'
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
let listController: AbortController | undefined
let listRequest = 0

function query(): AdminEntitlementQuery {
  return { page: page.value, page_size: pageSize.value, validity: validity.value, search: search.value || undefined, group_id: groupId.value || undefined }
}
async function loadList() {
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
function closeTeam() { closeUser(); selectedTeam.value = null; membersController?.abort(); ++membersRequest }
async function loadMembers() {
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
onBeforeUnmount(() => { ++listRequest; ++membersRequest; ++userRequest; listController?.abort(); membersController?.abort() })
defineExpose({ refresh: loadList })
</script>
