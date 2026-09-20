<template>
  <div class="space-y-2">
    <div class="overflow-x-auto rounded-lg border border-gray-200/70 dark:border-dark-600/70">
      <table class="w-full text-left text-xs text-gray-700 dark:text-gray-300">
        <caption class="sr-only">{{ t('groupBuy.quotaGrowth') }}</caption>
        <thead class="bg-gray-50/80 text-gray-500 dark:bg-dark-800/60 dark:text-gray-400">
          <tr>
            <th scope="col" class="px-3 py-2 font-medium">{{ t('groupBuy.quotaStage') }}</th>
            <th scope="col" class="px-3 py-2 text-right font-medium">{{ t('groupBuy.totalQuota') }}</th>
            <th scope="col" class="px-3 py-2 text-right font-medium">{{ t('groupBuy.weeklyQuota') }}</th>
            <th scope="col" class="px-3 py-2 text-right font-medium">{{ t('groupBuy.effectiveRate') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(step, index) in quotaSteps(product)"
            :key="step.members"
            class="border-t border-gray-200/60 dark:border-dark-600/60"
          >
            <th scope="row" class="whitespace-nowrap px-3 py-3 font-medium">
              {{ index === 0 ? t('groupBuy.initialPurchase') : t('groupBuy.reachMembers', { members: step.members }) }}
            </th>
            <td class="whitespace-nowrap px-3 py-3 text-right tabular-nums">
              <strong class="text-sm text-gray-900 dark:text-white">{{ usd(step.quota_usd) }}</strong>
              <p v-if="index > 0" class="gb-accent mt-1">
                {{ t('groupBuy.tierIncrease', { amount: usd(step.increase) }) }}
              </p>
            </td>
            <td class="whitespace-nowrap px-3 py-3 text-right tabular-nums">{{ usd(step.quota_usd / 4) }}</td>
            <td class="whitespace-nowrap px-3 py-3 text-right tabular-nums">
              {{ step.quota_usd > 0 ? (product.price_cny / step.quota_usd).toFixed(4) : '—' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.quotaGrowthHint') }}
    </p>
    <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.effectiveRateHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { GroupBuyProduct } from '@/types/groupBuy'
import { quotaSteps, usd } from './model'

defineProps<{ product: GroupBuyProduct }>()
const { t } = useI18n()
</script>
