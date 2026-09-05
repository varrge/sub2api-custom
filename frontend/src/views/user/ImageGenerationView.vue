<template>
  <AppLayout>
    <div class="studio-shell -m-4 md:-m-6 lg:-m-8">
      <!-- Top Studio Header -->
      <header class="flex h-12 shrink-0 items-center justify-between border-b border-gray-200/70 bg-white/80 px-4 backdrop-blur-md dark:border-dark-700/70 dark:bg-dark-900/80 sm:px-6">
        <div class="flex items-center gap-3">
          <div class="flex h-7 w-7 items-center justify-center rounded-lg border border-gray-200/70 bg-white text-gray-700 shadow-sm dark:border-dark-700 dark:bg-dark-800 dark:text-dark-100">
            <Icon name="sparkles" size="sm" />
          </div>
          <h1 class="text-sm font-semibold tracking-tight text-gray-900 dark:text-white sm:text-base">
            {{ t('imageGeneration.studioTitle') }}
          </h1>
        </div>

        <!-- Mobile History Toggle -->
        <button
          v-if="imageKeys.length"
          type="button"
          class="flex items-center gap-1.5 rounded-lg border border-gray-200/80 bg-white px-2.5 py-1.5 text-xs font-medium text-gray-700 shadow-sm transition hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700 lg:hidden"
          :aria-label="t('imageGeneration.historyTitle')"
          :aria-expanded="showMobileHistory"
          aria-controls="image-studio-history"
          @click="showMobileHistory = !showMobileHistory"
        >
          <Icon name="clock" size="sm" />
          <span>{{ t('imageGeneration.historyTitle') }}</span>
          <span v-if="results.length" class="flex h-4 min-w-4 items-center justify-center rounded-full bg-primary-100 px-1 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/60 dark:text-primary-300">
            {{ results.length }}
          </span>
        </button>
      </header>

      <!-- Loading Keys State -->
      <div v-if="loadingKeys" class="flex flex-1 items-center justify-center p-8">
        <div class="flex flex-col items-center justify-center text-center">
          <LoadingSpinner size="lg" class="text-primary-500" />
          <p class="mt-4 text-xs font-medium tracking-wide text-gray-500 dark:text-dark-400">{{ t('imageGeneration.loadingKeys') }}</p>
        </div>
      </div>

      <!-- No Keys Available State -->
      <div v-else-if="!imageKeys.length" class="flex flex-1 items-center justify-center p-6 text-center">
        <div class="w-full max-w-sm rounded-2xl border border-gray-200/80 bg-white/80 p-8 shadow-sm backdrop-blur-md dark:border-dark-700/80 dark:bg-dark-900/80">
          <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-dark-800 dark:text-dark-400">
            <Icon name="key" size="lg" />
          </div>
          <h2 class="mt-4 text-base font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.noKeysTitle') }}</h2>
          <p class="mt-2 text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('imageGeneration.noKeysDescription') }}</p>
          <RouterLink to="/keys" class="btn btn-primary mt-6 w-full justify-center shadow-sm">
            <Icon name="plus" size="sm" />
            {{ t('imageGeneration.manageKeys') }}
          </RouterLink>
        </div>
      </div>

      <!-- Main Studio Workspace -->
      <div v-else class="studio-grid" :class="{ 'studio-grid-history-open': showMobileHistory }">
        <!-- Main Canvas & Composer Section -->
        <section class="studio-canvas-section">
          <!-- Canvas Top Status Bar -->
          <div v-if="activeResult" class="canvas-header">
            <div class="flex min-w-0 items-center gap-2">
              <span class="shrink-0 text-xs font-medium text-gray-600 dark:text-dark-200">
                {{ activeResult ? t('imageGeneration.imageNumber', { index: results.indexOf(activeResult) + 1 }) : t('imageGeneration.canvasTitle') }}
              </span>
              <span v-if="activeResult" class="max-w-[32vw] truncate rounded-md bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-800 dark:text-dark-400 sm:max-w-48">
                {{ activeResult.model }}
              </span>
            </div>

            <div v-if="activeResult" class="flex shrink-0 items-center gap-1 sm:gap-2">
              <button
                type="button"
                class="btn btn-ghost btn-sm text-xs"
                :title="t('imageGeneration.preview')"
                :aria-label="t('imageGeneration.previewTitle')"
                @click="preview = activeResult"
              >
                <Icon name="search" size="sm" />
                <span class="hidden sm:inline">{{ t('imageGeneration.previewTitle') }}</span>
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm text-xs"
                :aria-label="t('imageGeneration.download')"
                @click="downloadResult(activeResult)"
              >
                <Icon name="download" size="sm" />
                <span class="hidden sm:inline">{{ t('imageGeneration.download') }}</span>
              </button>
            </div>
          </div>

          <!-- Interactive Canvas Viewport -->
          <div
            class="canvas-viewport"
            @dragenter.prevent="dragging = true"
            @dragover.prevent
            @dragleave.prevent="dragging = false"
            @drop.prevent="onDrop"
          >
            <!-- Rendering / Generating State -->
            <div
              v-if="generating"
              class="grid w-full max-w-4xl gap-4 p-4"
              :class="(pendingGeneration?.count || 1) > 1 ? 'grid-cols-1 sm:grid-cols-2' : 'grid-cols-1'"
            >
              <div
                v-for="index in pendingGeneration?.count || 1"
                :key="index"
                class="result-skeleton-card"
                :style="{ aspectRatio: ratioShapes[pendingGeneration?.aspectRatio || 'auto'] }"
              >
                <div class="absolute inset-0 flex flex-col items-center justify-center gap-3">
                  <div class="flex h-11 w-11 animate-pulse items-center justify-center rounded-xl bg-white/90 text-primary-500 shadow-sm backdrop-blur dark:bg-dark-800/90 dark:text-primary-400">
                    <Icon name="sparkles" size="md" />
                  </div>
                  <span class="text-xs font-medium tracking-tight text-gray-500 dark:text-dark-300">
                    {{ t('imageGeneration.rendering', { index }) }}
                  </span>
                </div>
              </div>
            </div>

            <!-- Active Result Display -->
            <div v-else-if="activeResult" class="flex h-full w-full items-center justify-center p-4">
              <button
                type="button"
                class="group relative flex h-full w-full items-center justify-center rounded-2xl focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
                @click="preview = activeResult"
              >
                <img
                  :src="activeResult.src"
                  :alt="t('imageGeneration.resultAlt', { index: results.indexOf(activeResult) + 1 })"
                  class="max-h-full max-w-full rounded-2xl object-contain shadow-2xl shadow-gray-900/10 transition-transform duration-300 group-hover:scale-[1.01] dark:shadow-black/50"
                />
                <div class="absolute inset-0 flex items-center justify-center rounded-2xl bg-black/0 opacity-0 transition-opacity duration-200 group-hover:bg-black/20 group-hover:opacity-100">
                  <span class="flex items-center gap-1.5 rounded-full bg-white/90 px-3 py-1.5 text-xs font-medium text-gray-900 shadow-md backdrop-blur dark:bg-dark-900/90 dark:text-white">
                    <Icon name="search" size="xs" />
                    {{ t('imageGeneration.preview') }}
                  </span>
                </div>
              </button>
            </div>

            <!-- Empty Canvas State -->
            <div v-else class="flex flex-col items-center justify-center text-center">
              <div class="empty-apple-icon">
                <Icon name="sparkles" size="lg" />
              </div>
              <h3 class="mt-4 text-sm font-semibold text-gray-800 dark:text-dark-100">
                {{ t('imageGeneration.emptyTitle') }}
              </h3>
              <p class="mt-1.5 max-w-sm text-xs leading-relaxed text-gray-400 dark:text-dark-400">
                {{ t('imageGeneration.resultPlaceholder') }}
              </p>
            </div>

            <!-- Drag & Drop Overlay -->
            <Transition name="fade-fast">
              <div
                v-if="dragging"
                class="pointer-events-none absolute inset-4 z-30 flex items-center justify-center rounded-2xl border-2 border-dashed border-primary-500/80 bg-primary-50/90 backdrop-blur-sm dark:bg-primary-950/90"
              >
                <div class="flex items-center gap-2 rounded-xl bg-white/90 px-4 py-2.5 text-sm font-medium text-primary-700 shadow-sm dark:bg-dark-900/90 dark:text-primary-300">
                  <Icon name="plus" size="sm" />
                  {{ t('imageGeneration.dropReference') }}
                </div>
              </div>
            </Transition>
          </div>

          <!-- Bottom Floating Composer -->
          <div class="composer-container">
            <form
              class="composer-card"
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

              <!-- Reference Image Thumbnail Bar -->
              <div v-if="referenceImages.length" class="reference-strip">
                <span class="reference-mode-label">
                  {{ t('imageGeneration.imageToImage') }} · {{ referenceImages.length }}
                </span>
                <div
                  v-for="(image, index) in referenceImages"
                  :key="image.url"
                  class="group relative h-9 w-9 shrink-0 overflow-hidden rounded-lg border border-gray-200/80 bg-gray-100 dark:border-dark-600 dark:bg-dark-800"
                >
                  <img :src="image.url" :alt="t('imageGeneration.referenceAlt', { index: index + 1 })" class="h-full w-full object-cover" />
                  <button
                    type="button"
                    class="absolute right-1 top-1 flex h-6 w-6 items-center justify-center rounded-full bg-gray-950/80 text-white shadow-sm transition hover:bg-red-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-white/80"
                    :aria-label="t('imageGeneration.removeReference')"
                    @click="removeReference(index)"
                  >
                    <Icon name="x" size="xs" />
                  </button>
                </div>
                <button
                  v-if="referenceImages.length < 4"
                  type="button"
                  class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-dashed border-gray-300 text-gray-400 transition hover:border-gray-500 hover:bg-gray-50 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-900/20 dark:border-dark-600 dark:hover:border-dark-400 dark:hover:bg-dark-800 dark:hover:text-dark-100 dark:focus-visible:ring-white/30"
                  :aria-label="t('imageGeneration.uploadReference')"
                  @click="fileInput?.click()"
                >
                  <Icon name="plus" size="sm" />
                </button>
              </div>

              <!-- Prompt Input Area -->
              <div class="relative px-4 pb-2 pt-3.5 sm:px-5 sm:pb-3 sm:pt-4">
                <textarea
                  id="image-prompt"
                  v-model="form.prompt"
                  maxlength="4000"
                  rows="3"
                  class="w-full resize-none bg-transparent text-[15px] leading-6 text-gray-900 outline-none placeholder:text-gray-400 focus:outline-none dark:text-gray-100 dark:placeholder:text-dark-400"
                  :placeholder="t('imageGeneration.promptPlaceholder')"
                  :aria-label="t('imageGeneration.prompt')"
                  @paste="onPaste"
                ></textarea>
              </div>

              <!-- Bottom Controls Toolbar -->
              <div class="composer-toolbar">
                <div class="composer-settings">
                  <!-- Adding a reference image automatically switches to image-to-image mode. -->
                  <button
                    type="button"
                    class="composer-icon-button"
                    :class="referenceImages.length >= 4 && 'invisible'"
                    :disabled="referenceImages.length >= 4"
                    :aria-label="t('imageGeneration.uploadReference')"
                    :title="t('imageGeneration.uploadReference')"
                    @click="fileInput?.click()"
                  >
                    <Icon name="upload" size="sm" />
                  </button>

                  <!-- Key Selector -->
                  <Select
                    id="image-api-key"
                    v-model="form.apiKeyId"
                    :options="keyOptions"
                    :aria-label="t('imageGeneration.apiKey')"
                    class="composer-select composer-key-select"
                  />

                  <!-- Model Selector -->
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
                    class="composer-select composer-model-select"
                  />

                  <!-- Aspect Ratio & Resolution Popover -->
                  <details ref="ratioDetails" class="group relative">
                    <summary class="apple-pill-btn list-none">
                      <span class="ratio-indicator" :style="{ aspectRatio: selectedRatioCss }"></span>
                      <span>{{ ratioLabel }}</span>
                      <Icon name="chevronDown" size="xs" class="transition-transform duration-200 group-open:rotate-180" />
                    </summary>

                    <!-- Ratio Dropdown Panel -->
                    <div class="ratio-popover-panel">
                        <!-- Ratio Section -->
                        <div class="mb-4">
                          <div class="flex items-center justify-between">
                            <span class="text-xs font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.aspectRatio') }}</span>
                            <span class="text-[10px] text-gray-400 dark:text-dark-400">{{ ratioOptions.length }} {{ t('imageGeneration.aspectRatio') }}</span>
                          </div>
                          <div class="mt-2.5 grid grid-cols-3 gap-1.5">
                            <button
                              v-for="ratio in ratioOptions"
                              :key="ratio.value"
                              type="button"
                              class="ratio-card"
                              :class="form.aspectRatio === ratio.value && 'ratio-card-active'"
                              :aria-pressed="form.aspectRatio === ratio.value"
                              @click="selectRatio(ratio.value)"
                            >
                              <span class="ratio-box" :style="{ aspectRatio: ratio.cssRatio }"></span>
                              <span class="text-[11px] font-medium">{{ ratio.value === 'auto' ? t('imageGeneration.auto') : ratio.value }}</span>
                            </button>
                          </div>
                        </div>

                        <!-- Quality / Resolution Section -->
                        <div class="mb-4 border-t border-gray-100 pt-3 dark:border-dark-700">
                          <span class="text-xs font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.resolution') }}</span>
                          <div class="mt-2 grid grid-cols-2 gap-2">
                            <button
                              v-for="quality in qualities"
                              :key="quality.value"
                              type="button"
                              class="quality-card"
                              :class="form.quality === quality.value && 'quality-card-active'"
                              :aria-pressed="form.quality === quality.value"
                              @click="form.quality = quality.value"
                            >
                              <span class="font-medium text-xs">{{ quality.label }}</span>
                              <span class="text-[10px] text-gray-400 dark:text-dark-400">
                                {{ quality.value === 'high' ? t('imageGeneration.qualityHigh') : t('imageGeneration.qualityStandard') }}
                              </span>
                            </button>
                          </div>
                        </div>

                        <!-- Pixel Dimensions Preview -->
                        <div class="border-t border-gray-100 pt-3 dark:border-dark-700">
                          <div class="flex items-center justify-between text-xs">
                            <span class="font-medium text-gray-500 dark:text-dark-300">{{ t('imageGeneration.size') }}</span>
                            <div class="flex items-center gap-1.5 font-mono text-[11px] text-gray-700 dark:text-dark-200">
                              <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-700">{{ displaySize.width }}</span>
                              <span class="text-gray-400">×</span>
                              <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-700">{{ displaySize.height }}</span>
                              <span class="text-[10px] text-gray-400">px</span>
                            </div>
                          </div>
                        </div>
                    </div>
                  </details>

                  <!-- Count Selector -->
                  <Select
                    v-model="form.count"
                    :options="countOptions"
                    :aria-label="t('imageGeneration.count')"
                    class="composer-select composer-count-select"
                  />

                  <!-- Submit Generation Button -->
                  <button
                    type="submit"
                    class="generate-button"
                    :disabled="generating || !form.model || !form.prompt.trim()"
                  >
                    <LoadingSpinner v-if="generating" size="sm" class="text-current" />
                    <Icon v-else name="sparkles" size="sm" />
                    <span>{{ generating ? t('imageGeneration.generating') : t('imageGeneration.generate') }}</span>
                  </button>
                </div>
              </div>

            </form>
            <p class="composer-note">{{ t('imageGeneration.requestHint') }}</p>
          </div>
        </section>

        <!-- Right History Column (desktop) / workspace panel (mobile) -->
        <aside
          id="image-studio-history"
          class="history-sidebar"
          :class="[showMobileHistory ? 'history-sidebar-mobile-open' : 'history-sidebar-mobile-closed']"
        >
          <!-- History Header -->
          <div class="border-b border-gray-100/80 px-4 py-3.5 dark:border-dark-700/80">
            <div class="flex items-center justify-between">
              <div>
                <h2 class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-300">
                  {{ t('imageGeneration.historyTitle') }}
                </h2>
                <p class="mt-0.5 text-[11px] text-gray-400 dark:text-dark-400">
                  {{ t('imageGeneration.historySession') }}
                </p>
              </div>
              <div class="flex items-center gap-1">
                <button
                  v-if="results.length"
                  type="button"
                  class="btn btn-ghost h-8 w-8 p-0 text-gray-400 hover:text-red-500 dark:text-dark-400 dark:hover:text-red-400"
                  :aria-label="t('imageGeneration.clear')"
                  :title="t('imageGeneration.clear')"
                  @click="clearResults"
                >
                  <Icon name="trash" size="xs" />
                </button>
                <button
                  type="button"
                  class="btn btn-ghost h-8 w-8 p-0 text-gray-400 lg:hidden"
                  :aria-label="t('common.close')"
                  @click="showMobileHistory = false"
                >
                  <Icon name="x" size="xs" />
                </button>
              </div>
            </div>

            <!-- Filter Segmented Control -->
            <div class="mt-3 grid grid-cols-3 gap-1 rounded-lg bg-gray-100 p-0.5 dark:bg-dark-800">
              <button
                v-for="filter in historyFilters"
                :key="filter.value"
                type="button"
                class="rounded-md py-1 text-[11px] font-medium transition"
                :class="historyFilter === filter.value
                  ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
                  : 'text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200'"
                @click="historyFilter = filter.value"
              >
                {{ filter.label }}
              </button>
            </div>
          </div>

          <!-- History Item List -->
          <div class="min-h-0 flex-1 overflow-y-auto p-3 space-y-2">
            <div v-if="!filteredResults.length" class="flex min-h-[200px] flex-col items-center justify-center text-center">
              <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-gray-100 text-gray-400 dark:bg-dark-800 dark:text-dark-400">
                <Icon name="clock" size="sm" />
              </div>
              <p class="mt-2 text-xs text-gray-400 dark:text-dark-400">{{ t('imageGeneration.noHistory') }}</p>
            </div>

            <article
              v-for="(result, index) in filteredResults"
              :key="result.id"
              class="history-item-card"
              :class="activeResult?.id === result.id ? 'history-item-card-active' : 'history-item-card-idle'"
            >
              <button
                type="button"
                class="flex min-w-0 flex-1 items-center gap-2.5 rounded-lg text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
                :aria-pressed="activeResult?.id === result.id"
                @click="activeResult = result; showMobileHistory = false"
              >
                <div class="h-14 w-14 shrink-0 overflow-hidden rounded-lg bg-gray-100 dark:bg-dark-800">
                  <img :src="result.src" :alt="t('imageGeneration.resultAlt', { index: index + 1 })" class="h-full w-full object-cover" />
                </div>

                <div class="min-w-0 flex-1">
                  <p class="truncate text-xs font-semibold text-gray-800 dark:text-dark-100">{{ result.model }}</p>
                  <div class="mt-1 flex items-center gap-1.5 text-[10px] text-gray-400 dark:text-dark-400">
                    <span class="rounded bg-gray-100 px-1 py-0.5 dark:bg-dark-700">
                      {{ result.mode === 'image' ? t('imageGeneration.imageToImage') : t('imageGeneration.textToImage') }}
                    </span>
                    <span>{{ result.createdAt.split(' ')[1] || result.createdAt }}</span>
                  </div>
                </div>
              </button>

              <button
                type="button"
                class="self-center rounded-lg p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-dark-200"
                :aria-label="t('imageGeneration.download')"
                @click.stop="downloadResult(result)"
              >
                <Icon name="download" size="xs" />
              </button>
            </article>
          </div>
        </aside>

      </div>

      <!-- Preview Lightbox Dialog -->
      <BaseDialog
        :show="Boolean(preview)"
        :title="t('imageGeneration.previewTitle')"
        width="extra-wide"
        close-on-click-outside
        @close="preview = null"
      >
        <div v-if="preview" class="space-y-4">
          <div class="flex max-h-[70vh] items-center justify-center overflow-hidden rounded-2xl bg-checker p-2">
            <img :src="preview.src" :alt="t('imageGeneration.previewTitle')" class="max-h-[68vh] max-w-full rounded-xl object-contain shadow-sm" />
          </div>
          <div v-if="preview.revisedPrompt" class="rounded-xl bg-gray-50/80 p-3 text-xs leading-relaxed text-gray-600 dark:bg-dark-800/80 dark:text-dark-200">
            <p class="font-semibold text-gray-700 dark:text-dark-100 mb-1">{{ t('imageGeneration.prompt') }}:</p>
            {{ preview.revisedPrompt }}
          </div>
        </div>
        <template #footer>
          <div class="flex w-full justify-between items-center">
            <button type="button" class="btn btn-ghost btn-sm" @click="preview = null">
              {{ t('common.close') }}
            </button>
            <button type="button" class="btn btn-primary btn-sm" @click="preview && downloadResult(preview)">
              <Icon name="download" size="sm" />
              {{ t('imageGeneration.download') }}
            </button>
          </div>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { keysAPI } from '@/api/keys'
import {
  generateImages,
  listImageModels,
  supportedImageAspectRatios,
  supportsImageGeneration,
  type GenerateImageOptions,
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
const showMobileHistory = ref(false)
const dragging = ref(false)
const pendingGeneration = shallowRef<GenerateImageOptions | null>(null)
const generating = computed(() => pendingGeneration.value !== null)
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
  if (generating.value) return

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

  const request: GenerateImageOptions = {
    apiKey: key.key,
    platform: key.group.platform,
    model: form.model.trim(),
    prompt,
    aspectRatio: form.aspectRatio,
    quality: form.quality,
    count: form.count,
    referenceImages: referenceImages.value.map(image => image.file),
  }
  pendingGeneration.value = request
  try {
    const mode: GenerationMode = request.referenceImages.length ? 'image' : 'text'
    const generated = await generateImages(request)
    if (!generated.length) throw new Error(t('imageGeneration.noImageReturned'))

    const now = new Date().toLocaleString()
    const newResults = generated.map((image, index): ImageResult => ({
      ...image,
      id: `${Date.now()}-${index}`,
      model: request.model,
      createdAt: now,
      aspectRatio: request.aspectRatio === 'auto' ? 'auto' : ratioShapes[request.aspectRatio],
      mode,
    }))
    results.value.unshift(...newResults)
    activeResult.value = newResults[0]
    appStore.showSuccess(t('imageGeneration.generated', { count: generated.length }))
  } catch (error) {
    appStore.showError(errorMessage(error, t('imageGeneration.generateFailed')))
  } finally {
    pendingGeneration.value = null
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
  @apply flex min-h-[28rem] flex-col overflow-hidden bg-[#f7f7f6] dark:bg-[#0b0d13];
  height: calc(100dvh - 4rem - 1px);
}

.studio-grid {
  @apply grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_280px] lg:overflow-hidden;
}

/* Canvas Viewport */
.studio-canvas-section {
  @apply relative flex min-h-0 min-w-0 flex-col;
}

.canvas-header {
  @apply flex h-10 shrink-0 items-center justify-between border-b border-gray-200/60 bg-white/50 px-5 backdrop-blur-sm dark:border-dark-700/60 dark:bg-dark-900/50;
}

.canvas-viewport {
  @apply relative flex min-h-0 flex-1 items-center justify-center overflow-auto p-4 sm:p-6;
  background-image: radial-gradient(rgba(0, 0, 0, 0.025) 1px, transparent 1px);
  background-size: 24px 24px;
}

.dark .canvas-viewport {
  background-image: radial-gradient(rgba(255, 255, 255, 0.03) 1px, transparent 1px);
}

.empty-apple-icon {
  @apply flex h-14 w-14 items-center justify-center rounded-2xl border border-gray-200/70 bg-white/70 text-gray-400 shadow-sm transition-transform duration-300 dark:border-dark-700 dark:bg-dark-800/70 dark:text-dark-400;
}

/* Composer */
.composer-container {
  @apply pointer-events-none shrink-0 px-3 pb-3 sm:px-6 sm:pb-5;
}

.composer-card {
  @apply pointer-events-auto mx-auto w-full max-w-3xl overflow-visible rounded-2xl border border-gray-200/80 bg-white/90 shadow-lg shadow-gray-900/[0.06] backdrop-blur-xl transition dark:border-dark-700/80 dark:bg-dark-900/90 dark:shadow-black/30;
  container: image-composer / inline-size;
}

.composer-toolbar {
  @apply flex items-center gap-2 border-t border-gray-100/80 px-2.5 py-2 dark:border-dark-700/80 sm:px-3;
}

.composer-settings {
  @apply grid min-w-0 flex-1 items-center gap-1.5;
  grid-template-columns: auto minmax(0, 1fr) minmax(0, 1fr);
}

.composer-icon-button {
  @apply flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-100/80 text-gray-500 transition hover:bg-gray-200/70 hover:text-gray-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-900/20 dark:bg-dark-800 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white dark:focus-visible:ring-white/30;
}

.reference-strip {
  @apply flex items-center gap-1.5 overflow-x-auto border-b border-gray-100/80 px-4 py-2 dark:border-dark-700/80 sm:px-5;
  scrollbar-width: none;
}

.reference-strip::-webkit-scrollbar {
  display: none;
}

.reference-mode-label {
  @apply mr-1 shrink-0 text-[10px] font-medium uppercase tracking-wide text-gray-400 dark:text-dark-400;
}

.composer-note {
  @apply pointer-events-auto mx-auto mt-1.5 max-w-3xl px-2 text-center text-[11px] text-gray-500 dark:text-dark-400;
}

/* Apple Pill Control Button */
.apple-pill-btn {
  @apply flex h-8 cursor-pointer items-center gap-1.5 rounded-lg bg-gray-100/80 px-2.5 text-xs font-medium text-gray-600 transition hover:bg-gray-200/70 hover:text-gray-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-900/20 dark:bg-dark-800 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white dark:focus-visible:ring-white/30;
}

.ratio-indicator {
  @apply block max-h-3.5 min-h-2 w-3.5 rounded-[2px] border-[1.5px] border-current opacity-80;
}

/* Ratio Popover Panel */
.ratio-popover-panel {
  @apply absolute bottom-[calc(100%+10px)] left-0 z-50 w-[min(380px,calc(100cqw-1.5rem))] rounded-2xl border border-gray-200/80 bg-white/95 p-4 shadow-xl shadow-gray-900/10 backdrop-blur-xl dark:border-dark-700 dark:bg-dark-900/95 dark:shadow-black/50;
  max-height: min(540px, calc(100vh - 14rem));
  overflow-y: auto;
}

@container image-composer (min-width: 42rem) {
  .ratio-popover-panel {
    @apply left-auto right-0;
  }
}

details > summary::-webkit-details-marker {
  display: none;
}

.ratio-card {
  @apply flex flex-col items-center justify-center gap-1.5 rounded-xl border border-gray-200/70 bg-gray-50/50 p-2 text-center text-gray-600 transition hover:border-gray-300 hover:bg-white focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:border-dark-700 dark:bg-dark-800/50 dark:text-dark-300 dark:hover:border-dark-600 dark:hover:bg-dark-800;
}

.ratio-card-active {
  @apply border-primary-500 bg-primary-50/70 text-primary-700 ring-1 ring-primary-500/30 dark:border-primary-500 dark:bg-primary-950/40 dark:text-primary-300;
}

.ratio-box {
  @apply block max-h-5 min-h-2.5 w-5 rounded-[2px] border-[1.5px] border-current opacity-80;
}

.quality-card {
  @apply flex flex-col items-center justify-center rounded-xl border border-gray-200/70 bg-gray-50/50 py-2 text-center transition hover:border-gray-300 hover:bg-white focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:border-dark-700 dark:bg-dark-800/50 dark:hover:border-dark-600 dark:hover:bg-dark-800;
}

.quality-card-active {
  @apply border-primary-500 bg-primary-50/70 text-primary-700 ring-1 ring-primary-500/30 dark:border-primary-500 dark:bg-primary-950/40 dark:text-primary-300;
}

/* History Sidebar */
.history-sidebar {
  @apply flex min-h-0 flex-col border-t border-gray-200/70 bg-white/55 backdrop-blur-md dark:border-dark-700/70 dark:bg-dark-900/55 lg:border-l lg:border-t-0;
}

@media (max-width: 1023px) {
  .history-sidebar-mobile-closed {
    @apply hidden;
  }
  .history-sidebar-mobile-open {
    @apply flex;
  }
  .studio-grid-history-open .studio-canvas-section {
    @apply hidden;
  }
}

.history-item-card {
  @apply flex gap-2.5 rounded-xl border p-2 transition-all duration-150;
}

.history-item-card-idle {
  @apply border-transparent hover:border-gray-200 hover:bg-gray-50/80 dark:hover:border-dark-700 dark:hover:bg-dark-800/60;
}

.history-item-card-active {
  @apply border-primary-500/60 bg-primary-50/80 shadow-sm dark:border-primary-500/60 dark:bg-primary-950/40;
}

/* Drag overlay transition */
.fade-fast-enter-active,
.fade-fast-leave-active {
  transition: opacity 0.15s ease;
}

.fade-fast-enter-from,
.fade-fast-leave-to {
  opacity: 0;
}

/* Custom Select styling inside composer */
.composer-select :deep(.select-trigger) {
  @apply h-8 rounded-lg border-transparent bg-gray-100/80 px-2.5 py-1 text-xs shadow-none hover:border-gray-200 hover:bg-gray-100 dark:border-transparent dark:bg-dark-800 dark:hover:border-dark-700 dark:hover:bg-dark-700;
}

.generate-button {
  @apply flex h-8 shrink-0 items-center justify-center gap-1.5 justify-self-end whitespace-nowrap rounded-lg bg-gray-900 px-3.5 text-xs font-semibold text-white shadow-sm transition hover:bg-black focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-900/30 focus-visible:ring-offset-2 active:scale-[0.98] disabled:cursor-not-allowed disabled:bg-gray-300 disabled:text-gray-500 disabled:shadow-none disabled:active:scale-100 dark:bg-white dark:text-gray-950 dark:hover:bg-gray-100 dark:focus-visible:ring-white/40 dark:focus-visible:ring-offset-dark-900 dark:disabled:bg-dark-700 dark:disabled:text-dark-400;
}

@media (max-width: 767px) {
  .composer-key-select,
  .composer-model-select,
  .composer-count-select {
    @apply w-full min-w-0;
  }
}

@container image-composer (min-width: 42rem) {
  .composer-settings {
    grid-template-columns: 2rem 10rem minmax(8rem, 1fr) auto 4.25rem auto;
  }
}

/* Skeleton loader */
.result-skeleton-card {
  @apply relative mx-auto w-full max-w-xl overflow-hidden rounded-2xl border border-gray-200/90 bg-gray-100 shadow-sm dark:border-dark-700 dark:bg-dark-800;
}

.result-skeleton-card::before {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(90deg, transparent 0%, rgba(255, 255, 255, 0.6) 50%, transparent 100%);
  animation: shimmer 1.8s infinite;
}

.dark .result-skeleton-card::before {
  background: linear-gradient(90deg, transparent 0%, rgba(255, 255, 255, 0.07) 50%, transparent 100%);
}

@keyframes shimmer {
  100% {
    transform: translateX(100%);
  }
}

/* Transparency Checkerboard */
.bg-checker {
  background-color: rgb(249 250 251);
  background-image: linear-gradient(45deg, rgb(229 231 235 / 45%) 25%, transparent 25%),
    linear-gradient(-45deg, rgb(229 231 235 / 45%) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgb(229 231 235 / 45%) 75%),
    linear-gradient(-45deg, transparent 75%, rgb(229 231 235 / 45%) 75%);
  background-size: 20px 20px;
  background-position: 0 0, 0 10px, 10px -10px, -10px 0;
}

.dark .bg-checker {
  background-color: rgb(17 24 39);
  background-image: linear-gradient(45deg, rgb(55 65 81 / 35%) 25%, transparent 25%),
    linear-gradient(-45deg, rgb(55 65 81 / 35%) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgb(55 65 81 / 35%) 75%),
    linear-gradient(-45deg, transparent 75%, rgb(55 65 81 / 35%) 75%);
}

@media (prefers-reduced-motion: reduce) {
  .result-skeleton-card::before {
    animation: none;
  }
  .fade-fast-enter-active,
  .fade-fast-leave-active {
    transition: none;
  }
}
</style>
