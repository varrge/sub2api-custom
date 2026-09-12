<template>
  <article
    class="rounded-2xl border-2 border-sky-100 bg-white p-5 dark:border-sky-900 dark:bg-dark-800"
    :class="{ 'border-blue-200 dark:border-blue-800': frozen }"
  >
    <div class="flex items-start justify-between gap-3">
      <div>
        <h3 class="font-semibold">{{ card.product_name }}</h3>
        <p class="mt-1 text-xs text-gray-500">
          {{ card.group_name }} · {{ card.platform }}
        </p>
      </div>
      <span
        class="badge"
        :class="frozen ? 'bg-blue-50 text-blue-600 dark:bg-blue-950 dark:text-blue-200' : card.status === 'active' ? 'badge-success' : 'badge-gray'"
      >
        {{ t(`groupBuy.${card.status}`) }}
      </span>
    </div>
    <p class="mt-3 font-mono text-sm">
      {{ t('groupBuy.cardCode') }} {{ card.code }}
    </p>
    <RouterLink
      v-if="card.team_code"
      :to="{ path: '/group-buy', query: { team_code: card.team_code } }"
      class="mt-1 block text-sm text-primary-600"
    >
      {{ t('groupBuy.teamCode') }} {{ card.team_code }}
    </RouterLink>
    <p v-else class="mt-1 text-sm text-gray-500">{{ t('groupBuy.solo') }}</p>
    <p v-if="card.status === 'active'" class="mt-3 text-sm text-sky-700 dark:text-sky-300">{{ t('userSubscriptions.daysRemaining', { days: Math.max(0, Math.ceil((Date.parse(card.expires_at) - Date.now()) / 86400000)) }) }}</p>
    <p v-if="frozen" class="mt-3 text-sm font-medium text-blue-700 dark:text-blue-200">{{ t('groupBuy.frozenRemaining', { days: Math.ceil((card.remaining_seconds ?? 0) / 86400) }) }}</p>
    <dl class="my-4 space-y-3 rounded-xl bg-gray-50 p-4 text-xs dark:bg-dark-700/50">
      <div>
        <dt class="text-gray-500">{{ t('groupBuy.obtained') }}</dt>
        <dd>{{ exactDate(card.starts_at) }}</dd>
      </div>
      <div>
        <dt class="text-gray-500">{{ t('groupBuy.expires') }}</dt>
        <dd>{{ exactDate(card.expires_at) }}</dd>
        <dd v-if="frozen" class="mt-1 text-blue-600 dark:text-blue-300">{{ t('groupBuy.frozenExpiry') }}</dd>
      </div>
    </dl>
    <div class="space-y-4">
      <div v-for="quota in quotas" :key="quota.label">
        <div class="mb-2 flex justify-between gap-3 text-sm">
          <span>{{ t(quota.label) }}</span>
          <span>{{ usd(quota.used) }} / {{ usd(quota.total) }}</span>
        </div>
        <div
          class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"
        >
          <div
            class="h-full rounded-full"
            :class="quota.used >= quota.total ? 'bg-amber-500' : 'bg-sky-500'"
            :style="{ width: `${progress(quota.used, quota.total)}%` }"
          />
        </div>
        <p class="mt-1 text-xs text-gray-500">{{ t('groupBuy.quotaRemaining', { amount: usd(Math.max(0, quota.total - quota.used)) }) }}</p>
      </div>
    </div>
    <p v-if="!frozen" class="mt-4 text-sm font-semibold text-sky-700 dark:text-sky-300">
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
    <p v-if="!frozen" class="mt-3 text-xs leading-5 text-gray-500">
      {{ t('groupBuy.weeklyWindow') }}:
      {{ exactDate(card.weekly_window_start) }} —
      {{ exactDate(card.weekly_window_end) }}
    </p>
    <p v-if="!frozen" class="mt-1 text-xs text-gray-500">
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
