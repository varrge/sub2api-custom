<template>
  <AppLayout>
    <div class="studio-shell -m-4 md:-m-6 lg:-m-8">
      <header class="border-b border-gray-200 bg-white px-5 pt-5 dark:border-dark-700 dark:bg-dark-900 sm:px-7">
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.studioTitle') }}</h1>
        <nav class="mt-4 flex gap-6" :aria-label="t('imageGeneration.studioTitle')">
          <span class="border-b-2 border-primary-500 pb-3 text-sm font-medium text-primary-600 dark:text-primary-300">
            {{ t('imageGeneration.imageTab') }}
          </span>
        </nav>
      </header>

      <div v-if="loadingKeys" class="flex flex-1 items-center justify-center">
        <div class="text-center">
          <LoadingSpinner size="lg" />
          <p class="mt-3 text-sm text-gray-500 dark:text-dark-300">{{ t('imageGeneration.loadingKeys') }}</p>
        </div>
      </div>

      <div v-else-if="!imageKeys.length" class="flex flex-1 items-center justify-center p-8 text-center">
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

      <div v-else class="studio-grid">
        <section class="flex min-h-[680px] min-w-0 flex-col bg-gray-50/70 dark:bg-dark-950 lg:min-h-0">
          <div class="flex h-12 shrink-0 items-center justify-between border-b border-gray-200/80 bg-white/80 px-5 dark:border-dark-700 dark:bg-dark-900/80">
            <span class="text-sm font-medium text-gray-700 dark:text-dark-100">
              {{ activeResult ? t('imageGeneration.imageNumber', { index: results.indexOf(activeResult) + 1 }) : t('imageGeneration.canvasTitle') }}
            </span>
            <button
              v-if="activeResult"
              type="button"
              class="btn btn-ghost btn-sm"
              @click="downloadResult(activeResult)"
            >
              <Icon name="download" size="sm" />
              {{ t('imageGeneration.download') }}
            </button>
          </div>

          <div
            class="relative flex min-h-[360px] flex-1 items-center justify-center overflow-auto p-5 sm:p-8"
            @dragenter.prevent="dragging = true"
            @dragover.prevent
            @dragleave.prevent="dragging = false"
            @drop.prevent="onDrop"
          >
            <div v-if="generating" class="grid w-full max-w-4xl gap-4" :class="form.count > 1 ? 'sm:grid-cols-2' : ''">
              <div
                v-for="index in form.count"
                :key="index"
                class="result-loading relative mx-auto w-full max-w-2xl overflow-hidden rounded-2xl border border-gray-200 bg-gray-100 dark:border-dark-700 dark:bg-dark-900"
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

            <button
              v-else-if="activeResult"
              type="button"
              class="group flex h-full w-full items-center justify-center focus:outline-none"
              @click="preview = activeResult"
            >
              <img
                :src="activeResult.src"
                :alt="t('imageGeneration.resultAlt', { index: results.indexOf(activeResult) + 1 })"
                class="max-h-full max-w-full rounded-2xl object-contain shadow-xl shadow-gray-900/10 transition group-hover:scale-[1.005] dark:shadow-black/30"
              />
            </button>

            <div v-else class="text-center text-gray-400 dark:text-dark-400">
              <div class="empty-orbit mx-auto">
                <span class="flex h-16 w-16 items-center justify-center rounded-2xl bg-white text-primary-500 shadow-xl shadow-primary-500/10 dark:bg-dark-700">
                  <Icon name="sparkles" size="xl" />
                </span>
              </div>
              <p class="mt-5 text-sm">{{ t('imageGeneration.resultPlaceholder') }}</p>
            </div>

            <div
              v-if="dragging"
              class="pointer-events-none absolute inset-4 flex items-center justify-center rounded-2xl border-2 border-dashed border-primary-500 bg-primary-50/90 text-sm font-medium text-primary-700 backdrop-blur dark:bg-primary-950/90 dark:text-primary-200"
            >
              {{ t('imageGeneration.dropReference') }}
            </div>
          </div>

          <form
            class="composer m-3 shrink-0 sm:m-5"
            @submit.prevent="submitGeneration"
            @dragenter.prevent="dragging = true"
            @dragover.prevent
            @dragleave.prevent="dragging = false"
            @drop.prevent="onDrop"
          >
            <input
              ref="fileInput"
              type="file"
              accept="image/png,image/jpeg,image/webp"
              multiple
              class="sr-only"
              @change="onFileChange"
            />

            <div v-if="referenceImages.length" class="flex gap-2 overflow-x-auto px-4 pt-3">
              <div
                v-for="(image, index) in referenceImages"
                :key="image.url"
                class="group relative h-14 w-14 shrink-0 overflow-hidden rounded-xl border border-gray-200 bg-gray-100 dark:border-dark-600 dark:bg-dark-800"
              >
                <img :src="image.url" :alt="t('imageGeneration.referenceAlt', { index: index + 1 })" class="h-full w-full object-cover" />
                <button
                  type="button"
                  class="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-md bg-gray-950/70 text-white opacity-0 transition hover:bg-red-500 group-hover:opacity-100 focus:opacity-100"
                  :aria-label="t('imageGeneration.removeReference')"
                  @click="removeReference(index)"
                >
                  <Icon name="x" size="xs" />
                </button>
              </div>
            </div>

            <textarea
              id="image-prompt"
              v-model="form.prompt"
              maxlength="4000"
              rows="3"
              class="min-h-[92px] w-full resize-none bg-transparent px-4 py-3 text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 dark:text-white dark:placeholder:text-dark-400"
              :placeholder="t('imageGeneration.promptPlaceholder')"
              @paste="onPaste"
            ></textarea>

            <div class="flex flex-wrap items-center gap-2 border-t border-gray-100 px-3 py-2.5 dark:border-dark-700">
              <div class="flex rounded-lg bg-gray-100 p-0.5 dark:bg-dark-800">
                <button
                  type="button"
                  class="mode-button"
                  :class="!referenceImages.length && 'mode-button-active'"
                  @click="chooseMode('text')"
                >
                  {{ t('imageGeneration.textToImage') }}
                </button>
                <button
                  type="button"
                  class="mode-button"
                  :class="referenceImages.length && 'mode-button-active'"
                  @click="chooseMode('image')"
                >
                  {{ t('imageGeneration.imageToImage') }}
                </button>
              </div>

              <Select
                id="image-api-key"
                v-model="form.apiKeyId"
                :options="keyOptions"
                :aria-label="t('imageGeneration.apiKey')"
                class="compact-select w-40"
              />

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
                class="compact-select min-w-40 flex-1 sm:max-w-52"
              />

              <details ref="ratioDetails" class="relative">
                <summary class="control-button list-none">
                  <span>{{ ratioLabel }}</span>
                  <Icon name="chevronDown" size="sm" />
                </summary>
                <div class="ratio-panel absolute bottom-[calc(100%+10px)] left-0 z-20 w-[min(430px,calc(100vw-3rem))] rounded-2xl border border-gray-200 bg-white p-4 shadow-2xl shadow-gray-900/15 dark:border-dark-600 dark:bg-dark-800 dark:shadow-black/40 sm:p-5">
                  <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.aspectRatio') }}</h3>
                  <div class="mt-3 grid grid-cols-3 gap-2">
                    <button
                      v-for="ratio in ratioOptions"
                      :key="ratio.value"
                      type="button"
                      class="ratio-option"
                      :class="form.aspectRatio === ratio.value && 'ratio-option-active'"
                      :aria-pressed="form.aspectRatio === ratio.value"
                      @click="selectRatio(ratio.value)"
                    >
                      <span class="ratio-icon" :style="{ aspectRatio: ratio.cssRatio }"></span>
                      <span>{{ ratio.value === 'auto' ? t('imageGeneration.auto') : ratio.value }}</span>
                    </button>
                  </div>

                  <h3 class="mt-5 text-sm font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.resolution') }}</h3>
                  <div class="mt-3 grid grid-cols-2 gap-2">
                    <button
                      v-for="quality in qualities"
                      :key="quality.value"
                      type="button"
                      class="ratio-option h-10"
                      :class="form.quality === quality.value && 'ratio-option-active'"
                      :aria-pressed="form.quality === quality.value"
                      @click="form.quality = quality.value"
                    >
                      {{ quality.label }}
                    </button>
                  </div>

                  <h3 class="mt-5 text-sm font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.size') }}</h3>
                  <div class="mt-3 flex items-center gap-2 text-sm">
                    <span class="text-gray-400">W</span>
                    <span class="size-value">{{ displaySize.width }}</span>
                    <span class="text-gray-300 dark:text-dark-500">×</span>
                    <span class="text-gray-400">H</span>
                    <span class="size-value">{{ displaySize.height }}</span>
                    <span class="ml-auto text-xs text-gray-400 dark:text-dark-400">{{ t('imageGeneration.pixels') }}</span>
                  </div>
                </div>
              </details>

              <Select
                v-model="form.count"
                :options="countOptions"
                :aria-label="t('imageGeneration.count')"
                class="compact-select w-20"
              />

              <button
                type="submit"
                class="btn btn-primary ml-auto min-w-24"
                :disabled="generating || !form.model || !form.prompt.trim()"
              >
                <LoadingSpinner v-if="generating" size="sm" class="text-white" />
                <Icon v-else name="sparkles" size="sm" />
                {{ generating ? t('imageGeneration.generating') : t('imageGeneration.generate') }}
              </button>
            </div>

            <p class="border-t border-gray-100 px-4 py-2 text-[11px] text-gray-400 dark:border-dark-700 dark:text-dark-400">
              {{ t('imageGeneration.requestHint') }}
            </p>
          </form>
        </section>

        <aside class="flex min-h-[520px] flex-col border-t border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900 lg:min-h-0 lg:border-l lg:border-t-0">
          <div class="border-b border-gray-100 px-5 py-5 dark:border-dark-700">
            <div class="flex items-start justify-between gap-3">
              <div>
                <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.historyTitle') }}</h2>
                <p class="mt-1 text-xs text-gray-400 dark:text-dark-400">{{ t('imageGeneration.historySession') }}</p>
              </div>
              <button
                v-if="results.length"
                type="button"
                class="btn btn-ghost btn-icon btn-sm"
                :aria-label="t('imageGeneration.clear')"
                @click="clearResults"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
            <div class="mt-4 grid grid-cols-3 gap-1 rounded-xl bg-gray-100 p-1 dark:bg-dark-800">
              <button
                v-for="filter in historyFilters"
                :key="filter.value"
                type="button"
                class="history-filter"
                :class="historyFilter === filter.value && 'history-filter-active'"
                @click="historyFilter = filter.value"
              >
                {{ filter.label }}
              </button>
            </div>
          </div>

          <div class="min-h-0 flex-1 overflow-y-auto p-3">
            <div v-if="!filteredResults.length" class="flex min-h-72 items-center justify-center text-sm text-gray-400 dark:text-dark-400">
              {{ t('imageGeneration.noHistory') }}
            </div>
            <div v-else class="space-y-2">
              <article
                v-for="(result, index) in filteredResults"
                :key="result.id"
                class="flex gap-3 rounded-xl border p-2 transition"
                :class="activeResult?.id === result.id
                  ? 'border-primary-400 bg-primary-50/70 dark:border-primary-700 dark:bg-primary-900/15'
                  : 'border-transparent hover:border-gray-200 hover:bg-gray-50 dark:hover:border-dark-600 dark:hover:bg-dark-800'"
              >
                <button type="button" class="h-16 w-16 shrink-0 overflow-hidden rounded-lg bg-gray-100 dark:bg-dark-800" @click="activeResult = result">
                  <img :src="result.src" :alt="t('imageGeneration.resultAlt', { index: index + 1 })" class="h-full w-full object-cover" />
                </button>
                <button type="button" class="min-w-0 flex-1 text-left" @click="activeResult = result">
                  <p class="truncate text-xs font-medium text-gray-700 dark:text-dark-100">{{ result.model }}</p>
                  <p class="mt-1 text-[11px] text-gray-400 dark:text-dark-400">
                    {{ result.mode === 'image' ? t('imageGeneration.imageToImage') : t('imageGeneration.textToImage') }}
                  </p>
                  <p class="mt-1 truncate text-[11px] text-gray-400 dark:text-dark-400">{{ result.createdAt }}</p>
                </button>
                <button type="button" class="self-center rounded-lg p-1.5 text-gray-400 hover:bg-white hover:text-primary-500 dark:hover:bg-dark-700" :aria-label="t('imageGeneration.download')" @click="downloadResult(result)">
                  <Icon name="download" size="sm" />
                </button>
              </article>
            </div>
          </div>
        </aside>
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
  supportedImageAspectRatios,
  supportsImageGeneration,
  type GeneratedImage,
  type ImageAspectRatio,
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

type GenerationMode = 'text' | 'image'
type HistoryFilter = 'all' | GenerationMode

interface ImageResult extends GeneratedImage {
  id: string
  model: string
  createdAt: string
  aspectRatio: string
  mode: GenerationMode
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
const ratioDetails = ref<HTMLDetailsElement | null>(null)
const dragging = ref(false)
const generating = ref(false)
const results = ref<ImageResult[]>([])
const activeResult = ref<ImageResult | null>(null)
const preview = ref<ImageResult | null>(null)
const historyFilter = ref<HistoryFilter>('all')
let modelRequestID = 0

const form = reactive({
  apiKeyId: null as number | null,
  model: '',
  prompt: '',
  aspectRatio: 'auto' as ImageAspectRatio,
  quality: 'standard' as ImageQuality,
  count: 1,
})

const ratioShapes: Record<ImageAspectRatio, string> = {
  auto: '1 / 1',
  '21:9': '21 / 9',
  '16:9': '16 / 9',
  '3:2': '3 / 2',
  '4:3': '4 / 3',
  '1:1': '1 / 1',
  '3:4': '3 / 4',
  '2:3': '2 / 3',
  '9:16': '9 / 16',
}

const qualities = computed(() => [
  { value: 'standard' as const, label: '1K' },
  { value: 'high' as const, label: '2K' },
])
const countOptions = [1, 2, 4].map(value => ({ value, label: `×${value}` }))
const historyFilters = computed<Array<{ value: HistoryFilter; label: string }>>(() => [
  { value: 'all', label: t('imageGeneration.historyAll') },
  { value: 'text', label: t('imageGeneration.textToImage') },
  { value: 'image', label: t('imageGeneration.imageToImage') },
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
const ratioOptions = computed(() => supportedImageAspectRatios(selectedKey.value?.group?.platform, form.model)
  .map(value => ({ value, cssRatio: ratioShapes[value] })))
const ratioLabel = computed(() => form.aspectRatio === 'auto' ? t('imageGeneration.autoRatio') : form.aspectRatio)
const selectedRatioCss = computed(() => form.aspectRatio === 'auto' ? '1 / 1' : ratioShapes[form.aspectRatio])
const filteredResults = computed(() => historyFilter.value === 'all'
  ? results.value
  : results.value.filter(result => result.mode === historyFilter.value))
const displaySize = computed(() => {
  const max = form.quality === 'high' ? 2048 : 1024
  if (form.aspectRatio === 'auto') return { width: max, height: max }
  const [widthRatio, heightRatio] = form.aspectRatio.split(':').map(Number)
  return widthRatio >= heightRatio
    ? { width: max, height: Math.round(max * heightRatio / widthRatio) }
    : { width: Math.round(max * widthRatio / heightRatio), height: max }
})

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

function clearReferences() {
  referenceImages.value.forEach(image => URL.revokeObjectURL(image.url))
  referenceImages.value = []
}

function chooseMode(mode: GenerationMode) {
  if (mode === 'text') clearReferences()
  else fileInput.value?.click()
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

function onPaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || [])
  if (!files.length) return
  event.preventDefault()
  addReferenceImages(files)
}

function removeReference(index: number) {
  const [removed] = referenceImages.value.splice(index, 1)
  if (removed) URL.revokeObjectURL(removed.url)
}

function selectRatio(value: ImageAspectRatio) {
  form.aspectRatio = value
  if (ratioDetails.value) ratioDetails.value.open = false
}

function clearResults() {
  results.value = []
  activeResult.value = null
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
    const mode: GenerationMode = referenceImages.value.length ? 'image' : 'text'
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
    const newResults = generated.map((image, index): ImageResult => ({
      ...image,
      id: `${Date.now()}-${index}`,
      model: form.model.trim(),
      createdAt: now,
      aspectRatio: form.aspectRatio === 'auto' ? 'auto' : ratioShapes[form.aspectRatio],
      mode,
    }))
    results.value.unshift(...newResults)
    activeResult.value = newResults[0]
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
watch(ratioOptions, options => {
  if (!options.some(option => option.value === form.aspectRatio)) form.aspectRatio = 'auto'
})

onMounted(() => void loadKeys())
onBeforeUnmount(clearReferences)
</script>

<style scoped>
.studio-shell {
  @apply flex min-h-[calc(100vh-4rem)] flex-col overflow-hidden bg-white dark:bg-dark-900;
}

.studio-grid {
  @apply grid min-h-0 flex-1 lg:grid-cols-[minmax(0,1fr)_320px] lg:overflow-hidden;
}

.composer {
  @apply rounded-2xl border border-gray-200 bg-white shadow-xl shadow-gray-900/10 dark:border-dark-600 dark:bg-dark-900 dark:shadow-black/30;
}

.mode-button {
  @apply rounded-md px-2.5 py-1.5 text-xs font-medium text-gray-500 transition dark:text-dark-300;
}

.mode-button-active {
  @apply bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white;
}

.control-button {
  @apply flex h-9 cursor-pointer items-center gap-1 rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-600 transition hover:border-gray-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200;
}

.ratio-option {
  @apply flex h-14 items-center justify-center gap-2 rounded-xl border border-gray-200 bg-white text-xs font-medium text-gray-500 transition hover:border-primary-300 hover:text-primary-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:border-dark-600 dark:bg-dark-900 dark:text-dark-300 dark:hover:border-primary-700 dark:hover:text-primary-300;
}

.ratio-option-active {
  @apply border-primary-500 bg-primary-50 text-primary-700 ring-1 ring-primary-500/20 dark:border-primary-500 dark:bg-primary-900/20 dark:text-primary-300;
}

.ratio-icon {
  @apply block max-h-5 min-h-3 w-5 rounded-[3px] border-2 border-current;
}

.size-value {
  @apply min-w-16 rounded-lg bg-gray-100 px-3 py-2 text-center tabular-nums text-gray-700 dark:bg-dark-900 dark:text-dark-100;
}

.history-filter {
  @apply rounded-lg px-2 py-1.5 text-xs text-gray-500 transition dark:text-dark-300;
}

.history-filter-active {
  @apply bg-white font-medium text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white;
}

.compact-select :deep(.select-trigger) {
  @apply h-9 rounded-lg px-3 py-1.5 text-xs;
}

.ratio-panel {
  max-height: min(620px, calc(100vh - 12rem));
  overflow-y: auto;
}

details > summary::-webkit-details-marker {
  display: none;
}

.result-loading {
  aspect-ratio: 1 / 1;
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
