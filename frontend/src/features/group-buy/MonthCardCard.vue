<template>
  <article class="gb-panel gb-panel-hover p-5 sm:p-6" :class="{ 'border-blue-200 dark:border-blue-800': frozen }">
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="font-semibold text-gray-900 dark:text-white">
          {{ card.product_name }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ card.group_name }} · {{ card.platform }}
        </p>
      </div>
      <span
        class="gb-pill"
        :class="frozen || card.status === 'active' ? 'gb-pill-teal' : 'gb-pill-gray'"
      >
        {{ t(`groupBuy.${card.status}`) }}
      </span>
    </div>
    <p class="mt-3">
      <span class="gb-code">{{ t('groupBuy.cardCode') }} {{ card.code }}</span>
    </p>
    <RouterLink
      v-if="card.team_code"
      :to="{ path: '/group-buy', query: { team_code: card.team_code } }"
      class="gb-accent mt-2 block text-sm font-medium hover:underline"
    >
      {{ t('groupBuy.teamCode') }} {{ card.team_code }}
    </RouterLink>
    <p v-else class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('groupBuy.solo') }}</p>
    <p v-if="card.status === 'active'" class="gb-accent-sky mt-3 text-sm font-medium">{{ t('userSubscriptions.daysRemaining', { days: Math.max(0, Math.ceil((Date.parse(card.expires_at) - Date.now()) / 86400000)) }) }}</p>
    <p v-if="frozen" class="mt-3 text-sm font-medium text-blue-700 dark:text-blue-200">{{ t('groupBuy.frozenRemaining', { days: Math.ceil((card.remaining_seconds ?? 0) / 86400) }) }}</p>
    <dl class="my-4 grid grid-cols-2 gap-3 text-xs">
      <div class="min-w-0">
        <dt class="gb-label">{{ t('groupBuy.obtained') }}</dt>
        <dd class="mt-0.5 text-gray-700 dark:text-gray-300">{{ exactDate(card.starts_at) }}</dd>
      </div>
      <div class="min-w-0">
        <dt class="gb-label">{{ t('groupBuy.expires') }}</dt>
        <dd class="mt-0.5 text-gray-700 dark:text-gray-300">{{ exactDate(card.expires_at) }}</dd>
        <dd v-if="frozen" class="mt-1 text-blue-600 dark:text-blue-300">{{ t('groupBuy.frozenExpiry') }}</dd>
      </div>
    </dl>
    <div class="space-y-4">
      <div v-for="quota in quotas" :key="quota.label">
        <div class="mb-2 flex justify-between gap-3 text-sm">
          <span class="text-gray-700 dark:text-gray-300">{{ t(quota.label) }}</span>
          <span class="text-gray-500 dark:text-gray-400">{{ usd(quota.used) }} / {{ usd(quota.total) }}</span>
        </div>
        <div class="gb-progress">
          <div
            class="gb-progress-fill"
            :class="{ 'gb-progress-fill-warn': quota.used >= quota.total }"
            :style="{ width: `${progress(quota.used, quota.total)}%` }"
          />
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.quotaRemaining', { amount: usd(Math.max(0, quota.total - quota.used)) }) }}</p>
      </div>
    </div>
    <p v-if="!frozen" class="gb-accent-sky mt-4 text-sm font-semibold">
      {{
        t('groupBuy.available', {
          amount: usd(
            card.status === 'active' && Date.parse(card.expires_at) > Date.now()
              ? availableQuota(card)
              : 0
          )
        })
      }}
    </p>
    <div v-if="frozen" class="mt-4 space-y-2 rounded-xl border border-blue-200 bg-blue-50/60 p-4 text-sm leading-6 text-blue-700 dark:border-blue-800 dark:bg-blue-950/30 dark:text-blue-200">
      <p>{{ t('groupBuy.frozenHint') }}</p>
      <p>{{ t('groupBuy.frozenQuota', { amount: usd(availableQuota(card)) }) }}</p>
      <p v-if="card.frozen_at" class="text-xs text-blue-500 dark:text-blue-300">{{ t('groupBuy.frozenAt', { time: exactDate(card.frozen_at) }) }}</p>
    </div>
    <p v-if="!frozen" class="mt-3 text-xs leading-5 text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.weeklyWindow') }}:
      {{ exactDate(card.weekly_window_start) }} —
      {{ exactDate(card.weekly_window_end) }}
    </p>
    <p v-if="!frozen" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
      {{
        Date.parse(card.weekly_window_end) >= Date.parse(card.expires_at)
          ? t('groupBuy.finalWindow')
          : `${t('groupBuy.nextReset')}: ${exactDate(card.weekly_window_end)}`
      }}
    </p>
    <template v-if="manageable && card.freeze_allowed !== false && (card.status === 'active' || frozen)">
      <button type="button" class="mt-4 w-full rounded-xl border border-blue-200 bg-white py-3 text-sm font-semibold text-blue-700 transition-colors hover:bg-blue-50 disabled:opacity-50 dark:border-blue-800 dark:bg-dark-800 dark:text-blue-200 dark:hover:bg-blue-950" :disabled="busy" @click="toggleFreeze">
        {{ busy ? t('common.loading') : frozen ? t('groupBuy.thaw') : t('groupBuy.freeze') }}
      </button>
      <p class="mt-2 text-xs leading-5 text-gray-500">{{ t('groupBuy.freezeHelp') }}</p>
    </template>
    <p v-if="error" role="alert" class="mt-2 text-sm text-red-600">{{ error }}</p>
    <slot />
  </article>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonthCard } from '@/types/groupBuy'
import { exactDate, usd, progress, availableQuota } from './model'
import './glass.css'
import { groupBuyAPI } from '@/api/groupBuy'
import { extractApiErrorMessage } from '@/utils/apiError'
const props = withDefaults(defineProps<{ card: MonthCard; manageable?: boolean }>(), { manageable: false })
const emit = defineEmits<{ changed: [card: MonthCard] }>()
const { t } = useI18n()
const frozen = computed(() => props.card.status === 'frozen')
const busy = ref(false)
const error = ref('')
async function toggleFreeze() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    const card = frozen.value
      ? await groupBuyAPI.thawCard(props.card.id)
      : await groupBuyAPI.freezeCard(props.card.id)
    emit('changed', card)
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('groupBuy.freezeFailed'))
  } finally { busy.value = false }
}
const quotas = computed(() => [
  {
    label: 'groupBuy.weeklyUsage',
    used: props.card.weekly_used_usd,
    total: props.card.weekly_quota_usd
  },
  {
    label: 'groupBuy.totalUsage',
    used: props.card.total_used_usd,
    total: props.card.total_quota_usd
  }
])
</script>
