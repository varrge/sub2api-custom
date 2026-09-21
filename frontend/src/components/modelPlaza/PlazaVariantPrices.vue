<template>
  <section :data-price-group="variant.group.id" :data-price-platform="variant.model.platform">
    <header class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 bg-gray-50/60 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/30">
      <div class="min-w-0">
        <p class="break-words text-sm font-semibold text-gray-800 dark:text-dark-100">{{ variant.group.name }}<span v-if="showPlatform" class="ml-1 text-xs font-normal text-gray-500">({{ variant.model.platform }})</span></p>
        <p class="mt-1 text-xs text-primary-600 dark:text-primary-300">{{ t('modelPlaza.catalog.multiplier', { rate: variant.effectiveRate }) }}</p>
      </div>
      <span v-if="variant.group.subscription_type === 'subscription'" class="rounded-md bg-violet-50 px-2 py-1 text-xs text-violet-600 dark:bg-violet-900/20 dark:text-violet-300">{{ t('modelPlaza.badges.subscription') }}</span>
      <span v-if="variant.group.is_exclusive" class="rounded-md bg-purple-50 px-2 py-1 text-xs text-purple-600 dark:bg-purple-900/20 dark:text-purple-300">{{ t('modelPlaza.badges.exclusive') }}</span>
      <button type="button" class="min-h-9 shrink-0 rounded text-xs text-primary-600 underline-offset-4 hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-primary-300" @click="$emit('open-group-detail', variant)">{{ t('modelPlaza.pricingDetail') }}</button>
      <span v-if="variant.model.pricing?.max_reasoning_effort_multiplier" class="text-xs text-amber-700 dark:text-amber-300" :title="t('modelPlaza.table.maxReasoningMultiplierHint', { multiplier: variant.model.pricing.max_reasoning_effort_multiplier })">{{ t('modelPlaza.table.maxReasoningMultiplierBadge', { multiplier: variant.model.pricing.max_reasoning_effort_multiplier }) }}</span>
      <span v-if="threshold" class="text-xs text-violet-600 dark:text-violet-300" :title="t('modelPlaza.catalog.tierHint')">&gt;{{ formatTokenLimit(threshold) }}</span>
    </header>
<div class="grid grid-cols-2 border-t border-gray-100 dark:border-dark-700">
      <div v-for="cell in cells" :key="cell.id" class="price-cell min-w-0 px-4 py-4">
        <p class="flex items-start gap-1.5 text-xs font-medium text-gray-600 dark:text-dark-300"><span class="mt-0.5 h-3 w-0.5 shrink-0 rounded bg-gray-400 dark:bg-dark-400"></span>{{ priceLabel(cell) }}</p>
        <p class="mt-2 flex flex-wrap items-baseline gap-x-1 leading-tight">
          <span class="price-value whitespace-nowrap font-mono text-[22px] font-bold tracking-tight text-gray-950 dark:text-white" :data-price="cell.id">{{ formatUsdDirect(cell.price) ?? '—' }}</span>
          <span v-if="cell.price !== null" class="text-[11px] text-gray-400">{{ t(cell.unitKey) }}</span>
        </p>
        <p v-if="cell.original !== null && cell.original !== cell.price" class="mt-2 text-[11px] text-gray-400">
          {{ t('modelPlaza.catalog.original') }} <s v-if="cell.price !== null">{{ formatUsdDirect(cell.original) }}</s><span v-else>{{ formatUsdDirect(cell.original) }}</span> {{ t(cell.unitKey) }}
        </p>
      </div>
      <p v-if="!cells.length" class="col-span-2 px-5 py-7 text-sm text-gray-400">{{ t('modelPlaza.detail.noPricing') }}</p>
    </div>
    <p v-if="hasSpecialPricing" class="px-4 py-2 text-[11px] text-amber-600 dark:text-amber-400">{{ t('modelPlaza.catalog.baseTierNote') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { type GroupModelVariant, type PlazaPriceCell, modelPriceCells, formatUsdDirect, formatTokenLimit } from './plaza-models'
const props = defineProps<{ variant: GroupModelVariant; showPlatform: boolean }>()
defineEmits<{ 'open-group-detail': [GroupModelVariant] }>()
const { t } = useI18n()
const cells = computed(() => modelPriceCells(props.variant))
const threshold = computed(() => props.variant.model.pricing?.billing_mode === 'token'
  ? props.variant.model.pricing.intervals.map(tier => tier.min_tokens).filter(value => value > 0).sort((a, b) => a - b)[0]
  : undefined)
const hasSpecialPricing = computed(() => !!threshold.value || !!props.variant.model.time_pricing?.periods.length || props.variant.group.peak_rate_enabled || !!props.variant.model.pricing?.max_reasoning_effort_multiplier)
function priceLabel(cell: PlazaPriceCell) {
  if (cell.id === 'cache_write_price' && cells.value.some(entry => entry.id === 'cache_write_1h_price')) return t('modelPlaza.catalog.cacheWrite5m')
  return cell.labelKey.startsWith('modelPlaza.') ? t(cell.labelKey) : cell.labelKey
}
</script>

<style scoped>
.price-cell { @apply border-b border-gray-100 dark:border-dark-700; }
.price-cell:nth-child(odd) { @apply border-r border-gray-100 dark:border-dark-700; }
.price-value { font-variant-numeric: tabular-nums; }
</style>
