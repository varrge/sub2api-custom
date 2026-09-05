<template>
  <div
    class="min-w-0 self-start rounded-2xl border border-gray-100 bg-white p-5 shadow-card transition hover:shadow-md dark:border-dark-700/60 dark:bg-dark-800/60"
  >
    <!-- 头部: 品牌图标 + 模型名 + 品牌文本 + 计费模式徽章 -->
    <div>
      <div class="flex items-start justify-between gap-3">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 dark:bg-dark-700/50">
            <PlatformIcon :platform="model.brand.platform" size="md" />
          </div>
          <div class="min-w-0 flex-1">
            <h3
              class="truncate text-base font-bold text-gray-900 dark:text-white"
              :title="model.name"
            >
              {{ model.name }}
            </h3>
            <p class="truncate text-xs font-medium uppercase tracking-wider text-gray-400 dark:text-dark-400">
              {{ model.brand.name }}
            </p>
          </div>
        </div>
        <span
          class="shrink-0 rounded-md bg-blue-50 px-2 py-0.5 text-xs font-semibold text-blue-600 dark:bg-blue-900/20 dark:text-blue-400"
        >
          {{ billingModeLabel }}
        </span>
      </div>

      <!-- 参考价提示 -->
      <div class="mt-3.5 flex items-center justify-between text-xs text-gray-400 dark:text-dark-400">
        <span>{{ priceSourceLabel }}</span>
        <span v-if="isTokenBilling" class="font-mono">USD / 1M tokens</span>
      </div>

      <!-- 价格行 -->
      <div class="mt-4 space-y-3 text-sm">
        <template v-if="isTokenBilling">
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-dark-300">{{ t('modelPlaza.card.inputPrice') }}</span>
            <span class="font-mono font-medium text-gray-900 dark:text-white">{{ inputPriceText }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-dark-300">{{ t('modelPlaza.card.outputPrice') }}</span>
            <span class="font-mono font-medium text-gray-900 dark:text-white">{{ outputPriceText }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-dark-300">{{ t('modelPlaza.card.cacheWritePrice') }}</span>
            <span class="font-mono font-medium text-gray-900 dark:text-white">{{ cacheWritePriceText }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-dark-300">{{ t('modelPlaza.card.cacheReadPrice') }}</span>
            <span class="font-mono font-medium text-gray-900 dark:text-white">{{ cacheReadPriceText }}</span>
          </div>
          <div v-if="hasCacheWrite1h" class="flex items-center justify-between gap-2">
            <span class="text-gray-500 dark:text-dark-300">{{ t('modelPlaza.card.cacheWrite1hPrice') }}</span>
            <span class="font-mono font-medium text-gray-900 dark:text-white">{{ cacheWrite1hPriceText }}</span>
          </div>
        </template>

        <template v-else>
          <div v-if="!hasAudioTiers" class="flex items-center justify-between gap-2">
            <span class="text-gray-500 dark:text-dark-300">{{ t('modelPlaza.card.unitPrice') }}</span>
            <span class="text-right font-mono font-medium text-gray-900 dark:text-white">{{ perRequestPriceText }} <span class="text-xs font-normal text-gray-400">{{ unitLabel }}</span></span>
          </div>
          <p class="text-xs text-gray-400">{{ t('modelPlaza.card.tierDetails') }}</p>
        </template>
      </div>
    </div>

    <!-- 底部: 可用分组展开与详情 -->
    <div class="mt-4 border-t border-gray-100 pt-3 dark:border-dark-700/60">
      <button
        type="button"
        class="flex w-full items-center justify-between rounded-lg px-2 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-dark-200 dark:hover:bg-dark-700/50"
        :aria-expanded="isExpanded"
        @click="isExpanded = !isExpanded"
      >
        <span class="flex items-center gap-1.5 text-primary-600 dark:text-primary-400">
          <Icon
            :name="isExpanded ? 'chevronDown' : 'chevronRight'"
            size="xs"
            class="h-3.5 w-3.5 transition-transform"
          />
          {{ t('modelPlaza.card.viewGroupMultipliers', { n: groupCount }) }}
        </span>
        <span class="text-gray-400 dark:text-dark-400">
          {{ isExpanded ? t('modelPlaza.card.collapse') : t('modelPlaza.card.expand') }}
        </span>
      </button>

      <!-- 展开的分组列表 -->
      <div v-show="isExpanded" class="mt-2.5 space-y-2">
        <div
          v-for="variant in model.variants"
          :key="`${variant.group.id}:${variant.model.platform}:${variant.model.name}`"
          class="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-gray-100 bg-gray-50/50 p-2 text-xs dark:border-dark-700/50 dark:bg-dark-900/30"
        >
          <div class="flex min-w-0 flex-wrap items-center gap-1.5">
            <span class="truncate font-medium text-gray-800 dark:text-dark-100" :title="variant.group.name">
              {{ variant.group.name }}
            </span>
            <span
              v-if="variant.group.is_exclusive"
              class="rounded bg-purple-50 px-1 py-0.5 text-[10px] font-medium text-purple-600 dark:bg-purple-900/20 dark:text-purple-400"
            >
              {{ t('modelPlaza.badges.exclusive') }}
            </span>
            <span
              v-if="variant.group.subscription_type === 'subscription'"
              class="rounded bg-violet-50 px-1 py-0.5 text-[10px] font-medium text-violet-600 dark:bg-violet-900/20 dark:text-violet-400"
            >
              {{ t('modelPlaza.badges.subscription') }}
            </span>
            <span v-if="variant.group.platform === 'composite'" class="text-[10px] text-gray-400">{{ variant.model.platform }}</span>
          </div>

          <div class="flex items-center gap-2">
            <span class="font-mono font-semibold text-primary-600 dark:text-primary-400">
              {{ variant.effectiveRate }}x
            </span>
            <button
              type="button"
              class="rounded bg-white px-2 py-1 text-[11px] font-medium text-gray-700 shadow-sm ring-1 ring-inset ring-gray-200 hover:bg-gray-50 dark:bg-dark-800 dark:text-dark-200 dark:ring-dark-600 dark:hover:bg-dark-700"
              @click="emit('open-group-detail', variant)"
            >
              {{ t('modelPlaza.card.details') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import {
  type AggregatedPlazaModel,
  type GroupModelVariant,
  formatUsdPerMillion,
  formatUsdDirect,
  nonTokenUnitKey
} from './plaza-models'

const props = defineProps<{
  model: AggregatedPlazaModel
}>()

const emit = defineEmits<{
  (e: 'open-group-detail', variant: GroupModelVariant): void
}>()

const { t } = useI18n()
const isExpanded = ref(false)

const isTokenBilling = computed(() => props.model.billingMode === 'token')
const groupCount = computed(() => new Set(props.model.variants.map(variant => variant.group.id)).size)
const unitLabel = computed(() => t(nonTokenUnitKey(props.model.billingMode)))
const hasAudioTiers = computed(() => props.model.billingMode === 'per_request' && props.model.variants.some(
  variant => variant.model.pricing?.intervals.some(tier => ['realtime', 'tts', 'stt'].includes(tier.tier_label?.toLowerCase() ?? ''))
))

const billingModeLabel = computed(() => {
  switch (props.model.billingMode) {
    case 'token':
      return t('modelPlaza.billingModes.token')
    case 'image':
      return t('modelPlaza.billingModes.image')
    case 'per_request':
      return t('modelPlaza.billingModes.perRequest')
    case 'video':
      return t('modelPlaza.billingModes.video')
    default:
      return props.model.billingMode
  }
})

const priceSourceLabel = computed(() => {
  if (isTokenBilling.value && props.model.officialPricing) {
    return t('modelPlaza.card.referencePricing')
  }
  return props.model.hasPriceDifferences
    ? t('modelPlaza.card.groupPriceRange')
    : t('modelPlaza.card.basePricing')
})

function formatTokenPrice(officialVal: number | null | undefined, range?: [number, number]): string {
  if (props.model.officialPricing) {
    return formatUsdPerMillion(officialVal) ?? '-'
  }
  if (!range) return '-'
  if (range[0] === range[1]) {
    return formatUsdPerMillion(range[0]) ?? '-'
  }
  const minStr = formatUsdPerMillion(range[0]) ?? '-'
  const maxStr = formatUsdPerMillion(range[1]) ?? '-'
  return `${minStr} ~ ${maxStr}`
}

function formatDirectPrice(officialVal: number | null | undefined, range?: [number, number]): string {
  if (officialVal !== null && officialVal !== undefined) {
    return formatUsdDirect(officialVal) ?? '-'
  }
  if (!range) return '-'
  if (range[0] === range[1]) {
    return formatUsdDirect(range[0]) ?? '-'
  }
  const minStr = formatUsdDirect(range[0]) ?? '-'
  const maxStr = formatUsdDirect(range[1]) ?? '-'
  return `${minStr} ~ ${maxStr}`
}

const inputPriceText = computed(() =>
  formatTokenPrice(props.model.officialPricing?.input_price, props.model.priceRanges.input)
)

const outputPriceText = computed(() =>
  formatTokenPrice(props.model.officialPricing?.output_price, props.model.priceRanges.output)
)

const cacheWritePriceText = computed(() =>
  formatTokenPrice(props.model.officialPricing?.cache_write_price, props.model.priceRanges.cacheWrite)
)

const cacheReadPriceText = computed(() =>
  formatTokenPrice(props.model.officialPricing?.cache_read_price, props.model.priceRanges.cacheRead)
)
const hasCacheWrite1h = computed(() => props.model.officialPricing
  ? props.model.officialPricing.cache_write_1h_price != null
  : props.model.priceRanges.cacheWrite1h != null)
const cacheWrite1hPriceText = computed(() => formatTokenPrice(
  props.model.officialPricing?.cache_write_1h_price, props.model.priceRanges.cacheWrite1h
))

const perRequestPriceText = computed(() =>
  formatDirectPrice(null, props.model.priceRanges.perRequest)
)

</script>
