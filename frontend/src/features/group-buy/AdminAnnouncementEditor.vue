<template>
  <section class="gb-panel space-y-4 p-4 sm:p-5">
    <h2 class="font-semibold text-gray-900 dark:text-white">
      {{ t('groupBuy.announcementTitle') }}
    </h2>
    <p class="text-sm text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.announcementHint') }}
    </p>

    <p v-if="error" class="gb-notice gb-notice-error p-3" role="alert">
      {{ error }}
      <button
        type="button"
        class="ml-2 underline"
        :disabled="loading || saving"
        @click="emit('retry')"
      >
        {{ t('groupBuy.retry') }}
      </button>
    </p>
    <p v-else-if="success" class="gb-notice gb-notice-info p-3" role="status">
      {{ success }}
    </p>

    <div v-if="loading" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400" role="status">
      {{ t('common.loading') }}
    </div>
    <template v-else>
      <label class="block text-sm" for="gb-announcement-content">
        {{ t('groupBuy.announcementContent') }}
      </label>
      <textarea
        id="gb-announcement-content"
        :value="modelValue"
        class="input mt-1 w-full text-sm leading-6"
        rows="8"
        maxlength="10000"
        :disabled="disabled || loading || saving"
        :placeholder="t('groupBuy.announcementPlaceholder')"
        :aria-label="t('groupBuy.announcementContent')"
        :aria-invalid="tooLong || undefined"
        aria-describedby="gb-announcement-meta"
        @input="onInput"
      />
      <div
        id="gb-announcement-meta"
        class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400"
      >
        <span>{{ t('groupBuy.announcementDefaultHint') }}</span>
        <span :class="tooLong ? 'font-medium text-red-600 dark:text-red-400' : ''">
          {{ codepointCount }}/{{ MAX_CODEPOINTS }}
        </span>
      </div>
      <p v-if="tooLong" class="gb-notice gb-notice-warn p-3" role="alert">
        {{ t('groupBuy.announcementTooLong', { max: MAX_CODEPOINTS }) }}
      </p>

      <div class="space-y-2">
        <p class="gb-label">{{ t('groupBuy.announcementPreview') }}</p>
        <div class="gb-notice gb-notice-info whitespace-pre-wrap break-words p-4">{{ previewText }}</div>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="disabled || loading || saving"
          @click="emit('reset')"
        >
          {{ t('groupBuy.announcementReset') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="disabled || loading || saving || !dirty || tooLong"
          @click="emit('save')"
        >
          {{ saving ? t('common.processing') : t('groupBuy.announcementSave') }}
        </button>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import './glass.css'

const MAX_CODEPOINTS = 5000

const props = withDefaults(
  defineProps<{
    disabled?: boolean
    modelValue?: string
    loading?: boolean
    saving?: boolean
    error?: string
    success?: string
    dirty?: boolean
  }>(),
  {
    disabled: false,
    modelValue: '',
    loading: false,
    saving: false,
    error: '',
    success: '',
    dirty: false
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'save'): void
  (e: 'reset'): void
  (e: 'retry'): void
}>()

const { t } = useI18n()

const codepointCount = computed(() => Array.from(props.modelValue).length)
const tooLong = computed(() => codepointCount.value > MAX_CODEPOINTS)
const previewText = computed(
  () => props.modelValue.trim() || `${t('groupBuy.subtitle')}\n${t('groupBuy.rules')}`
)

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
}
</script>
