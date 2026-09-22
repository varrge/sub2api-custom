<template>
  <button
    v-if="!entryHidden"
    ref="triggerRef"
    type="button"
    class="btn-ghost btn-icon shrink-0 text-amber-600 dark:text-amber-400"
    :class="show ? 'bg-amber-100/80 dark:bg-amber-400/15' : ''"
    :title="t('activityLeaderboard.entry')"
    :aria-label="t('activityLeaderboard.entry')"
    aria-haspopup="dialog"
    :aria-expanded="show"
    aria-controls="activity-leaderboard-popover"
    data-testid="header-activity-leaderboard"
    @click="togglePopover"
  >
    <Icon name="trophy" size="md" />
  </button>

  <Teleport to="body">
    <Transition name="leaderboard-popover">
      <div
        v-if="show"
        id="activity-leaderboard-popover"
        ref="panelRef"
        role="dialog"
        aria-modal="false"
        :aria-label="t('activityLeaderboard.entry')"
        class="fixed z-[70] flex max-h-[min(560px,calc(100vh_-_5rem))] w-[min(400px,calc(100vw_-_1rem))] origin-top-right flex-col overflow-hidden rounded-2xl border border-amber-200/70 bg-white/95 shadow-2xl shadow-amber-900/10 backdrop-blur-xl focus:outline-none dark:border-amber-400/20 dark:bg-dark-900/95 dark:shadow-black/40"
        :style="panelStyle"
        tabindex="-1"
        data-testid="activity-leaderboard-popover"
      >
        <div class="flex shrink-0 items-center justify-between gap-2 border-b border-amber-200/60 bg-gradient-to-r from-amber-50/90 via-orange-50/50 to-transparent px-3.5 py-2.5 dark:border-amber-400/15 dark:from-amber-950/50 dark:via-dark-900 dark:to-transparent">
          <div class="flex min-w-0 items-center gap-2">
            <Icon name="trophy" size="sm" class="shrink-0 text-amber-600 dark:text-amber-400" />
            <h2 class="truncate text-sm font-bold tracking-tight text-gray-900 dark:text-white">{{ displayTitle }}</h2>
            <span v-if="data" class="shrink-0 rounded-full bg-amber-100/80 px-2 py-0.5 text-[10px] font-medium text-amber-800 dark:bg-amber-400/15 dark:text-amber-200" data-testid="leaderboard-status">{{ t(data.demo ? 'activityLeaderboard.demo' : `activityLeaderboard.${data.status}`) }}</span>
          </div>
          <button
            type="button"
            class="shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-amber-100/70 hover:text-amber-800 focus-visible:ring-2 focus-visible:ring-amber-500/40 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-amber-200"
            :aria-label="t('activityLeaderboard.close')"
            data-testid="leaderboard-close"
            @click="closePopover(true)"
          >
            <Icon name="x" size="sm" />
          </button>
        </div>

        <div :aria-busy="loading" class="min-h-0 flex-1 space-y-3.5 overflow-y-auto overscroll-contain p-3.5">
          <div class="rounded-xl border border-amber-200/60 bg-gradient-to-br from-amber-50 via-orange-50/80 to-white px-3 py-2.5 dark:border-amber-400/15 dark:from-amber-950/40 dark:via-dark-800 dark:to-dark-900">
            <p v-if="displaySubtitle" class="break-words whitespace-pre-wrap text-xs font-medium text-amber-800 dark:text-amber-200">{{ displaySubtitle }}</p>
            <p v-if="data" class="mt-1 text-[11px] leading-5 text-gray-600 dark:text-dark-300">
              {{ t('activityLeaderboard.period', { start: formatTime(data.starts_at), end: formatTime(data.ends_at) }) }}
            </p>
            <p v-if="displayReward" class="break-words whitespace-pre-wrap mt-1 text-[11px] leading-5 text-amber-800/90 dark:text-amber-200/90">{{ displayReward }}</p>
          </div>

          <p v-if="loading && !data" role="status" class="py-10 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.loading') }}</p>
          <div v-if="error" role="alert" class="flex flex-wrap items-center justify-between gap-2 rounded-xl bg-red-50 p-3 text-xs text-red-700 dark:bg-red-950/30 dark:text-red-300">
            <span>{{ t(data ? 'activityLeaderboard.stale' : 'activityLeaderboard.error') }}</span>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="fetchLeaderboard">{{ t('activityLeaderboard.retry') }}</button>
          </div>

          <template v-if="data">
            <p v-if="data.demo" role="status" data-testid="leaderboard-demo-notice" class="rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] leading-5 text-amber-800 dark:border-amber-400/20 dark:bg-amber-400/10 dark:text-amber-200">
              {{ t('activityLeaderboard.demoNotice') }}
              <span v-if="data.demo_expires_at" class="block">{{ t('activityLeaderboard.demoExpiry', { time: formatTime(data.demo_expires_at) }) }}</span>
            </p>
            <p v-if="data.status === 'ended'" class="rounded-xl bg-gray-50 px-3 py-2.5 text-[11px] leading-5 text-gray-600 dark:bg-dark-800 dark:text-dark-300">{{ t('activityLeaderboard.endedNotice') }}</p>

            <div v-if="(data.status === 'upcoming' && !data.demo) || !data.entries.length" class="rounded-xl border border-dashed border-gray-200 px-4 py-7 text-center dark:border-dark-600" data-testid="leaderboard-empty">
              <Icon name="moon" size="lg" class="mx-auto mb-2 text-amber-500" />
              <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t(data.status === 'upcoming' ? 'activityLeaderboard.upcomingTitle' : 'activityLeaderboard.emptyTitle') }}</p>
              <p class="mx-auto mt-1.5 max-w-xs text-xs leading-5 text-gray-500 dark:text-dark-300">{{ t(data.status === 'upcoming' ? 'activityLeaderboard.upcomingDescription' : 'activityLeaderboard.emptyDescription') }}</p>
            </div>

            <template v-if="data.status !== 'upcoming' || data.demo">
              <div class="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-amber-200 bg-amber-50/60 p-3 dark:border-amber-400/20 dark:bg-amber-400/5" data-testid="leaderboard-me">
                <div>
                  <p class="text-[11px] text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.myRank') }}</p>
                  <p class="mt-0.5 text-sm font-bold text-gray-900 dark:text-white">{{ data.me ? `#${data.me.rank}` : t('activityLeaderboard.unranked') }}</p>
                </div>
                <div v-if="data.me" class="text-right">
                  <p class="text-sm font-semibold tabular-nums text-amber-700 dark:text-amber-300" :title="data.me.amount">{{ formatAmount(data.me.amount) }}</p>
                  <p class="mt-0.5 text-[11px] text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.amount') }}</p>
                </div>
                <p v-else class="max-w-[220px] text-[11px] leading-5 text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.unrankedHint') }}</p>
              </div>

              <section v-if="data.entries.length" aria-labelledby="festival-ranking-title">
                <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <h3 id="festival-ranking-title" class="text-xs font-semibold text-gray-900 dark:text-white">{{ t('activityLeaderboard.top') }}</h3>
                  <span class="text-[11px] text-gray-500 dark:text-dark-300">{{ t('activityLeaderboard.participants', { count: data.participant_count }) }}</span>
                </div>
                <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
                  <table class="w-full text-left text-xs">
                    <thead class="bg-gray-50 text-[11px] text-gray-500 dark:bg-dark-800 dark:text-dark-300">
                      <tr>
                        <th scope="col" class="px-2.5 py-2 font-medium">{{ t('activityLeaderboard.rank') }}</th>
                        <th scope="col" class="px-2 py-2 font-medium">{{ t('activityLeaderboard.participant') }}</th>
                        <th scope="col" class="px-2.5 py-2 text-right font-medium">{{ t('activityLeaderboard.amount') }}</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                      <tr v-for="entry in data.entries" :key="entry.alias" :class="entry.is_me ? 'bg-amber-50/60 dark:bg-amber-400/5' : ''" data-testid="leaderboard-row">
                        <td class="px-2.5 py-2">
                          <span class="inline-flex h-6 min-w-6 items-center justify-center rounded-md text-xs font-semibold tabular-nums" :class="rankClass(entry.rank)">{{ entry.rank }}</span>
                        </td>
                        <td class="px-2 py-2 text-gray-700 dark:text-dark-200">
                          <span class="break-all font-mono text-[11px]">{{ t('activityLeaderboard.anonymous', { alias: entry.alias }) }}</span>
                          <span v-if="entry.is_me" class="ml-1 inline-block rounded bg-amber-100 px-1 text-[10px] text-amber-800 dark:bg-amber-400/20 dark:text-amber-200">{{ t('activityLeaderboard.you') }}</span>
                        </td>
                        <td class="px-2.5 py-2 text-right font-medium tabular-nums text-gray-900 dark:text-white" :title="entry.amount">{{ formatAmount(entry.amount) }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </section>
            </template>

            <div class="flex flex-wrap items-center justify-between gap-2 text-[11px] text-gray-500 dark:text-dark-400">
              <span>{{ t('activityLeaderboard.updated', { time: formatTime(data.updated_at) }) }} · {{ t('activityLeaderboard.refreshHint') }}</span>
              <button type="button" class="inline-flex items-center gap-1 rounded-lg px-2 py-1 hover:bg-gray-100 disabled:opacity-50 dark:hover:bg-dark-700" :disabled="loading" @click="fetchLeaderboard">
                <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />{{ t('activityLeaderboard.refresh') }}
              </button>
            </div>

            <details class="rounded-xl bg-gray-50 px-3 py-2.5 text-[11px] leading-5 text-gray-600 dark:bg-dark-800/70 dark:text-dark-300">
              <summary class="cursor-pointer font-semibold text-gray-800 dark:text-dark-100">{{ t('activityLeaderboard.rulesTitle') }}</summary>
              <ul class="mt-1.5 list-disc space-y-1 pl-4">
                <li>{{ t('activityLeaderboard.rulesScope') }}</li>
                <li>{{ t('activityLeaderboard.rulesExclusions') }}</li>
                <li>{{ t('activityLeaderboard.rulesRanking') }}</li>
                <li>{{ t('activityLeaderboard.rulesPrivacy') }}</li>
              </ul>
            </details>
          </template>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getActivityLeaderboard,
  getActivityLeaderboardConfig,
  type ActivityLeaderboard,
  type ActivityLeaderboardPublicConfig,
} from '@/api/activityLeaderboard'
import { activityLeaderboardConfigVersion } from '@/utils/activityLeaderboardEvents'
import Icon from '@/components/icons/Icon.vue'

const { t, locale } = useI18n()
const show = ref(false)
const data = ref<ActivityLeaderboard | null>(null)
const loading = ref(false)
const error = ref(false)
const config = ref<ActivityLeaderboardPublicConfig | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const panelStyle = ref({ top: '0px', left: '0px' })
let controller: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined
let configController: AbortController | undefined
let configTimer: ReturnType<typeof setInterval> | undefined

const CONFIG_REFRESH_MS = 60 * 1000

// Show the entry only after the server confirms that the feature is enabled.
const entryHidden = computed(() => config.value?.enabled !== true)

// Headline texts come from the API (leaderboard payload first, public config
// as a pre-load stand-in); locale strings are the last-resort fallback.
const displayTitle = computed(() => data.value?.title || config.value?.title || t('activityLeaderboard.title'))
const displaySubtitle = computed(() => data.value?.subtitle ?? config.value?.subtitle ?? t('activityLeaderboard.subtitle'))
const displayReward = computed(() => data.value?.reward_description ?? config.value?.reward_description ?? t('activityLeaderboard.rewards'))

async function fetchConfig() {
  configController?.abort()
  const request = new AbortController()
  configController = request
  try {
    const result = await getActivityLeaderboardConfig(request.signal)
    if (!request.signal.aborted) {
      const changed = JSON.stringify(config.value) !== JSON.stringify(result)
      config.value = result
      if (changed) {
        stop()
        data.value = null
        error.value = false
        if (!result.enabled || result.status === 'disabled') closePopover(false)
        else if (show.value) void fetchLeaderboard()
      }
    }
  } catch {
    // Keep the previously known config on failure.
  } finally {
    if (configController === request) configController = undefined
  }
}

function onVisibilityChange() {
  if (!document.hidden) void fetchConfig()
}

const VIEWPORT_MARGIN = 8
const PANEL_WIDTH = 400

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
    if (!request.signal.aborted) {
      if (!result.enabled || result.status === 'disabled') {
        if (config.value) config.value = { ...config.value, enabled: false, status: 'disabled' }
        data.value = null
        closePopover(false)
      } else data.value = result
    }
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

// Anchor the panel under the trophy button, aligned to its right edge and
// clamped inside the viewport. Teleported to body so the glass header's
// backdrop-filter cannot clip or offset it.
function updatePosition() {
  const trigger = triggerRef.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const width = Math.min(PANEL_WIDTH, window.innerWidth - VIEWPORT_MARGIN * 2)
  const maxLeft = Math.max(window.innerWidth - width - VIEWPORT_MARGIN, VIEWPORT_MARGIN)
  const left = Math.min(Math.max(rect.right - width, VIEWPORT_MARGIN), maxLeft)
  const maxTop = Math.max(window.innerHeight - 240, VIEWPORT_MARGIN)
  const top = Math.min(Math.max(rect.bottom + VIEWPORT_MARGIN, VIEWPORT_MARGIN), maxTop)
  panelStyle.value = { top: `${top}px`, left: `${left}px` }
}

function togglePopover() {
  if (show.value) closePopover(false)
  else show.value = true
}

function closePopover(restoreFocus: boolean) {
  if (!show.value) return
  show.value = false
  if (restoreFocus) triggerRef.value?.focus()
}

function onPointerDown(event: PointerEvent) {
  const target = event.target as Node | null
  if (!target) return
  if (panelRef.value?.contains(target) || triggerRef.value?.contains(target)) return
  closePopover(false)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') closePopover(true)
}

function addListeners() {
  document.addEventListener('pointerdown', onPointerDown)
  document.addEventListener('keydown', onKeydown)
  window.addEventListener('resize', updatePosition)
  window.addEventListener('scroll', updatePosition, true)
}

function removeListeners() {
  document.removeEventListener('pointerdown', onPointerDown)
  document.removeEventListener('keydown', onKeydown)
  window.removeEventListener('resize', updatePosition)
  window.removeEventListener('scroll', updatePosition, true)
}

watch(show, async (open) => {
  if (open) {
    data.value = null
    error.value = false
    updatePosition()
    addListeners()
    void fetchLeaderboard()
    await nextTick()
    panelRef.value?.focus()
  } else {
    removeListeners()
    stop()
  }
})

// Disabling the activity hides the entry and closes an open popover.
watch(entryHidden, (hidden) => {
  if (hidden) closePopover(false)
})

// An admin saving the settings bumps this counter; refetch immediately.
watch(activityLeaderboardConfigVersion, () => void fetchConfig())

onMounted(() => {
  void fetchConfig()
  configTimer = setInterval(() => {
    if (!document.hidden) void fetchConfig()
  }, CONFIG_REFRESH_MS)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  removeListeners()
  stop()
  clearInterval(configTimer)
  configController?.abort()
  configController = undefined
  document.removeEventListener('visibilitychange', onVisibilityChange)
})

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

<style scoped>
.leaderboard-popover-enter-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.leaderboard-popover-leave-active {
  transition: opacity 0.12s ease, transform 0.12s ease;
}

.leaderboard-popover-enter-from,
.leaderboard-popover-leave-to {
  opacity: 0;
  transform: translateY(-6px) scale(0.98);
}
</style>
