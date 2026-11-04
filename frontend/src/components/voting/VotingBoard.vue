<template>
  <div class="flex flex-col min-h-0 flex-1">
    <!-- Compact Poll Switcher -->
    <div
      v-if="polls.length > 1 || pages > 1 || (!poll && polls.length > 0)"
      class="border-b border-slate-200/80 bg-slate-50/70 p-2.5 dark:border-dark-700/80 dark:bg-dark-800/50 shrink-0"
    >
      <div class="flex items-center gap-2">
        <label for="poll-selector" class="sr-only">{{ t('voting.board.title') }}</label>
        <select
          id="poll-selector"
          class="input block min-w-0 w-full flex-1 rounded-lg border-slate-200 bg-white py-1 px-2.5 text-xs text-slate-800 shadow-sm focus:border-purple-500 focus:ring-purple-500 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-100"
          :value="poll?.id ?? ''"
          :disabled="loading || detailLoading || submitting || isConfirming"
          @change="handleSelectChange"
        >
          <option v-if="!poll" value="" disabled>{{ t('voting.board.selectToView') }}</option>
          <option
            v-for="item in polls"
            :key="item.id"
            :value="item.id"
          >
            {{ item.title }} {{ item.has_voted ? `(${t('voting.board.votedBadge')})` : '' }}
          </option>
        </select>

        <!-- Pagination Controls -->
        <div v-if="pages > 1" class="flex items-center gap-1 shrink-0">
          <button
            type="button"
            class="btn btn-secondary btn-sm p-1 text-slate-600 dark:text-dark-300"
            :disabled="page <= 1 || loading || detailLoading || submitting || isConfirming"
            :aria-label="t('voting.board.prevPage')"
            @click="emit('page-change', page - 1)"
          >
            <Icon name="chevronLeft" size="sm" class="h-3.5 w-3.5" />
          </button>
          <span class="text-[11px] tabular-nums text-slate-500 dark:text-dark-400 px-0.5">
            {{ page }}/{{ pages }}
          </span>
          <button
            type="button"
            class="btn btn-secondary btn-sm p-1 text-slate-600 dark:text-dark-300"
            :disabled="page >= pages || loading || detailLoading || submitting || isConfirming"
            :aria-label="t('voting.board.nextPage')"
            @click="emit('page-change', page + 1)"
          >
            <Icon name="chevronRight" size="sm" class="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </div>

    <!-- Error Banner -->
    <div
      v-if="error"
      role="alert"
      class="mx-3 mt-2 flex items-center justify-between gap-2 rounded-lg border border-red-200 bg-red-50 p-2.5 text-xs text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300 shrink-0"
    >
      <div class="flex items-center gap-1.5 min-w-0">
        <Icon name="exclamationCircle" size="sm" class="shrink-0 text-red-500 dark:text-red-400" />
        <span class="break-words">{{ error }}</span>
      </div>
      <button
        type="button"
        class="shrink-0 font-medium underline hover:opacity-80 disabled:opacity-50"
        :disabled="loading || submitting"
        @click="emit('refresh')"
      >
        {{ t('voting.board.retry') }}
      </button>
    </div>

    <!-- Scrollable Main Content -->
    <div class="flex-1 overflow-y-auto overscroll-contain min-h-0 p-3 space-y-3" :aria-busy="loading || detailLoading">
      <!-- Loading List State -->
      <div
        v-if="loading && !polls.length"
        class="flex flex-col items-center justify-center py-8 text-center text-xs text-slate-500 dark:text-dark-400"
      >
        <Icon name="refresh" size="md" class="animate-spin text-purple-600 dark:text-purple-400" />
        <p class="mt-2">{{ t('voting.board.loadingList') }}</p>
      </div>

      <!-- Global Error State (No polls loaded) -->
      <div
        v-else-if="error && !polls.length"
        class="flex flex-col items-center justify-center py-8 text-center"
      >
        <Icon name="exclamationCircle" size="md" class="text-red-500 dark:text-red-400" />
        <p class="mt-2 text-xs font-medium text-slate-800 dark:text-dark-100">{{ error }}</p>
        <button
          type="button"
          :disabled="submitting"
          class="btn btn-secondary btn-sm mt-3 inline-flex items-center gap-1 text-xs py-1 px-2.5"
          @click="emit('refresh')"
        >
          <Icon name="refresh" size="sm" />
          {{ t('voting.board.retry') }}
        </button>
      </div>

      <!-- Empty Polls State -->
      <div
        v-else-if="!polls.length"
        class="flex flex-col items-center justify-center py-8 text-center"
      >
        <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-slate-100 text-slate-400 dark:bg-dark-700 dark:text-dark-400">
          <Icon name="chartBar" size="md" />
        </div>
        <p class="mt-2 text-xs font-medium text-slate-800 dark:text-dark-100">{{ t('voting.board.noPolls') }}</p>
        <p class="mt-0.5 text-[11px] text-slate-500 dark:text-dark-400">{{ t('voting.board.noPollsHint') }}</p>
      </div>

      <!-- Detail Loading State -->
      <div
        v-else-if="detailLoading"
        class="flex flex-col items-center justify-center py-8 text-center text-xs text-slate-500 dark:text-dark-400"
      >
        <Icon name="refresh" size="md" class="animate-spin text-purple-600 dark:text-purple-400" />
        <p class="mt-2">{{ t('voting.board.loadingDetail') }}</p>
      </div>

      <!-- Empty Detail Selection Prompt -->
      <div
        v-else-if="!poll"
        class="flex flex-col items-center justify-center py-8 text-center"
      >
        <Icon name="infoCircle" size="md" class="text-slate-400 dark:text-dark-400" />
        <p class="mt-2 text-xs font-medium text-slate-800 dark:text-dark-100">{{ t('voting.board.selectToView') }}</p>
        <p class="mt-0.5 text-[11px] text-slate-500 dark:text-dark-400">{{ t('voting.board.selectToViewHint') }}</p>
      </div>

      <!-- Active Poll Details -->
      <div v-else class="space-y-3">
        <!-- Optional Cover Image -->
        <div
          v-if="poll.image"
          class="overflow-hidden rounded-lg border border-slate-200/80 bg-slate-100 dark:border-dark-700/80 dark:bg-dark-900"
        >
          <img :src="poll.image" :alt="poll.title" class="max-h-28 w-full object-contain mx-auto" />
        </div>

        <!-- Title & Meta Header -->
        <div class="space-y-1.5">
          <div class="flex flex-wrap items-center gap-1.5">
            <span :class="['inline-flex items-center rounded-md px-1.5 py-0.5 text-[11px] font-medium ring-1 ring-inset', getStatusClass(poll.status)]">
              {{ getStatusText(poll.status) }}
            </span>
            <span class="inline-flex items-center gap-1 text-[11px] text-slate-500 dark:text-dark-400">
              <Icon name="users" size="sm" class="h-3 w-3" />
              {{ t('voting.board.voterCount', { count: poll.voter_count }) }}
            </span>
            <span
              v-if="poll.has_voted"
              class="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-1.5 py-0.5 text-[11px] font-medium text-emerald-700 ring-1 ring-inset ring-emerald-600/20 dark:bg-emerald-950/30 dark:text-emerald-400 dark:ring-emerald-500/30"
            >
              <Icon name="check" size="sm" class="h-3 w-3" />
              {{ t('voting.board.votedBadge') }}
            </span>
          </div>

          <h3 class="text-sm font-bold text-slate-900 dark:text-dark-50 break-words">
            {{ poll.title }}
          </h3>

          <p
            v-if="poll.description"
            class="whitespace-pre-line text-xs leading-relaxed text-slate-600 dark:text-dark-300 break-words"
          >
            {{ poll.description }}
          </p>
        </div>

        <!-- Immutable Has-Voted Notice -->
        <div
          v-if="poll.has_voted"
          class="flex items-center gap-2 rounded-lg border border-emerald-200/60 bg-emerald-50/70 p-2.5 text-xs text-emerald-800 dark:border-emerald-800/30 dark:bg-emerald-950/20 dark:text-emerald-300"
        >
          <Icon name="checkCircle" size="sm" class="shrink-0 text-emerald-600 dark:text-emerald-400" />
          <span>{{ t('voting.board.hasVotedNotice') }}</span>
        </div>

        <!-- Closed Poll Notice -->
        <div
          v-else-if="poll.status === 'closed'"
          class="flex items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 p-2.5 text-xs text-slate-700 dark:border-dark-700 dark:bg-dark-900/50 dark:text-dark-300"
        >
          <Icon name="infoCircle" size="sm" class="shrink-0 text-slate-500 dark:text-dark-400" />
          <span>{{ t('voting.board.closedNotice') }}</span>
        </div>

        <!-- Options List -->
        <div class="space-y-2 pt-0.5">
          <label
            v-for="opt in poll.options"
            :key="opt.id"
            :class="[
              'group relative flex items-start gap-2.5 overflow-hidden rounded-xl border p-2.5 transition-colors',
              'focus-within:ring-2 focus-within:ring-purple-500/50 focus-within:ring-offset-1 dark:focus-within:ring-offset-dark-900',
              isOptionSelected(opt.id)
                ? 'border-purple-500/80 bg-purple-50/40 ring-1 ring-purple-500/30 dark:border-purple-500/60 dark:bg-purple-950/20'
                : 'border-slate-200/80 bg-white hover:border-slate-300 dark:border-dark-700 dark:bg-dark-800/80 dark:hover:border-dark-600',
              isInputDisabled ? 'cursor-default' : 'cursor-pointer'
            ]"
          >
            <!-- Native sr-only Checkbox for Accessibility -->
            <input
              type="checkbox"
              class="sr-only"
              :checked="isOptionSelected(opt.id)"
              :disabled="isInputDisabled"
              :aria-label="opt.title"
              :data-testid="`vote-option-${opt.id}`"
              @change="toggleOption(opt.id)"
            />

            <!-- Results Progress Bar Fill Layer -->
            <div
              v-if="shouldShowResults"
              class="absolute inset-y-0 left-0 bg-purple-500/10 pointer-events-none transition-all duration-500 dark:bg-purple-400/15"
              :style="{ width: `${calcPercentage(opt.votes, poll.voter_count)}%` }"
            />

            <div class="relative flex items-start gap-2.5 w-full min-w-0">
              <!-- Visual Checkbox Indicator -->
              <div class="pt-0.5 shrink-0">
                <div
                  :class="[
                    'flex h-4 w-4 items-center justify-center rounded border transition-colors',
                    isOptionSelected(opt.id)
                      ? 'border-purple-600 bg-purple-600 text-white dark:border-purple-500 dark:bg-purple-500'
                      : 'border-slate-300 bg-white group-hover:border-slate-400 dark:border-dark-600 dark:bg-dark-800',
                    isInputDisabled ? 'opacity-70' : ''
                  ]"
                >
                  <Icon v-if="isOptionSelected(opt.id)" name="check" size="sm" class="h-3 w-3 stroke-[2.5]" />
                </div>
              </div>

              <!-- Optional Option Thumbnail -->
              <img
                v-if="opt.image"
                :src="opt.image"
                :alt="opt.title"
                class="h-14 w-14 shrink-0 rounded-lg border border-slate-200 bg-slate-100 object-contain dark:border-dark-700 dark:bg-dark-900"
              />

              <!-- Option Content & Results Details -->
              <div class="min-w-0 flex-1">
                <div class="flex items-start justify-between gap-1.5">
                  <span class="min-w-0 text-xs font-medium text-slate-800 dark:text-dark-100 [overflow-wrap:anywhere]">
                    {{ opt.title }}
                  </span>
                  <span
                    v-if="poll.my_option_ids?.includes(opt.id)"
                    class="shrink-0 rounded bg-emerald-100/80 px-1.5 py-0.5 text-[10px] font-medium text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300"
                  >
                    {{ t('voting.board.myChoices') }}
                  </span>
                </div>

                <div
                  v-if="shouldShowResults"
                  class="mt-1.5 flex items-center justify-between text-[11px] text-slate-500 dark:text-dark-400"
                >
                  <span>{{ t('voting.board.votesCount', { count: opt.votes }) }}</span>
                  <span class="font-medium text-slate-700 dark:text-dark-300">
                    {{ t('voting.board.percentage', { pct: calcPercentage(opt.votes, poll.voter_count) }) }}
                  </span>
                </div>
              </div>
            </div>
          </label>
        </div>
      </div>
    </div>

    <!-- Footer: Visible Actions / Submission Confirmation -->
    <div v-if="canVote" class="shrink-0">
      <!-- Submitting Progress State -->
      <div
        v-if="submitting"
        class="flex items-center justify-center gap-2 border-t border-slate-200/80 bg-slate-50/80 px-3 py-2.5 text-xs text-slate-500 dark:border-dark-700/80 dark:bg-dark-800/80 dark:text-dark-400"
      >
        <Icon name="refresh" size="sm" class="animate-spin text-purple-600 dark:text-purple-400" />
        <span>{{ t('voting.board.submitting') }}</span>
      </div>

      <!-- Inline Irreversible Confirmation State -->
      <div
        v-else-if="isConfirming"
        ref="confirmationRef" tabindex="-1" role="status"
        class="border-t border-slate-200/80 bg-slate-50/90 p-3 space-y-2 dark:border-dark-700/80 dark:bg-dark-800/90"
      >
        <div class="space-y-0.5">
          <p class="text-xs font-semibold text-slate-900 dark:text-dark-100">
            {{ t('voting.board.confirmSubmitTitle') }}
          </p>
          <p class="text-xs text-slate-500 dark:text-dark-400">
            {{ t('voting.board.confirmSubmitMessage', { count: selection.length }) }}
          </p>
        </div>
        <div class="flex items-center justify-end gap-2 pt-0.5">
          <button
            type="button"
            class="btn btn-secondary btn-sm text-xs py-1 px-2.5"
            :disabled="submitting"
            @click="cancelConfirmation"
          >
            {{ t('voting.board.cancel') }}
          </button>
          <button
            type="button"
            data-testid="vote-confirm"
            class="btn btn-primary btn-sm text-xs py-1 px-2.5 inline-flex items-center gap-1.5 shadow-sm"
            :disabled="submitting"
            @click="handleConfirmSubmit"
          >
            <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
            <span>{{ submitting ? t('voting.board.submitting') : t('voting.board.confirmSubmit') }}</span>
          </button>
        </div>
      </div>

      <!-- Normal Voting Action Footer -->
      <div
        v-else
        class="flex items-center justify-between gap-2 border-t border-slate-200/80 bg-slate-50/80 px-3 py-2.5 dark:border-dark-700/80 dark:bg-dark-800/80"
      >
        <div class="min-w-0">
          <p class="text-xs font-medium text-slate-700 dark:text-dark-300">
            {{ t('voting.board.selectedCount', { count: selection.length }) }}
          </p>
          <p class="text-[11px] text-amber-600 dark:text-amber-400">
            {{ t('voting.board.finalNotice') }}
          </p>
        </div>
        <button
          type="button"
          data-testid="vote-submit" ref="submitRef"
          class="btn btn-primary btn-sm shrink-0 inline-flex items-center gap-1.5 text-xs py-1.5 px-3 shadow-sm transition-all active:scale-[0.98]"
          :disabled="loading || detailLoading || selection.length === 0"
          @click="isConfirming = true"
        >
          <Icon name="checkCircle" size="sm" />
          <span>{{ t('voting.board.submitVote') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { Poll, PollSummary, PollStatus } from '@/api/voting'

const props = withDefaults(
  defineProps<{
    polls: PollSummary[]
    poll: Poll | null
    loading?: boolean
    detailLoading?: boolean
    submitting?: boolean
    selection?: number[]
    error?: string
    page?: number
    pages?: number
  }>(),
  {
    loading: false,
    detailLoading: false,
    submitting: false,
    selection: () => [],
    error: '',
    page: 1,
    pages: 1
  }
)

const emit = defineEmits<{
  (e: 'select', id: number): void
  (e: 'update:selection', ids: number[]): void
  (e: 'submit'): void
  (e: 'refresh'): void
  (e: 'page-change', page: number): void
}>()

const { t } = useI18n()
const isConfirming = ref(false)
const confirmationRef = ref<HTMLElement>()
const submitRef = ref<HTMLButtonElement>()
watch(isConfirming, async value => {
  if (value) { await nextTick(); confirmationRef.value?.focus() }
})
async function cancelConfirmation() {
  isConfirming.value = false
  await nextTick()
  submitRef.value?.focus()
}

watch(
  () => [props.poll?.id, props.poll?.revision, props.loading, props.submitting],
  () => {
    isConfirming.value = false
  }
)

const canVote = computed(() => {
  if (!props.poll) return false
  return props.poll.status === 'open' && !props.poll.has_voted
})

const isVotingDisabled = computed(() => {
  if (!props.poll) return true
  return props.poll.has_voted || props.poll.status !== 'open' || props.submitting || props.loading || props.detailLoading
})

const isInputDisabled = computed(() => {
  if (!props.poll) return true
  return (
    props.poll.has_voted ||
    props.poll.status !== 'open' ||
    props.loading ||
    props.detailLoading ||
    props.submitting ||
    isConfirming.value
  )
})

const shouldShowResults = computed(() => {
  if (!props.poll) return false
  return props.poll.has_voted || props.poll.status === 'closed'
})

function isOptionSelected(id: number): boolean {
  return props.selection.includes(id)
}

function toggleOption(id: number) {
  if (isInputDisabled.value) return
  const current = [...props.selection]
  const index = current.indexOf(id)
  if (index === -1) {
    current.push(id)
  } else {
    current.splice(index, 1)
  }
  emit('update:selection', current)
}

function handleSelectChange(e: Event) {
  const value = Number((e.target as HTMLSelectElement).value)
  if (value) {
    emit('select', value)
  }
}

function handleConfirmSubmit() {
  if (!isConfirming.value || isVotingDisabled.value || props.selection.length === 0) return
  isConfirming.value = false
  emit('submit')
}

function calcPercentage(votes: number, totalVoters: number): string {
  if (!totalVoters || totalVoters <= 0) return '0.0'
  const pct = Math.min(100, Math.max(0, (votes / totalVoters) * 100))
  return pct.toFixed(1)
}

function getStatusText(status: PollStatus): string {
  switch (status) {
    case 'draft':
      return t('voting.board.statusDraft')
    case 'open':
      return t('voting.board.statusOpen')
    case 'closed':
      return t('voting.board.statusClosed')
    default:
      return status
  }
}

function getStatusClass(status: PollStatus): string {
  switch (status) {
    case 'draft':
      return 'bg-slate-100 text-slate-600 ring-slate-400/20 dark:bg-dark-700 dark:text-dark-300 dark:ring-dark-500/30'
    case 'open':
      return 'bg-emerald-50 text-emerald-700 ring-emerald-600/20 dark:bg-emerald-950/30 dark:text-emerald-400 dark:ring-emerald-500/30'
    case 'closed':
      return 'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-950/30 dark:text-amber-400 dark:ring-amber-500/30'
    default:
      return 'bg-slate-100 text-slate-600 ring-slate-400/20'
  }
}
</script>
