<template>
  <div class="gb-panel overflow-x-auto">
    <div v-if="loading" class="flex justify-center p-10">
      <LoadingSpinner size="lg" />
    </div>
    <p
      v-else-if="!items.length"
      class="p-8 text-center text-gray-500 dark:text-gray-400"
    >
      {{ t('groupBuy.noMemberCards') }}
    </p>
    <template v-else>
      <AdminQuotaSelectionBar
        v-if="selectionEnabled"
        :count="selectedIds.length"
        :disabled="disabled"
        @adjust="emit('adjustSelected')"
        @clear="emit('clearSelection')"
      />
      <table
        class="gb-table w-full text-left text-sm text-gray-700 dark:text-gray-300"
      >
        <thead>
          <tr>
            <th v-if="selectionEnabled" scope="col" class="w-10">
              <input
                type="checkbox"
                :checked="allPageSelected"
                :indeterminate="pageSelectionPartial"
                :disabled="disabled || !eligibleIds.length"
                :aria-label="t('groupBuy.selectAllPage')"
                @change="emit('toggleAll')"
              />
            </th>
            <th scope="col">{{ t('groupBuy.memberCustomer') }}</th>
            <th scope="col">{{ t('groupBuy.cardCode') }}</th>
            <th scope="col">{{ t('groupBuy.totalUsage') }}</th>
            <th scope="col">{{ t('groupBuy.weeklyUsage') }}</th>
            <th scope="col">{{ t('groupBuy.availableQuota') }}</th>
            <th scope="col">{{ t('groupBuy.memberValidity') }}</th>
            <th scope="col">{{ t('groupBuy.order') }}</th>
            <th scope="col">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="card in items" :key="card.id">
            <td v-if="selectionEnabled">
              <input
                type="checkbox"
                :checked="selectedIds.includes(card.id)"
                :disabled="disabled || !eligible(card)"
                :aria-label="t('groupBuy.adjustQuota')"
                @change="emit('toggleSelect', card.id)"
              />
            </td>
            <td>
              <p class="font-medium text-gray-900 dark:text-white">
                {{ card.username || `#${card.user_id}` }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ card.email }}
              </p>
              <p class="font-mono text-xs text-gray-500 dark:text-gray-400">
                #{{ card.user_id }}
              </p>
            </td>
            <td>
              <span class="gb-code whitespace-nowrap">{{ card.code }}</span>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ card.product_name }} · {{ card.group_name }}</p>
              <p class="mt-1">
                <span
                  class="gb-pill"
                  :class="
                    card.status === 'active' ? 'gb-pill-teal' : 'gb-pill-gray'
                  "
                >
                  {{ t(`groupBuy.${card.status}`) }}
                </span>
              </p>
            </td>
            <td
              v-for="(quota, quotaIndex) in quotas(card)"
              :key="quotaIndex"
            >
              <div class="min-w-32">
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ usd(quota.used) }} / {{ usd(quota.total) }}
                </p>
                <div class="gb-progress mt-1.5">
                  <div
                    class="gb-progress-fill"
                    :class="{
                      'gb-progress-fill-warn': quota.used >= quota.total
                    }"
                    :style="{ width: `${progress(quota.used, quota.total)}%` }"
                  />
                </div>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{
                    t('groupBuy.quotaRemaining', {
                      amount: usd(Math.max(0, quota.total - quota.used))
                    })
                  }}
                </p>
              </div>
            </td>
            <td>
              <span class="gb-accent-sky font-semibold">{{
                usd(effectiveAvailable(card))
              }}</span>
            </td>
            <td>
              <p class="text-xs whitespace-nowrap">
                <span class="gb-label">{{ t('groupBuy.obtained') }}:</span>
                {{ exactDate(card.starts_at) }}
              </p>
              <p class="mt-1 text-xs whitespace-nowrap">
                <span class="gb-label">{{ t('groupBuy.expires') }}:</span>
                {{ exactDate(card.expires_at) }}
              </p>
            </td>
            <td>
              <RouterLink
                :to="{
                  path: '/admin/orders',
                  query: { order_id: card.order_id, order_type: 'month_card' }
                }"
                class="gb-accent font-medium hover:underline"
              >
                {{ t('groupBuy.order') }} #{{ card.order_id }}
              </RouterLink>
            </td>
            <td>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm whitespace-nowrap"
                  @click="emit('inspectUser', card.user_id)"
                >
                  {{ t('groupBuy.inspectAllocations') }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm whitespace-nowrap"
                  :disabled="disabled || !eligible(card)"
                  @click="emit('adjustQuota', card)"
                >
                  {{ t('groupBuy.adjustQuota') }}
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import AdminQuotaSelectionBar from './AdminQuotaSelectionBar.vue'
import type { AdminMonthCard } from '@/types/groupBuy'
import { exactDate, usd, progress, availableQuota } from './model'
import './glass.css'

const props = withDefaults(
  defineProps<{
    items: AdminMonthCard[]
    loading: boolean
    selectedIds?: number[]
    selectableIds?: number[]
    disabled?: boolean
  }>(),
  { selectedIds: () => [], selectableIds: () => [], disabled: false }
)
const emit = defineEmits<{
  inspectUser: [userId: number]
  toggleSelect: [id: number]
  toggleAll: []
  adjustQuota: [card: AdminMonthCard]
  adjustSelected: []
  clearSelection: []
}>()
const { t } = useI18n()

const selectionEnabled = computed(() => props.selectableIds.length > 0)
const eligible = (card: AdminMonthCard) => props.selectableIds.includes(card.id)
const eligibleIds = computed(() =>
  props.items.filter(eligible).map((card) => card.id)
)
const allPageSelected = computed(
  () =>
    eligibleIds.value.length > 0 &&
    eligibleIds.value.every((id) => props.selectedIds.includes(id))
)
const pageSelectionPartial = computed(
  () =>
    !allPageSelected.value &&
    eligibleIds.value.some((id) => props.selectedIds.includes(id))
)

const quotas = (card: AdminMonthCard) => [
  { used: card.total_used_usd, total: card.total_quota_usd },
  { used: card.weekly_used_usd, total: card.weekly_quota_usd }
]

const effectiveAvailable = (card: AdminMonthCard) =>
  card.status === 'active' && Date.parse(card.starts_at) <= Date.now() && Date.parse(card.expires_at) > Date.now()
    ? availableQuota(card)
    : 0
</script>
