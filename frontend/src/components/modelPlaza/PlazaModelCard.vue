<template>
  <article class="model-card flex min-w-0 flex-col overflow-hidden rounded-[22px] border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800/60" :data-model="model.name">
    <header class="min-h-32 px-5 pb-4 pt-5">
      <div class="flex items-start gap-3">
        <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-600 dark:bg-dark-800">
          <PlatformIcon :platform="model.brand.platform" size="lg" :class="model.brand.id === 'claude' ? 'text-orange-500' : 'text-gray-900 dark:text-white'" />
        </div>
        <div class="min-w-0 flex-1">
          <h3 class="break-words text-[17px] font-bold leading-snug tracking-tight text-gray-950 [overflow-wrap:anywhere] dark:text-white">{{ model.name }}</h3>
          <div class="mt-2 flex flex-wrap items-center gap-1.5">
            <span v-if="selected" class="rounded-full border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs font-semibold text-blue-600 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-300">{{ t('modelPlaza.catalog.multiplier', { rate: selected.effectiveRate }) }}</span>
            <span v-if="selected?.model.pricing?.max_reasoning_effort_multiplier" class="rounded-full border border-amber-200 bg-amber-50 px-2 py-1 text-[11px] text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300" :title="t('modelPlaza.table.maxReasoningMultiplierHint', { multiplier: selected.model.pricing.max_reasoning_effort_multiplier })">{{ t('modelPlaza.table.maxReasoningMultiplierBadge', { multiplier: selected.model.pricing.max_reasoning_effort_multiplier }) }}</span>
            <span v-if="threshold" class="rounded-full border border-violet-200 bg-violet-50 px-2 py-1 text-[11px] font-semibold text-violet-600 dark:border-violet-800 dark:bg-violet-900/20 dark:text-violet-300" :title="t('modelPlaza.catalog.tierHint')">&gt;{{ formatTokenLimit(threshold) }}</span>
          </div>
          <div v-if="selected" class="mt-2 flex flex-wrap items-center gap-1.5">
            <span v-if="isText" class="capability" :title="t('modelPlaza.catalog.types.text')">T</span>
            <span v-if="selected.model.metadata?.supports_vision === true" class="capability" :title="t('modelPlaza.catalog.vision')" :aria-label="t('modelPlaza.catalog.vision')">
              <svg class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><rect x="2.5" y="3" width="15" height="14" rx="2" /><path d="m4 14 4-4 3 3 2-2 3 3" /><circle cx="12.8" cy="7" r="1" /></svg>
            </span>
            <span v-if="!isText" class="text-xs text-gray-400">{{ t('modelPlaza.catalog.types.' + modelType) }}</span>
          </div>
        </div>
      </div>
    </header>

    <div v-if="metadata?.context_window || metadata?.max_output_tokens" class="flex flex-wrap gap-x-4 gap-y-1 border-t border-gray-100 px-5 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
      <span v-if="metadata.context_window">{{ t('modelPlaza.catalog.contextWindow') }} <strong class="ml-1 font-semibold tabular-nums text-gray-700 dark:text-dark-100">{{ formatTokenLimit(metadata.context_window) }}</strong></span>
      <span v-if="metadata.max_output_tokens">{{ t('modelPlaza.catalog.maxOutput') }} <strong class="ml-1 font-semibold tabular-nums text-gray-700 dark:text-dark-100">{{ formatTokenLimit(metadata.max_output_tokens) }}</strong></span>
    </div>

    <div v-if="selected" class="grid grid-cols-2 border-t border-gray-100 dark:border-dark-700">
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
    <div v-else class="border-t border-gray-100 px-5 py-7 dark:border-dark-700">
      <p class="font-mono text-2xl text-gray-400">—</p>
      <p class="mt-3 text-sm leading-6 text-gray-500 dark:text-dark-400">{{ t('modelPlaza.catalog.unavailableInReference') }}</p>
    </div>
    <p v-if="hasSpecialPricing" class="px-5 py-2 text-[11px] text-amber-600 dark:text-amber-400">{{ t('modelPlaza.catalog.baseTierNote') }}</p>

    <footer class="mt-auto border-t border-gray-100 bg-gray-50/40 p-4 dark:border-dark-700 dark:bg-dark-900/30">
      <div class="mb-2 flex items-start justify-between gap-3 text-xs">
        <p class="min-w-0 text-gray-400">{{ t('modelPlaza.catalog.groupPrices') }}<span v-if="selected" class="ml-1 break-words text-gray-600 dark:text-dark-300">{{ selected.group.name }}</span></p>
        <button v-if="selected" type="button" class="shrink-0 rounded text-primary-600 underline-offset-4 hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-primary-300" @click="emit('open-group-detail', selected)">{{ t('modelPlaza.pricingDetail') }}</button>
      </div>
      <div class="flex flex-wrap gap-1.5">
        <button v-for="variant in model.variants" :key="variantKey(variant)" type="button" class="group-chip" :class="{ 'group-chip-active': selected && variantKey(selected) === variantKey(variant) }" :aria-pressed="!!selected && variantKey(selected) === variantKey(variant)" @click="overrideKey = variantKey(variant)">
          {{ variant.group.name }}<span v-if="hasDuplicateGroup(variant)" class="ml-1 opacity-70">({{ variant.model.platform }})</span>
          ×{{ variant.effectiveRate }}
          <span v-if="variant.group.subscription_type === 'subscription'" class="ml-1 text-[10px] opacity-70">{{ t('modelPlaza.badges.subscription') }}</span>
        </button>
      </div>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import {
  type AggregatedPlazaModel, type GroupModelVariant, type PlazaPriceCell,
  formatUsdDirect, formatTokenLimit, modelPriceCells, plazaModelType, selectPriceVariant, variantKey
} from './plaza-models'

const props = defineProps<{ model: AggregatedPlazaModel; priceGroupId: number | null }>()
const emit = defineEmits<{ 'open-group-detail': [GroupModelVariant] }>()
const { t } = useI18n()
const overrideKey = ref<string | null>(null)
watch([() => props.priceGroupId, () => props.model.id], () => { overrideKey.value = null })
const selected = computed(() => selectPriceVariant(props.model, props.priceGroupId, overrideKey.value))
const cells = computed(() => selected.value ? modelPriceCells(selected.value) : [])
const metadata = computed(() => selected.value?.model.metadata)
const modelType = computed(() => selected.value ? plazaModelType(selected.value.model) : 'other')
const isText = computed(() => modelType.value === 'text')
const threshold = computed(() => {
  if (selected.value?.model.pricing?.billing_mode !== 'token') return undefined
  return selected.value.model.pricing.intervals.map(tier => tier.min_tokens).filter(value => value > 0).sort((a, b) => a - b)[0]
})
const hasSpecialPricing = computed(() => !!selected.value && (
  !!threshold.value || !!selected.value.model.time_pricing?.periods.length || selected.value.group.peak_rate_enabled || !!selected.value.model.pricing?.max_reasoning_effort_multiplier
))
function priceLabel(cell: PlazaPriceCell) {
  if (cell.id === 'cache_write_price' && cells.value.some(entry => entry.id === 'cache_write_1h_price')) return t('modelPlaza.catalog.cacheWrite5m')
  return cell.labelKey.startsWith('modelPlaza.') ? t(cell.labelKey) : cell.labelKey
}
function hasDuplicateGroup(variant: GroupModelVariant) {
  return props.model.variants.filter(entry => entry.group.id === variant.group.id).length > 1
}
</script>

<style scoped>
.price-cell { @apply border-b border-gray-100 dark:border-dark-700; }
.price-cell:nth-child(odd) { @apply border-r border-gray-100 dark:border-dark-700; }
:deep(.price-value) { font-variant-numeric: tabular-nums; }
.capability { @apply inline-flex h-6 min-w-8 items-center justify-center rounded-full border border-gray-200 bg-gray-50 px-2 font-serif text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300; }
.group-chip { @apply max-w-full break-words rounded-full border border-gray-200 bg-white px-2.5 py-1 text-left text-xs font-medium text-gray-500 transition hover:border-primary-300 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300; }
.group-chip-active { @apply border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-700 dark:bg-primary-900/30 dark:text-primary-300; }
</style>
