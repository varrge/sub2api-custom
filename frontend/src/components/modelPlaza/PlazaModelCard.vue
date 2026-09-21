<template>
  <article class="model-card flex min-w-0 flex-col overflow-hidden rounded-[22px] border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800/60" :data-model="model.name">
    <header class="min-h-32 px-5 pb-4 pt-5">
      <div class="flex items-start gap-3">
        <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-600 dark:bg-dark-800">
          <PlatformIcon :platform="model.brand.platform" size="lg" :class="model.brand.id === 'claude' ? 'text-orange-500' : 'text-gray-900 dark:text-white'" />
        </div>
        <div class="min-w-0 flex-1">
          <h3 class="break-words text-[17px] font-bold leading-snug tracking-tight text-gray-950 [overflow-wrap:anywhere] dark:text-white">{{ model.name }}</h3>
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

    <div v-if="priceVariants.length">
      <PlazaVariantPrices v-for="variant in priceVariants" :key="variantKey(variant)" :variant="variant" :show-platform="hasDuplicateGroup(variant)" @open-group-detail="emit('open-group-detail', $event)" />
    </div>
    <div v-else class="border-t border-gray-100 px-5 py-7 dark:border-dark-700">
      <p class="font-mono text-2xl text-gray-400">—</p>
      <p class="mt-3 text-sm leading-6 text-gray-500 dark:text-dark-400">{{ t(priceGroupIds?.length === 0 ? 'modelPlaza.catalog.chooseReferenceGroups' : 'modelPlaza.catalog.unavailableInReference') }}</p>
    </div>

    <footer v-if="priceGroupIds === undefined" class="mt-auto border-t border-gray-100 bg-gray-50/40 p-4 dark:border-dark-700 dark:bg-dark-900/30">
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
import PlazaVariantPrices from './PlazaVariantPrices.vue'
import {
  type AggregatedPlazaModel, type GroupModelVariant,
  formatTokenLimit, plazaModelType, selectPriceVariant, variantKey
} from './plaza-models'

const props = defineProps<{ model: AggregatedPlazaModel; priceGroupId: number | null; priceGroupIds?: number[] }>()
const emit = defineEmits<{ 'open-group-detail': [GroupModelVariant] }>()
const { t } = useI18n()
const overrideKey = ref<string | null>(null)
watch([() => props.priceGroupId, () => props.priceGroupIds, () => props.model.id], () => { overrideKey.value = null })
const priceVariants = computed(() => {
  const selectedIds = props.priceGroupIds
  if (selectedIds !== undefined) return props.model.variants.filter(variant => selectedIds.includes(variant.group.id))
  const variant = selectPriceVariant(props.model, props.priceGroupId, overrideKey.value)
  return variant ? [variant] : []
})
const selected = computed(() => priceVariants.value[0] ?? null)
const metadata = computed(() => selected.value?.model.metadata)
const modelType = computed(() => selected.value ? plazaModelType(selected.value.model) : 'other')
const isText = computed(() => modelType.value === 'text')
function hasDuplicateGroup(variant: GroupModelVariant) {
  return props.model.variants.filter(entry => entry.group.id === variant.group.id).length > 1
}
</script>

<style scoped>
:deep(.price-value) { font-variant-numeric: tabular-nums; }
.capability { @apply inline-flex h-6 min-w-8 items-center justify-center rounded-full border border-gray-200 bg-gray-50 px-2 font-serif text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300; }
.group-chip { @apply max-w-full break-words rounded-full border border-gray-200 bg-white px-2.5 py-1 text-left text-xs font-medium text-gray-500 transition hover:border-primary-300 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300; }
.group-chip-active { @apply border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-700 dark:bg-primary-900/30 dark:text-primary-300; }
</style>
