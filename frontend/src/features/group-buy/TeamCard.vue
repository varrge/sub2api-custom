<template>
  <article
    class="rounded-2xl border-2 border-violet-200 bg-white p-5 shadow-sm dark:border-violet-900 dark:bg-dark-800"
    data-testid="team-card"
  >
    <div class="flex items-start justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold">{{ team.product.name }}</h2>
        <p class="mt-1 text-sm text-gray-500">
          {{ team.product.group_name }} · {{ team.product.platform }}
        </p>
      </div>
      <span
        class="rounded-full bg-violet-50 px-3 py-1 text-xs text-violet-700 dark:bg-violet-950 dark:text-violet-200"
      >
        {{ t(`groupBuy.${status}`) }}
      </span>
    </div>
    <div class="mt-4 flex flex-wrap items-center justify-between gap-2">
      <strong class="text-2xl text-violet-600 dark:text-violet-300">
        {{ cny(team.product.price_cny) }}
      </strong>
      <button
        class="font-mono text-sm text-gray-500 hover:underline"
        :title="t('groupBuy.copyCode')"
        @click="copy(team.code)"
      >
        {{ t('groupBuy.teamCode') }} {{ team.code }}
      </button>
    </div>
    <div class="mt-4 rounded-xl bg-violet-50 p-3 dark:bg-violet-950/40">
      <div class="text-sm font-medium">
        {{
          status === 'recruiting'
            ? t('groupBuy.remaining', { time: countdown })
            : t('groupBuy.closedHint')
        }}
      </div>
      <p class="mt-1 text-xs text-gray-500">
        {{ t('groupBuy.closes') }} {{ exactDate(team.closes_at) }}
      </p>
    </div>
    <div class="mt-4 flex justify-between text-sm">
      <span>{{ t('groupBuy.members') }}</span>
      <strong>{{ team.member_count }} / {{ team.product.max_members }}</strong>
    </div>
    <div
      class="mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"
    >
      <div
        class="h-full rounded-full bg-violet-500"
        :style="{
          width: `${progress(team.member_count, team.product.max_members)}%`
        }"
      />
    </div>
    <div class="mt-5 flex justify-between gap-3">
      <div>
        <p class="text-xs text-gray-500">{{ t('groupBuy.perCard') }}</p>
        <strong class="text-2xl">{{ usd(team.current_quota_usd) }}</strong>
      </div>
      <p class="self-end text-xs text-emerald-600">
        {{
          t('groupBuy.bonus', {
            amount: usd(
              Math.max(0, team.current_quota_usd - team.product.base_quota_usd)
            )
          })
        }}
      </p>
    </div>
    <p class="mt-3 text-sm font-medium text-violet-700 dark:text-violet-300">
      {{
        team.next_members > 0
          ? t('groupBuy.nextTier', {
              amount: usd(team.next_quota_usd),
              count: remainingMembers(team).next
            })
          : t('groupBuy.maxTier')
      }}
    </p>
    <p class="mt-1 text-xs text-gray-500">
      {{ t('groupBuy.toFull', { count: remainingMembers(team).full }) }}
    </p>
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
      to="/subscriptions"
      class="mt-3 block text-center text-sm text-primary-600"
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
import {
  canJoin,
  cny,
  usd,
  exactDate,
  progress,
  remainingMembers
} from './model'
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
