<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="document"
        class="fixed inset-0 z-[70] flex bg-black/50 backdrop-blur-sm sm:items-center sm:justify-center sm:p-6"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="dialogTitleId"
        @click.self="close"
      >
        <div
          ref="panelRef"
          class="flex h-full w-full flex-col bg-white dark:bg-dark-800 sm:h-auto sm:max-h-[85vh] sm:max-w-3xl sm:rounded-2xl sm:border sm:border-gray-200 sm:shadow-2xl dark:sm:border-dark-700"
        >
          <div
            class="flex flex-shrink-0 items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700 sm:px-6 sm:py-4"
          >
            <h3
              :id="dialogTitleId"
              class="min-w-0 text-lg font-semibold text-gray-900 [overflow-wrap:anywhere] dark:text-white"
            >
              {{ document.title }}
            </h3>
            <button
              type="button"
              class="-mr-2 shrink-0 rounded-xl p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:text-dark-500 dark:hover:bg-dark-700 dark:hover:text-dark-300"
              :aria-label="t('common.close')"
              @click="close"
            >
              <Icon name="x" size="md" />
            </button>
          </div>
          <div
            ref="bodyRef"
            class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-3 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500/30 sm:px-6 sm:py-4"
            tabindex="0"
            role="region"
            :aria-label="t('groupBuy.ruleContent')"
            @scroll.passive="onScroll"
          >
            <p
              ref="contentRef"
              class="text-sm leading-7 whitespace-pre-wrap text-gray-700 [overflow-wrap:anywhere] dark:text-gray-300"
            >{{ document.content }}</p>
          </div>
          <div
            class="flex flex-shrink-0 flex-wrap items-center justify-end gap-3 border-t border-gray-200 px-4 py-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] dark:border-dark-700 sm:px-6 sm:py-4"
          >
            <p v-if="!reachedBottom" class="mr-auto text-xs text-gray-500 dark:text-gray-400">
              {{ t('groupBuy.ruleScrollHint') }}
            </p>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="!reachedBottom || confirming"
              @click="confirm"
            >
              {{ confirming ? t('common.processing') : t('groupBuy.ruleReadDone') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script lang="ts">
let rulesDialogCounter = 0
</script>

<script setup lang="ts">
import { nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { PublishedMonthCardRule } from '@/types/monthCardRules'

const props = withDefaults(
  defineProps<{ document: PublishedMonthCardRule | null; confirming?: boolean }>(),
  { confirming: false }
)
const emit = defineEmits<{ close: []; confirm: [document: PublishedMonthCardRule] }>()
const { t } = useI18n()

const dialogTitleId = `rules-read-title-${++rulesDialogCounter}`
const panelRef = ref<HTMLElement | null>(null)
const bodyRef = ref<HTMLElement | null>(null)
const contentRef = ref<HTMLElement | null>(null)
const reachedBottom = ref(false)
let previousActiveElement: HTMLElement | null = null
let resizeObserver: ResizeObserver | null = null

function checkBottom() {
  const el = bodyRef.value
  if (!el || !props.document) return
  if (el.scrollHeight - el.clientHeight <= 2 || el.scrollTop + el.clientHeight >= el.scrollHeight - 8) {
    reachedBottom.value = true
  }
}

function onScroll() {
  checkBottom()
}

function close() {
  emit('close')
}

function confirm() {
  if (!props.document || !reachedBottom.value || props.confirming) return
  emit('confirm', props.document)
}

function onKeydown(event: KeyboardEvent) {
  if (!props.document) return
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
    return
  }
  if (event.key !== 'Tab') return
  const panel = panelRef.value
  if (!panel) return
  const focusable = Array.from(
    panel.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    )
  )
  if (!focusable.length) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

function observeGeometry() {
  resizeObserver?.disconnect()
  if (typeof ResizeObserver === 'undefined') return
  resizeObserver = new ResizeObserver(() => checkBottom())
  if (bodyRef.value) resizeObserver.observe(bodyRef.value)
  if (contentRef.value) resizeObserver.observe(contentRef.value)
}

watch(
  () => [props.document?.id ?? '', props.document?.version ?? '', props.document ? 1 : 0],
  async ([id, , open]) => {
    reachedBottom.value = false
    document.removeEventListener('keydown', onKeydown)
    resizeObserver?.disconnect()
    document.body?.classList.remove('modal-open')
    if (open && id) {
      document.addEventListener('keydown', onKeydown)
      document.body?.classList.add('modal-open')
      previousActiveElement = document.activeElement as HTMLElement
      await nextTick()
      if (bodyRef.value) bodyRef.value.scrollTop = 0
      observeGeometry()
      checkBottom()
      const first = panelRef.value?.querySelector<HTMLElement>('button')
      first?.focus()
    } else {
      if (previousActiveElement && typeof previousActiveElement.focus === 'function') {
        previousActiveElement.focus()
      }
      previousActiveElement = null
    }
  },
  { immediate: true, flush: 'post' }
)

onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown)
  resizeObserver?.disconnect()
  document.body?.classList.remove('modal-open')
})
</script>
