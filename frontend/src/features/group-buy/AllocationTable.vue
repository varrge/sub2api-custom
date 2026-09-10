<template>
  <section class="card overflow-hidden p-5">
    <h2 class="mb-4 font-semibold">{{ t('groupBuy.allocations') }}</h2>
    <p v-if="!requests.length" class="text-sm text-gray-500">
      {{ t('groupBuy.noAllocations') }}
    </p>
    <div v-else class="overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-xs text-gray-500">
          <tr>
            <th class="p-2">{{ t('groupBuy.request') }}</th>
            <th class="p-2">{{ t('groupBuy.allocation') }}</th>
            <th class="p-2">{{ t('groupBuy.balanceCharge') }}</th>
            <th class="p-2">{{ t('groupBuy.actualCost') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="request in requests"
            :key="request.key"
            class="border-t border-gray-100 dark:border-dark-700"
          >
            <td class="p-2">
              <p
                class="max-w-56 truncate font-mono"
                :title="request.request_id"
              >
                {{ request.request_id }}
              </p>
              <p class="text-xs text-gray-500">
                {{ exactDate(request.started_at) }}
              </p>
              <p class="text-xs text-gray-500">
                API Key #{{ request.api_key_id }} ·
                {{ t('groupBuy.group') }} #{{ request.group_id }}
              </p>
            </td>
            <td class="p-2">
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
            <td class="p-2">
              {{
                usd(
                  request.parts
                    .filter((part) => part.kind === 'balance')
                    .reduce((sum, part) => sum + part.amount_usd, 0)
                )
              }}
            </td>
            <td class="p-2 font-semibold">
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
