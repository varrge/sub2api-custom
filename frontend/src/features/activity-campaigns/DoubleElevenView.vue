<template>
  <AppLayout>
    <div class="mx-auto w-full min-w-0 max-w-5xl space-y-6">
      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center gap-3 py-16">
        <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('activityCampaigns.loading') }}</p>
      </div>

      <!-- Load error -->
      <div v-else-if="loadError" class="py-16 text-center">
        <Icon name="exclamationCircle" size="xl" class="mx-auto text-red-500" />
        <p class="mt-3 text-sm text-gray-700 dark:text-dark-300">{{ loadError }}</p>
        <button class="btn btn-secondary mt-4" data-testid="d11-retry" @click="fetchState">
          {{ t('activityCampaigns.retry') }}
        </button>
      </div>

      <!-- Disabled -->
      <div v-else-if="phase === 'disabled'" class="empty-state py-16" data-testid="d11-disabled">
        <div class="empty-state-icon"><Icon name="ban" size="xl" class="text-gray-400 dark:text-dark-500" /></div>
        <p class="empty-state-title">{{ t('activityCampaigns.disabledTitle') }}</p>
        <p class="empty-state-description">{{ t('activityCampaigns.disabledDescription') }}</p>
      </div>

      <template v-else-if="campaign">
        <!-- Hero -->
        <header
          class="relative overflow-hidden rounded-2xl bg-gradient-primary p-5 text-on-primary shadow-glow sm:p-7"
          data-testid="d11-hero"
        >
          <Icon
            name="sparkles"
            size="xl"
            class="pointer-events-none absolute -right-3 -top-4 h-28 w-28 opacity-20"
          />
          <Icon
            name="gift"
            size="xl"
            class="pointer-events-none absolute -bottom-5 right-24 hidden h-24 w-24 opacity-10 sm:block"
          />
          <div class="relative">
            <div class="flex flex-wrap items-center gap-2">
              <span
                class="inline-flex items-center gap-1 rounded-full bg-black/15 px-2.5 py-0.5 text-xs font-semibold"
                data-testid="d11-phase"
              >
                <Icon :name="phase === 'active' ? 'fire' : phase === 'upcoming' ? 'clock' : 'checkCircle'" size="xs" />
                {{ phaseLabel }}
              </span>
              <span class="text-xs opacity-80">Double 11 · Wave 1</span>
            </div>
            <h1 class="mt-2 break-words text-2xl font-extrabold sm:text-3xl">{{ campaign.title }}</h1>
            <p v-if="campaign.subtitle" class="mt-1 break-words text-sm font-medium opacity-90">
              {{ campaign.subtitle }}
            </p>
            <p v-if="campaign.description" class="mt-2 max-w-2xl whitespace-pre-line break-words text-sm opacity-80">
              {{ campaign.description }}
            </p>

            <!-- Countdown, anchored to server_time -->
            <div v-if="phase === 'upcoming' || phase === 'active'" class="mt-4">
              <p class="text-xs font-semibold uppercase tracking-wide opacity-80">
                {{ phase === 'upcoming' ? t('activityCampaigns.countdownToStart') : t('activityCampaigns.countdownToEnd') }}
              </p>
              <div class="mt-1.5 flex items-center gap-1.5" data-testid="d11-countdown">
                <span v-if="countdown.days > 0" class="d11-digit">{{ t('activityCampaigns.countdownDays', { n: countdown.days }) }}</span>
                <span class="d11-digit">{{ countdown.hours }}</span>
                <span class="font-bold opacity-70">:</span>
                <span class="d11-digit">{{ countdown.minutes }}</span>
                <span class="font-bold opacity-70">:</span>
                <span class="d11-digit">{{ countdown.seconds }}</span>
              </div>
            </div>

            <p class="mt-3 flex items-center gap-1 text-xs opacity-80">
              <Icon name="calendar" size="xs" />
              {{ t('activityCampaigns.window', { start: formatCn(campaign.starts_at), end: formatCn(campaign.ends_at) }) }}
            </p>
          </div>
        </header>

        <p
          v-if="phase === 'upcoming'"
          class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
          data-testid="d11-upcoming-notice"
        >
          {{ t('activityCampaigns.upcomingNotice') }}
        </p>
        <p
          v-else-if="phase === 'ended'"
          class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300"
          data-testid="d11-ended-notice"
        >
          {{ t('activityCampaigns.endedNotice') }}
        </p>

        <!-- Progress -->
        <section class="card p-4 sm:p-5" data-testid="d11-progress">
          <div class="flex items-center justify-between gap-3">
            <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-dark-300">
              <Icon name="chart" size="sm" class="text-primary-500" />
              {{ t('activityCampaigns.progressTitle') }}
            </h2>
            <span class="text-sm font-semibold text-primary-600 dark:text-primary-400">
              {{ t('activityCampaigns.cardsProgress', { earned: earnedCount }) }}
            </span>
          </div>
          <div class="mt-3 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <div
              class="h-full rounded-full bg-gradient-primary transition-all duration-500"
              :style="{ width: `${(earnedCount / 3) * 100}%` }"
            ></div>
          </div>
        </section>

        <!-- Check-in + rewards grid -->
        <div class="grid gap-6 lg:grid-cols-2">
          <!-- Check-in -->
          <section class="card p-4 sm:p-5" data-testid="d11-checkin">
            <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-dark-300">
              <Icon name="calendar" size="sm" class="text-primary-500" />
              {{ t('activityCampaigns.checkin.title') }}
            </h2>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
              {{ t('activityCampaigns.checkin.description') }}
            </p>

            <div class="mt-4 flex items-center gap-2">
              <span
                v-for="day in 3"
                :key="day"
                class="flex h-8 w-8 items-center justify-center rounded-full border text-xs font-bold transition-colors"
                :class="checkinCount >= day
                  ? 'border-primary-500 bg-gradient-primary text-on-primary'
                  : 'border-gray-200 text-gray-400 dark:border-dark-600 dark:text-dark-500'"
                :data-testid="`d11-checkin-dot-${day}`"
              >
                <Icon v-if="checkinCount >= day" name="check" size="sm" />
                <template v-else>{{ day }}</template>
              </span>
              <span class="ml-1 text-xs font-medium text-gray-600 dark:text-dark-300">
                {{ t('activityCampaigns.checkin.days', { count: checkinCount }) }}
              </span>
            </div>
            <p v-if="checkinCount >= 3" class="mt-2 flex items-center gap-1 text-xs font-medium text-emerald-600 dark:text-emerald-400">
              <Icon name="badge" size="sm" />
              {{ t('activityCampaigns.checkin.milestone') }}
            </p>

            <button
              class="btn btn-primary mt-4 w-full py-2.5"
              data-testid="d11-checkin-button"
              :disabled="!canCheckin"
              @click="doCheckin"
            >
              <svg v-if="checkinPending" class="-ml-1 mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <Icon v-else-if="checkedToday" name="checkCircle" size="sm" class="mr-1.5" />
              {{ checkinButtonLabel }}
            </button>
            <p v-if="checkinError" class="mt-2 text-xs text-red-600 dark:text-red-400" data-testid="d11-checkin-error">
              {{ checkinError }}
            </p>

            <template v-if="checkinDates.length">
              <p class="mt-4 text-xs font-medium text-gray-500 dark:text-dark-400">
                {{ t('activityCampaigns.checkin.datesLabel') }}
              </p>
              <ul class="mt-1.5 flex flex-wrap gap-1.5">
                <li
                  v-for="date in checkinDates"
                  :key="date"
                  class="rounded-full bg-primary-50 px-2.5 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                >
                  {{ date }}
                </li>
              </ul>
            </template>
          </section>

          <!-- Rewards -->
          <section class="card p-4 sm:p-5" data-testid="d11-rewards">
            <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-dark-300">
              <Icon name="gift" size="sm" class="text-primary-500" />
              {{ t('activityCampaigns.rewards.title') }}
            </h2>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
              {{ t('activityCampaigns.rewards.subtitle') }}
            </p>

            <ul class="mt-4 space-y-2.5">
              <li
                v-for="reward in rewardCards"
                :key="reward.task"
                class="flex items-center gap-3 rounded-xl border p-3 transition-colors"
                :class="reward.delivered
                  ? 'border-emerald-200 bg-emerald-50/60 dark:border-emerald-800 dark:bg-emerald-900/10'
                  : reward.earned
                    ? 'border-primary-300 bg-primary-50/60 dark:border-primary-700 dark:bg-primary-900/10'
                    : 'border-gray-200 dark:border-dark-600'"
                :data-testid="`d11-reward-${reward.task}`"
              >
                <div
                  class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg"
                  :class="reward.delivered
                    ? 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400'
                    : reward.earned
                      ? 'bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400'
                      : 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-dark-500'"
                >
                  <Icon :name="reward.icon" size="md" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ reward.label }}</p>
                  <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('activityCampaigns.rewards.rewardPerTask') }}</p>
                </div>
                <span
                  v-if="reward.delivered"
                  class="inline-flex flex-shrink-0 items-center gap-1 rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                >
                  <Icon name="check" size="xs" />
                  {{ t('activityCampaigns.rewards.claimed') }}
                </span>
                <button
                  v-else-if="reward.earned"
                  class="btn btn-primary btn-sm flex-shrink-0"
                  :data-testid="`d11-claim-${reward.task}`"
                  :disabled="busy"
                  @click="doClaim(reward.task)"
                >
                  {{ claimingTask === reward.task ? t('activityCampaigns.rewards.claiming') : t('activityCampaigns.rewards.claim') }}
                </button>
                <span
                  v-else
                  class="inline-flex flex-shrink-0 items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-semibold text-gray-500 dark:bg-dark-700 dark:text-dark-400"
                >
                  <Icon name="lock" size="xs" />
                  {{ t('activityCampaigns.rewards.locked') }}
                </span>
              </li>
            </ul>
            <p v-if="claimError" class="mt-3 text-xs text-red-600 dark:text-red-400" data-testid="d11-claim-error">
              {{ claimError }}
            </p>

            <RouterLink
              to="/activities/box"
              class="mt-4 inline-flex items-center gap-1.5 text-sm font-semibold text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-500"
              data-testid="d11-go-box"
            >
              {{ t('activityCampaigns.goBox') }}
              <Icon name="arrowRight" size="sm" />
            </RouterLink>
          </section>
        </div>

        <!-- Quiz -->
        <section class="card p-4 sm:p-5" data-testid="d11-quiz">
          <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-dark-300">
            <Icon name="questionCircle" size="sm" class="text-primary-500" />
            {{ t('activityCampaigns.quiz.title') }}
          </h2>
          <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
            {{ t('activityCampaigns.quiz.description') }}
          </p>

          <div
            v-if="quizCompleted"
            class="mt-4 flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm font-medium text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-400"
            data-testid="d11-quiz-completed"
          >
            <Icon name="checkCircle" size="md" />
            {{ t('activityCampaigns.quiz.completed') }}
          </div>

          <template v-else>
            <ol class="mt-4 space-y-5">
              <li
                v-for="(question, qi) in campaign.questions"
                :key="question.id"
                class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800"
                :data-testid="`d11-quiz-question-${qi}`"
              >
                <p class="text-sm font-medium text-gray-900 dark:text-white">
                  <span class="mr-2 inline-flex h-5 w-5 items-center justify-center rounded-full bg-gradient-primary text-xs font-bold text-on-primary">
                    {{ qi + 1 }}
                  </span>
                  {{ question.prompt }}
                </p>
                <div class="mt-3 grid gap-2 sm:grid-cols-2">
                  <button
                    v-for="(option, oi) in question.options"
                    :key="oi"
                    type="button"
                    class="rounded-lg border px-3 py-2.5 text-left text-sm transition-colors"
                    :class="answers[question.id] === oi
                      ? 'border-primary-500 bg-primary-50 font-semibold text-primary-700 dark:border-primary-500 dark:bg-primary-900/30 dark:text-primary-400'
                      : 'border-gray-200 text-gray-600 hover:border-primary-300 hover:bg-primary-50/60 dark:border-dark-600 dark:text-dark-300 dark:hover:border-primary-700'"
                    :aria-pressed="answers[question.id] === oi"
                    :data-testid="`d11-quiz-option-${qi}-${oi}`"
                    @click="selectAnswer(question.id, oi)"
                  >
                    {{ option }}
                  </button>
                </div>
              </li>
            </ol>

            <button
              class="btn btn-primary mt-4 w-full py-2.5 sm:w-auto sm:px-10"
              data-testid="d11-quiz-submit"
              :disabled="busy || phase !== 'active'"
              @click="submitQuiz"
            >
              <svg v-if="quizPending" class="-ml-1 mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              {{ quizPending ? t('activityCampaigns.quiz.submitting') : t('activityCampaigns.quiz.submit') }}
            </button>
            <p v-if="quizError" class="mt-3 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300" data-testid="d11-quiz-error">
              {{ quizError }}
            </p>
          </template>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  activityCampaignsAPI,
  type ActivityCampaignState,
  type ActivityTask,
} from '@/api/activityCampaigns'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const userId = computed(() => (authStore.user ? String(authStore.user.id) : ''))

const state = ref<ActivityCampaignState | null>(null)
const loading = ref(false)
const loadError = ref('')
let stateController: AbortController | null = null
let disposed = false
let identityGeneration = 0

const campaign = computed(() => state.value?.campaign ?? null)

// Clock anchored to the server's server_time: the client clock only drives the
// tick, the offset from the last server response anchors absolute time.
const clockOffsetMs = ref(0)
const now = ref(Date.now())
const serverNow = computed(() => now.value + clockOffsetMs.value)
let clockTimer: ReturnType<typeof setInterval> | null = null

type Phase = 'disabled' | 'upcoming' | 'active' | 'ended'
const phase = computed<Phase>(() => {
  const s = state.value
  if (!s || !s.campaign) return 'disabled'
  if (s.status === 'disabled' || !s.campaign.enabled) return 'disabled'
  const start = Date.parse(s.campaign.starts_at)
  const end = Date.parse(s.campaign.ends_at)
  if (Number.isFinite(start) && serverNow.value < start) return 'upcoming'
  if (Number.isFinite(end) && serverNow.value >= end) return 'ended'
  return 'active'
})

const phaseLabel = computed(() => t(`activityCampaigns.phase${phase.value.charAt(0).toUpperCase()}${phase.value.slice(1)}`))

const countdownTarget = computed(() => {
  if (phase.value === 'upcoming') return Date.parse(campaign.value?.starts_at ?? '')
  if (phase.value === 'active') return Date.parse(campaign.value?.ends_at ?? '')
  return NaN
})

const pad2 = (n: number) => String(n).padStart(2, '0')
const countdown = computed(() => {
  const remaining = Math.max(0, countdownTarget.value - serverNow.value)
  const totalSeconds = Math.floor(remaining / 1000)
  const days = Math.floor(totalSeconds / 86400)
  const hours = Math.floor((totalSeconds % 86400) / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  return { days, hours: pad2(hours), minutes: pad2(minutes), seconds: pad2(seconds) }
})

// All date labels are rendered in Asia/Shanghai regardless of browser timezone.
const formatCn = (iso: string): string => {
  const timestamp = Date.parse(iso)
  if (!Number.isFinite(timestamp)) return iso
  return new Intl.DateTimeFormat(locale.value, {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(timestamp))
}

const checkinDates = computed(() => state.value?.checkin_dates ?? [])
const checkinCount = computed(() => Math.min(checkinDates.value.length, 3))
const checkedToday = computed(() => {
  const day = new Date(serverNow.value + 8 * 3600_000).toISOString().slice(0, 10)
  return checkinDates.value.includes(day)
})

const checkinPending = ref(false)
const checkinError = ref('')
const canCheckin = computed(
  () => phase.value === 'active' && !checkedToday.value && !busy.value,
)
const checkinButtonLabel = computed(() => {
  if (checkinPending.value) return t('activityCampaigns.checkin.pending')
  if (checkedToday.value) return t('activityCampaigns.checkin.checkedToday')
  if (phase.value !== 'active') return t('activityCampaigns.checkin.notActive')
  return t('activityCampaigns.checkin.button')
})

// Quiz
const answers = ref<Record<string, number>>({})
const quizPending = ref(false)
const quizError = ref('')
const quizCompleted = computed(() => state.value?.quiz_completed ?? false)
let loadedRevision: number | null = null

// Rewards
const claimingTask = ref<ActivityTask | null>(null)
const claimError = ref('')
const busy = computed(() => loading.value || checkinPending.value || quizPending.value || claimingTask.value !== null)

const REWARD_META: Record<ActivityTask, { labelKey: string; icon: 'calendar' | 'badge' | 'questionCircle' }> = {
  first_checkin: { labelKey: 'activityCampaigns.rewards.taskFirstCheckin', icon: 'calendar' },
  three_checkins: { labelKey: 'activityCampaigns.rewards.taskThreeCheckins', icon: 'badge' },
  quiz: { labelKey: 'activityCampaigns.rewards.taskQuiz', icon: 'questionCircle' },
}

const rewardByTask = computed(() => {
  const map = new Map<ActivityTask, { earned: boolean; delivered: boolean }>()
  for (const reward of state.value?.rewards ?? []) {
    map.set(reward.task, { earned: true, delivered: reward.delivered_at !== null })
  }
  return map
})

const rewardCards = computed(() =>
  (Object.keys(REWARD_META) as ActivityTask[]).map((task) => {
    const status = rewardByTask.value.get(task)
    return {
      task,
      label: t(REWARD_META[task].labelKey),
      icon: REWARD_META[task].icon,
      earned: status?.earned ?? false,
      delivered: status?.delivered ?? false,
    }
  }),
)

const earnedCount = computed(() => (state.value?.rewards ?? []).length)

const applyState = (next: ActivityCampaignState) => {
  const serverTime = Date.parse(next.server_time)
  now.value = Date.now()
  if (Number.isFinite(serverTime)) clockOffsetMs.value = serverTime - now.value
  // A config revision change invalidates previously selected answers.
  if (loadedRevision !== null && next.campaign.revision !== loadedRevision) {
    answers.value = {}
  }
  loadedRevision = next.campaign.revision
  state.value = next
}

const statusOf = (error: unknown): number => (error as { status?: number })?.status ?? 0

const fetchState = async () => {
  stateController?.abort()
  stateController = new AbortController()
  const signal = stateController.signal
  const uid = userId.value
  const generation = identityGeneration
  loading.value = true
  loadError.value = ''
  try {
    const data = await activityCampaignsAPI.get()
    if (disposed || signal.aborted || identityGeneration !== generation || userId.value !== uid) return
    applyState(data)
  } catch (error) {
    if (disposed || signal.aborted || identityGeneration !== generation || userId.value !== uid) return
    loadError.value = extractApiErrorMessage(error, t('activityCampaigns.loadFailed'))
  } finally {
    if (!disposed && !signal.aborted && identityGeneration === generation) loading.value = false
  }
}

const doCheckin = async () => {
  if (!canCheckin.value) return
  const uid = userId.value
  const generation = identityGeneration
  checkinPending.value = true
  checkinError.value = ''
  try {
    const data = await activityCampaignsAPI.checkin()
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    applyState(data)
  } catch (error) {
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    checkinError.value = extractApiErrorMessage(error, t('activityCampaigns.checkin.failed'))
  } finally {
    if (!disposed && identityGeneration === generation && userId.value === uid) checkinPending.value = false
  }
}

const selectAnswer = (questionId: string, optionIndex: number) => {
  if (busy.value || quizCompleted.value) return
  answers.value = { ...answers.value, [questionId]: optionIndex }
  quizError.value = ''
}

const submitQuiz = async () => {
  const s = state.value
  if (!s || busy.value || quizCompleted.value || phase.value !== 'active') return
  const questions = s.campaign.questions
  if (questions.some((q) => answers.value[q.id] === undefined)) {
    quizError.value = t('activityCampaigns.quiz.incomplete')
    return
  }
  const uid = userId.value
  const generation = identityGeneration
  const revision = s.campaign.revision
  quizPending.value = true
  quizError.value = ''
  try {
    const data = await activityCampaignsAPI.quiz(revision, { ...answers.value })
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    applyState(data)
  } catch (error) {
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    const status = statusOf(error)
    if (status === 422) {
      // Wrong answers: retry freely, the server never reveals which ones.
      quizError.value = t('activityCampaigns.quiz.wrong')
    } else if (status === 409) {
      // Config changed: reload and discard stale answers.
      quizError.value = t('activityCampaigns.quiz.configChanged')
      answers.value = {}
      loadedRevision = null
      void fetchState()
    } else {
      quizError.value = extractApiErrorMessage(error, t('activityCampaigns.quiz.failed'))
    }
  } finally {
    if (!disposed && identityGeneration === generation && userId.value === uid) quizPending.value = false
  }
}

const doClaim = async (task: ActivityTask) => {
  if (busy.value || phase.value === 'disabled') return
  const status = rewardByTask.value.get(task)
  if (!status?.earned || status.delivered) return
  const uid = userId.value
  const generation = identityGeneration
  claimingTask.value = task
  claimError.value = ''
  try {
    const data = await activityCampaignsAPI.claim(task)
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    applyState(data)
  } catch (error) {
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    // Show the failure but keep the earned state untouched for a safe retry.
    claimError.value = extractApiErrorMessage(error, t('activityCampaigns.rewards.claimFailed'))
  } finally {
    if (!disposed && identityGeneration === generation && userId.value === uid) claimingTask.value = null
  }
}

// Account switch: never leak another user's state, selections or errors.
watch(userId, (next, previous) => {
  if (previous === undefined || next === previous) return
  identityGeneration++
  stateController?.abort()
  state.value = null
  answers.value = {}
  loadedRevision = null
  checkinError.value = ''
  quizError.value = ''
  claimError.value = ''
  checkinPending.value = false
  quizPending.value = false
  claimingTask.value = null
  clockOffsetMs.value = 0
  if (next) void fetchState()
}, { flush: 'sync' })

onMounted(() => {
  void fetchState()
  clockTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  disposed = true
  identityGeneration++
  stateController?.abort()
  if (clockTimer !== null) {
    clearInterval(clockTimer)
    clockTimer = null
  }
})
</script>

<style scoped>
.d11-digit {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 2.5rem;
  padding: 0.375rem 0.5rem;
  border-radius: 0.5rem;
  background: rgba(0, 0, 0, 0.15);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 1.125rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
</style>
