<template>
  <section class="card p-5 sm:p-6" data-testid="invitation-level">
    <div v-if="showLevel" class="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
      <div class="min-w-0">
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('invitations.level.title') }}</p>
        <p class="mt-1 flex flex-wrap items-center gap-2">
          <span class="break-words text-2xl font-semibold text-gray-900 dark:text-white">{{ overview.level.name }}</span>
          <span v-if="overview.override_level_id" class="badge badge-primary" :title="t('invitations.level.assignedHint')">
            {{ t('invitations.level.assigned') }}
          </span>
        </p>
      </div>
      <div v-if="rebatesEnabled" class="text-right">
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('invitations.level.rate') }}</p>
        <p class="mt-1 text-2xl font-semibold tabular-nums text-primary-700 dark:text-primary-300" data-testid="invitation-rate">
          {{ formatRate(overview.level.rebate_rate) }}
        </p>
      </div>
    </div>

    <div v-if="showLevel" class="mt-5">
      <div class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 text-sm">
        <span class="text-gray-700 dark:text-gray-200">
          {{ t('invitations.level.rechargeTotal') }}
          <span class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatMoney(overview.recharge_total) }}</span>
        </span>
        <span v-if="overview.next_level" class="text-gray-600 dark:text-dark-300" data-testid="invitation-next-level">
          {{ t('invitations.level.next', { amount: formatMoney(overview.next_level_remaining), level: overview.next_level.name }) }}
        </span>
        <span v-else class="text-gray-600 dark:text-dark-300">{{ t('invitations.level.highest') }}</span>
      </div>
      <div
        v-if="overview.next_level"
        class="mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"
        role="progressbar"
        :aria-label="t('invitations.level.progress')"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="progress"
      >
        <div class="h-full rounded-full bg-primary-500" :style="{ width: `${progress}%` }"></div>
      </div>
      <p v-if="rebatesEnabled && overview.next_level" class="mt-2 text-xs text-gray-500 dark:text-dark-400">
        {{ t('invitations.level.nextRate', { rate: formatRate(overview.next_level.rebate_rate) }) }}
      </p>
      <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('invitations.level.rechargeHint') }}</p>
    </div>

    <h2 v-if="!showLevel" class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('invitations.quota.title') }}</h2>
    <dl class="mt-5 grid gap-4 sm:grid-cols-2" :class="{ 'border-t border-gray-100 pt-5 dark:border-dark-700': showLevel }">
      <div v-for="meter in meters" :key="meter.key" :data-testid="`invitation-quota-${meter.key}`">
        <dt class="text-sm text-gray-500 dark:text-dark-400">{{ t(`invitations.quota.${meter.key}`) }}</dt>
        <dd class="mt-1">
          <span class="text-lg font-semibold tabular-nums" :class="meter.full ? 'text-amber-700 dark:text-amber-400' : 'text-gray-900 dark:text-white'">
            {{ t('invitations.quota.count', { used: meter.used, limit: meter.limit === 0 ? t('invitations.unlimited') : meter.limit }) }}
          </span>
          <div v-if="meter.limit > 0" class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700" aria-hidden="true">
            <div class="h-full rounded-full" :class="meter.full ? 'bg-amber-500' : 'bg-gray-400 dark:bg-dark-400'" :style="{ width: `${meter.percent}%` }"></div>
          </div>
        </dd>
      </div>
    </dl>
    <p class="mt-3 text-xs text-gray-500 dark:text-dark-400">{{ t('invitations.quota.hint') }}</p>

    <div v-if="showLevel && showLadder && ladder.length > 1" class="mt-5 border-t border-gray-100 pt-5 dark:border-dark-700" data-testid="invitation-ladder">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('invitations.level.ladder') }}</h3>
      <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('invitations.level.ladderHint') }}</p>
      <ol class="mt-2 max-h-80 divide-y divide-gray-100 overflow-y-auto dark:divide-dark-700">
        <li
          v-for="item in ladder"
          :key="item.id"
          class="flex items-start justify-between gap-3 py-2.5"
          :aria-current="item.current ? 'true' : undefined"
          :data-testid="`invitation-ladder-${item.id}`"
        >
          <div class="min-w-0">
            <p class="flex flex-wrap items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
              <span class="break-words">{{ item.name }}</span>
              <span v-if="item.current" class="badge badge-primary">{{ t('invitations.level.current') }}</span>
            </p>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ item.detail }}</p>
          </div>
          <p v-if="rebatesEnabled" class="shrink-0 text-sm font-semibold tabular-nums" :class="item.current ? 'text-primary-700 dark:text-primary-300' : 'text-gray-700 dark:text-gray-200'">{{ item.rate }}</p>
        </li>
      </ol>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { InvitationOverview } from '@/api/invitations'
import { formatMoney, formatRate } from '@/utils/invitationFormat'

const props = withDefaults(defineProps<{
  overview: InvitationOverview
  showLevel?: boolean
  /** The admin panel sits next to the rules editor, where the same ladder would be noise. */
  showLadder?: boolean
  showRebates?: boolean
}>(), { showLevel: true, showLadder: true, showRebates: true })
const { t } = useI18n()
const rebatesEnabled = computed(() => props.showRebates && props.overview.rebate_enabled !== false)

const progress = computed(() => {
  const next = props.overview.next_level
  if (!next || next.spend_threshold <= 0) return 100
  return Math.max(0, Math.min(100, Math.round((props.overview.recharge_total / next.spend_threshold) * 100)))
})

const ladder = computed(() => {
  const limit = (value: number) => (value === 0 ? t('invitations.unlimited') : String(value))
  return (props.overview.levels ?? []).map((level) => ({
    id: level.id,
    name: level.name,
    current: level.id === props.overview.level.id,
    rate: formatRate(level.rebate_rate),
    detail: t('invitations.level.ladderDetail', {
      threshold: level.spend_threshold > 0
        ? t('invitations.level.ladderFrom', { amount: formatMoney(level.spend_threshold) })
        : t('invitations.level.ladderStart'),
      lifetime: limit(level.lifetime_invite_limit),
      unused: limit(level.unused_invite_limit)
    })
  }))
})

const meters = computed(() => {
  const { level, lifetime_used, unused_count } = props.overview
  return ([
    ['lifetime', lifetime_used, level.lifetime_invite_limit],
    ['unused', unused_count, level.unused_invite_limit]
  ] as const).map(([key, used, limit]) => ({
    key,
    used,
    limit,
    full: limit > 0 && used >= limit,
    percent: limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0
  }))
})
</script>
