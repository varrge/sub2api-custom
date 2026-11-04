<template>
  <button
    ref="triggerRef" type="button" data-testid="header-voting"
    class="inline-flex shrink-0 items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm font-medium text-gray-600 transition hover:bg-gray-100 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-primary-400"
    :class="show ? 'bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-400' : ''"
    :title="t('votingEntry.title')" :aria-label="t('votingEntry.title')"
    aria-haspopup="dialog" :aria-expanded="show" :aria-controls="panelID"
    @click="show ? close(false) : open()"
  >
    <Icon name="checkCircle" size="md" />
    <span class="hidden sm:inline">{{ t('votingEntry.title') }}</span>
  </button>
  <Teleport to="body">
    <div
      v-if="show" :id="panelID" ref="panelRef" role="dialog" aria-modal="false"
      :aria-label="t('votingEntry.title')" tabindex="-1" data-testid="voting-popover"
      class="fixed z-[70] flex w-[min(400px,calc(100vw_-_1rem))] flex-col overflow-hidden rounded-2xl border border-primary-200/60 bg-white shadow-2xl shadow-primary-900/10 focus:outline-none dark:border-primary-400/20 dark:bg-dark-900 dark:shadow-black/40"
      :style="panelStyle"
    >
      <div class="flex shrink-0 items-center justify-between gap-2 border-b border-gray-100 bg-gradient-to-r from-primary-50 to-white px-4 py-2.5 dark:border-dark-700 dark:from-primary-950/30 dark:to-dark-900">
        <h2 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
          <Icon name="checkCircle" size="sm" class="text-primary-500" />{{ t('votingEntry.title') }}
        </h2>
        <div class="flex gap-1">
          <button type="button" class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700" :aria-label="t('voting.board.refresh')" :disabled="loading || detailLoading || submitting" @click="refresh">
            <Icon name="refresh" size="sm" :class="loading || detailLoading ? 'animate-spin' : ''" />
          </button>
          <button type="button" class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700" :aria-label="t('common.close')" data-testid="voting-close" @click="close(true)">
            <Icon name="x" size="sm" />
          </button>
        </div>
      </div>
      <VotingBoard
        v-model:selection="selection" :polls="polls" :poll="poll" :loading="loading"
        :detail-loading="detailLoading" :submitting="submitting" :error="error" :page="page" :pages="pages"
        @select="selectPoll" @submit="submit" @refresh="refresh" @page-change="changePage"
      />
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, useId, watch, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import VotingBoard from './VotingBoard.vue'
import { votingAPI, type Poll, type PollSummary } from '@/api/voting'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const app = useAppStore()
const auth = useAuthStore()
const route = useRoute()
const panelID = `voting-${useId()}`
const show = ref(false)
const triggerRef = ref<HTMLButtonElement>()
const panelRef = ref<HTMLElement>()
const panelStyle = ref<CSSProperties>({})
const polls = ref<PollSummary[]>([])
const poll = ref<Poll | null>(null)
const selection = ref<number[]>([])
const loading = ref(false)
const detailLoading = ref(false)
const submitting = ref(false)
const error = ref('')
const page = ref(1)
const pages = ref(0)
let listController: AbortController | undefined
let detailController: AbortController | undefined
let disposed = false
let sessionToken: string | null = null

function sameUser() {
  try {
    return !!auth.user?.id && JSON.parse(localStorage.getItem('auth_user') || 'null')?.id === auth.user.id
  } catch { return false }
}
function sameSession() {
  return sameUser() && !!sessionToken && localStorage.getItem('auth_token') === sessionToken
}
function invalidateSession() {
  close(false)
  polls.value = []
  poll.value = null
  selection.value = []
  error.value = ''
}

function message(e: unknown) {
  return (e as { message?: string })?.message || t('votingEntry.failed')
}
async function selectPoll(id: number) {
  if (!show.value || submitting.value || disposed) return
  detailController?.abort()
  const controller = detailController = new AbortController()
  poll.value = null
  selection.value = []
  detailLoading.value = true
  error.value = ''
  try {
    const result = await votingAPI.get(id, controller.signal)
    if (!controller.signal.aborted) {
      poll.value = result
      selection.value = [...result.my_option_ids]
    }
  } catch (e) {
    if (!controller.signal.aborted) error.value = message(e)
  } finally {
    if (!controller.signal.aborted) detailLoading.value = false
  }
}
async function refresh() {
  if (!show.value || disposed || submitting.value) return
  stopReads()
  const controller = listController = new AbortController()
  const previousID = poll.value?.id
  loading.value = true
  error.value = ''
  try {
    const result = await votingAPI.list(page.value, controller.signal)
    if (controller.signal.aborted) return
    polls.value = result.items
    pages.value = result.pages
    if (!result.items.length && page.value > 1) {
      page.value = Math.max(1, result.pages)
      await refresh()
      return
    }
    const selected = result.items.find(p => p.id === previousID) || result.items[0]
    if (selected) await selectPoll(selected.id)
    else { poll.value = null; selection.value = [] }
  } catch (e) {
    if (!controller.signal.aborted) error.value = message(e)
  } finally {
    if (!controller.signal.aborted) loading.value = false
  }
}
function changePage(value: number) {
  if (submitting.value || loading.value || value < 1 || value > pages.value) return
  page.value = value
  void refresh()
}
async function submit() {
  if (!sameSession()) {
    invalidateSession()
    if (!disposed) app.showWarning(t('votingEntry.sessionChanged'))
    return
  }
  const current = poll.value
  if (!show.value || disposed || loading.value || detailLoading.value || !current || current.has_voted || current.status !== 'open' || submitting.value || selection.value.length === 0) return
  const choices = [...selection.value]
  if (new Set(choices).size !== choices.length || choices.some(id => !current.options.some(o => o.id === id))) return
  submitting.value = true
  error.value = ''
  // Confirmation and result branches replace their buttons; keep keyboard focus in the panel.
  void nextTick(() => { if (show.value) panelRef.value?.focus() })
  let failure = ''
  try {
    await votingAPI.vote(current.id, current.revision, choices)
    // A committed ballot remains locked even if the result refresh fails.
    current.has_voted = true
    current.my_option_ids = choices
    if (!disposed) app.showSuccess(t('votingEntry.submitted'))
  } catch (e) {
    failure = message(e)
  } finally {
    submitting.value = false
  }
  if (!disposed && show.value) {
    await refresh()
    if (failure) error.value = failure
  }
}
function updatePosition() {
  const trigger = triggerRef.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const width = Math.min(400, window.innerWidth - 16)
  const left = Math.min(Math.max(rect.right - width, 8), Math.max(8, window.innerWidth - width - 8))
  const top = Math.min(Math.max(rect.bottom + 8, 8), Math.max(8, window.innerHeight - 240))
  panelStyle.value = { top: `${top}px`, left: `${left}px`, maxHeight: `min(600px, calc(100dvh - ${top + 8}px))` }
}
async function open() {
  sessionToken = localStorage.getItem('auth_token')
  if (!sameSession()) {
    app.showWarning(t('votingEntry.sessionChanged'))
    return
  }
  show.value = true
  updatePosition()
  document.addEventListener('pointerdown', onPointerDown)
  document.addEventListener('keydown', onKeydown)
  window.addEventListener('resize', updatePosition)
  window.addEventListener('scroll', updatePosition, true)
  window.addEventListener('storage', onStorage)
  // A POST may still be in flight after closing; retain its locked state.
  if (!submitting.value) void refresh()
  await nextTick()
  panelRef.value?.focus()
}
function stopReads() {
  listController?.abort()
  detailController?.abort()
  loading.value = false
  detailLoading.value = false
}
function removeListeners() {
  document.removeEventListener('pointerdown', onPointerDown)
  document.removeEventListener('keydown', onKeydown)
  window.removeEventListener('resize', updatePosition)
  window.removeEventListener('scroll', updatePosition, true)
  window.removeEventListener('storage', onStorage)
}
function close(restoreFocus: boolean) {
  show.value = false
  stopReads()
  removeListeners()
  if (restoreFocus) triggerRef.value?.focus()
}
function onPointerDown(event: PointerEvent) {
  const target = event.target as Node | null
  if (target && !panelRef.value?.contains(target) && !triggerRef.value?.contains(target)) close(false)
}
function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') { event.preventDefault(); close(true) }
}
function onStorage(event: StorageEvent) {
  if ((!event.key || event.key === 'auth_user' || event.key === 'auth_token') && !sameSession()) invalidateSession()
}
watch(() => auth.user?.id, invalidateSession, { flush: 'sync' })
watch(() => route.fullPath, () => close(false))
watch(() => app.cachedPublicSettings?.voting_enabled, enabled => {
  if (enabled === false) close(false)
})
onBeforeUnmount(() => { disposed = true; stopReads(); removeListeners() })
</script>
