<template>
  <div ref="containerRef" class="relative inline-block select-none">
    <button
      ref="triggerRef"
      data-testid="shared-credit-trigger"
      type="button"
      class="group h-10 w-[94px] sm:w-[145px] px-2 sm:px-2.5 rounded-xl border border-amber-700/20 dark:border-amber-500/20 bg-amber-50/70 hover:bg-amber-100/80 dark:bg-amber-950/25 dark:hover:bg-amber-900/40 text-amber-900 dark:text-amber-100 transition-colors focus:outline-none focus:ring-2 focus:ring-amber-500/40 flex items-center gap-1.5 sm:gap-2 text-left"
      :aria-expanded="isOpen"
      :aria-label="`${t('sharedCredit.header.title')} ${formattedDisplay}`"
      :aria-controls="popoverId"
      @click="togglePopover"
      @keydown.esc.prevent="closePopover(true)"
    >
      <svg class="hidden sm:block w-4 h-4 shrink-0 text-amber-700 dark:text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <rect x="2" y="5" width="20" height="14" rx="2" stroke-width="1.8" />
        <line x1="2" y1="10" x2="22" y2="10" stroke-width="1.8" />
      </svg>
      <div class="min-w-0 flex-1 leading-none py-0.5">
        <div class="text-[10px] sm:text-[11px] text-amber-800/80 dark:text-amber-300/80 truncate font-medium flex items-center justify-between">
          <span>{{ t('sharedCredit.header.title') }}</span>
          <span v-if="statusBadgeText" class="text-[9px] text-amber-700 dark:text-amber-400 font-normal sm:hidden">{{ statusBadgeText }}</span>
        </div>
        <div class="text-xs sm:text-sm font-semibold tracking-tight text-amber-950 dark:text-amber-100 truncate" :title="formattedExact">
          {{ formattedDisplay }}
        </div>
      </div>
    </button>

    <Teleport to="body">
    <div
      v-if="isOpen"
      :id="popoverId"
      ref="popoverRef"
      role="region"
      :aria-labelledby="`${popoverId}-title`"
      :style="popoverPosition"
      class="fixed w-80 max-w-[calc(100vw-24px)] overflow-y-auto z-50 rounded-2xl border border-amber-800/15 dark:border-amber-500/20 bg-white/95 dark:bg-stone-900/95 backdrop-blur-md shadow-xl p-3.5 sm:p-4 text-xs text-stone-700 dark:text-stone-300"
      @keydown.esc.prevent="closePopover(true)"
    >
      <div class="flex items-center justify-between pb-2 border-b border-amber-900/10 dark:border-amber-400/10">
        <div class="min-w-0 pr-2">
          <div :id="`${popoverId}-title`" class="text-sm font-bold text-stone-900 dark:text-stone-100 break-words">
            {{ summary?.name || t('sharedCredit.header.title') }}
          </div>
          <div class="text-[11px] text-amber-800 dark:text-amber-400 mt-0.5 flex items-center gap-1.5">
            <span>{{ t(`sharedCredit.activity.status.${summary?.status || 'active'}`) }}</span>
          </div>
        </div>
        <button ref="closeRef" type="button" :aria-label="t('sharedCredit.header.close')" class="p-1 rounded-lg text-stone-400 hover:text-stone-600 dark:hover:text-stone-200 hover:bg-stone-100 dark:hover:bg-stone-800" @click="closePopover(true)">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
        </button>
      </div>

      <p v-if="!failed && summary?.needs_recharge" role="status" class="mt-3 rounded-lg bg-red-50 p-2 text-red-700 dark:bg-red-950/40 dark:text-red-300">{{ t('sharedCredit.activity.balanceNotice') }}</p>

      <div v-if="failed" class="my-3 p-2 rounded-lg bg-red-50 dark:bg-red-950/40 text-red-600 dark:text-red-400 text-center flex items-center justify-between">
        <span>{{ t('sharedCredit.header.loadFailed') }}</span>
        <button type="button" class="underline text-xs" @click="$emit('refresh')">{{ t('sharedCredit.header.retry') }}</button>
      </div>

      <div v-if="!failed" class="space-y-3 mt-3">
        <div>
          <div class="flex justify-between text-[11px] text-stone-500 dark:text-stone-400 mb-1">
            <span>{{ t('sharedCredit.header.remainingQuota') }}</span>
            <span :title="formattedExact">{{ formattedDisplay }} / ${{ summary?.total_usd || '0.00' }}</span>
          </div>
          <div class="w-full h-1.5 rounded-full bg-stone-200 dark:bg-stone-800 overflow-hidden">
            <div class="h-full bg-amber-600 dark:bg-amber-500 rounded-full transition-all duration-300" :style="{ width: `${progressPct}%` }" />
          </div>
        </div>

        <div class="grid grid-cols-2 gap-2 text-[11px] bg-amber-50/50 dark:bg-stone-800/40 p-2 rounded-lg border border-amber-900/5 dark:border-amber-500/10">
          <div><span class="text-stone-400">{{ t('sharedCredit.header.startAt') }}:</span> <div>{{ summary?.starts_at ? formatDate(summary.starts_at) : t('sharedCredit.header.immediate') }}</div></div>
          <div><span class="text-stone-400">{{ t('sharedCredit.header.endAt') }}:</span> <div>{{ formatDate(summary?.ends_at) }}</div></div>
        </div>

        <div>
          <div class="text-[11px] text-stone-500 dark:text-stone-400 mb-1.5">{{ t('sharedCredit.header.availableGroups') }}</div>
          <div v-if="summary?.groups?.length" class="flex flex-wrap gap-1 max-h-20 overflow-y-auto">
            <span v-for="grp in summary.groups" :key="grp.id" class="max-w-full break-words px-1.5 py-0.5 rounded bg-amber-100/70 dark:bg-amber-900/30 text-amber-900 dark:text-amber-200 text-[10px]">{{ grp.name }}</span>
          </div>
          <div v-else class="text-[11px] text-stone-400 italic">{{ t('sharedCredit.header.emptyGroups') }}</div>
        </div>

        <div class="text-[10px] text-stone-400 dark:text-stone-500 leading-normal space-y-0.5 pt-1 border-t border-stone-100 dark:border-stone-800">
          <div>• {{ t('sharedCredit.header.poolDisclaimer') }}</div>
          <div>• {{ t('sharedCredit.header.currencyNotice') }}</div>
          <div>• {{ t('sharedCredit.header.accessNotice') }}</div>
          <div>• {{ t('sharedCredit.header.refreshNotice') }}</div>
        </div>
      </div>

      <div class="mt-3 pt-2.5 border-t border-amber-900/10 dark:border-amber-400/10 flex justify-end">
        <button
          type="button"
          class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-lg bg-amber-100/70 hover:bg-amber-200/70 dark:bg-amber-900/40 dark:hover:bg-amber-800/50 text-amber-900 dark:text-amber-100 disabled:opacity-50"
          :disabled="loading"
          @click="$emit('refresh')"
        >
          <svg class="w-3 h-3 animate-spin" v-if="loading" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"/></svg>
          <svg v-else class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
          <span>{{ t('sharedCredit.header.refresh') }}</span>
        </button>
      </div>
    </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { onClickOutside, useEventListener } from '@vueuse/core'
import type { UserSharedCreditSummary } from '@/api/sharedCredit'

const props = defineProps<{
  summary?: UserSharedCreditSummary | null
  loading?: boolean
  failed?: boolean
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
}>()

const { t, locale } = useI18n()
const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const popoverRef = ref<HTMLElement | null>(null)
const closeRef = ref<HTMLButtonElement | null>(null)
const popoverId = `shared-credit-${useId()}`
const popoverPosition = ref<Record<string, string>>({})

onClickOutside(containerRef, () => closePopover(), { ignore: [popoverRef] })
useEventListener(window, 'resize', positionPopover)
useEventListener(window, 'keydown', (event) => {
  if (isOpen.value && event.key === 'Escape') closePopover(true)
})

function positionPopover() {
  if (!isOpen.value || !triggerRef.value) return
  const rect = triggerRef.value.getBoundingClientRect()
  const top = rect.bottom + 8
  popoverPosition.value = {
    top: `${top}px`,
    right: `${Math.max(12, Math.min(window.innerWidth - rect.right, window.innerWidth - 332))}px`,
    maxHeight: `calc(100dvh - ${top + 12}px)`
  }
}

const statusBadgeText = computed(() => {
  if (props.summary?.status === 'scheduled') return t('sharedCredit.header.scheduled')
  if (props.summary?.status === 'exhausted') return t('sharedCredit.header.exhausted')
  return ''
})

const formattedDisplay = computed(() => {
  if (props.failed) return '$--'
  if (!props.summary || props.summary.remaining_usd == null) return '$--'
  const val = Number(props.summary.remaining_usd)
  if (!Number.isFinite(val)) return '$--'
  if (val > 0 && val < 0.01) return '<$0.01'
  return `$${val.toFixed(2)}`
})

const formattedExact = computed(() => {
  if (props.failed) return ''
  return props.summary?.remaining_usd ? `$${props.summary.remaining_usd} USD` : ''
})

const progressPct = computed(() => {
  const rem = Number.parseFloat(props.summary?.remaining_usd || '0')
  const tot = Number.parseFloat(props.summary?.total_usd || '0')
  if (tot <= 0 || Number.isNaN(rem) || Number.isNaN(tot)) return 0
  return Math.max(0, Math.min(100, (rem / tot) * 100))
})

function formatDate(dt?: string | null): string {
  if (!dt) return '—'
  try {
    return new Date(dt).toLocaleString(locale.value, { year: 'numeric', month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return dt
  }
}

async function togglePopover() {
  isOpen.value = !isOpen.value
  if (isOpen.value) {
    positionPopover()
    emit('refresh')
    await nextTick()
    closeRef.value?.focus()
  }
}

function closePopover(focusTrigger = false) {
  isOpen.value = false
  if (focusTrigger) triggerRef.value?.focus()
}
</script>
