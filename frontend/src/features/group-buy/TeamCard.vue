<template>
  <article
    class="gb-panel gb-panel-hover flex flex-col p-5 sm:p-6"
    data-testid="team-card"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <h2 class="truncate text-lg font-semibold text-gray-900 dark:text-white">
          {{ team.product.name }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ team.product.group_name }} · {{ team.product.platform }}
        </p>
      </div>
      <span
        class="gb-pill"
        :class="status === 'recruiting' ? 'gb-pill-teal' : 'gb-pill-gray'"
      >
        {{ t(`groupBuy.${status}`) }}
      </span>
    </div>
    <div class="mt-4 flex flex-wrap items-center justify-between gap-2">
      <strong class="gb-accent text-2xl font-bold">
        {{ cny(team.product.price_cny) }}
      </strong>
      <button
        class="gb-code"
        :title="t('groupBuy.copyCode')"
        @click="copy(team.code)"
      >
        {{ t('groupBuy.teamCode') }} {{ team.code }}
      </button>
    </div>
    <div class="gb-strip mt-4 p-3">
      <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
        {{
          status === 'recruiting'
            ? t('groupBuy.remaining', { time: countdown })
            : t('groupBuy.closedHint')
        }}
      </div>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('groupBuy.closes') }} {{ exactDate(team.closes_at) }}
      </p>
    </div>
    <div class="mt-4 flex justify-between text-sm">
      <span class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.members') }}</span>
      <strong class="text-gray-900 dark:text-white">{{ team.member_count }} / {{ team.product.max_members }}</strong>
    </div>
    <div class="gb-progress mt-2">
      <div
        class="gb-progress-fill"
        :style="{
          width: `${progress(team.member_count, team.product.max_members)}%`
        }"
      />
    </div>
    <div class="mt-5 flex justify-between gap-3">
      <div>
        <p class="gb-label">{{ t('groupBuy.perCard') }}</p>
        <strong class="text-2xl font-bold text-gray-900 dark:text-white">{{ usd(team.current_quota_usd) }}</strong>
      </div>
      <p class="self-end text-xs text-emerald-600 dark:text-emerald-400">
        {{
          t('groupBuy.bonus', {
            amount: usd(
              Math.max(0, team.current_quota_usd - team.product.base_quota_usd)
            )
          })
        }}
      </p>
    </div>
    <p class="gb-accent mt-3 text-sm font-medium">
      {{
        team.next_members > 0
          ? t('groupBuy.nextTier', {
              amount: usd(team.next_quota_usd),
              count: remainingMembers(team).next
            })
          : t('groupBuy.maxTier')
      }}
    </p>
    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.toFull', { count: remainingMembers(team).full }) }}
    </p>
    <QuotaLadder class="mt-4" :product="team.product" />
    <div class="mt-5 flex flex-wrap gap-2">
      <button
        class="btn btn-primary flex-1"
        :disabled="!canJoin(team, now)"
        @click="$emit('join', team)"
      >
        {{ team.joined ? t('groupBuy.joined') : t('groupBuy.join') }}
      </button>
      <button class="btn btn-secondary" @click="copy(shareLink)">
        {{ t('groupBuy.copyLink') }}
      </button>
    </div>
    <RouterLink
      v-if="team.joined"
      to="/my-group-buy"
      class="gb-accent mt-3 block text-center text-sm font-medium hover:underline"
    >
      {{ t('groupBuy.viewCards') }}
    </RouterLink>
  </article>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import type { GroupBuyTeam } from '@/types/groupBuy'
import QuotaLadder from './QuotaLadder.vue'
import {
  canJoin,
  cny,
  usd,
  exactDate,
  progress,
  remainingMembers
} from './model'
import './glass.css'
const props = defineProps<{ team: GroupBuyTeam; now: number }>()
defineEmits<{ join: [team: GroupBuyTeam] }>()
const { t } = useI18n()
const app = useAppStore()
const status = computed(() =>
  props.team.status === 'recruiting'
    ? props.team.member_count >= props.team.product.max_members
      ? 'full'
      : Date.parse(props.team.closes_at) <= props.now
        ? 'closed'
        : 'recruiting'
    : props.team.status
)
const countdown = computed(() => {
  const s = Math.max(
    0,
    Math.floor((Date.parse(props.team.closes_at) - props.now) / 1000)
  )
  return `${Math.floor(s / 3600)}:${String(Math.floor(s / 60) % 60).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
})
const shareLink = computed(
  () =>
    `${window.location.origin}/group-buy?team_code=${encodeURIComponent(props.team.code)}`
)
async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value)
    app.showSuccess(t('groupBuy.copied'))
  } catch {
    app.showError(t('groupBuy.copyFailed'))
  }
}
</script>
