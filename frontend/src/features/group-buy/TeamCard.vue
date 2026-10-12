<template>
  <article
    class="gb-panel gb-panel-hover gb-team-card flex flex-col p-4 sm:p-5"
    data-testid="team-card"
  >
    <div class="flex min-w-0 items-start gap-3">
      <span class="gb-team-card__icon" aria-hidden="true">
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
          class="h-5 w-5"
        >
          <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
          <circle cx="9" cy="7" r="4" />
          <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
          <path d="M16 3.13a4 4 0 0 1 0 7.75" />
        </svg>
      </span>
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <h2
            class="min-w-0 truncate text-base font-semibold text-gray-900 dark:text-white"
            :title="team.product.name"
          >
            {{ team.product.name }}
          </h2>
          <span
            class="gb-pill gb-team-card__status shrink-0"
            :class="`gb-team-card__status--${status}`"
          >
            {{ t(`groupBuy.${status}`) }}
          </span>
        </div>
        <p
          class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400"
          :title="`${team.product.group_name} · ${team.product.platform}`"
        >
          {{ team.product.group_name }} · {{ team.product.platform }}
        </p>
        <div class="mt-1.5 flex items-center gap-2">
          <button
            class="gb-code gb-team-card__code"
            type="button"
            :title="`${t('groupBuy.copyCode')}: ${team.code}`"
            @click="copy(team.code)"
          >
            {{ t('groupBuy.teamCode') }} {{ team.code }}
          </button>
          <span v-if="status === 'recruiting'" class="gb-team-card__countdown">
            {{ t('groupBuy.remaining', { time: countdown }) }}
          </span>
        </div>
      </div>
    </div>

    <div class="mt-3">
      <div class="flex items-center justify-between gap-2 text-xs">
        <span class="gb-label">{{ t('groupBuy.membersProgress') }}</span>
        <strong class="tabular-nums text-gray-900 dark:text-white">
          {{ team.member_count }} /
          {{ t('groupBuy.previewMembers', { members: team.product.max_members }) }}
        </strong>
      </div>
      <div class="gb-progress mt-1.5">
        <div
          class="gb-progress-fill"
          :style="{
            width: `${progress(team.member_count, team.product.max_members)}%`
          }"
        />
      </div>
      <p
        class="mt-1.5 truncate text-xs font-medium text-gray-600 dark:text-gray-300"
        :title="
          hasNext
            ? t('groupBuy.nextTier', {
                amount: usd(team.next_quota_usd),
                count: remainingMembers(team).next
              })
            : undefined
        "
      >
        {{
          hasNext
            ? t('groupBuy.nextTier', {
                amount: usdShort(team.next_quota_usd),
                count: remainingMembers(team).next
              })
            : t('groupBuy.maxTier')
        }}
      </p>
      <p
        v-if="status !== 'recruiting'"
        class="mt-1 text-xs text-gray-500 dark:text-gray-400"
      >
        {{ t('groupBuy.closedHint') }}
      </p>
    </div>

    <div class="gb-team-card__quotas mt-3">
      <div class="min-w-0">
        <p class="gb-label">{{ t('groupBuy.perCard') }}</p>
        <strong
          class="gb-team-card__quota-value"
          :title="usd(team.current_quota_usd)"
        >
          {{ usdShort(team.current_quota_usd) }}
        </strong>
      </div>
      <div class="min-w-0">
        <p class="gb-label">{{ t('groupBuy.nextTierAdd') }}</p>
        <strong
          class="gb-team-card__quota-value gb-team-card__quota-value--boost"
          :title="hasNext ? `+${usd(boost)}` : t('groupBuy.maxTier')"
        >
          {{ hasNext ? `+${usdShort(boost)}` : '—' }}
        </strong>
      </div>
      <div class="min-w-0">
        <p class="gb-label">{{ t('groupBuy.nextTierQuota') }}</p>
        <strong
          class="gb-team-card__quota-value"
          :title="hasNext ? usd(team.next_quota_usd) : t('groupBuy.maxTier')"
        >
          {{ hasNext ? usdShort(team.next_quota_usd) : '—' }}
        </strong>
      </div>
    </div>

    <details class="gb-team-card__details mt-3 mb-3">
      <summary>{{ t('groupBuy.tierLadderTitle') }}</summary>
      <p class="mt-2 text-xs text-emerald-600 dark:text-emerald-400">
        {{
          t('groupBuy.bonus', {
            amount: usd(
              Math.max(0, team.current_quota_usd - team.product.base_quota_usd)
            )
          })
        }}
      </p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('groupBuy.toFull', { count: remainingMembers(team).full }) }}
      </p>
      <QuotaLadder class="mt-3" :product="team.product" />
    </details>

    <div
      class="gb-team-card__footer mt-auto flex flex-col gap-3 border-t border-gray-200/70 pt-3 dark:border-dark-600/70"
    >
      <div class="flex min-w-0 items-baseline gap-2">
        <strong
          class="shrink-0 text-sm font-semibold whitespace-nowrap text-gray-900 dark:text-white"
        >
          {{ cny(team.product.price_cny) }}
        </strong>
        <p
          class="truncate text-xs text-gray-500 dark:text-gray-400"
          :title="`${t('groupBuy.closes')} ${exactDate(team.closes_at)}`"
        >
          {{ t('groupBuy.closes') }} {{ shortDate(team.closes_at) }}
        </p>
      </div>
      <div class="gb-team-card__actions flex shrink-0 gap-2">
        <button
          class="btn btn-secondary gb-team-card__action"
          type="button"
          @click="copy(shareLink)"
        >
          {{ t('groupBuy.copyLink') }}
        </button>
        <button
          class="btn btn-primary gb-team-card__action"
          type="button"
          :disabled="!canJoin(team, now)"
          @click="$emit('join', team)"
        >
          {{ team.joined ? t('groupBuy.joined') : t('groupBuy.join') }}
        </button>
      </div>
    </div>
    <RouterLink
      v-if="team.joined"
      to="/my-group-buy"
      class="gb-team-card__accent mt-2 block text-center text-xs font-medium hover:underline"
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
const hasNext = computed(() => props.team.next_members > 0)
const boost = computed(() =>
  hasNext.value
    ? Math.max(0, props.team.next_quota_usd - props.team.current_quota_usd)
    : 0
)
const usdShort = (amount: number) =>
  amount !== 0 && Math.abs(amount) < 0.01
    ? usd(amount)
    : new Intl.NumberFormat('en-US', {
        style: 'currency',
        currency: 'USD',
        maximumFractionDigits: 2
      }).format(amount)
const shortDate = (value: string) =>
  new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  }).format(new Date(value))
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
<style scoped>
/* 本卡局部淡紫主题，仅作用于拼团大厅队伍卡，不影响全局样式 */
.gb-team-card {
  container-type: inline-size;
  border-color: rgb(221 214 254 / 0.65);
}
.dark .gb-team-card {
  border-color: rgb(139 92 246 / 0.28);
}
.gb-team-card__icon {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 0.625rem;
  background: rgb(237 233 254 / 0.8);
  color: rgb(124 58 237);
}
.dark .gb-team-card__icon {
  background: rgb(139 92 246 / 0.18);
  color: rgb(196 181 253);
}
.gb-team-card__code {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  overflow-wrap: normal;
  text-align: left;
}
.gb-team-card__countdown {
  flex-shrink: 0;
  border-radius: 9999px;
  padding: 0.25rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  background: rgb(245 243 255 / 0.9);
  border: 1px solid rgb(221 214 254 / 0.8);
  color: rgb(109 40 217);
}
.dark .gb-team-card__countdown {
  background: rgb(139 92 246 / 0.16);
  border-color: rgb(139 92 246 / 0.35);
  color: rgb(196 181 253);
}
.gb-team-card__status--recruiting {
  background: rgb(236 253 245 / 0.9);
  color: rgb(4 120 87);
}
.dark .gb-team-card__status--recruiting {
  background: rgb(6 95 70 / 0.4);
  color: rgb(110 231 183);
}
.gb-team-card__status--full {
  background: rgb(237 233 254 / 0.9);
  color: rgb(109 40 217);
}
.dark .gb-team-card__status--full {
  background: rgb(139 92 246 / 0.2);
  color: rgb(196 181 253);
}
.gb-team-card__status--closed,
.gb-team-card__status--cancelled {
  background: rgb(245 245 247 / 0.8);
  color: rgb(110 110 118);
}
.dark .gb-team-card__status--closed,
.dark .gb-team-card__status--cancelled {
  background: rgb(46 46 53 / 0.6);
  color: rgb(161 161 170);
}
.gb-team-card .gb-progress-fill {
  background: linear-gradient(90deg, #8b5cf6 0%, #c4b5fd 100%);
}
.gb-team-card__quotas {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
  border-radius: 0.75rem;
  padding: 0.625rem 0.75rem;
  background: rgb(245 243 255 / 0.65);
  border: 1px solid rgb(237 233 254 / 0.9);
}
.dark .gb-team-card__quotas {
  background: rgb(139 92 246 / 0.08);
  border-color: rgb(139 92 246 / 0.22);
}
.gb-team-card__quota-value {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.9375rem;
  font-variant-numeric: tabular-nums;
  color: rgb(17 17 20);
}
.dark .gb-team-card__quota-value {
  color: #f8fafc;
}
.gb-team-card__quota-value--boost {
  color: rgb(5 150 105);
}
.dark .gb-team-card__quota-value--boost {
  color: rgb(52 211 153);
}
.gb-team-card__details > summary {
  cursor: pointer;
  user-select: none;
  font-size: 0.8125rem;
  font-weight: 500;
  color: rgb(109 40 217);
}
.dark .gb-team-card__details > summary {
  color: rgb(196 181 253);
}
.gb-team-card__details > summary:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}
.gb-team-card__details > summary:focus-visible {
  outline: 2px solid rgb(139 92 246 / 0.6);
  outline-offset: 2px;
  border-radius: 0.25rem;
}
.gb-team-card__accent {
  color: rgb(109 40 217);
}
.dark .gb-team-card__accent {
  color: rgb(196 181 253);
}
.gb-team-card .btn-primary {
  background-image: linear-gradient(to bottom, #a78bfa, #8b5cf6);
  box-shadow: 0 4px 10px rgb(139 92 246 / 0.25);
}
.gb-team-card .btn-primary:hover:not(:disabled) {
  filter: brightness(1.05);
  box-shadow: 0 6px 14px rgb(139 92 246 / 0.3);
}
.gb-team-card__action {
  flex: 1 1 0%;
}
/* 按卡片实际宽度切换底部布局：宽卡一行、窄卡两行，与视口断点无关 */
@container (min-width: 470px) {
  .gb-team-card__footer {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
  }
  .gb-team-card__action {
    flex: none;
  }
}
</style>
