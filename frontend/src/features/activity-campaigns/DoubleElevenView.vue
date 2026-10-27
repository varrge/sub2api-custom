<template>
  <AppLayout>
    <div class="festival-page mx-auto w-full min-w-0 max-w-5xl space-y-6">

      <!-- Distinct Wave Navigation -->
      <nav class="flex flex-wrap gap-2 border-b border-gray-200 dark:border-dark-700 pb-px mb-6" :aria-label="t('activityCampaigns.waves.nav')">
        <button
          v-for="waveId in waves"
          :key="waveId"
          @click="selectWave(waveId)"
          :disabled="busy"
          :aria-pressed="selectedWaveId === waveId"
          class="border-b-2 px-4 py-2.5 text-sm font-medium transition-all"
          :class="selectedWaveId === waveId
            ? 'border-primary-500 text-primary-600 dark:text-primary-400 font-bold'
            : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200'"
          :data-testid="`wave-tab-${waveId}`"
        >
          {{ t(`activityCampaigns.waves.${waveId === waveOneID ? 'wave1' : waveId === waveTwoID ? 'wave2' : 'wave3'}`) }}
        </button>
      </nav>
      <p v-if="currentStateObj.error && state" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-700">
        {{ currentStateObj.error }}
        <button type="button" class="ml-2 underline" :disabled="busy" @click="fetchSingleState(selectedWaveId)">{{ t('activityCampaigns.retry') }}</button>
      </p>

      <!-- Global Loading (when there's no cached state for selected wave) -->
      <div v-if="currentStateObj.loading && !state" class="flex items-center justify-center gap-3 py-16">
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
      <div v-else-if="currentStateObj.error && !state" class="py-16 text-center">
        <Icon name="exclamationCircle" size="xl" class="mx-auto text-red-500" />
        <p class="mt-3 text-sm text-gray-700 dark:text-dark-300">{{ currentStateObj.error }}</p>
        <button class="btn btn-secondary mt-4" data-testid="d11-retry" @click="fetchSingleState(selectedWaveId)">
          {{ t('activityCampaigns.retry') }}
        </button>
      </div>

      <!-- Disabled -->
      <div v-else-if="phase === 'disabled'" class="empty-state py-16" data-testid="d11-disabled">
        <div class="empty-state-icon"><Icon name="lock" size="xl" class="text-gray-400 dark:text-dark-500" /></div>
        <p class="empty-state-title">{{ t('activityCampaigns.disabledTitle') }}</p>
        <p class="empty-state-description">{{ t('activityCampaigns.disabledDescription') }}</p>
      </div>

      <template v-else-if="campaign">
        <!-- Decorative Hero section with unchanged whale-girl PNG -->
        <header class="festival-hero" data-testid="d11-hero">
          <div class="festival-marquee" aria-hidden="true">
            <span>✦ DOUBLE ELEVEN ✦</span><span>{{ t('activityCampaigns.festival.marquee') }}</span><span>✦ 11.11 ✦</span>
          </div>
          <div class="festival-hero-main">
            <div class="festival-copy">
              <div class="festival-eyebrow">
                <span class="festival-wave">{{ t(`activityCampaigns.waves.${selectedWaveId === waveOneID ? 'wave1' : selectedWaveId === waveTwoID ? 'wave2' : 'wave3'}`) }}</span>
                <span class="festival-phase" data-testid="d11-phase"><i></i>{{ phaseLabel }}</span>
              </div>
              <p class="festival-overline">{{ t('activityCampaigns.festival.overline') }}</p>
              <h1 class="text-gray-900 dark:text-white">{{ campaign.title }}</h1>
              <p v-if="campaign.subtitle" class="festival-subtitle">{{ campaign.subtitle }}</p>
              <p v-if="campaign.description" class="festival-description">{{ campaign.description }}</p>
              <div class="festival-perks">
                <span><Icon name="gift" size="sm" />{{ selectedWaveId === waveTwoID ? t('activityCampaigns.wave2.purchaseGift') : t('activityCampaigns.festival.free') }}</span>
                <span><Icon name="sparkles" size="sm" />{{ selectedWaveId === waveOneID ? t('activityCampaigns.festival.threeCards') : t('activityCampaigns.festival.lucky') }}</span>
              </div>
            </div>
            <div class="festival-art" aria-hidden="true">
              <svg viewBox="0 0 430 350" fill="none" focusable="false">
                <defs>
                  <linearGradient id="d11-gold" x1="90" y1="20" x2="300" y2="300" gradientUnits="userSpaceOnUse"><stop stop-color="#FFE5A6"/><stop offset="1" stop-color="#D9904B"/></linearGradient>
                </defs>
                <circle cx="228" cy="181" r="147" fill="#F2D8AD" fill-opacity=".6"/>
                <circle cx="228" cy="181" r="133" stroke="#C79865" stroke-opacity=".35" stroke-dasharray="3 9"/>
                <text x="26" y="105" fill="#B86943" fill-opacity=".17" font-size="101" font-weight="900" font-family="Arial, sans-serif" letter-spacing="-8">11.11</text>
                <ellipse cx="234" cy="327" rx="160" ry="13" fill="#7E482A" fill-opacity=".1"/>
                <g class="festival-confetti">
                  <path d="M39 156c-13-16 17-16 10-30s-19 0-12 8" stroke="#B96147" stroke-width="5" stroke-linecap="round"/>
                  <path d="M381 117c18 11-13 19-3 31s17-6 10-12" stroke="#C4954A" stroke-width="5" stroke-linecap="round"/>
                  <path d="m335 52 4 15 15 4-15 4-4 15-4-15-15-4 15-4z" fill="#B45139"/>
                  <path d="m86 199 3 10 10 3-10 3-3 10-3-10-10-3 10-3z" fill="#B58A44"/>
                  <path d="m366 219 3 10 10 3-10 3-3 10-3-10-10-3 10-3z" fill="#B45139"/>
                  <path d="m70 69 7-12M393 196l9 5M131 27l-3-10" stroke="#C88B55" stroke-width="5" stroke-linecap="round"/>
                  <path d="m291 24 8 4-4 8-8-4zM52 247l7-2 2 8-7 2z" fill="#B65D44"/>
                  <circle cx="398" cy="80" r="4" fill="#B58A44"/><circle cx="24" cy="208" r="3" fill="#B45139"/>
                </g>
                <g transform="rotate(-9 87 283)">
                  <rect x="38" y="264" width="96" height="61" rx="8" fill="#B75B43"/>
                  <rect x="32" y="250" width="108" height="25" rx="5" fill="#C76F52"/>
                  <path d="M81 250v75" stroke="#F1C983" stroke-width="17"/>
                  <path d="M89 250c-50 0-46-36-24-25 9 5 16 16 24 25Zm0 0c42-6 44-35 23-27-8 4-15 16-23 27Z" fill="#F1C983" stroke="#E2AD64" stroke-width="3"/>
                </g>
                <image class="festival-whale animate-bounce-gentle" href="/activity-campaigns/whale-girl.png" x="28" y="-18" width="390" height="390" preserveAspectRatio="xMidYMid meet" />
                <g transform="rotate(10 340 291)">
                  <rect x="305" y="274" width="76" height="51" rx="6" fill="url(#d11-gold)"/>
                  <rect x="299" y="260" width="88" height="21" rx="5" fill="#F4D69A"/>
                  <path d="M343 260v65" stroke="#AD5841" stroke-width="13"/>
                  <path d="M343 260c-35-1-35-26-20-20 8 3 13 13 20 20Zm0 0c30-5 33-29 17-23-8 3-11 15-17 23Z" fill="#AD5841"/>
                </g>
              </svg>
              <span class="festival-art-sticker">{{ t('activityCampaigns.festival.lucky') }}<Icon name="sparkles" size="sm" /></span>
            </div>
          </div>
          <div class="festival-ticket-footer">
            <div v-if="phase === 'upcoming' || phase === 'active'" class="festival-countdown-wrap">
              <span class="festival-countdown-label">{{ phase === 'upcoming' ? t('activityCampaigns.countdownToStart') : t('activityCampaigns.countdownToEnd') }}</span>
              <div class="festival-countdown" data-testid="d11-countdown">
                <span v-if="countdown.days > 0" class="d11-digit">{{ t('activityCampaigns.countdownDays', { n: countdown.days }) }}</span>
                <span class="d11-digit">{{ countdown.hours }}</span><b>:</b>
                <span class="d11-digit">{{ countdown.minutes }}</span><b>:</b>
                <span class="d11-digit">{{ countdown.seconds }}</span>
              </div>
            </div>
            <div class="festival-dates-wrap">
              <p class="festival-dates"><Icon name="calendar" size="sm" />{{ t('activityCampaigns.window', { start: formatCn(campaign.starts_at), end: formatCn(campaign.ends_at) }) }}</p>
            </div>
          </div>
        </header>

        <p
          v-if="phase === 'upcoming'"
          class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400 font-medium"
          data-testid="d11-upcoming-notice"
        >
          {{ t('activityCampaigns.upcomingNotice') }}
        </p>
        <p
          v-else-if="phase === 'ended'"
          class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300 font-medium"
          data-testid="d11-ended-notice"
        >
          {{ selectedWaveId === waveThreeID ? t('activityCampaigns.wave3.endedNotice') : t('activityCampaigns.endedNotice') }}
        </p>

        <!-- ======================================================== -->
        <!-- WAVE 1: CHECK-IN & QUIZ                                  -->
        <!-- ======================================================== -->
        <template v-if="selectedWaveId === waveOneID">
          <!-- Progress -->
          <section class="festival-progress card p-4 sm:p-5" data-testid="d11-progress">
            <span class="festival-section-tag">{{ t('activityCampaigns.festival.passport') }}</span>
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
            <section class="festival-task festival-checkin card p-4 sm:p-5" data-testid="d11-checkin">
              <div class="festival-task-label"><span>01</span>{{ t('activityCampaigns.festival.checkinTag') }}<Icon name="sparkles" size="sm" /></div>
              <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-dark-300">
                <Icon name="calendar" size="sm" class="text-primary-500" />
                {{ t('activityCampaigns.checkin.title') }}
              </h2>
              <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
                {{ t('activityCampaigns.checkin.description') }}
              </p>

              <div class="festival-stamps">
                <span
                  v-for="day in 3"
                  :key="day"
                  class="festival-stamp"
                  :class="checkinCount >= day ? 'festival-stamp-done' : ''"
                  :data-testid="`d11-checkin-dot-${day}`"
                >
                  <small>{{ t('activityCampaigns.festival.stampDay', { n: day }) }}</small>
                  <Icon v-if="checkinCount >= day" name="check" size="lg" />
                  <Icon v-else :name="day === 2 ? 'sparkles' : 'gift'" size="lg" />
                  <em>{{ checkinCount >= day ? t('activityCampaigns.festival.stamped') : '✦' }}</em>
                </span>
                <span class="festival-stamp-count">
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
            <section class="festival-task festival-rewards card p-4 sm:p-5" data-testid="d11-rewards">
              <div class="festival-task-label"><span>02</span>{{ t('activityCampaigns.festival.rewardsTag') }}<Icon name="gift" size="sm" /></div>
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
                  class="festival-reward-ticket flex items-center gap-3 rounded-xl border p-3 transition-colors"
                  :class="reward.delivered
                    ? 'border-emerald-200 bg-emerald-50/60 dark:border-emerald-800 dark:bg-emerald-900/10'
                    : reward.earned
                      ? 'border-primary-300 bg-primary-50/60 dark:border-primary-700 dark:bg-primary-900/10'
                      : 'border-gray-200 dark:border-dark-600'"
                  :data-testid="`d11-reward-${reward.task}`"
                >
                  <div
                    class="festival-ticket-icon flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg"
                    :class="reward.delivered
                      ? 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400'
                      : reward.earned
                        ? 'bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400'
                        : 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-dark-500'"
                  >
                    <Icon :name="reward.icon" size="md" />
                  </div>
                  <div class="festival-ticket-copy min-w-0 flex-1">
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
                    class="btn btn-primary btn-sm flex-shrink-0 animate-pulse-gentle"
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
          <section class="festival-task festival-quiz card p-4 sm:p-5" data-testid="d11-quiz">
            <div class="festival-task-label"><span>03</span>{{ t('activityCampaigns.festival.quizTag') }}<Icon name="questionCircle" size="sm" /></div>
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
                  class="festival-question rounded-xl bg-gray-50 p-4 dark:bg-dark-800"
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
                      class="festival-answer rounded-lg border px-3 py-2.5 text-left text-sm transition-colors"
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

        <!-- ======================================================== -->
        <!-- WAVE 2: MONTH CARD拼团                                   -->
        <!-- ======================================================== -->
        <template v-else-if="selectedWaveId === waveTwoID">
          <!-- Products Info table/grid -->
          <section class="festival-task card p-4 sm:p-5" data-testid="d11-wave2-products">
            <div class="festival-task-label"><span>01</span>{{ t('activityCampaigns.wave2.productsTitle') }}<Icon name="sparkles" size="sm" /></div>
            <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-dark-300">
              <Icon name="grid" size="sm" class="text-primary-500" />
              {{ t('activityCampaigns.wave2.productsTitle') }}
            </h2>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
              {{ t('activityCampaigns.wave2.description') }}
            </p>

            <div class="mt-4 grid gap-4 sm:grid-cols-3">
              <!-- Product 1 (58) -->
              <div class="rounded-xl border border-gray-200 dark:border-dark-600 bg-gray-50/50 dark:bg-dark-800/30 p-4 text-center flex flex-col justify-between">
                <div>
                  <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('activityCampaigns.wave2.monthlyCard', { price: 58 }) }}</h3>
                  <p class="text-xs text-gray-400 mt-0.5">{{ t('activityCampaigns.wave2.originalPrice') }}: ￥58</p>
                  <hr class="my-3 border-dashed border-gray-200 dark:border-dark-700" />
                  <div class="space-y-2 text-left text-xs">
                    <p class="flex items-center gap-1.5 text-gray-700 dark:text-dark-300">
                      <Icon name="gift" size="xs" class="text-primary-500" />
                      <strong>{{ t('activityCampaigns.wave2.purchaseGift') }}:</strong>
                      <span>{{ t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierLow'), qty: 1 }) }}</span>
                    </p>
                    <p class="flex items-center gap-1.5 text-gray-700 dark:text-dark-300">
                      <Icon name="users" size="xs" class="text-primary-500" />
                      <strong>{{ t('activityCampaigns.wave2.groupGift') }}:</strong>
                      <span>{{ t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierLow'), qty: 1 }) }}</span>
                    </p>
                  </div>
                </div>
              </div>

              <!-- Product 2 (128) -->
              <div class="rounded-xl border border-primary-200 dark:border-primary-900 bg-primary-50/20 dark:bg-primary-900/10 p-4 text-center flex flex-col justify-between relative overflow-hidden">
                <div>
                  <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('activityCampaigns.wave2.monthlyCard', { price: 128 }) }}</h3>
                  <p class="text-xs text-gray-400 mt-0.5">{{ t('activityCampaigns.wave2.originalPrice') }}: ￥128</p>
                  <hr class="my-3 border-dashed border-gray-200 dark:border-primary-900/50" />
                  <div class="space-y-2 text-left text-xs">
                    <p class="flex items-center gap-1.5 text-gray-700 dark:text-dark-300">
                      <Icon name="gift" size="xs" class="text-[#E2AD64]" />
                      <strong>{{ t('activityCampaigns.wave2.purchaseGift') }}:</strong>
                      <span>{{ t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierLow'), qty: 2 }) }}</span>
                    </p>
                    <p class="flex items-center gap-1.5 text-gray-700 dark:text-dark-300">
                      <Icon name="users" size="xs" class="text-[#E2AD64]" />
                      <strong>{{ t('activityCampaigns.wave2.groupGift') }}:</strong>
                      <span>{{ t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierLow'), qty: 1 }) }}</span>
                    </p>
                  </div>
                </div>
              </div>

              <!-- Product 3 (198) -->
              <div class="rounded-xl border border-[#B86943]/30 bg-[#B86943]/5 dark:bg-[#B86943]/10 p-4 text-center flex flex-col justify-between">
                <div>
                  <h3 class="text-sm font-bold text-[#B86943] dark:text-[#E9A485]">{{ t('activityCampaigns.wave2.monthlyCard', { price: 198 }) }}</h3>
                  <p class="text-xs text-gray-400 mt-0.5">{{ t('activityCampaigns.wave2.originalPrice') }}: ￥198</p>
                  <hr class="my-3 border-dashed border-[#B86943]/20" />
                  <div class="space-y-2 text-left text-xs">
                    <p class="flex items-center gap-1.5 text-gray-700 dark:text-dark-300">
                      <Icon name="gift" size="xs" class="text-[#B86943]" />
                      <strong>{{ t('activityCampaigns.wave2.purchaseGift') }}:</strong>
                      <span>{{ t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierMedium'), qty: 1 }) }}</span>
                    </p>
                    <p class="flex items-center gap-1.5 text-gray-700 dark:text-dark-300">
                      <Icon name="users" size="xs" class="text-[#B86943]" />
                      <strong>{{ t('activityCampaigns.wave2.groupGift') }}:</strong>
                      <span>{{ t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierLow'), qty: 2 }) }}</span>
                    </p>
                  </div>
                </div>
              </div>
            </div>

            <div class="mt-5 text-center">
              <RouterLink
                to="/group-buy"
                class="btn btn-primary px-8 py-3 rounded-xl font-semibold shadow-lg text-white"
                data-testid="d11-go-groupbuy"
              >
                <Icon name="users" size="sm" class="mr-1.5 inline-block" />
                {{ t('activityCampaigns.wave2.buyNow') }}
              </RouterLink>
            </div>
          </section>

          <!-- User purchases progress list -->
          <section class="festival-task card p-4 sm:p-5 space-y-4" data-testid="d11-wave2-progress">
            <div class="festival-task-label">
              <span>02</span>
              {{ t('activityCampaigns.wave2.progressTitle') }}
              <button
                class="ml-auto inline-flex items-center gap-1 text-xs text-primary-600 dark:text-primary-400 focus:outline-none"
                :disabled="currentStateObj.loading || busy"
                @click="fetchSingleState(waveTwoID)"
                data-testid="d11-refresh-w2"
              >
                <Icon name="refresh" size="xs" :class="currentStateObj.loading ? 'animate-spin' : ''" />
                {{ t('activityCampaigns.refresh') }}
              </button>
            </div>

            <h2 class="text-sm font-semibold text-gray-700 dark:text-dark-300 flex items-center gap-2">
              <Icon name="chart" size="sm" class="text-[#E2AD64]" />
              {{ t('activityCampaigns.wave2.progressTitle') }}
            </h2>

            <div v-if="!state?.purchases || state.purchases.length === 0" class="py-8 text-center text-gray-400 text-sm" data-testid="d11-no-purchases">
              <Icon name="inbox" size="xl" class="mx-auto mb-2 text-gray-300 dark:text-dark-600" />
              {{ t('activityCampaigns.wave2.noPurchases') }}
            </div>

            <div v-else class="space-y-4">
              <div
                v-for="purchase in state?.purchases?.slice(0, 3) || []"
                :key="purchase.card_id"
                class="border border-gray-200 dark:border-dark-700 rounded-xl p-4 bg-gray-50/40 dark:bg-dark-900/30 space-y-3"
                :data-testid="`d11-purchase-${purchase.card_id}`"
              >
                <div class="flex flex-wrap items-center justify-between gap-2 border-b border-dashed border-gray-200 dark:border-dark-700 pb-2">
                  <div>
                    <h4 class="text-sm font-bold text-gray-900 dark:text-white">{{ purchase.product_name }}</h4>
                    <p class="text-[10px] text-gray-400 space-x-3 mt-0.5">
                      <span>{{ t('activityCampaigns.wave2.orderId', { id: purchase.order_id }) }}</span>
                      <span v-if="purchase.card_code">{{ t('activityCampaigns.wave2.cardCode', { code: purchase.card_code }) }}</span>
                    </p>
                  </div>
                  <span class="text-[11px] text-gray-400 font-medium">
                    {{ t('activityCampaigns.wave2.paidAt', { time: formatCn(purchase.paid_at) }) }}
                  </span>
                </div>

                <!-- Perforated tickets for gifts -->
                <ul class="space-y-2">
                  <!-- Purchase task -->
                  <li
                    class="festival-reward-ticket festival-reward-ticket-group flex items-center justify-between gap-3 rounded-xl border p-3"
                    :class="getRewardStatus(purchase.purchase_task).delivered
                      ? 'border-emerald-200 bg-emerald-50/30 dark:border-emerald-800'
                      : getRewardStatus(purchase.purchase_task).earned
                        ? 'border-primary-300 bg-primary-50/30'
                        : 'border-gray-200 dark:border-dark-700'"
                    :data-testid="`d11-purchase-gift-${purchase.card_id}`"
                  >
                    <div class="festival-ticket-content flex items-center gap-3">
                      <div
                        class="festival-ticket-icon flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full"
                        :class="getRewardStatus(purchase.purchase_task).delivered
                          ? 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30'
                          : getRewardStatus(purchase.purchase_task).earned
                            ? 'bg-primary-100 text-primary-600'
                            : 'bg-gray-100 text-gray-400 dark:bg-dark-800'"
                      >
                        <Icon name="gift" size="sm" />
                      </div>
                      <div class="festival-ticket-copy min-w-0">
                        <p class="text-xs font-bold text-gray-800 dark:text-dark-200">{{ t('activityCampaigns.wave2.personalGift') }}</p>
                        <p class="text-[10px] text-gray-400 mt-0.5">
                          {{ purchase.price_cny === 198 ? t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierMedium'), qty: 1 }) : purchase.price_cny === 128 ? t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierLow'), qty: 2 }) : t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierLow'), qty: 1 }) }}
                        </p>
                      </div>
                    </div>

                    <div class="festival-ticket-status flex items-center">
                      <span
                        v-if="!purchase.eligible"
                        class="text-xs font-medium text-red-500 bg-red-50 dark:bg-red-950/20 px-2 py-1 rounded"
                      >
                        {{ t('activityCampaigns.wave2.ineligible') }}
                      </span>
                      <span
                        v-else-if="getRewardStatus(purchase.purchase_task).delivered"
                        class="inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2.5 py-1 text-[11px] font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                      >
                        <Icon name="check" size="xs" />
                        {{ t('activityCampaigns.rewards.claimed') }}
                      </span>
                      <button
                        v-else-if="getRewardStatus(purchase.purchase_task).earned"
                        class="btn btn-primary btn-xs px-2.5 py-1"
                        :disabled="busy"
                        @click="doClaim(purchase.purchase_task)"
                        :data-testid="`d11-claim-purchase-${purchase.order_id}`"
                      >
                        {{ claimingTask === purchase.purchase_task ? t('activityCampaigns.rewards.claiming') : t('activityCampaigns.rewards.claim') }}
                      </button>
                      <span
                        v-else
                        class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-[11px] font-semibold text-gray-500 dark:bg-dark-800"
                      >
                        <Icon name="lock" size="xs" />
                        {{ t('activityCampaigns.rewards.locked') }}
                      </span>
                    </div>
                  </li>

                  <!-- Group Buy/Team task -->
                  <li
                    v-if="purchase.team_task"
                    class="festival-reward-ticket festival-reward-ticket-group flex items-center justify-between gap-3 rounded-xl border p-3"
                    :class="getRewardStatus(purchase.team_task).delivered
                      ? 'border-emerald-200 bg-emerald-50/30 dark:border-emerald-800'
                      : getRewardStatus(purchase.team_task).earned
                        ? 'border-primary-300 bg-primary-50/30'
                        : 'border-gray-200 dark:border-dark-700'"
                    :data-testid="`d11-team-gift-${purchase.card_id}`"
                  >
                    <div class="festival-ticket-content flex items-center gap-3">
                      <div
                        class="festival-ticket-icon flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full"
                        :class="getRewardStatus(purchase.team_task).delivered
                          ? 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30'
                          : getRewardStatus(purchase.team_task).earned
                            ? 'bg-primary-100 text-primary-600'
                            : 'bg-gray-100 text-gray-400 dark:bg-dark-800'"
                      >
                        <Icon name="users" size="sm" />
                      </div>
                      <div class="festival-ticket-copy min-w-0">
                        <p class="text-xs font-bold text-gray-800 dark:text-dark-200">
                          {{ t('activityCampaigns.wave2.teamBonus', { code: purchase.team_code, members: purchase.team_members }) }}
                        </p>
                        <p class="text-[10px] text-gray-400 mt-0.5">
                          {{ purchase.price_cny === 198 ? t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierLow'), qty: 2 }) : t('activityCampaigns.wave2.cardQty', { tier: t('activityCampaigns.wave2.tierLow'), qty: 1 }) }}
                          ·
                          <span
                            :class="purchase.team_qualified ? 'text-emerald-600 dark:text-emerald-400 font-semibold' : 'text-amber-500 font-medium'"
                          >
                            {{ purchase.team_qualified ? t('activityCampaigns.wave2.teamQualified') : t('activityCampaigns.wave2.teamRecruiting') }}
                          </span>
                        </p>
                      </div>
                    </div>

                    <div class="festival-ticket-status flex items-center">
                      <span
                        v-if="!purchase.eligible"
                        class="text-xs font-medium text-red-500 bg-red-50 px-2 py-1 rounded"
                      >
                        {{ t('activityCampaigns.wave2.ineligible') }}
                      </span>
                      <span
                        v-else-if="getRewardStatus(purchase.team_task).delivered"
                        class="inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2.5 py-1 text-[11px] font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                      >
                        <Icon name="check" size="xs" />
                        {{ t('activityCampaigns.rewards.claimed') }}
                      </span>
                      <button
                        v-else-if="getRewardStatus(purchase.team_task).earned"
                        class="btn btn-primary btn-xs px-2.5 py-1"
                        :disabled="busy"
                        @click="doClaim(purchase.team_task)"
                        :data-testid="`d11-claim-team-${purchase.order_id}`"
                      >
                        {{ claimingTask === purchase.team_task ? t('activityCampaigns.rewards.claiming') : t('activityCampaigns.rewards.claim') }}
                      </button>
                      <span
                        v-else
                        class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-[11px] font-semibold text-gray-500 dark:bg-dark-800"
                      >
                        <Icon name="lock" size="xs" />
                        {{ t('activityCampaigns.rewards.locked') }}
                      </span>
                    </div>
                  </li>
                </ul>
              </div>
            </div>

            <p class="text-xs text-gray-400 dark:text-dark-500 mt-2 italic leading-relaxed">
              {{ t('activityCampaigns.wave2.note') }}
            </p>
          </section>
        </template>

        <!-- ======================================================== -->
        <!-- WAVE 3: 寻鲸寻宝                                         -->
        <!-- ======================================================== -->
        <template v-else-if="selectedWaveId === waveThreeID">
          <!-- Found progress + Milestone Rewards -->
          <section class="festival-progress card p-4 sm:p-5" data-testid="d11-wave3-progress">
            <span class="festival-section-tag">{{ t('activityCampaigns.festival.passport') }}</span>
            <div class="flex items-center justify-between gap-3">
              <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-dark-300">
                <Icon name="chart" size="sm" class="text-primary-500" />
                {{ t('activityCampaigns.wave3.foundCount', { count: foundTreasureCount }) }}
              </h2>
            </div>

            <!-- Progress Bar -->
            <div class="mt-3 h-2.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700 relative">
              <div
                class="h-full rounded-full bg-gradient-primary transition-all duration-500"
                :style="{ width: `${(foundTreasureCount / 5) * 100}%` }"
              ></div>
            </div>

            <!-- Milestone Perforated Tickets -->
            <ul class="mt-5 space-y-2.5">
              <!-- Threshold 3 whales found -->
              <li
                class="festival-reward-ticket festival-reward-ticket-group flex items-center justify-between gap-3 rounded-xl border p-3 transition-all"
                :class="getRewardStatus('treasure_three').delivered
                  ? 'border-emerald-200 bg-emerald-50/60 dark:border-emerald-800 dark:bg-emerald-900/10'
                  : getRewardStatus('treasure_three').earned
                    ? 'border-primary-300 bg-primary-50/60 dark:border-primary-700'
                    : 'border-gray-200 dark:border-dark-600'"
                data-testid="d11-reward-treasure-three"
              >
                <div class="festival-ticket-content flex items-center gap-3">
                  <div
                    class="festival-ticket-icon flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg"
                    :class="getRewardStatus('treasure_three').delivered
                      ? 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30'
                      : getRewardStatus('treasure_three').earned
                        ? 'bg-primary-100 text-primary-600'
                        : 'bg-gray-100 text-gray-400 dark:bg-dark-700'"
                  >
                    <Icon name="badge" size="md" />
                  </div>
                  <div class="festival-ticket-copy min-w-0 flex-1">
                    <p class="text-sm font-bold text-gray-900 dark:text-white">{{ t('activityCampaigns.wave3.taskTreasureThree') }}</p>
                    <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('activityCampaigns.wave3.rewardCard') }}</p>
                  </div>
                </div>

                <div class="festival-ticket-status flex items-center">
                  <span
                    v-if="getRewardStatus('treasure_three').delivered"
                    class="inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                  >
                    <Icon name="check" size="xs" />
                    {{ t('activityCampaigns.rewards.claimed') }}
                  </span>
                  <button
                    v-else-if="getRewardStatus('treasure_three').earned"
                    class="btn btn-primary btn-sm flex-shrink-0"
                    :disabled="busy || phase !== 'active'"
                    @click="doClaim('treasure_three')"
                    data-testid="d11-claim-treasure-three"
                  >
                    {{ claimingTask === 'treasure_three' ? t('activityCampaigns.rewards.claiming') : t('activityCampaigns.rewards.claim') }}
                  </button>
                  <span
                    v-else
                    class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-semibold text-gray-500 dark:bg-dark-700"
                  >
                    <Icon name="lock" size="xs" />
                    {{ t('activityCampaigns.rewards.locked') }}
                  </span>
                </div>
              </li>

              <!-- Threshold 5 whales found -->
              <li
                class="festival-reward-ticket festival-reward-ticket-group flex items-center justify-between gap-3 rounded-xl border p-3 transition-all"
                :class="getRewardStatus('treasure_five').delivered
                  ? 'border-emerald-200 bg-emerald-50/60 dark:border-emerald-800 dark:bg-emerald-900/10'
                  : getRewardStatus('treasure_five').earned
                    ? 'border-primary-300 bg-primary-50/60 dark:border-primary-700'
                    : 'border-gray-200 dark:border-dark-600'"
                data-testid="d11-reward-treasure-five"
              >
                <div class="festival-ticket-content flex items-center gap-3">
                  <div
                    class="festival-ticket-icon flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg"
                    :class="getRewardStatus('treasure_five').delivered
                      ? 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30'
                      : getRewardStatus('treasure_five').earned
                        ? 'bg-primary-100 text-primary-600'
                        : 'bg-gray-100 text-gray-400 dark:bg-dark-700'"
                  >
                    <Icon name="trophy" size="md" />
                  </div>
                  <div class="festival-ticket-copy min-w-0 flex-1">
                    <p class="text-sm font-bold text-gray-900 dark:text-white">{{ t('activityCampaigns.wave3.taskTreasureFive') }}</p>
                    <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('activityCampaigns.wave3.rewardCard') }}</p>
                  </div>
                </div>

                <div class="festival-ticket-status flex items-center">
                  <span
                    v-if="getRewardStatus('treasure_five').delivered"
                    class="inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                  >
                    <Icon name="check" size="xs" />
                    {{ t('activityCampaigns.rewards.claimed') }}
                  </span>
                  <button
                    v-else-if="getRewardStatus('treasure_five').earned"
                    class="btn btn-primary btn-sm flex-shrink-0"
                    :disabled="busy || phase !== 'active'"
                    @click="doClaim('treasure_five')"
                    data-testid="d11-claim-treasure-five"
                  >
                    {{ claimingTask === 'treasure_five' ? t('activityCampaigns.rewards.claiming') : t('activityCampaigns.rewards.claim') }}
                  </button>
                  <span
                    v-else
                    class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-semibold text-gray-500 dark:bg-dark-700"
                  >
                    <Icon name="lock" size="xs" />
                    {{ t('activityCampaigns.rewards.locked') }}
                  </span>
                </div>
              </li>
            </ul>
          </section>

          <!-- Progressive Clues Cards Grid -->
          <section class="festival-task card p-4 sm:p-5" data-testid="d11-wave3-clues">
            <div class="festival-task-label"><span>01</span>{{ t('activityCampaigns.wave3.clueTitle') }}<Icon name="sparkles" size="sm" /></div>
            <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-dark-300">
              <Icon name="lightbulb" size="sm" class="text-primary-500" />
              {{ t('activityCampaigns.wave3.clueTitle') }}
            </h2>

            <div class="mt-4 grid gap-4 sm:grid-cols-2">
              <div
                v-for="(treasure, ti) in state?.treasures || []"
                :key="treasure.id"
                class="rounded-xl border p-4 bg-gray-50/40 dark:bg-dark-800/30 flex flex-col justify-between"
                :class="treasure.found
                  ? 'border-emerald-200 dark:border-emerald-800'
                  : treasure.revealed
                    ? 'border-primary-200 dark:border-primary-900'
                    : 'border-gray-200 dark:border-dark-700 opacity-60'"
                :data-testid="`d11-clue-${treasure.id}`"
              >
                <div>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-xs font-bold text-[#E2AD64]">🐳 WHALE #{{ ti + 1 }}</span>
                    <span
                      v-if="treasure.found"
                      class="text-[10px] font-semibold text-emerald-600 bg-emerald-50 dark:bg-emerald-950/20 px-2 py-0.5 rounded-full"
                    >
                      {{ t('activityCampaigns.wave3.statusFound') }}
                    </span>
                    <span
                      v-else-if="!treasure.revealed"
                      class="text-[10px] font-semibold text-gray-400 bg-gray-100 dark:bg-dark-800 px-2 py-0.5 rounded-full"
                    >
                      {{ t('activityCampaigns.wave3.clueLocked') }}
                    </span>
                  </div>

                  <p class="text-xs text-gray-700 dark:text-dark-300 font-medium mt-2 leading-relaxed">
                    <span v-if="treasure.revealed" data-testid="clue-text">
                      {{ t('activityCampaigns.wave3.clueRevealed', { clue: treasure.clue }) }}
                    </span>
                    <span v-else class="text-gray-400 italic">
                      {{ t('activityCampaigns.wave3.mysteryClue') }}
                    </span>
                  </p>

                  <!-- Fun line if found -->
                  <p v-if="treasure.found" class="text-[11px] text-gray-500 italic mt-2 bg-white dark:bg-dark-900/40 p-2 rounded border border-gray-100 dark:border-dark-800 leading-relaxed">
                    {{ t('activityCampaigns.shared.foundMessage', { line: treasure.line }) }}
                  </p>
                </div>

                <p v-if="!treasure.found && treasure.revealed" class="mt-4 text-right text-xs text-primary-600">
                  {{ t('activityCampaigns.wave3.statusNotFound') }}
                </p>
              </div>
            </div>
          </section>
        </template>

        <p v-if="claimError" class="mt-3 text-xs text-red-600 dark:text-red-400 text-center" data-testid="d11-claim-error-global">
          {{ claimError }}
        </p>

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
  waveOneID,
  waveTwoID,
  waveThreeID
} from '@/api/activityCampaigns'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const userId = computed(() => (authStore.user ? String(authStore.user.id) : ''))

const waves = [waveOneID, waveTwoID, waveThreeID]
const selectedWaveId = ref(waveOneID)

const waveStates = ref<Record<string, { state: ActivityCampaignState | null; error: string; loading: boolean }>>({
  [waveOneID]: { state: null, error: '', loading: false },
  [waveTwoID]: { state: null, error: '', loading: false },
  [waveThreeID]: { state: null, error: '', loading: false }
})

const loading = ref(false)
const clockOffsetMs = ref(0)
const now = ref(Date.now())
const serverNow = computed(() => now.value + clockOffsetMs.value)

let clockTimer: ReturnType<typeof setInterval> | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null
let disposed = false
let identityGeneration = 0
const requestGenerations: Record<string, number> = {}

const currentStateObj = computed(() => waveStates.value[selectedWaveId.value])
const state = computed(() => currentStateObj.value?.state ?? null)
const campaign = computed(() => state.value?.campaign ?? null)

type Phase = 'disabled' | 'upcoming' | 'active' | 'ended'

function getPhase(s: ActivityCampaignState | null, live = false): Phase {
  if (!s || !s.campaign) return 'disabled'
  if (s.status === 'disabled' || !s.campaign.enabled) return 'disabled'
  const start = Date.parse(s.campaign.starts_at)
  const end = Date.parse(s.campaign.ends_at)
  const sNow = live ? (now.value + clockOffsetMs.value) : Date.parse(s.server_time)
  if (Number.isFinite(start) && sNow < start) return 'upcoming'
  if (Number.isFinite(end) && sNow >= end) return 'ended'
  return 'active'
}

const phase = computed<Phase>(() => getPhase(state.value, true))
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

// Wave 1 Check-in progress
const checkinDates = computed(() => state.value?.checkin_dates ?? [])
const checkinCount = computed(() => Math.min(checkinDates.value.length, 3))
const checkedToday = computed(() => {
  const day = new Date(serverNow.value + 8 * 3600_000).toISOString().slice(0, 10)
  return checkinDates.value.includes(day)
})

const checkinPending = ref(false)
const checkinError = ref('')
const canCheckin = computed(() => selectedWaveId.value === waveOneID && phase.value === 'active' && !checkedToday.value && !busy.value)
const checkinButtonLabel = computed(() => {
  if (checkinPending.value) return t('activityCampaigns.checkin.pending')
  if (checkedToday.value) return t('activityCampaigns.checkin.checkedToday')
  if (phase.value !== 'active') return t('activityCampaigns.checkin.notActive')
  return t('activityCampaigns.checkin.button')
})

// Wave 1 Quiz
const answers = ref<Record<string, number>>({})
const quizPending = ref(false)
const quizError = ref('')
const quizCompleted = computed(() => state.value?.quiz_completed ?? false)
let loadedRevision: number | null = null

// Rewards / Claims
const claimingTask = ref<ActivityTask | null>(null)
const claimError = ref('')
const busy = computed(() => loading.value || currentStateObj.value.loading || checkinPending.value || quizPending.value || claimingTask.value !== null)

const REWARD_META: Record<string, { labelKey: string; icon: 'calendar' | 'badge' | 'questionCircle' }> = {
  first_checkin: { labelKey: 'activityCampaigns.rewards.taskFirstCheckin', icon: 'calendar' },
  three_checkins: { labelKey: 'activityCampaigns.rewards.taskThreeCheckins', icon: 'badge' },
  quiz: { labelKey: 'activityCampaigns.rewards.taskQuiz', icon: 'questionCircle' },
}

const rewardByTask = computed(() => {
  const map = new Map<string, { earned: boolean; delivered: boolean }>()
  for (const reward of state.value?.rewards ?? []) {
    map.set(reward.task, { earned: true, delivered: reward.delivered_at !== null })
  }
  return map
})

const rewardCards = computed(() =>
  ['first_checkin', 'three_checkins', 'quiz'].map((task) => {
    const status = rewardByTask.value.get(task)
    const meta = REWARD_META[task] || { labelKey: '', icon: 'gift' }
    return {
      task,
      label: t(meta.labelKey),
      icon: meta.icon,
      earned: status?.earned ?? false,
      delivered: status?.delivered ?? false,
    }
  }),
)

const earnedCount = computed(() => (state.value?.rewards ?? []).filter(r => ['first_checkin', 'three_checkins', 'quiz'].includes(r.task)).length)

// Wave 2 & 3 custom reward status retriever
const getRewardStatus = (task: string) => {
  const reward = state.value?.rewards?.find((r) => r.task === task)
  return {
    earned: !!reward,
    delivered: reward ? reward.delivered_at !== null : false
  }
}

const foundTreasureCount = computed(() => state.value?.treasures?.filter(t => t.found).length ?? 0)

const applyServerTime = (serverTimeStr: string) => {
  const serverTime = Date.parse(serverTimeStr)
  if (Number.isFinite(serverTime)) {
    clockOffsetMs.value = serverTime - Date.now()
    now.value = Date.now()
  }
}

const applyState = (id: string, next: ActivityCampaignState) => {
  if (next.campaign.id !== id) throw new Error(t('activityCampaigns.loadFailed'))
  if (next.server_time) {
    applyServerTime(next.server_time)
  }
  if (id === waveOneID && loadedRevision !== null && next.campaign.revision !== loadedRevision) {
    answers.value = {}
  }
  if (id === waveOneID) {
    loadedRevision = next.campaign.revision
  }
  waveStates.value[id].state = next
}

const fetchSingleState = async (id: string, silent = false) => {
  const targetObj = waveStates.value[id]
  if (!targetObj || !userId.value) return
  const uid = userId.value
  const generation = identityGeneration
  const request = requestGenerations[id] = (requestGenerations[id] ?? 0) + 1
  const current = () => !disposed && generation === identityGeneration && uid === userId.value && requestGenerations[id] === request
  if (!silent) targetObj.loading = true
  targetObj.error = ''
  try {
    const data = await activityCampaignsAPI.get(id)
    if (!current()) return
    applyState(id, data)
  } catch (error) {
    if (!current()) return
    targetObj.error = extractApiErrorMessage(error, t('activityCampaigns.loadFailed'))
  } finally {
    if (current() && !silent) {
      targetObj.loading = false
    }
  }
}

const fetchAllStates = async () => {
  const generation = identityGeneration
  loading.value = true
  await Promise.allSettled(waves.map(id => fetchSingleState(id)))
  if (disposed || generation !== identityGeneration) return
  loading.value = false

  // Determine selected wave
  let saved: string | null = null
  try { saved = localStorage.getItem(`d11_focused_wave:${userId.value}`) } catch { /* Storage is optional. */ }
  if (saved && waves.includes(saved)) {
    selectedWaveId.value = saved
  } else {
    // Default selected wave: first active by server_time/status else next upcoming else most recent ended
    const active = waves.find(id => getPhase(waveStates.value[id].state, false) === 'active')
    if (active) {
      selectedWaveId.value = active
    } else {
      const upcoming = waves.find(id => getPhase(waveStates.value[id].state, false) === 'upcoming')
      if (upcoming) {
        selectedWaveId.value = upcoming
      } else {
        const ended = [...waves].reverse().find(id => getPhase(waveStates.value[id].state, false) === 'ended')
        if (ended) {
          selectedWaveId.value = ended
        } else {
          selectedWaveId.value = waveOneID
        }
      }
    }
  }
}

const selectWave = (id: string) => {
  if (busy.value || !waves.includes(id)) return
  selectedWaveId.value = id
  try { localStorage.setItem(`d11_focused_wave:${userId.value}`, id) } catch { /* Storage is optional. */ }
  claimError.value = ''
  // Ensure we fetch state on selection
  void fetchSingleState(id)
}

const doCheckin = async () => {
  if (!canCheckin.value) return
  const uid = userId.value
  const generation = identityGeneration
  const wave = selectedWaveId.value
  requestGenerations[wave] = (requestGenerations[wave] ?? 0) + 1
  checkinPending.value = true
  checkinError.value = ''
  try {
    const data = await activityCampaignsAPI.checkin(wave)
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    applyState(wave, data)
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
  if (!s || selectedWaveId.value !== waveOneID || busy.value || quizCompleted.value || phase.value !== 'active') return
  const questions = s.campaign.questions
  if (questions.some((q) => answers.value[q.id] === undefined)) {
    quizError.value = t('activityCampaigns.quiz.incomplete')
    return
  }
  const uid = userId.value
  const generation = identityGeneration
  const wave = selectedWaveId.value
  requestGenerations[wave] = (requestGenerations[wave] ?? 0) + 1
  const revision = s.campaign.revision
  quizPending.value = true
  quizError.value = ''
  try {
    const data = await activityCampaignsAPI.quiz(revision, { ...answers.value }, wave)
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    applyState(wave, data)
  } catch (error) {
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    const status = (error as { status?: number })?.status ?? 0
    if (status === 422) {
      quizError.value = t('activityCampaigns.quiz.wrong')
    } else if (status === 409) {
      quizError.value = t('activityCampaigns.quiz.configChanged')
      answers.value = {}
      loadedRevision = null
      void fetchSingleState(wave)
    } else {
      quizError.value = extractApiErrorMessage(error, t('activityCampaigns.quiz.failed'))
    }
  } finally {
    if (!disposed && identityGeneration === generation && userId.value === uid) quizPending.value = false
  }
}

const doClaim = async (task: string) => {
  if (busy.value || phase.value === 'disabled' || phase.value === 'upcoming' ||
      (selectedWaveId.value === waveThreeID && phase.value !== 'active')) return
  const status = getRewardStatus(task)
  if (!status.earned || status.delivered) return
  const uid = userId.value
  const generation = identityGeneration
  const wave = selectedWaveId.value
  requestGenerations[wave] = (requestGenerations[wave] ?? 0) + 1
  claimingTask.value = task
  claimError.value = ''
  try {
    const data = await activityCampaignsAPI.claim(task, wave)
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    applyState(wave, data)
  } catch (error) {
    if (disposed || identityGeneration !== generation || userId.value !== uid) return
    claimError.value = extractApiErrorMessage(error, t('activityCampaigns.rewards.claimFailed'))
  } finally {
    if (!disposed && identityGeneration === generation && userId.value === uid) claimingTask.value = null
  }
}

// Background polling for Wave 2
function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    if (selectedWaveId.value === waveTwoID && userId.value && !busy.value) {
      void fetchSingleState(waveTwoID, true) // Silent polling
    }
  }, 30000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// Watchers
watch(selectedWaveId, (next) => {
  if (next === waveTwoID) {
    startPolling()
  } else {
    stopPolling()
  }
})

// Account switcher
watch(userId, (next, previous) => {
  if (previous === undefined || next === previous) return
  identityGeneration++
  answers.value = {}
  loadedRevision = null
  checkinError.value = ''
  quizError.value = ''
  claimError.value = ''
  checkinPending.value = false
  quizPending.value = false
  claimingTask.value = null
  clockOffsetMs.value = 0
  loading.value = false

  // Clear all states
  for (const id of waves) {
    waveStates.value[id].state = null
    waveStates.value[id].error = ''
    waveStates.value[id].loading = false
  }

  if (next) {
    void fetchAllStates()
  }
}, { flush: 'sync' })

onMounted(() => {
  void fetchAllStates()
  clockTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)

  if (selectedWaveId.value === waveTwoID) {
    startPolling()
  }
})

onUnmounted(() => {
  disposed = true
  identityGeneration++
  stopPolling()
  if (clockTimer !== null) {
    clearInterval(clockTimer)
    clockTimer = null
  }
})
</script>

<style scoped src="./festival.css"></style>
