<template>
  <section class="gb-panel overflow-hidden p-5 sm:p-6">
    <h2 class="mb-4 font-semibold text-gray-900 dark:text-white">{{ t('groupBuy.allocations') }}</h2>
    <p v-if="!requests.length" class="text-sm text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.noAllocations') }}
    </p>
    <div v-else class="-mx-5 overflow-x-auto px-5 sm:-mx-6 sm:px-6">
      <table class="gb-table w-full text-left text-sm text-gray-700 dark:text-gray-300">
        <thead>
          <tr>
            <th>{{ t('groupBuy.request') }}</th>
            <th>{{ t('groupBuy.allocation') }}</th>
            <th>{{ t('groupBuy.balanceCharge') }}</th>
            <th>{{ t('groupBuy.actualCost') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="request in requests" :key="request.key">
            <td>
              <p
                class="max-w-56 truncate font-mono text-xs"
                :title="request.request_id"
              >
                {{ request.request_id }}
              </p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                {{ exactDate(request.started_at) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                API Key #{{ request.api_key_id }} ·
                {{ t('groupBuy.group') }} #{{ request.group_id }}
              </p>
            </td>
            <td>
              <p
                v-for="part in request.parts.filter(
                  (part) => part.kind !== 'balance'
                )"
                :key="part.id"
              >
                {{
                  part.kind === 'card'
                    ? t('groupBuy.cardCode')
                    : t('groupBuy.legacy')
                }}
                {{
                  (part.kind === 'card' ? cards.find((card) => card.id === part.entitlement_id)?.code : undefined) ||
                  `#${part.entitlement_id}`
                }}: {{ usd(part.amount_usd) }}
              </p>
            </td>
            <td>
              {{
                usd(
                  request.parts
                    .filter((part) => part.kind === 'balance')
                    .reduce((sum, part) => sum + part.amount_usd, 0)
                )
              }}
            </td>
            <td class="font-semibold text-gray-900 dark:text-white">
              {{
                usd(
                  request.parts.reduce((sum, part) => sum + part.amount_usd, 0)
                )
              }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ChargeAllocation, MonthCard } from '@/types/groupBuy'
import { exactDate, usd } from './model'
import './glass.css'
const props = withDefaults(
  defineProps<{ allocations: ChargeAllocation[]; cards?: MonthCard[] }>(),
  { cards: () => [] }
)
const { t } = useI18n()
const requests = computed(() => {
  const groups = new Map<string, ChargeAllocation[]>()
  for (const part of props.allocations) {
    const key = `${part.api_key_id}:${part.request_id}`
    groups.set(key, [...(groups.get(key) || []), part])
  }
  return Array.from(groups, ([key, parts]) => ({ ...parts[0], key, parts }))
})
</script>
