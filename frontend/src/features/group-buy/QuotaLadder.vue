<template>
  <div class="space-y-2" data-test="quota-ladder">
    <div class="relative min-w-0 overflow-x-auto rounded-lg border border-gray-200/70 dark:border-dark-600/70">
      <table class="w-full text-left text-xs text-gray-700 dark:text-gray-300">
        <caption class="sr-only">{{ discounted ? t('groupBuy.quotaGrowthDiscountedCaption') : t('groupBuy.quotaGrowth') }}</caption>
        <thead class="bg-gray-50/80 text-gray-500 dark:bg-dark-800/60 dark:text-gray-400">
          <tr>
            <th scope="col" class="px-2 sm:px-3 py-2 font-medium">{{ t('groupBuy.quotaStage') }}</th>
            <th scope="col" class="px-2 sm:px-3 py-2 text-right font-medium">{{ t('groupBuy.totalQuota') }}</th>
            <th scope="col" class="px-2 sm:px-3 py-2 text-right font-medium">{{ t('groupBuy.weeklyQuota') }}</th>
            <th scope="col" class="px-2 sm:px-3 py-2 text-right font-medium">{{ discounted ? t('groupBuy.effectiveRateDiscounted') : t('groupBuy.effectiveRate') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(step, index) in steps"
            :key="step.members"
            class="border-t border-gray-200/60 dark:border-dark-600/60"
          >
            <th scope="row" class="whitespace-nowrap px-2 sm:px-3 py-3 font-medium">
              {{ index === 0 ? t('groupBuy.initialPurchase') : t('groupBuy.reachMembers', { members: step.members }) }}
            </th>
            <td class="whitespace-nowrap px-2 sm:px-3 py-3 text-right tabular-nums">
              <strong class="text-sm text-gray-900 dark:text-white">{{ usd(step.quota_usd) }}</strong>
              <p v-if="index > 0" class="gb-accent mt-1 whitespace-normal">
                {{ t('groupBuy.tierIncrease', { amount: usd(step.increase) }) }}
              </p>
            </td>
            <td class="whitespace-nowrap px-2 sm:px-3 py-3 text-right tabular-nums">{{ usd(step.quota_usd / 4) }}</td>
            <td class="whitespace-nowrap px-2 sm:px-3 py-3 text-right tabular-nums">
              <template v-if="discounted">
                <span data-test="original-rate" class="mb-1 block text-gray-400 line-through dark:text-gray-500 sm:mb-0 sm:mr-2 sm:inline">
                  <span class="sr-only">{{ t('groupBuy.rateOriginal') }} </span>{{ step.originalRate }}
                </span>
                <span data-test="current-rate" class="block font-semibold text-green-600 dark:text-green-400 sm:inline">
                  <span class="sr-only">{{ t('groupBuy.effectiveRateDiscounted') }} </span>{{ step.effectiveRate }}
                </span>
              </template>
              <span v-else data-test="current-rate">{{ step.effectiveRate }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.quotaGrowthHint') }}
    </p>
    <p v-if="discounted" class="text-xs leading-5 text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.effectiveRateDiscountedHint') }}
    </p>
    <p v-else class="text-xs leading-5 text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.effectiveRateHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupBuyProduct } from '@/types/groupBuy'
import { quotaSteps, usd } from './model'

const props = defineProps<{ product: GroupBuyProduct; amountCny?: number; originalAmountCny?: number }>()
// Checkout supplies the validated coupon amount before payment fees. Other
// callers continue to show the product's listed price; never mutate its snapshot.
const originalAmount = computed(() => props.originalAmountCny ?? props.product.price_cny)
const amount = computed(() => props.amountCny ?? originalAmount.value)
const discounted = computed(() => amount.value < originalAmount.value)
const steps = computed(() => quotaSteps(props.product).map(step => ({
  ...step,
  originalRate: step.quota_usd > 0 ? (originalAmount.value / step.quota_usd).toFixed(4) : '—',
  effectiveRate: step.quota_usd > 0 ? (amount.value / step.quota_usd).toFixed(4) : '—'
})))
const { t } = useI18n()
</script>
