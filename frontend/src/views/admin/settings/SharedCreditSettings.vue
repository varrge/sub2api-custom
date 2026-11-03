<template>
  <section class="space-y-6" data-testid="shared-credit-settings">
    <header class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-200 pb-4 dark:border-dark-700">
      <div>
        <h2 class="flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-dark-50">
          <Icon name="creditCard" class="text-primary-500" />
          {{ t('sharedCredit.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
          {{ t('sharedCredit.description') }}
        </p>
      </div>
      <div class="flex items-center gap-3">
        <button
          type="button"
          class="btn btn-secondary flex items-center gap-1.5 text-sm"
          :disabled="loading || saving"
          @click="handleExplicitReload"
          data-testid="shared-credit-reload"
        >
          <Icon name="refresh" :class="{ 'animate-spin': loading }" class="h-4 w-4" />
          {{ t('sharedCredit.reload') }}
        </button>
        <button
          type="button"
          class="btn btn-primary flex items-center gap-1.5 text-sm"
          :disabled="saving || loading || loadingGroups || groupsError || !dirty || !!validationError || conflictDetected"
          @click="handleSave"
          data-testid="shared-credit-save"
        >
          <svg v-if="saving" class="h-4 w-4 animate-spin text-white" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <Icon v-else name="check" class="h-4 w-4" />
          {{ t('sharedCredit.save') }}
        </button>
      </div>
    </header>

    <!-- 409 Conflict Banner -->
    <div
      v-if="conflictDetected"
      class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200"
      role="alert"
    >
      <div class="flex items-center gap-2">
        <Icon name="exclamationTriangle" class="h-5 w-5 flex-shrink-0 text-amber-500" />
        <span>{{ t('sharedCredit.conflictDetected') }}</span>
      </div>
      <button
        type="button"
        class="btn btn-secondary text-xs"
        @click="handleForceReloadOverwritingDraft"
        data-testid="shared-credit-reload-conflict"
      >
        {{ t('sharedCredit.overwriteAndReload') }}
      </button>
    </div>

    <!-- Error Banner -->
    <div
      v-if="error"
      class="flex items-center justify-between rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/40 dark:text-red-300"
      role="alert"
    >
      <span>{{ error }}</span>
      <button type="button" class="btn btn-secondary text-xs" :disabled="loading || saving" @click="handleExplicitReload">
        {{ t('sharedCredit.retry') }}
      </button>
    </div>

    <!-- Loading State -->
    <div v-if="loading && !serverState" class="p-12 text-center text-sm text-gray-500" role="status">
      <div class="inline-flex items-center gap-2">
        <svg class="h-5 w-5 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
        {{ t('sharedCredit.loading') }}
      </div>
    </div>

    <div v-else-if="serverState" class="space-y-6">
      <!-- Stat Cards -->
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('sharedCredit.stats.total') }}</div>
          <div class="mt-1 break-all text-base font-bold tabular-nums text-gray-900 dark:text-dark-50 sm:text-xl">
            ${{ serverState?.total_usd || '0.00' }}
          </div>
          <div class="mt-1 text-xs text-gray-400">{{ t('sharedCredit.stats.totalHint') }}</div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('sharedCredit.stats.used') }}</div>
          <div class="mt-1 break-all text-base font-bold tabular-nums text-emerald-600 dark:text-emerald-400 sm:text-xl">
            ${{ serverState?.used_usd || '0.00' }}
          </div>
          <div class="mt-1 text-xs text-gray-400">{{ t('sharedCredit.stats.usedHint') }}</div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('sharedCredit.stats.reserved') }}</div>
          <div class="mt-1 break-all text-base font-bold tabular-nums text-amber-600 dark:text-amber-400 sm:text-xl">
            ${{ serverState?.reserved_usd || '0.00' }}
          </div>
          <div class="mt-1 text-xs text-gray-400">{{ t('sharedCredit.stats.reservedHint') }}</div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('sharedCredit.stats.remaining') }}</div>
          <div class="mt-1 break-all text-base font-bold tabular-nums text-primary-600 dark:text-primary-400 sm:text-xl">
            ${{ serverState?.remaining_usd || '0.00' }}
          </div>
          <div class="mt-1 text-xs text-gray-400">{{ t('sharedCredit.stats.remainingHint') }}</div>
        </div>
      </div>

      <!-- Explanation Notice -->
      <div class="rounded-xl border border-blue-100 bg-blue-50/50 p-4 text-xs leading-relaxed text-blue-900 dark:border-blue-900/40 dark:bg-blue-950/20 dark:text-blue-300">
        <p class="font-medium text-blue-800 dark:text-blue-200">{{ t('sharedCredit.rules.title') }}</p>
        <ul class="mt-2 list-inside list-disc space-y-1">
          <li>{{ t('sharedCredit.rules.rule1') }}</li>
          <li>{{ t('sharedCredit.rules.rule2') }}</li>
          <li>{{ t('sharedCredit.rules.rule3') }}</li>
          <li>{{ t('sharedCredit.rules.rule4') }}</li>
        </ul>
      </div>

      <!-- Settings Form Controls (Plain div, no nested form) -->
      <fieldset :disabled="saving || loading" class="min-w-0 space-y-6 rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
        <!-- Global Enable Toggle -->
        <div class="flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-900/40">
          <div>
            <span class="block text-sm font-semibold text-gray-900 dark:text-dark-50">{{ t('sharedCredit.fields.enable') }}</span>
            <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('sharedCredit.fields.enableHint') }}</span>
          </div>
          <Toggle
            v-model="draft.enabled"
            data-testid="shared-credit-enabled"
            :aria-label="t('sharedCredit.fields.enable')"
          />
        </div>

        <!-- Cumulative Total Budget -->
        <SharedCreditActivityFields
          v-model:name="draft.name"
          v-model:starts-at="draft.starts_at"
          v-model:ends-at="draft.ends_at"
          :status="serverState.status"
        />
        <div class="space-y-1.5">
          <label for="shared-credit-total-input" class="block text-sm font-medium text-gray-700 dark:text-dark-300">
            {{ t('sharedCredit.fields.totalUsd') }}
          </label>
          <div class="relative max-w-md">
            <span class="absolute inset-y-0 left-0 flex items-center pl-3 text-sm text-gray-500">$</span>
            <input
              id="shared-credit-total-input"
              type="text"
              v-model="draft.total_usd"
              class="input pl-7"
              placeholder="0.00000000"
              autocomplete="off"
              data-testid="shared-credit-total"
            />
          </div>
          <p v-if="validationError" class="text-xs text-red-600 dark:text-red-400">{{ validationError }}</p>
          <p v-else class="text-xs text-gray-500 dark:text-dark-400">
            {{ t('sharedCredit.fields.totalUsdHint') }}
          </p>
        </div>

        <!-- Group Multi-selection -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <label class="block text-sm font-medium text-gray-700 dark:text-dark-300">
              {{ t('sharedCredit.fields.participatingGroups') }}
            </label>
            <span class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('sharedCredit.fields.selectedCount', { count: draft.group_ids.length }) }}
            </span>
          </div>
          <p class="text-xs text-gray-500 dark:text-dark-400">
            {{ t('sharedCredit.fields.participatingGroupsHint') }}
          </p>

          <div v-if="groupsError" class="text-sm text-red-600" role="alert">
            {{ t('sharedCredit.groupsFailed') }} <button type="button" class="btn btn-secondary" :disabled="loadingGroups" @click="loadGroups">{{ t('sharedCredit.retry') }}</button>
          </div>
          <div v-if="loadingGroups" class="py-4 text-xs text-gray-400">
            {{ t('sharedCredit.loadingGroups') }}
          </div>
          <div v-else-if="allGroups.length === 0" class="py-4 text-xs text-gray-400">
            {{ t('sharedCredit.noGroups') }}
          </div>
          <div
            v-else
            class="grid max-h-64 grid-cols-1 gap-2 overflow-y-auto rounded-lg border border-gray-200 p-3 dark:border-dark-700 sm:grid-cols-2 lg:grid-cols-3"
          >
            <label
              v-for="group in allGroups"
              :key="group.id"
              class="flex cursor-pointer items-center gap-2.5 rounded-md p-2 hover:bg-gray-50 dark:hover:bg-dark-700/60"
              :data-testid="`shared-credit-group-${group.id}`"
            >
              <input
                type="checkbox"
                class="checkbox rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
                :checked="draft.group_ids.includes(group.id)"
                @change="toggleGroup(group.id)"
              />
              <div class="min-w-0 flex-1 truncate text-xs">
                <span class="font-medium text-gray-900 dark:text-dark-100">{{ group.name }}</span>
                <span class="ml-1.5 text-gray-400">(ID: {{ group.id }})</span>
                <span v-if="group.status === 'inactive'" class="ml-1 text-amber-500">[{{ t('sharedCredit.inactive') }}]</span>
              </div>
            </label>
          </div>
        </div>
      </fieldset>
      <p v-if="saved" role="status" class="text-sm text-emerald-600">{{ t('sharedCredit.saved') }}</p>

      <!-- Usage Ledger Section -->
      <div class="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-wrap items-center justify-between border-b border-gray-200 p-4 dark:border-dark-700 sm:px-6">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-dark-50">{{ t('sharedCredit.ledger.title') }}</h3>
          <button
            type="button"
            class="btn btn-secondary px-2.5 py-1 text-xs"
            :disabled="loadingLedger"
            @click="loadUsage(ledgerPage)"
          >
            <Icon name="refresh" :class="{ 'animate-spin': loadingLedger }" class="h-3.5 w-3.5" />
            {{ t('sharedCredit.ledger.refresh') }}
          </button>
        </div>

        <p v-if="ledgerError" role="alert" class="p-4 text-sm text-red-600">{{ t('sharedCredit.ledgerFailed') }}</p>
        <div v-else-if="loadingLedger && ledgerItems.length === 0" class="p-8 text-center text-xs text-gray-500">
          {{ t('sharedCredit.loading') }}
        </div>
        <div v-else-if="ledgerItems.length === 0" class="p-8 text-center text-xs text-gray-400">
          {{ t('sharedCredit.ledger.empty') }}
        </div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 text-left text-xs dark:divide-dark-700">
            <thead class="bg-gray-50 text-gray-500 dark:bg-dark-900/50 dark:text-dark-400">
              <tr>
                <th class="py-3 pl-4 pr-3 sm:pl-6">{{ t('sharedCredit.ledger.time') }}</th>
                <th class="px-3 py-3">{{ t('sharedCredit.ledger.user') }}</th>
                <th class="px-3 py-3">{{ t('sharedCredit.ledger.group') }}</th>
                <th class="px-3 py-3">{{ t('sharedCredit.ledger.model') }}</th>
                <th class="px-3 py-3 text-right">{{ t('sharedCredit.ledger.amountBase') }}</th>
                <th class="py-3 pl-3 pr-4 text-right sm:pr-6">{{ t('sharedCredit.ledger.coveredCost') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700/60">
              <tr v-for="item in ledgerItems" :key="item.id" class="hover:bg-gray-50/50 dark:hover:bg-dark-700/30">
                <td class="whitespace-nowrap py-3 pl-4 pr-3 font-mono text-gray-500 sm:pl-6">
                  {{ formatTime(item.created_at) }}
                </td>
                <td class="whitespace-nowrap px-3 py-3 text-gray-900 dark:text-dark-100">
                  User #{{ item.user_id }}
                </td>
                <td class="whitespace-nowrap px-3 py-3 text-gray-600 dark:text-dark-300">
                  {{ groupNameMap[item.group_id] || `#${item.group_id}` }}
                </td>
                <td class="whitespace-nowrap px-3 py-3 font-mono text-gray-600 dark:text-dark-300">
                  {{ item.model }}
                </td>
                <td class="whitespace-nowrap px-3 py-3 text-right font-mono font-medium text-emerald-600 dark:text-emerald-400">
                  ${{ item.amount_usd }}
                </td>
                <td class="whitespace-nowrap py-3 pl-3 pr-4 text-right font-mono text-gray-500 sm:pr-6">
                  ${{ item.covered_cost_usd }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Ledger Pagination -->
        <div
          v-if="totalLedgerCount > 20"
          class="flex items-center justify-between border-t border-gray-200 px-4 py-3 dark:border-dark-700 sm:px-6"
        >
          <div class="text-xs text-gray-500">
            {{ t('sharedCredit.ledger.paginationInfo', { page: ledgerPage, total: Math.ceil(totalLedgerCount / 20) }) }}
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="btn btn-secondary px-2.5 py-1 text-xs"
              :disabled="ledgerPage <= 1 || loadingLedger"
              @click="loadUsage(ledgerPage - 1)"
            >
              {{ t('sharedCredit.ledger.prev') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary px-2.5 py-1 text-xs"
              :disabled="ledgerPage * 20 >= totalLedgerCount || loadingLedger"
              @click="loadUsage(ledgerPage + 1)"
            >
              {{ t('sharedCredit.ledger.next') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import SharedCreditActivityFields from './SharedCreditActivityFields.vue'
import Icon from '@/components/icons/Icon.vue'
import * as groupsApi from '@/api/admin/groups'
import {
  getState,
  updateState,
  getUsage,
  type SharedCreditState,
  type SharedCreditAllocation
} from '@/api/admin/sharedCredit'

const { t } = useI18n()

interface GroupItem {
  id: number
  name: string
  status?: string | boolean
}

const serverState = ref<SharedCreditState | null>(null)
const draft = ref<{
	name: string
	starts_at: string | null
	ends_at: string | null
  enabled: boolean
  total_usd: string
  group_ids: number[]
  revision: number
}>({
	name: '',
	starts_at: null,
	ends_at: null,
  enabled: false,
  total_usd: '0.00',
  group_ids: [],
  revision: 0
})

const allGroups = ref<GroupItem[]>([])
const groupNameMap = computed<Record<number, string>>(() => {
  const map: Record<number, string> = {}
  for (const g of allGroups.value) {
    map[g.id] = g.name
  }
  return map
})

const ledgerItems = ref<SharedCreditAllocation[]>([])
const totalLedgerCount = ref(0)
const ledgerPage = ref(1)

const loading = ref(false)
const saving = ref(false)
const loadingGroups = ref(false)
const loadingLedger = ref(false)
const conflictDetected = ref(false)
const error = ref('')
const groupsError = ref(false)
const ledgerError = ref(false)
const saved = ref(false)

const dirty = computed(() => {
  if (!serverState.value) return false
  if (draft.value.name !== serverState.value.name || draft.value.starts_at !== serverState.value.starts_at || draft.value.ends_at !== serverState.value.ends_at) return true
  if (draft.value.enabled !== serverState.value.enabled) return true
  if (draft.value.total_usd !== serverState.value.total_usd) return true
  const draftSet = new Set(draft.value.group_ids)
  const serverSet = new Set(serverState.value.group_ids)
  if (draftSet.size !== serverSet.size) return true
  for (const id of draftSet) {
    if (!serverSet.has(id)) return true
  }
  return false
})

const validationError = computed<string>(() => {
  if (draft.value.enabled && !draft.value.ends_at) return t('sharedCredit.errors.endRequired')
  if ((draft.value.starts_at && !Number.isFinite(Date.parse(draft.value.starts_at))) || (draft.value.ends_at && !Number.isFinite(Date.parse(draft.value.ends_at)))) return t('sharedCredit.errors.invalidDates')
  if (draft.value.starts_at && draft.value.ends_at && Date.parse(draft.value.starts_at) >= Date.parse(draft.value.ends_at)) return t('sharedCredit.errors.invalidDates')
  const val = draft.value.total_usd.trim()
  if (!val) return t('sharedCredit.errors.totalRequired')
  if (val.length > 22 || !/^\d+(\.\d{1,8})?$/.test(val)) {
    return t('sharedCredit.errors.invalidTotalFormat')
  }
  const num = parseFloat(val)
  if (isNaN(num) || num < 0 || num > 1000000000) {
    return t('sharedCredit.errors.totalRange')
  }
  if (serverState.value) {
    const units = (s: string) => { const [whole, fraction = ''] = s.split('.'); return BigInt(whole || '0') * 100000000n + BigInt(fraction.padEnd(8, '0')) }
    const minimum = units(serverState.value.used_usd) + units(serverState.value.reserved_usd)
    if (units(val) < minimum) {
      return t('sharedCredit.errors.totalLessThanUsed', {
        min: `${minimum / 100000000n}.${(minimum % 100000000n).toString().padStart(8, '0')}`
      })
    }
  }
  return ''
})

const loadGroups = async () => {
  loadingGroups.value = true
  groupsError.value = false
  try {
    const res = await groupsApi.getAllIncludingInactive()
    allGroups.value = res
  } catch {
    groupsError.value = true
  } finally {
    loadingGroups.value = false
  }
}

let stateRequestId = 0
const loadState = async (forceOverwriteDraft = false) => {
  if (saving.value) return
  const hadDraft = dirty.value
  const currentReq = ++stateRequestId
  loading.value = true
  error.value = ''
  try {
    const state = await getState()
    if (currentReq !== stateRequestId) return
    serverState.value = state
    if (!hadDraft || forceOverwriteDraft) {
      draft.value = {
        name: state.name,
        starts_at: state.starts_at,
        ends_at: state.ends_at,
        enabled: state.enabled,
        total_usd: state.total_usd,
        group_ids: [...state.group_ids],
        revision: state.revision
      }
      conflictDetected.value = false
    }
  } catch (err: any) {
    if (currentReq === stateRequestId) {
      error.value = err?.message || t('sharedCredit.loadFailed')
    }
  } finally {
    if (currentReq === stateRequestId) {
      loading.value = false
    }
  }
}

const handleExplicitReload = async () => {
  if (dirty.value) {
    if (!window.confirm(t('sharedCredit.confirmDiscardChanges'))) {
      return
    }
  }
  await loadState(true)
  await loadUsage(ledgerPage.value)
}

const handleForceReloadOverwritingDraft = async () => {
  await loadState(true)
}

const toggleGroup = (groupId: number) => {
  const idx = draft.value.group_ids.indexOf(groupId)
  if (idx >= 0) {
    draft.value.group_ids.splice(idx, 1)
  } else {
    draft.value.group_ids.push(groupId)
  }
}

const handleSave = async () => {
  if (saving.value || loading.value || !serverState.value || validationError.value || groupsError.value || loadingGroups.value || conflictDetected.value) return
  saving.value = true
  saved.value = false
  error.value = ''
  conflictDetected.value = false
  try {
    const updated = await updateState({
      name: draft.value.name.trim(),
      starts_at: draft.value.starts_at,
      ends_at: draft.value.ends_at,
      enabled: draft.value.enabled,
      total_usd: draft.value.total_usd.trim(),
      group_ids: [...draft.value.group_ids],
      revision: draft.value.revision
    })
    serverState.value = updated
    draft.value = {
      name: updated.name,
      starts_at: updated.starts_at,
      ends_at: updated.ends_at,
      enabled: updated.enabled,
      total_usd: updated.total_usd,
      group_ids: [...updated.group_ids],
      revision: updated.revision
    }
    await loadUsage(1)
    saved.value = true
  } catch (err: any) {
    if (err?.status === 409 || err?.response?.status === 409) {
      conflictDetected.value = true
    } else {
      error.value = err?.message || t('sharedCredit.saveFailed')
    }
  } finally {
    saving.value = false
  }
}

let usageRequestId = 0
const loadUsage = async (page: number) => {
  const currentReq = ++usageRequestId
  loadingLedger.value = true
  ledgerError.value = false
  try {
    const res = await getUsage(page)
    if (currentReq !== usageRequestId) return
    ledgerItems.value = res.items || []
    totalLedgerCount.value = res.total || 0
    ledgerPage.value = res.page || page
  } catch {
    if (currentReq === usageRequestId) ledgerError.value = true
  } finally {
    if (currentReq === usageRequestId) {
      loadingLedger.value = false
    }
  }
}

const formatTime = (iso: string) => {
  if (!iso) return '-'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

onMounted(() => {
  loadGroups()
  loadState()
  loadUsage(1)
})
</script>
