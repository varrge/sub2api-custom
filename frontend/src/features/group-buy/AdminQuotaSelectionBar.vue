<template>
  <div
    class="gb-strip mb-3 flex flex-wrap items-center gap-3 p-3 text-sm text-gray-700 dark:text-gray-300"
  >
    <span class="font-medium text-gray-900 dark:text-white">
      {{ t('groupBuy.quotaSelectedCount', { count }) }}
    </span>
    <span class="text-xs text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.quotaSelectionPageHint') }}
    </span>
    <div class="ml-auto flex items-center gap-2">
      <button
        type="button"
        class="btn btn-secondary btn-sm whitespace-nowrap"
        :disabled="disabled || count === 0"
        @click="emit('clear')"
      >
        {{ t('groupBuy.clearSelection') }}
      </button>
      <button
        type="button"
        class="btn btn-primary btn-sm whitespace-nowrap"
        :disabled="disabled || count === 0"
        @click="emit('adjust')"
      >
        {{ t('groupBuy.batchAdjustQuota') }}
      </button>
      <button
        type="button"
        class="btn btn-secondary btn-sm whitespace-nowrap"
        :disabled="disabled || count === 0"
        @click="emit('reset')"
      >
        {{ t('groupBuy.batchResetUsage') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import './glass.css'

defineProps<{ count: number; disabled?: boolean }>()
const emit = defineEmits<{ adjust: []; reset: []; clear: [] }>()
const { t } = useI18n()
</script>
