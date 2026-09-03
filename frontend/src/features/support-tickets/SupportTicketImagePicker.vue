<template>
  <div>
    <label :for="inputId" class="input-label mb-1.5 block">{{ t('supportTickets.form.images') }}</label>
    <input
      :id="inputId"
      ref="input"
      type="file"
      accept="image/jpeg,image/png,image/webp"
      multiple
      :disabled="disabled || modelValue.length >= SUPPORT_TICKET_IMAGES_MAX"
      class="block w-full text-sm text-gray-600 file:mr-3 file:rounded-lg file:border-0 file:bg-primary-50 file:px-3 file:py-2 file:font-medium file:text-primary-700 hover:file:bg-primary-100 disabled:opacity-60 dark:text-gray-300 dark:file:bg-primary-900/30 dark:file:text-primary-300"
      data-test="ticket-images"
      @change="addImages"
    />
    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
      {{ t('supportTickets.form.imageHint') }}
    </p>
    <p v-if="error" class="input-error-text mt-1.5" role="alert">{{ error }}</p>

    <div v-if="previews.length" class="mt-3 grid grid-cols-3 gap-3">
      <div
        v-for="preview in previews"
        :key="preview.url"
        class="relative aspect-square overflow-hidden rounded-xl border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-900"
      >
        <img :src="preview.url" :alt="preview.file.name" class="h-full w-full object-cover" />
        <button
          type="button"
          class="absolute right-1 top-1 rounded-full bg-black/70 px-2 py-1 text-xs text-white hover:bg-black"
          :aria-label="t('supportTickets.form.removeImage', { name: preview.file.name })"
          :disabled="disabled"
          data-test="remove-ticket-image"
          @click="removeImage(preview.file)"
        >
          ×
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  SUPPORT_TICKET_IMAGE_MAX,
  SUPPORT_TICKET_IMAGE_TYPES,
  SUPPORT_TICKET_IMAGES_MAX,
} from './types'

interface Preview {
  file: File
  url: string
}

const props = withDefaults(defineProps<{
  modelValue: File[]
  disabled?: boolean
}>(), {
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [files: File[]]
}>()

const { t } = useI18n()
const inputId = `support-ticket-images-${Math.random().toString(36).slice(2, 9)}`
const input = ref<HTMLInputElement | null>(null)
const previews = ref<Preview[]>([])
const error = ref('')

function revoke(preview: Preview): void {
  URL.revokeObjectURL(preview.url)
}

watch(
  () => props.modelValue,
  (files) => {
    for (const preview of previews.value.filter((item) => !files.includes(item.file))) revoke(preview)
    previews.value = previews.value.filter((item) => files.includes(item.file))
    for (const file of files) {
      if (!previews.value.some((item) => item.file === file)) {
        previews.value.push({ file, url: URL.createObjectURL(file) })
      }
    }
  },
  { immediate: true },
)

function addImages(event: Event): void {
  error.value = ''
  const selected = Array.from((event.target as HTMLInputElement).files ?? [])
  if (selected.some((file) => !SUPPORT_TICKET_IMAGE_TYPES.includes(file.type as typeof SUPPORT_TICKET_IMAGE_TYPES[number]))) {
    error.value = t('supportTickets.errors.imageType')
  } else if (selected.some((file) => file.size > SUPPORT_TICKET_IMAGE_MAX)) {
    error.value = t('supportTickets.errors.imageSize')
  } else if (props.modelValue.length + selected.length > SUPPORT_TICKET_IMAGES_MAX) {
    error.value = t('supportTickets.errors.imageCount')
  } else {
    emit('update:modelValue', [...props.modelValue, ...selected])
  }
  if (input.value) input.value.value = ''
}

function removeImage(file: File): void {
  error.value = ''
  emit('update:modelValue', props.modelValue.filter((item) => item !== file))
}

onBeforeUnmount(() => {
  for (const preview of previews.value) revoke(preview)
})
</script>
