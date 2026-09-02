<template>
  <AppLayout>
    <div class="mx-auto max-w-[1480px]">
    <header class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <div class="mb-2 flex items-center gap-2">
          <span class="flex h-9 w-9 items-center justify-center rounded-xl bg-primary-500 text-white shadow-lg shadow-primary-500/25">
            <Icon name="sparkles" size="md" :stroke-width="1.8" />
          </span>
          <span class="rounded-full border border-primary-200 bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:border-primary-800/60 dark:bg-primary-900/20 dark:text-primary-300">
            {{ t('imageGeneration.badge') }}
          </span>
        </div>
        <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-3xl">
          {{ t('imageGeneration.title') }}
        </h1>
        <p class="mt-1.5 max-w-2xl text-sm leading-6 text-gray-500 dark:text-dark-300">
          {{ t('imageGeneration.subtitle') }}
        </p>
      </div>
      <div class="flex items-center gap-2 text-xs text-gray-400 dark:text-dark-400">
        <span class="h-2 w-2 rounded-full bg-emerald-500 shadow-[0_0_0_4px_rgba(16,185,129,0.12)]"></span>
        {{ t('imageGeneration.providers') }}
      </div>
    </header>

    <div v-if="loadingKeys" class="card flex min-h-[520px] items-center justify-center">
      <div class="text-center">
        <LoadingSpinner size="lg" />
        <p class="mt-3 text-sm text-gray-500 dark:text-dark-300">{{ t('imageGeneration.loadingKeys') }}</p>
      </div>
    </div>

    <div v-else-if="!imageKeys.length" class="card flex min-h-[520px] items-center justify-center p-8 text-center">
      <div class="max-w-md">
        <span class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-dark-300">
          <Icon name="key" size="xl" />
        </span>
        <h2 class="mt-5 text-lg font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.noKeysTitle') }}</h2>
        <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-300">{{ t('imageGeneration.noKeysDescription') }}</p>
        <RouterLink to="/keys" class="btn btn-primary mt-5">
          <Icon name="plus" size="sm" />
          {{ t('imageGeneration.manageKeys') }}
        </RouterLink>
      </div>
    </div>

    <div v-else class="grid items-start gap-6 xl:grid-cols-[420px_minmax(0,1fr)]">
      <form class="card overflow-hidden xl:sticky xl:top-24" @submit.prevent="submitGeneration">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div class="flex items-center justify-between">
            <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.createTitle') }}</h2>
            <span class="rounded-lg bg-gray-100 px-2 py-1 text-[11px] font-medium text-gray-500 dark:bg-dark-700 dark:text-dark-300">
              {{ t('imageGeneration.step') }}
            </span>
          </div>
        </div>

        <div class="space-y-5 p-5">
          <div>
            <label for="image-api-key" class="input-label">{{ t('imageGeneration.apiKey') }}</label>
            <Select
              id="image-api-key"
              v-model="form.apiKeyId"
              :options="keyOptions"
              :aria-label="t('imageGeneration.apiKey')"
            />
            <p v-if="selectedKey?.group" class="mt-1.5 flex items-center gap-1.5 text-xs text-gray-400 dark:text-dark-400">
              <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
              {{ selectedKey.group.name }} · {{ selectedKey.group.platform }}
            </p>
          </div>

          <div>
            <div class="mb-1.5 flex items-center justify-between">
              <label for="image-model" class="input-label !mb-0">{{ t('imageGeneration.model') }}</label>
              <span v-if="loadingModels" class="text-xs text-primary-500">{{ t('imageGeneration.loadingModels') }}</span>
            </div>
            <Select
              id="image-model"
              v-model="form.model"
              :options="modelOptions"
              :placeholder="t('imageGeneration.modelPlaceholder')"
              :empty-text="modelError || t('imageGeneration.noModels')"
              :disabled="loadingModels"
              :loading="loadingModels"
              :creatable="true"
              searchable
              :aria-label="t('imageGeneration.model')"
            />
            <p class="mt-1.5 text-xs text-gray-400 dark:text-dark-400">{{ t('imageGeneration.modelHint') }}</p>
          </div>

          <div>
            <div class="mb-1.5 flex items-center justify-between">
              <label for="image-prompt" class="input-label !mb-0">{{ t('imageGeneration.prompt') }}</label>
              <span class="text-xs tabular-nums text-gray-400 dark:text-dark-400">{{ form.prompt.length }}/4000</span>
            </div>
            <textarea
              id="image-prompt"
              v-model="form.prompt"
              maxlength="4000"
              rows="6"
              class="input min-h-[142px] resize-y leading-6"
              :placeholder="t('imageGeneration.promptPlaceholder')"
            ></textarea>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <button
                v-for="example in promptExamples"
                :key="example"
                type="button"
                class="rounded-full border border-gray-200 px-2.5 py-1 text-xs text-gray-500 transition hover:border-primary-300 hover:bg-primary-50 hover:text-primary-600 dark:border-dark-600 dark:text-dark-300 dark:hover:border-primary-700 dark:hover:bg-primary-900/20 dark:hover:text-primary-300"
                @click="form.prompt = example"
              >
                {{ example }}
              </button>
            </div>
          </div>

          <div>
            <div class="mb-2 flex items-center justify-between">
              <span class="input-label !mb-0">{{ t('imageGeneration.referenceImages') }}</span>
              <span class="text-xs text-gray-400 dark:text-dark-400">{{ referenceImages.length }}/4</span>
            </div>
            <input
              ref="fileInput"
              type="file"
              accept="image/png,image/jpeg,image/webp"
              multiple
              class="sr-only"
              @change="onFileChange"
            />
            <button
              v-if="referenceImages.length < 4"
              type="button"
              class="upload-zone group w-full"
              :class="dragging && 'upload-zone-active'"
              @click="fileInput?.click()"
              @dragenter.prevent="dragging = true"
              @dragover.prevent
              @dragleave.prevent="dragging = false"
              @drop.prevent="onDrop"
            >
              <span class="flex h-9 w-9 items-center justify-center rounded-xl bg-gray-100 text-gray-400 transition group-hover:bg-primary-100 group-hover:text-primary-600 dark:bg-dark-700 dark:text-dark-300 dark:group-hover:bg-primary-900/30 dark:group-hover:text-primary-300">
                <Icon name="upload" size="md" />
              </span>
              <span class="text-left">
                <span class="block text-sm font-medium text-gray-700 dark:text-dark-100">{{ t('imageGeneration.uploadReference') }}</span>
                <span class="block text-xs text-gray-400 dark:text-dark-400">{{ t('imageGeneration.uploadHint') }}</span>
              </span>
            </button>
            <div v-if="referenceImages.length" class="mt-2.5 grid grid-cols-4 gap-2">
              <div v-for="(image, index) in referenceImages" :key="image.url" class="group relative aspect-square overflow-hidden rounded-xl border bg-gray-100 dark:bg-dark-800">
                <img :src="image.url" :alt="t('imageGeneration.referenceAlt', { index: index + 1 })" class="h-full w-full object-cover" />
                <button
                  type="button"
                  class="absolute right-1 top-1 flex h-6 w-6 items-center justify-center rounded-lg bg-gray-950/70 text-white opacity-0 backdrop-blur transition hover:bg-red-500 group-hover:opacity-100 focus:opacity-100"
                  :aria-label="t('imageGeneration.removeReference')"
                  @click="removeReference(index)"
                >
                  <Icon name="x" size="xs" />
                </button>
              </div>
            </div>
          </div>

          <div>
            <span class="input-label">{{ t('imageGeneration.aspectRatio') }}</span>
            <div class="grid grid-cols-3 gap-2">
              <button
                v-for="ratio in ratios"
                :key="ratio.value"
                type="button"
                class="option-button"
                :class="form.aspectRatio === ratio.value && 'option-button-active'"
                :aria-pressed="form.aspectRatio === ratio.value"
                @click="form.aspectRatio = ratio.value"
              >
                <span class="ratio-icon" :style="{ aspectRatio: ratio.cssRatio }"></span>
                <span>{{ ratio.value }}</span>
              </button>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div>
              <span class="input-label">{{ t('imageGeneration.quality') }}</span>
              <div class="flex rounded-xl bg-gray-100 p-1 dark:bg-dark-900/70">
                <button
                  v-for="quality in qualities"
                  :key="quality.value"
                  type="button"
                  class="segment-button"
                  :class="form.quality === quality.value && 'segment-button-active'"
                  :aria-pressed="form.quality === quality.value"
                  @click="form.quality = quality.value"
                >
                  {{ quality.label }}
                </button>
              </div>
            </div>
            <div>
              <span class="input-label">{{ t('imageGeneration.count') }}</span>
              <div class="flex rounded-xl bg-gray-100 p-1 dark:bg-dark-900/70">
                <button
                  v-for="count in counts"
                  :key="count"
                  type="button"
                  class="segment-button"
                  :class="form.count === count && 'segment-button-active'"
                  :aria-pressed="form.count === count"
                  @click="form.count = count"
                >
                  {{ count }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="border-t border-gray-100 bg-gray-50/60 p-5 dark:border-dark-700 dark:bg-dark-900/30">
          <button type="submit" class="btn btn-primary btn-lg w-full" :disabled="generating || !form.model || !form.prompt.trim()">
            <LoadingSpinner v-if="generating" size="sm" class="text-white" />
            <Icon v-else name="sparkles" size="md" />
            {{ generating ? t('imageGeneration.generating') : t('imageGeneration.generate') }}
          </button>
          <p class="mt-2.5 flex items-start justify-center gap-1.5 text-center text-[11px] leading-4 text-gray-400 dark:text-dark-400">
            <Icon name="shield" size="xs" class="mt-0.5 shrink-0" />
            {{ t('imageGeneration.requestHint') }}
          </p>
        </div>
      </form>

      <section class="card min-h-[680px] overflow-hidden">
        <div class="flex min-h-[65px] items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
          <div>
            <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.resultsTitle') }}</h2>
            <p class="mt-0.5 text-xs text-gray-400 dark:text-dark-400">
              {{ results.length ? t('imageGeneration.resultCount', { count: results.length }) : t('imageGeneration.resultsHint') }}
            </p>
          </div>
          <button v-if="results.length && !generating" type="button" class="btn btn-ghost btn-sm" @click="results = []">
            <Icon name="trash" size="sm" />
            {{ t('imageGeneration.clear') }}
          </button>
        </div>

        <div class="p-4 sm:p-6">
          <div v-if="generating" class="grid gap-4 md:grid-cols-2">
            <div
              v-for="index in form.count"
              :key="index"
              class="result-loading relative overflow-hidden rounded-2xl border bg-gray-100 dark:bg-dark-900"
              :style="{ aspectRatio: selectedRatioCss }"
            >
              <div class="absolute inset-0 flex flex-col items-center justify-center gap-3">
                <span class="flex h-12 w-12 animate-pulse items-center justify-center rounded-2xl bg-white/80 text-primary-500 shadow-sm dark:bg-dark-700/80">
                  <Icon name="sparkles" size="lg" />
                </span>
                <span class="text-xs font-medium text-gray-500 dark:text-dark-300">{{ t('imageGeneration.rendering', { index }) }}</span>
              </div>
            </div>
          </div>

          <div v-else-if="results.length" class="grid gap-4 md:grid-cols-2">
            <article v-for="(result, index) in results" :key="result.id" class="group overflow-hidden rounded-2xl border bg-white shadow-sm transition hover:-translate-y-0.5 hover:shadow-lg dark:bg-dark-800">
              <button type="button" class="relative block w-full overflow-hidden bg-checker focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500" @click="preview = result">
                <img
                  :src="result.src"
                  :alt="t('imageGeneration.resultAlt', { index: index + 1 })"
                  class="w-full object-contain transition duration-500 group-hover:scale-[1.015]"
                  :style="{ aspectRatio: result.aspectRatio }"
                  loading="lazy"
                />
                <span class="absolute inset-x-0 bottom-0 flex translate-y-full items-center justify-center bg-gradient-to-t from-gray-950/70 to-transparent pb-4 pt-12 text-xs font-medium text-white transition group-hover:translate-y-0 group-focus-within:translate-y-0">
                  <Icon name="eye" size="sm" class="mr-1.5" />
                  {{ t('imageGeneration.preview') }}
                </span>
              </button>
              <div class="flex items-center gap-3 p-3.5">
                <div class="min-w-0 flex-1">
                  <p class="truncate text-xs font-medium text-gray-700 dark:text-dark-100">{{ result.model }}</p>
                  <p class="mt-0.5 text-[11px] text-gray-400 dark:text-dark-400">{{ result.createdAt }}</p>
                </div>
                <button type="button" class="btn btn-secondary btn-icon !p-2" :aria-label="t('imageGeneration.download')" @click="downloadResult(result)">
                  <Icon name="download" size="sm" />
                </button>
              </div>
            </article>
          </div>

          <div v-else class="flex min-h-[540px] items-center justify-center text-center">
            <div class="max-w-sm">
              <div class="empty-orbit mx-auto">
                <span class="flex h-16 w-16 items-center justify-center rounded-2xl bg-white text-primary-500 shadow-xl shadow-primary-500/10 dark:bg-dark-700">
                  <Icon name="sparkles" size="xl" />
                </span>
              </div>
              <h3 class="mt-6 font-semibold text-gray-800 dark:text-dark-100">{{ t('imageGeneration.emptyTitle') }}</h3>
              <p class="mt-2 text-sm leading-6 text-gray-400 dark:text-dark-400">{{ t('imageGeneration.emptyDescription') }}</p>
            </div>
          </div>
        </div>
      </section>
    </div>

    <BaseDialog
      :show="Boolean(preview)"
      :title="t('imageGeneration.previewTitle')"
      width="extra-wide"
      close-on-click-outside
      @close="preview = null"
    >
      <div v-if="preview" class="space-y-4">
        <div class="flex max-h-[72vh] items-center justify-center overflow-hidden rounded-2xl bg-checker">
          <img :src="preview.src" :alt="t('imageGeneration.previewTitle')" class="max-h-[72vh] max-w-full object-contain" />
        </div>
        <div v-if="preview.revisedPrompt" class="rounded-xl bg-gray-50 p-3 text-sm leading-6 text-gray-600 dark:bg-dark-800 dark:text-dark-200">
          {{ preview.revisedPrompt }}
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-primary" @click="preview && downloadResult(preview)">
          <Icon name="download" size="sm" />
          {{ t('imageGeneration.download') }}
        </button>
      </template>
    </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { keysAPI } from '@/api/keys'
import {
  generateImages,
  listImageModels,
  supportsImageGeneration,
  type GeneratedImage,
  type ImageQuality,
} from '@/api/imageGeneration'
import { BaseDialog, LoadingSpinner } from '@/components/common'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import type { ApiKey } from '@/types'

interface ReferenceImage {
  file: File
  url: string
}

interface ImageResult extends GeneratedImage {
  id: string
  model: string
  createdAt: string
  aspectRatio: string
}

const { t } = useI18n()
const appStore = useAppStore()
const apiKeys = ref<ApiKey[]>([])
const loadingKeys = ref(true)
const loadingModels = ref(false)
const modelError = ref('')
const availableModels = ref<string[]>([])
const referenceImages = ref<ReferenceImage[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const dragging = ref(false)
const generating = ref(false)
const results = ref<ImageResult[]>([])
const preview = ref<ImageResult | null>(null)
let modelRequestID = 0

const form = reactive({
  apiKeyId: null as number | null,
  model: '',
  prompt: '',
  aspectRatio: '1:1' as '1:1' | '3:2' | '2:3',
  quality: 'standard' as ImageQuality,
  count: 1,
})

const ratios: Array<{ value: typeof form.aspectRatio; cssRatio: string }> = [
  { value: '1:1', cssRatio: '1 / 1' },
  { value: '3:2', cssRatio: '3 / 2' },
  { value: '2:3', cssRatio: '2 / 3' },
]
const counts = [1, 2, 4]
const qualities = computed(() => [
  { value: 'standard' as const, label: t('imageGeneration.qualityStandard') },
  { value: 'high' as const, label: t('imageGeneration.qualityHigh') },
])
const promptExamples = computed(() => [
  t('imageGeneration.exampleProduct'),
  t('imageGeneration.examplePortrait'),
  t('imageGeneration.examplePoster'),
])

const imageKeys = computed(() => apiKeys.value.filter(key =>
  key.status === 'active' &&
  key.group?.allow_image_generation === true &&
  supportsImageGeneration(key.group.platform)
))
const selectedKey = computed(() => imageKeys.value.find(key => key.id === form.apiKeyId) || null)
const keyOptions = computed(() => imageKeys.value.map(key => ({
  value: key.id,
  label: `${key.name} · ${key.group?.name || ''}`,
})))
const modelOptions = computed(() => availableModels.value.map(model => ({ value: model, label: model })))
const selectedRatioCss = computed(() => ratios.find(item => item.value === form.aspectRatio)?.cssRatio || '1 / 1')

async function loadKeys() {
  loadingKeys.value = true
  try {
    const keys: ApiKey[] = []
    let page = 1
    while (true) {
      const response = await keysAPI.list(page, 100, {
        status: 'active',
        sort_by: 'created_at',
        sort_order: 'desc',
      })
      keys.push(...(response.items || []))
      if (page >= response.pages || !response.items?.length) break
      page += 1
    }
    apiKeys.value = keys
    form.apiKeyId = imageKeys.value[0]?.id || null
  } catch (error) {
    appStore.showError(errorMessage(error, t('imageGeneration.loadKeysFailed')))
  } finally {
    loadingKeys.value = false
  }
}

async function loadModels() {
  const key = selectedKey.value
  const requestID = ++modelRequestID
  availableModels.value = []
  form.model = ''
  modelError.value = ''
  if (!key?.group) return

  loadingModels.value = true
  try {
    const models = await listImageModels(key.key, key.group.platform)
    if (requestID !== modelRequestID) return
    availableModels.value = models
    form.model = models[0] || ''
  } catch (error) {
    if (requestID !== modelRequestID) return
    modelError.value = errorMessage(error, t('imageGeneration.loadModelsFailed'))
  } finally {
    if (requestID === modelRequestID) loadingModels.value = false
  }
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

function addReferenceImages(files: File[]) {
  const accepted = new Set(['image/png', 'image/jpeg', 'image/webp'])
  const valid: File[] = []
  for (const file of files) {
    if (!accepted.has(file.type)) {
      appStore.showError(t('imageGeneration.invalidFileType'))
      continue
    }
    if (file.size > 10 * 1024 * 1024) {
      appStore.showError(t('imageGeneration.fileTooLarge', { name: file.name }))
      continue
    }
    valid.push(file)
  }

  const slots = Math.max(0, 4 - referenceImages.value.length)
  referenceImages.value.push(...valid.slice(0, slots).map(file => ({
    file,
    url: URL.createObjectURL(file),
  })))
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  addReferenceImages(Array.from(input.files || []))
  input.value = ''
}

function onDrop(event: DragEvent) {
  dragging.value = false
  addReferenceImages(Array.from(event.dataTransfer?.files || []))
}

function removeReference(index: number) {
  const [removed] = referenceImages.value.splice(index, 1)
  if (removed) URL.revokeObjectURL(removed.url)
}

async function submitGeneration() {
  const key = selectedKey.value
  const prompt = form.prompt.trim()
  if (!key?.group) {
    appStore.showError(t('imageGeneration.selectKey'))
    return
  }
  if (!form.model.trim()) {
    appStore.showError(t('imageGeneration.selectModel'))
    return
  }
  if (!prompt) {
    appStore.showError(t('imageGeneration.promptRequired'))
    return
  }

  generating.value = true
  try {
    const generated = await generateImages({
      apiKey: key.key,
      platform: key.group.platform,
      model: form.model.trim(),
      prompt,
      aspectRatio: form.aspectRatio,
      quality: form.quality,
      count: form.count,
      referenceImages: referenceImages.value.map(image => image.file),
    })
    if (!generated.length) throw new Error(t('imageGeneration.noImageReturned'))

    const now = new Date().toLocaleString()
    results.value.unshift(...generated.map((image, index) => ({
      ...image,
      id: `${Date.now()}-${index}`,
      model: form.model.trim(),
      createdAt: now,
      aspectRatio: selectedRatioCss.value,
    })))
    appStore.showSuccess(t('imageGeneration.generated', { count: generated.length }))
  } catch (error) {
    appStore.showError(errorMessage(error, t('imageGeneration.generateFailed')))
  } finally {
    generating.value = false
  }
}

function clickDownload(src: string, extension: string, openInNewTab = false) {
  const link = document.createElement('a')
  link.href = src
  if (extension) link.download = `sub2api-image-${Date.now()}.${extension}`
  if (openInNewTab) {
    link.target = '_blank'
    link.rel = 'noopener noreferrer'
  }
  document.body.appendChild(link)
  link.click()
  link.remove()
}

async function downloadResult(result: ImageResult) {
  if (result.src.startsWith('data:')) {
    clickDownload(result.src, result.src.startsWith('data:image/jpeg') ? 'jpg' : result.src.startsWith('data:image/webp') ? 'webp' : 'png')
    return
  }

  try {
    const response = await fetch(result.src)
    if (!response.ok) throw new Error(response.statusText)
    const blob = await response.blob()
    const url = URL.createObjectURL(blob)
    clickDownload(url, blob.type === 'image/jpeg' ? 'jpg' : blob.type === 'image/webp' ? 'webp' : 'png')
    setTimeout(() => URL.revokeObjectURL(url), 0)
  } catch {
    clickDownload(result.src, '', true)
  }
}

watch(() => form.apiKeyId, () => void loadModels())

onMounted(() => void loadKeys())
onBeforeUnmount(() => {
  referenceImages.value.forEach(image => URL.revokeObjectURL(image.url))
})
</script>

<style scoped>
.upload-zone {
  @apply flex items-center justify-center gap-3 rounded-2xl border border-dashed border-gray-300 bg-gray-50/70 px-4 py-4 transition hover:border-primary-400 hover:bg-primary-50/60 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:border-dark-600 dark:bg-dark-900/40 dark:hover:border-primary-700 dark:hover:bg-primary-900/10;
}

.upload-zone-active {
  @apply border-primary-500 bg-primary-50 ring-2 ring-primary-500/20 dark:border-primary-500 dark:bg-primary-900/20;
}

.option-button {
  @apply flex h-12 items-center justify-center gap-2 rounded-xl border border-gray-200 bg-white text-xs font-medium text-gray-500 transition hover:border-primary-300 hover:text-primary-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300 dark:hover:border-primary-700 dark:hover:text-primary-300;
}

.option-button-active {
  @apply border-primary-500 bg-primary-50 text-primary-700 ring-1 ring-primary-500/20 dark:border-primary-500 dark:bg-primary-900/20 dark:text-primary-300;
}

.ratio-icon {
  @apply block max-h-5 min-h-3 w-5 rounded-[3px] border-2 border-current;
}

.segment-button {
  @apply min-w-0 flex-1 rounded-lg px-2 py-2 text-xs font-medium text-gray-500 transition focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:text-dark-300;
}

.segment-button-active {
  @apply bg-white text-primary-700 shadow-sm dark:bg-dark-700 dark:text-primary-300;
}

.result-loading::before {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(100deg, transparent 20%, rgb(255 255 255 / 55%) 50%, transparent 80%);
  animation: shimmer 1.8s infinite;
}

.dark .result-loading::before {
  background: linear-gradient(100deg, transparent 20%, rgb(255 255 255 / 6%) 50%, transparent 80%);
}

.bg-checker {
  background-color: rgb(249 250 251);
  background-image: linear-gradient(45deg, rgb(229 231 235 / 45%) 25%, transparent 25%),
    linear-gradient(-45deg, rgb(229 231 235 / 45%) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgb(229 231 235 / 45%) 75%),
    linear-gradient(-45deg, transparent 75%, rgb(229 231 235 / 45%) 75%);
  background-size: 24px 24px;
  background-position: 0 0, 0 12px, 12px -12px, -12px 0;
}

.dark .bg-checker {
  background-color: rgb(17 24 39);
  background-image: linear-gradient(45deg, rgb(55 65 81 / 35%) 25%, transparent 25%),
    linear-gradient(-45deg, rgb(55 65 81 / 35%) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgb(55 65 81 / 35%) 75%),
    linear-gradient(-45deg, transparent 75%, rgb(55 65 81 / 35%) 75%);
}

.empty-orbit {
  @apply relative flex h-28 w-28 items-center justify-center rounded-full border border-dashed border-primary-200 bg-primary-50/50 dark:border-primary-800/60 dark:bg-primary-900/10;
}

.empty-orbit::after {
  content: '';
  @apply absolute h-2.5 w-2.5 rounded-full bg-primary-400;
  top: 8px;
  right: 17px;
  box-shadow: 0 0 0 5px rgb(99 102 241 / 10%);
}

@keyframes shimmer {
  100% { transform: translateX(100%); }
}
</style>
