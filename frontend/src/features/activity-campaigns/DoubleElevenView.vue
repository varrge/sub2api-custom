<template>
  <AppLayout>
    <div class="festival-page mx-auto w-full min-w-0 max-w-5xl space-y-6">
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
        <!-- The illustrated scene is decorative; live event details stay text. -->
        <header class="festival-hero" data-testid="d11-hero">
          <div class="festival-marquee" aria-hidden="true">
            <span>✦ DOUBLE ELEVEN ✦</span><span>{{ t('activityCampaigns.festival.marquee') }}</span><span>✦ 11.11 ✦</span>
          </div>
          <div class="festival-hero-main">
            <div class="festival-copy">
              <div class="festival-eyebrow">
                <span class="festival-wave">{{ t('activityCampaigns.festival.wave') }}</span>
                <span class="festival-phase" data-testid="d11-phase"><i></i>{{ phaseLabel }}</span>
              </div>
              <p class="festival-overline">{{ t('activityCampaigns.festival.overline') }}</p>
              <h1>{{ campaign.title }}</h1>
              <p v-if="campaign.subtitle" class="festival-subtitle">{{ campaign.subtitle }}</p>
              <p v-if="campaign.description" class="festival-description">{{ campaign.description }}</p>
              <div class="festival-perks">
                <span><Icon name="gift" size="sm" />{{ t('activityCampaigns.festival.free') }}</span>
                <span><Icon name="sparkles" size="sm" />{{ t('activityCampaigns.festival.threeCards') }}</span>
              </div>
            </div>
            <div class="festival-art" aria-hidden="true">
              <svg viewBox="0 0 430 350" fill="none" focusable="false">
                <defs>
                  <linearGradient id="d11-gold" x1="90" y1="20" x2="300" y2="300" gradientUnits="userSpaceOnUse"><stop stop-color="#FFE5A6"/><stop offset="1" stop-color="#D9904B"/></linearGradient>
                  <linearGradient id="d11-rust" x1="150" y1="190" x2="290" y2="330" gradientUnits="userSpaceOnUse"><stop stop-color="#BC5B43"/><stop offset="1" stop-color="#873E30"/></linearGradient>
                </defs>
                <circle cx="228" cy="181" r="147" fill="#F2D8AD" fill-opacity=".6"/>
                <circle cx="228" cy="181" r="133" stroke="#C79865" stroke-opacity=".35" stroke-dasharray="3 9"/>
                <text x="26" y="105" fill="#B86943" fill-opacity=".17" font-size="101" font-weight="900" font-family="Arial, sans-serif" letter-spacing="-8">11.11</text>
                <ellipse cx="234" cy="327" rx="160" ry="13" fill="#7E482A" fill-opacity=".1"/>
                <!-- Ribbon curls and cut-paper confetti -->
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
                <!-- Left present -->
                <g transform="rotate(-9 87 283)">
                  <rect x="38" y="264" width="96" height="61" rx="8" fill="#B75B43"/>
                  <rect x="32" y="250" width="108" height="25" rx="5" fill="#C76F52"/>
                  <path d="M81 250v75" stroke="#F1C983" stroke-width="17"/>
                  <path d="M89 250c-50 0-46-36-24-25 9 5 16 16 24 25Zm0 0c42-6 44-35 23-27-8 4-15 16-23 27Z" fill="#F1C983" stroke="#E2AD64" stroke-width="3"/>
                </g>
                <!-- Lucky cow mascot -->
                <g class="festival-cow">
                  <path d="M158 221c-17 31-19 81-8 100h142c12-35 1-78-15-100Z" fill="url(#d11-rust)"/>
                  <path d="m201 222 21 23 20-23" stroke="#F9D698" stroke-width="4"/>
                  <circle cx="222" cy="260" r="3" fill="#F9D698"/>
                  <path d="M157 121c-15-8-25-21-21-38 17 5 26 15 31 29M275 121c15-8 25-21 21-38-17 5-26 15-31 29" fill="#F9E1AA" stroke="#9C714B" stroke-width="3"/>
                  <path d="M154 139c-44-25-58 0-28 20 12 9 21 6 31-2M278 139c44-25 58 0 28 20-12 9-21 6-31-2" fill="#F9F1DF" stroke="#9C714B" stroke-width="3"/>
                  <path d="M146 146c-21-12-27-3-11 6M286 146c21-12 27-3 11 6" stroke="#D59A83" stroke-width="8" stroke-linecap="round"/>
                  <path d="M148 155c0-44 27-62 69-62s69 18 69 62v24c0 40-27 61-69 61s-69-21-69-61z" fill="#FFFAEA" stroke="#9C714B" stroke-width="3"/>
                  <path d="M155 126c11-24 35-33 57-33 0 19-10 24-23 26-13 1-12 26-32 28z" fill="#986A4E"/>
                  <path d="M252 116c19 6 25 24 22 39-3 12-27 17-34 4-7-13 1-33 12-43" fill="#A77A59"/>
                  <ellipse cx="187" cy="164" rx="6" ry="9" fill="#4C3529"/><ellipse cx="246" cy="164" rx="6" ry="9" fill="#4C3529"/>
                  <circle cx="189" cy="161" r="2" fill="white"/><circle cx="248" cy="161" r="2" fill="white"/>
                  <ellipse cx="168" cy="184" rx="12" ry="7" fill="#EFB89D"/><ellipse cx="266" cy="184" rx="12" ry="7" fill="#EFB89D"/>
                  <rect x="178" y="181" width="77" height="44" rx="22" fill="#F0D1A6"/>
                  <ellipse cx="198" cy="197" rx="3" ry="4" fill="#936448"/><ellipse cx="234" cy="197" rx="3" ry="4" fill="#936448"/>
                  <path d="M210 210q7 6 14 0" stroke="#936448" stroke-width="2.5" stroke-linecap="round"/>
                  <!-- Party crown -->
                  <path d="m190 97-7-27 19 9 14-24 13 24 20-10-8 29z" fill="url(#d11-gold)" stroke="#C28C45" stroke-width="2.5"/>
                  <circle cx="216" cy="84" r="4" fill="#B4523C"/>
                  <!-- Three reward tickets -->
                  <g transform="rotate(-13 221 276)"><rect x="163" y="249" width="110" height="64" rx="9" fill="#DCA861" stroke="#B77A3C" stroke-width="2"/></g>
                  <g transform="rotate(8 221 276)"><rect x="168" y="246" width="110" height="64" rx="9" fill="#AF543E" stroke="#8A402E" stroke-width="2"/></g>
                  <path d="M177 245h87a8 8 0 0 1 8 8v12a10 10 0 0 0 0 20v12a8 8 0 0 1-8 8h-87a8 8 0 0 1-8-8v-12a10 10 0 0 0 0-20v-12a8 8 0 0 1 8-8Z" fill="#FFF0C8" stroke="#CA954F" stroke-width="2"/>
                  <path d="M246 253v44" stroke="#D4A46C" stroke-dasharray="3 4"/>
                  <path d="m207 255 4 10 11 1-8 7 3 11-10-6-9 6 2-11-8-7 11-1z" fill="#BD7950"/>
                  <path d="M193 293h30" stroke="#D1A273" stroke-width="3" stroke-linecap="round"/>
                  <text x="253" y="281" font-size="19" fill="#98613C" font-family="Arial, sans-serif" font-weight="800">1</text>
                  <ellipse cx="156" cy="273" rx="13" ry="19" fill="#FFF4DD" stroke="#9C714B" stroke-width="2.5" transform="rotate(-19 156 273)"/>
                  <ellipse cx="285" cy="273" rx="13" ry="19" fill="#FFF4DD" stroke="#9C714B" stroke-width="2.5" transform="rotate(19 285 273)"/>
                </g>
                <!-- Right present -->
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
            <p class="festival-dates"><Icon name="calendar" size="sm" />{{ t('activityCampaigns.window', { start: formatCn(campaign.starts_at), end: formatCn(campaign.ends_at) }) }}</p>
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
                :class="checkinCount >= day
                  ? 'festival-stamp-done' : ''"
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
                  class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg"
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

<style scoped src="./festival.css"></style>
