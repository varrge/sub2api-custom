<template>
  <div class="flex gap-1 overflow-x-auto border-b border-gray-200 dark:border-dark-700" role="tablist" :aria-label="label" @keydown="onKeydown">
    <button
      v-for="tab in tabs"
      :id="`${idPrefix}-tab-${tab.key}`"
      :key="tab.key"
      ref="buttons"
      type="button"
      role="tab"
      class="-mb-px flex shrink-0 items-center gap-2 border-b-2 px-4 py-2.5 text-sm font-medium focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500"
      :class="tab.key === modelValue
        ? 'border-primary-600 text-gray-900 dark:border-primary-400 dark:text-white'
        : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-gray-200'"
      :aria-selected="tab.key === modelValue"
      :aria-controls="`${idPrefix}-panel-${tab.key}`"
      :tabindex="tab.key === modelValue ? 0 : -1"
      :data-testid="`${idPrefix}-tab-${tab.key}`"
      @click="emit('update:modelValue', tab.key)"
    >
      <span>{{ tab.label }}</span>
      <span v-if="tab.count" class="badge badge-warning tabular-nums">{{ tab.count }}</span>
      <span v-if="tab.dot" class="h-2 w-2 rounded-full bg-amber-500" role="img" :aria-label="tab.dot"></span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref } from 'vue'

export interface InvitationTab {
  key: string
  label: string
  /** Shown as a number badge when greater than zero. */
  count?: number
  /** Accessible text for an attention dot, e.g. "Unsaved changes". */
  dot?: string
}

const props = defineProps<{ modelValue: string; tabs: InvitationTab[]; label: string; idPrefix: string }>()
const emit = defineEmits<{ 'update:modelValue': [key: string] }>()
const buttons = ref<HTMLButtonElement[]>([])

async function onKeydown(event: KeyboardEvent) {
  const current = props.tabs.findIndex((tab) => tab.key === props.modelValue)
  let next = current
  if (event.key === 'ArrowRight') next = (current + 1) % props.tabs.length
  else if (event.key === 'ArrowLeft') next = (current - 1 + props.tabs.length) % props.tabs.length
  else if (event.key === 'Home') next = 0
  else if (event.key === 'End') next = props.tabs.length - 1
  else return
  event.preventDefault()
  emit('update:modelValue', props.tabs[next].key)
  await nextTick()
  buttons.value[next]?.focus()
}
</script>
