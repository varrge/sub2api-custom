<template>
  <button
    type="button"
    class="btn-ghost btn-icon shrink-0 text-amber-600 dark:text-amber-400"
    :title="t('activityLeaderboard.entry')"
    :aria-label="t('activityLeaderboard.entry')"
    aria-haspopup="dialog"
    :aria-expanded="show"
    data-testid="header-activity-leaderboard"
    @click="show = true"
  >
    <Icon name="trophy" size="md" />
  </button>

  <BaseDialog :show="show" :title="t('activityLeaderboard.entry')" width="wide" @close="show = false">
    <div :aria-busy="loading" class="space-y-5">
      <div class="relative overflow-hidden rounded-2xl border border-amber-200/70 bg-gradient-to-br from-amber-50 via-orange-50 to-white p-5 dark:border-amber-400/20 dark:from-amber-950/60 dark:via-dark-800 dark:to-dark-900 sm:p-6">
        <div class="pointer-events-none absolute -right-8 -top-10 h-40 w-40 rounded-full border-[20px] border-amber-200/30 dark:border-amber-300/5" aria-hidden="true"></div>
        <div class="relative">
          <div class="mb-3 flex flex-wrap items-center gap-2 text-xs font-medium text-amber-800 dark:text-amber-200">
            <Icon name="trophy" size="sm" />
            <span>{{ t('activityLeaderboard.subtitle') }}</span>
            <span v-if="data" class="rounded-full bg-white/75 px-2 py-1 dark:bg-white/10" data-testid="leaderboard-status">{{ t(`activityLeaderboard.${data.status}`) }}</span>
          </div>
          <h2 class="text-xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-2xl">{{ t('activityLeaderboard.title') }}</h2>
          <p v-if="data" class="mt-2 text-xs leading-6 text-gray-600 dark:text-dark-300 sm:text-sm">
            {{ t('activityLeaderboard.period', { start: formatTime(data.starts_at), end: formatTime(data.ends_at) }) }}
          </p>
          <p class="mt-3 text-xs leading-5 text-amber-800 dark:text-amber-200">{{ t('activityLeaderboard.rewards') }}</p>
        </div>
      </div>

      <p v-if="loading && !data" role="status" class="py-12 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.loading') }}</p>
      <div v-if="error" role="alert" class="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">
        <span>{{ t(data ? 'activityLeaderboard.stale' : 'activityLeaderboard.error') }}</span>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="fetchLeaderboard">{{ t('activityLeaderboard.retry') }}</button>
      </div>

      <template v-if="data">
        <p v-if="data.status === 'ended'" class="rounded-xl bg-gray-50 p-3 text-xs leading-5 text-gray-600 dark:bg-dark-800 dark:text-dark-300">{{ t('activityLeaderboard.endedNotice') }}</p>

        <div v-if="data.status === 'upcoming' || !data.entries.length" class="rounded-2xl border border-dashed border-gray-200 px-4 py-9 text-center dark:border-dark-600" data-testid="leaderboard-empty">
          <Icon name="moon" size="xl" class="mx-auto mb-3 text-amber-500" />
          <p class="font-semibold text-gray-900 dark:text-white">{{ t(data.status === 'upcoming' ? 'activityLeaderboard.upcomingTitle' : 'activityLeaderboard.emptyTitle') }}</p>
          <p class="mx-auto mt-2 max-w-md text-sm leading-6 text-gray-500 dark:text-dark-300">{{ t(data.status === 'upcoming' ? 'activityLeaderboard.upcomingDescription' : 'activityLeaderboard.emptyDescription') }}</p>
        </div>

        <template v-if="data.status !== 'upcoming'">
          <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50/60 p-4 dark:border-amber-400/20 dark:bg-amber-400/5" data-testid="leaderboard-me">
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.myRank') }}</p>
              <p class="mt-1 font-bold text-gray-900 dark:text-white">{{ data.me ? `#${data.me.rank}` : t('activityLeaderboard.unranked') }}</p>
            </div>
            <div v-if="data.me" class="text-right">
              <p class="font-semibold tabular-nums text-amber-700 dark:text-amber-300" :title="data.me.amount">{{ formatAmount(data.me.amount) }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.amount') }}</p>
            </div>
            <p v-else class="max-w-sm text-xs leading-5 text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.unrankedHint') }}</p>
          </div>

          <section v-if="data.entries.length" aria-labelledby="festival-ranking-title">
            <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
              <h3 id="festival-ranking-title" class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('activityLeaderboard.top') }}</h3>
              <span class="text-xs text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.participants', { count: data.participant_count }) }}</span>
            </div>
            <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
              <table class="w-full text-left text-xs sm:text-sm">
                <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-300">
                  <tr>
                    <th scope="col" class="px-3 py-3 font-medium">{{ t('activityLeaderboard.rank') }}</th>
                    <th scope="col" class="px-2 py-3 font-medium">{{ t('activityLeaderboard.participant') }}</th>
                    <th scope="col" class="px-3 py-3 text-right font-medium">{{ t('activityLeaderboard.amount') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="entry in data.entries" :key="entry.alias" :class="entry.is_me ? 'bg-amber-50/60 dark:bg-amber-400/5' : ''" data-testid="leaderboard-row">
                    <td class="px-3 py-3">
                      <span class="inline-flex h-7 min-w-7 items-center justify-center rounded-lg font-semibold tabular-nums" :class="rankClass(entry.rank)">{{ entry.rank }}</span>
                    </td>
                    <td class="px-2 py-3 text-gray-700 dark:text-dark-200">
                      <span class="break-all font-mono text-[11px] sm:text-xs">{{ t('activityLeaderboard.anonymous', { alias: entry.alias }) }}</span>
                      <span v-if="entry.is_me" class="ml-1 inline-block rounded bg-amber-100 px-1 text-[10px] text-amber-800 dark:bg-amber-400/20 dark:text-amber-200">{{ t('activityLeaderboard.you') }}</span>
                    </td>
                    <td class="px-3 py-3 text-right font-medium tabular-nums text-gray-900 dark:text-white" :title="entry.amount">{{ formatAmount(entry.amount) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </template>

        <div class="flex flex-wrap items-center justify-between gap-2 text-[11px] text-gray-500 dark:text-dark-400">
          <span>{{ t('activityLeaderboard.updated', { time: formatTime(data.updated_at) }) }} · {{ t('activityLeaderboard.refreshHint') }}</span>
          <button type="button" class="inline-flex items-center gap-1 rounded-lg px-2 py-1.5 hover:bg-gray-100 disabled:opacity-50 dark:hover:bg-dark-700" :disabled="loading" @click="fetchLeaderboard">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />{{ t('activityLeaderboard.refresh') }}
          </button>
        </div>

        <details class="rounded-xl bg-gray-50 p-4 text-xs leading-6 text-gray-600 dark:bg-dark-800/70 dark:text-dark-300">
          <summary class="cursor-pointer font-semibold text-gray-800 dark:text-dark-100">{{ t('activityLeaderboard.rulesTitle') }}</summary>
          <ul class="mt-2 list-disc space-y-1 pl-4">
            <li>{{ t('activityLeaderboard.rulesScope') }}</li>
            <li>{{ t('activityLeaderboard.rulesExclusions') }}</li>
            <li>{{ t('activityLeaderboard.rulesRanking') }}</li>
            <li>{{ t('activityLeaderboard.rulesPrivacy') }}</li>
          </ul>
        </details>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getActivityLeaderboard, type ActivityLeaderboard } from '@/api/activityLeaderboard'
import BaseDialog from './BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const { t, locale } = useI18n()
const show = ref(false)
const data = ref<ActivityLeaderboard | null>(null)
const loading = ref(false)
const error = ref(false)
let controller: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined

function stop() {
  clearTimeout(timer)
  controller?.abort()
  controller = undefined
  loading.value = false
}

function scheduleRefresh() {
  clearTimeout(timer)
  if (!show.value) return
  timer = setTimeout(() => {
    if (document.hidden) scheduleRefresh()
    else void fetchLeaderboard()
  }, Math.max(60, data.value?.refresh_seconds ?? 60) * 1000)
}

async function fetchLeaderboard() {
  if (loading.value || !show.value) return
  clearTimeout(timer)
  const request = new AbortController()
  controller = request
  loading.value = true
  error.value = false
  try {
    const result = await getActivityLeaderboard(request.signal)
    if (!request.signal.aborted) data.value = result
  } catch {
    if (!request.signal.aborted) error.value = true
  } finally {
    if (controller === request) {
      loading.value = false
      controller = undefined
      scheduleRefresh()
    }
  }
}

watch(show, (open) => {
  if (open) {
    data.value = null
    error.value = false
    void fetchLeaderboard()
  } else stop()
})
onBeforeUnmount(stop)

function formatTime(value: string) {
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-GB', {
    timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
  }).format(new Date(value))
}

function formatAmount(value: string) {
  return new Intl.NumberFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    minimumFractionDigits: 2, maximumFractionDigits: 4,
  }).format(Number(value))
}

function rankClass(rank: number) {
  if (rank === 1) return 'bg-amber-100 text-amber-800 dark:bg-amber-400/20 dark:text-amber-200'
  if (rank === 2) return 'bg-slate-200 text-slate-700 dark:bg-slate-500/25 dark:text-slate-200'
  if (rank === 3) return 'bg-orange-100 text-orange-800 dark:bg-orange-400/20 dark:text-orange-200'
  return 'text-gray-500 dark:text-dark-400'
}
</script>
