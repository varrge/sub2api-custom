<template>
  <div class="video-panel">
    <div v-if="loadingKeys || !studio.ready" class="flex flex-1 flex-col items-center justify-center gap-3 p-8">
      <LoadingSpinner size="lg" />
      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('videoGeneration.loading') }}</p>
    </div>
    <div v-else-if="!videoKeys.length && !studio.sessions.some(session => session.results.length || session.pending)" class="flex flex-1 items-center justify-center p-6 text-center">
      <div class="max-w-sm">
        <Icon name="key" size="xl" class="mx-auto text-gray-400" />
        <h2 class="mt-4 font-semibold text-gray-900 dark:text-white">{{ t('videoGeneration.noKeysTitle') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('videoGeneration.noKeysDescription') }}</p>
        <RouterLink to="/keys" class="btn btn-primary mt-5">{{ t('imageGeneration.manageKeys') }}</RouterLink>
      </div>
    </div>
    <template v-else>
      <div class="flex shrink-0 items-center justify-between border-b border-gray-200/60 px-4 py-2 dark:border-dark-700/60">
        <p class="text-[11px] text-gray-500 dark:text-dark-400">{{ t('videoGeneration.providers') }}</p>
        <button ref="historyToggle" type="button" class="btn btn-ghost btn-sm lg:hidden" :aria-expanded="showMobileHistory" aria-controls="video-studio-history" @click="showMobileHistory = !showMobileHistory">
          <Icon name="clock" size="sm" />{{ t('videoGeneration.historyTitle') }}
        </button>
      </div>
      <div class="video-grid" :class="{ 'video-history-open': showMobileHistory }">
        <section class="video-workspace">
          <p v-if="studio.storageError" role="alert" class="video-notice">{{ storageErrorText }}</p>
          <p v-if="studio.activeSession?.interrupted" role="status" class="video-notice">{{ t('videoGeneration.sessionInterrupted') }}</p>
          <p v-if="sessionError" role="alert" class="shrink-0 bg-red-50 px-4 py-2 text-xs text-red-700 dark:bg-red-950 dark:text-red-200">{{ sessionError }}</p>
          <div class="video-canvas">
            <div v-if="pending" class="flex max-w-md flex-col items-center gap-3 p-6 text-center" role="status" aria-live="polite">
              <LoadingSpinner v-if="generating" size="lg" />
              <Icon v-else name="clock" size="xl" class="text-amber-500" />
              <h2 class="text-base font-semibold text-gray-800 dark:text-white">{{ statusLabel }}</h2>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ pending.model }} · {{ pending.duration }}s · {{ pending.resolution }}</p>
              <p v-if="pending.task" class="max-w-full break-all font-mono text-[11px] text-gray-400">{{ t('videoGeneration.taskId') }}: {{ pending.task.id }}</p>
              <p class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('videoGeneration.backgroundHint') }}</p>
              <button v-if="pending.phase === 'paused'" type="button" class="btn btn-secondary btn-sm" @click="resumePolling">
                <Icon name="refresh" size="sm" />{{ t('videoGeneration.resume') }}
              </button>
            </div>
            <div v-else-if="activeResult" class="flex h-full min-h-0 w-full flex-col">
              <div class="flex min-w-0 shrink-0 items-center justify-between gap-2 pb-2">
                <div class="min-w-0 text-xs text-gray-500 dark:text-dark-400">
                  <p class="truncate font-medium text-gray-800 dark:text-dark-100">{{ activeResult.model }}</p>
                  <p class="mt-0.5">{{ activeResult.duration }}s · {{ activeResult.resolution }} · {{ activeResult.aspectRatio }}</p>
                </div>
                <button type="button" class="btn btn-secondary btn-sm shrink-0" :aria-label="t('videoGeneration.download')" :disabled="downloading || loadingPlayback" @click="downloadResult">
                  <Icon name="download" size="sm" /><span class="hidden sm:inline">{{ t('videoGeneration.download') }}</span>
                </button>
              </div>
              <div class="relative flex min-h-0 flex-1 items-center justify-center overflow-hidden rounded-xl bg-gray-950">
                <video v-if="playbackUrl" :key="activeResult.id" :src="playbackUrl" controls playsinline preload="metadata" :aria-label="activeResult.prompt" class="h-full min-h-0 w-full object-contain" @error="onPlaybackError"></video>
                <div v-else class="flex flex-col items-center gap-3 p-5 text-center">
                  <LoadingSpinner v-if="loadingPlayback" size="lg" />
                  <Icon v-else name="play" size="xl" class="text-gray-500" />
                  <p class="text-xs text-gray-300">{{ loadingPlayback ? t('videoGeneration.loadingVideo') : playbackError }}</p>
                  <button v-if="!loadingPlayback" type="button" class="btn btn-secondary btn-sm" @click="loadPlayback">{{ t('videoGeneration.retryPlayback') }}</button>
                </div>
              </div>
              <p class="mt-2 line-clamp-2 shrink-0 text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ activeResult.prompt }}</p>
            </div>
            <div v-else class="flex max-w-sm flex-col items-center p-5 text-center">
              <div class="mb-5 flex h-16 w-20 items-center justify-center rounded-2xl border border-gray-200 bg-gray-50 text-primary-500 shadow-sm dark:border-dark-700 dark:bg-dark-800">
                <Icon name="play" size="xl" />
              </div>
              <h2 class="text-lg font-semibold text-gray-800 dark:text-white">{{ t('videoGeneration.emptyTitle') }}</h2>
              <p class="mt-2 text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('videoGeneration.emptyDescription') }}</p>
            </div>
          </div>

          <form class="video-composer" @submit.prevent="submitGeneration" @dragover.prevent @drop.prevent="onDrop">
            <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp" class="sr-only" @change="onReferenceChange" />
            <div v-if="referenceUrl" class="flex items-center gap-3 border-b border-gray-100 px-3 py-2 dark:border-dark-700">
              <img :src="referenceUrl" :alt="t('videoGeneration.referenceAlt')" class="h-10 w-14 rounded-lg object-cover" />
              <span class="flex-1 text-xs text-gray-500 dark:text-dark-400">{{ t('videoGeneration.imageToVideo') }}</span>
              <button type="button" class="btn btn-ghost btn-sm" :aria-label="t('videoGeneration.removeReference')" @click="removeReference"><Icon name="x" size="sm" /></button>
            </div>
            <label for="video-prompt" class="sr-only">{{ t('videoGeneration.prompt') }}</label>
            <textarea id="video-prompt" v-model="form.prompt" rows="2" :placeholder="t('videoGeneration.promptPlaceholder')" class="video-prompt"></textarea>
            <div class="video-controls">
              <div class="grid min-w-0 grid-cols-2 gap-2">
                <div class="min-w-0">
                  <label for="video-api-key" class="sr-only">{{ t('imageGeneration.apiKey') }}</label>
                  <Select id="video-api-key" v-model="form.apiKeyId" :options="keyOptions" :aria-label="t('imageGeneration.apiKey')" />
                </div>
                <div class="min-w-0">
                  <label for="video-model" class="sr-only">{{ t('imageGeneration.model') }}</label>
                  <Select id="video-model" v-model="form.model" :options="modelOptions" searchable creatable :placeholder="loadingModels ? t('imageGeneration.loadingModels') : t('videoGeneration.modelPlaceholder')" :aria-label="t('imageGeneration.model')" />
                </div>
              </div>
              <p v-if="modelError" role="status" class="text-xs text-amber-700 dark:text-amber-300">{{ modelError }}</p>
              <div class="grid grid-cols-3 gap-2">
                <div class="min-w-0">
                  <label for="video-ratio" class="video-label">{{ t('imageGeneration.aspectRatio') }}</label>
                  <Select id="video-ratio" v-model="form.aspectRatio" :options="ratioOptions" :aria-label="t('imageGeneration.aspectRatio')" />
                </div>
                <div class="min-w-0">
                  <label for="video-duration" class="video-label">{{ t('videoGeneration.duration') }}</label>
                  <Select id="video-duration" v-model="form.duration" :options="durationOptions" :aria-label="t('videoGeneration.duration')" />
                </div>
                <div class="min-w-0">
                  <label for="video-resolution" class="video-label">{{ t('imageGeneration.resolution') }}</label>
                  <Select id="video-resolution" v-model="form.resolution" :options="resolutionOptions" :aria-label="t('imageGeneration.resolution')" />
                </div>
              </div>
              <div class="flex min-w-0 items-center justify-between gap-2">
                <button type="button" class="btn btn-ghost btn-sm min-w-0" :title="t('videoGeneration.uploadHint')" @click="fileInput?.click()">
                  <Icon name="upload" size="sm" /><span class="truncate">{{ t('videoGeneration.uploadReference') }}</span>
                </button>
                <button type="submit" class="btn btn-primary btn-sm shrink-0" :disabled="!canGenerate">
                  <LoadingSpinner v-if="generating" size="sm" /><Icon v-else name="play" size="sm" />
                  {{ t(generating ? 'videoGeneration.generating' : 'videoGeneration.generate') }}
                </button>
              </div>
              <p v-if="requiresReference && !referenceUrl" role="status" class="text-xs text-amber-700 dark:text-amber-300">{{ t('videoGeneration.requiresReference') }}</p>
            </div>
          </form>
          <p class="video-footnote">{{ t('videoGeneration.requestHint') }}</p>
        </section>

        <aside id="video-studio-history" class="video-history" :class="{ 'video-history-visible': showMobileHistory }">
          <div class="shrink-0 border-b border-gray-100 p-4 dark:border-dark-700">
            <div class="flex items-center justify-between">
              <h2 class="text-xs font-semibold text-gray-600 dark:text-dark-200">{{ t('videoGeneration.historyTitle') }}</h2>
              <button type="button" class="btn btn-ghost btn-sm lg:hidden" :aria-label="t('common.close')" @click="closeHistory"><Icon name="x" size="sm" /></button>
            </div>
            <p class="mt-1 text-[11px] text-gray-400">{{ t('videoGeneration.historyHint') }}</p>
            <button type="button" class="btn btn-secondary btn-sm mt-3 w-full justify-center" @click="newSession"><Icon name="plus" size="sm" />{{ t('imageGeneration.newSession') }}</button>
          </div>
          <div class="max-h-44 shrink-0 space-y-1 overflow-y-auto border-b border-gray-100 p-2 dark:border-dark-700" data-testid="video-sessions">
            <div v-for="session in studio.sessions" :key="session.id" class="flex items-center gap-1 rounded-lg" :class="session.id === studio.activeId ? 'bg-primary-50 dark:bg-primary-500/10' : ''">
              <button type="button" class="min-w-0 flex-1 px-2 py-2 text-left" :aria-pressed="session.id === studio.activeId" @click="chooseSession(session.id)">
                <span class="block truncate text-xs font-medium text-gray-800 dark:text-dark-100">{{ session.title || t('imageGeneration.untitledSession') }}</span>
                <span class="block text-[10px] text-gray-500 dark:text-dark-400">{{ session.pending ? t('videoGeneration.generating') : t('videoGeneration.sessionVideoCount', { count: session.results.length }) }}</span>
              </button>
              <button type="button" class="btn btn-ghost btn-sm shrink-0 px-2" :disabled="Boolean(session.pending)" :aria-label="t('imageGeneration.deleteSession')" @click="deletingSessionId = session.id"><Icon name="trash" size="xs" /></button>
            </div>
          </div>
          <div class="min-h-0 flex-1 space-y-2 overflow-y-auto p-3">
            <p v-if="!results.length" class="py-10 text-center text-xs text-gray-400">{{ t('imageGeneration.noHistory') }}</p>
            <button v-for="result in results" :key="result.id" type="button" class="video-result" :aria-pressed="activeResult?.id === result.id" @click="chooseResult(result)">
              <span class="flex h-11 w-14 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-400 dark:bg-dark-800"><Icon name="play" size="sm" /></span>
              <span class="min-w-0">
                <span class="block truncate text-xs font-medium text-gray-800 dark:text-dark-100">{{ result.prompt }}</span>
                <span class="mt-1 block truncate text-[10px] text-gray-500 dark:text-dark-400">{{ result.model }} · {{ result.duration }}s · {{ result.resolution }}</span>
                <span class="mt-0.5 block text-[10px] text-gray-400">{{ result.createdAt }}</span>
              </span>
            </button>
          </div>
        </aside>
      </div>
    </template>
    <ConfirmDialog :show="Boolean(deletingSessionId)" :title="t('imageGeneration.deleteSession')" :message="t('videoGeneration.deleteSessionConfirm')" @confirm="deleteSession" @cancel="deletingSessionId = null" />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { fetchVideoBlob, getVideoTask, listVideoModels, supportedVideoResolutions, supportsVideoGeneration, videoContentURL, videoProtocolForModel, videoRequiresReference, type VideoAspectRatio } from '@/api/videoGeneration'
import { ConfirmDialog, LoadingSpinner } from '@/components/common'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useVideoStudioStore, type VideoStudioForm, type VideoStudioResult } from '@/stores/videoStudio'
import type { ApiKey, GroupPlatform } from '@/types'

const props = defineProps<{ apiKeys: ApiKey[]; loadingKeys: boolean }>()
const { t } = useI18n()
const app = useAppStore()
const studio = useVideoStudioStore()
const showMobileHistory = ref(false)
const historyToggle = ref<HTMLButtonElement | null>(null)
const deletingSessionId = ref<string | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const referenceUrl = ref('')
const availableModels = ref<string[]>([])
const loadingModels = ref(false)
const modelError = ref('')
const playbackUrl = ref('')
const playbackError = ref('')
const loadingPlayback = ref(false)
const downloading = ref(false)
let modelRequestId = 0
let playbackRequestId = 0
let playbackController: AbortController | null = null
let downloadController: AbortController | null = null
let playbackExtension = 'mp4'
let disposed = false

const emptyForm: VideoStudioForm = { apiKeyId: null, model: '', prompt: '', duration: 5, aspectRatio: '16:9', resolution: '720p' }
const form = computed(() => studio.activeSession?.form ?? emptyForm)
const pending = computed(() => studio.activeSession?.pending ?? null)
const generating = computed(() => Boolean(pending.value && pending.value.phase !== 'paused'))
const results = computed(() => studio.activeSession?.results ?? [])
const activeResult = computed(() => results.value.find(result => result.id === studio.activeSession?.activeResultId) ?? null)
const message = (error: unknown, fallback: string) => {
  const text = error instanceof Error ? error.message : typeof error === 'string' ? error : fallback
  return text.startsWith('videoGeneration.') ? t(text) : text
}
const sessionError = computed(() => message(studio.activeSession?.error || '', 'videoGeneration.generateFailed'))
const storageErrorText = computed(() => t(studio.storageError === 'conflict' ? 'videoGeneration.storageConflict' : 'videoGeneration.storageError'))
const statusLabel = computed(() => t(`videoGeneration.${pending.value?.phase === 'submitting' ? 'submitting' : pending.value?.phase === 'paused' ? 'paused' : pending.value?.task?.status === 'queued' ? 'queued' : 'running'}`))

function eligibleGroups(key: ApiKey) {
  return (key.groups?.length ? key.groups : key.group ? [key.group] : [])
    .filter(group => group.allow_image_generation && supportsVideoGeneration(group.platform))
}
const videoKeys = computed(() => props.apiKeys.filter(key => key.status === 'active' && eligibleGroups(key).length))
const selectedKey = computed(() => videoKeys.value.find(key => key.id === form.value.apiKeyId) ?? null)
const keyOptions = computed(() => videoKeys.value.map(key => ({
  value: key.id, label: `${key.name} · ${eligibleGroups(key).map(group => group.name).join(' / ')}`,
})))
const modelOptions = computed(() => availableModels.value.map(value => ({ value, label: value })))
const protocol = computed(() => videoProtocolForModel(form.value.model))
const platform = computed<GroupPlatform | undefined>(() => {
  if (!selectedKey.value) return undefined
  return eligibleGroups(selectedKey.value).find(group =>
    group.platform === 'composite' || group.platform === (protocol.value === 'grok' ? 'grok' : 'openai'),
  )?.platform
})
const requiresReference = computed(() => videoRequiresReference(form.value.model))
const resolutionOptions = computed(() => supportedVideoResolutions(form.value.model).map(value => ({ value, label: value })))
const ratioOptions = (['16:9', '9:16', '1:1', '4:3', '3:4', '21:9'] satisfies VideoAspectRatio[]).map(value => ({ value, label: value }))
const durationOptions = [5, 10].map(value => ({ value, label: `${value}s` }))
const canGenerate = computed(() => studio.ready && !pending.value && Boolean(
  selectedKey.value && platform.value && protocol.value && form.value.prompt.trim() &&
  (!requiresReference.value || studio.activeSession?.referenceFile),
))

async function loadModels() {
  const requestId = ++modelRequestId
  const key = selectedKey.value
  availableModels.value = []
  modelError.value = ''
  loadingModels.value = false
  if (!key) return
  loadingModels.value = true
  const groups = eligibleGroups(key)
  try {
    const models = await listVideoModels(key.key, groups.length > 1 ? 'composite' : groups[0]!.platform)
    if (requestId !== modelRequestId || disposed) return
    availableModels.value = models
    if (!form.value.model) form.value.model = models[0] || ''
    if (!models.length) modelError.value = t('videoGeneration.noModels')
  } catch (error) {
    if (requestId === modelRequestId && !disposed) modelError.value = message(error, 'videoGeneration.loadModelsFailed')
  } finally {
    if (requestId === modelRequestId) loadingModels.value = false
  }
}

function closeHistory() {
  showMobileHistory.value = false
  historyToggle.value?.focus()
}
function newSession() {
  studio.createSession()
  closeHistory()
}
function chooseSession(id: string) {
  studio.activeId = id
  closeHistory()
}
function chooseResult(result: VideoStudioResult) {
  if (studio.activeSession) studio.activeSession.activeResultId = result.id
  closeHistory()
}
function deleteSession() {
  if (deletingSessionId.value) studio.deleteSession(deletingSessionId.value)
  deletingSessionId.value = null
}
function removeReference() {
  if (studio.activeSession) studio.activeSession.referenceFile = null
}
function addReference(file?: File) {
  if (!file || !studio.activeSession) return
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
    app.showError(t('imageGeneration.invalidFileType'))
    return
  }
  if (file.size > 10 * 1024 * 1024) {
    app.showError(t('imageGeneration.fileTooLarge', { name: file.name }))
    return
  }
  studio.activeSession.referenceFile = file
}
function onReferenceChange(event: Event) {
  const input = event.target as HTMLInputElement
  addReference(input.files?.[0])
  input.value = ''
}
function onDrop(event: DragEvent) {
  addReference(event.dataTransfer?.files[0])
}
async function submitGeneration() {
  if (!canGenerate.value || !selectedKey.value || !platform.value) return
  try {
    const count = await studio.generate(selectedKey.value.key, platform.value)
    if (count) app.showSuccess(t('videoGeneration.generated'))
  } catch (error) {
    if (!disposed) app.showError(message(error, 'videoGeneration.generateFailed'))
  }
}
async function resumePolling() {
  const key = props.apiKeys.find(key => key.id === pending.value?.apiKeyId && key.status !== 'inactive')
  if (!key) { app.showError(t('videoGeneration.missingKey')); return }
  try {
    await studio.refreshJob(key.key)
  } catch (error) {
    if (!disposed) app.showError(message(error, 'videoGeneration.pollingPaused'))
  }
}
function clearPlayback() {
  playbackController?.abort()
  playbackController = null
  if (playbackUrl.value.startsWith('blob:')) URL.revokeObjectURL(playbackUrl.value)
  playbackUrl.value = ''
  downloadController?.abort()
  downloadController = null
  downloading.value = false
}
async function loadPlayback() {
  const result = activeResult.value
  const requestId = ++playbackRequestId
  clearPlayback()
  playbackError.value = ''
  loadingPlayback.value = false
  if (!result || pending.value) return
  const key = props.apiKeys.find(key => key.id === result.apiKeyId && key.status !== 'inactive')
  if (!key) { playbackError.value = t('videoGeneration.missingKey'); return }
  const controller = new AbortController()
  playbackController = controller
  loadingPlayback.value = true
  const isCurrent = () => !disposed && requestId === playbackRequestId && !controller.signal.aborted
  try {
    // Refresh signed URLs before reopening a result from browser history.
    const task = await getVideoTask(key.key, result.task.protocol, result.task.id, controller.signal)
    if (!isCurrent()) return
    if (task.protocol === 'seedance') {
      // Native video playback also works when the CDN does not enable CORS downloads.
      playbackUrl.value = videoContentURL(task)
    } else {
      const blob = await fetchVideoBlob(key.key, task, controller.signal)
      if (!isCurrent()) return
      playbackExtension = blob.type === 'video/webm' ? 'webm' : 'mp4'
      playbackUrl.value = URL.createObjectURL(blob)
    }
    result.task = task
  } catch (error) {
    if (isCurrent()) playbackError.value = message(error, 'videoGeneration.playbackFailed')
  } finally {
    if (requestId === playbackRequestId) loadingPlayback.value = false
  }
}
function onPlaybackError() {
  clearPlayback()
  playbackError.value = t('videoGeneration.playbackFailed')
}
async function downloadResult() {
  const result = activeResult.value
  if (!result || downloading.value) return
  downloading.value = true
  const id = result.id
  let temporaryUrl = ''
  let controller: AbortController | null = null
  try {
    if (!playbackUrl.value) await loadPlayback()
    if (!playbackUrl.value || activeResult.value?.id !== id || disposed) return
    controller = new AbortController()
    downloadController = controller
    downloading.value = true
    let url = playbackUrl.value
    let openInTab = false
    if (result.task.protocol === 'seedance') {
      try {
        const key = props.apiKeys.find(key => key.id === result.apiKeyId && key.status !== 'inactive')
        if (!key) throw new Error(t('videoGeneration.missingKey'))
        const blob = await fetchVideoBlob(key.key, result.task, controller.signal)
        if (activeResult.value?.id !== id || disposed || controller.signal.aborted) return
        playbackExtension = blob.type === 'video/webm' ? 'webm' : 'mp4'
        temporaryUrl = URL.createObjectURL(blob)
        url = temporaryUrl
      } catch {
        if (controller.signal.aborted || disposed || activeResult.value?.id !== id) return
        url = videoContentURL(result.task)
        openInTab = true
        app.showInfo(t('videoGeneration.downloadOpened'))
      }
    }
    if (activeResult.value?.id !== id || disposed || controller.signal.aborted) return
    const link = document.createElement('a')
    link.href = url
    if (openInTab) {
      link.target = '_blank'
      link.rel = 'noopener noreferrer'
    } else link.download = `sub2api-video-${result.task.id}.${playbackExtension}`
    document.body.appendChild(link)
    link.click()
    link.remove()
  } catch (error) {
    if (!disposed && !controller?.signal.aborted) app.showError(message(error, 'videoGeneration.playbackFailed'))
  } finally {
    if (temporaryUrl) setTimeout(() => URL.revokeObjectURL(temporaryUrl), 1000)
    if (controller && downloadController === controller) {
      downloadController = null
      downloading.value = false
    }
  }
}

watch([() => studio.activeId, videoKeys, () => props.loadingKeys], () => {
  if (!studio.activeSession || props.loadingKeys) return
  if (!videoKeys.value.some(key => key.id === form.value.apiKeyId)) {
    if (form.value.apiKeyId !== null) form.value.model = ''
    form.value.apiKeyId = videoKeys.value[0]?.id ?? null
  }
}, { immediate: true })
watch([() => studio.activeId, selectedKey], ([id, key], [previousId, previousKey]) => {
  if (id === previousId && previousKey && previousKey.id !== key?.id) form.value.model = ''
  void loadModels()
}, { immediate: true })
watch(resolutionOptions, options => {
  if (options.length && !options.some(option => option.value === form.value.resolution)) form.value.resolution = options[options.length - 1]!.value
})
watch([protocol, platform], ([modelProtocol, modelPlatform]) => {
  if (form.value.model && (!modelProtocol || !modelPlatform)) modelError.value = t('videoGeneration.unsupportedModel')
  else if (modelError.value === t('videoGeneration.unsupportedModel')) modelError.value = ''
}, { immediate: true })
watch(() => studio.activeSession?.referenceFile, file => {
  if (referenceUrl.value) URL.revokeObjectURL(referenceUrl.value)
  referenceUrl.value = file ? URL.createObjectURL(file) : ''
}, { immediate: true })
watch([activeResult, () => Boolean(pending.value), () => props.loadingKeys], () => {
  if (!props.loadingKeys) void loadPlayback()
}, { immediate: true })
watch([() => studio.ready, () => props.apiKeys, () => props.loadingKeys], () => {
  if (studio.ready && !props.loadingKeys) studio.resumeJobs(props.apiKeys)
}, { immediate: true })
onBeforeUnmount(() => {
  disposed = true
  modelRequestId++
  playbackRequestId++
  clearPlayback()
  if (referenceUrl.value) URL.revokeObjectURL(referenceUrl.value)
  void studio.flush()
})
</script>

<style scoped>
.video-panel { @apply flex min-h-0 flex-1 flex-col overflow-hidden; }
.video-grid { @apply grid min-h-0 flex-1 grid-cols-1 grid-rows-1 lg:grid-cols-[minmax(0,1fr)_260px]; }
.video-workspace { @apply flex min-h-0 min-w-0 flex-col overflow-y-auto px-3 pb-2 pt-3 sm:px-4; }
.video-canvas {
  @apply flex min-h-40 flex-1 items-center justify-center overflow-hidden rounded-2xl border p-3 sm:p-4;
  background: var(--surface-card);
  border-color: var(--surface-border);
}
.video-composer {
  @apply mx-auto mt-3 w-full max-w-4xl shrink-0 rounded-2xl border;
  background: var(--surface-card);
  border-color: var(--surface-border);
  box-shadow: var(--surface-shadow);
}
.video-composer:focus-within { @apply ring-2 ring-primary-500/30; }
.video-prompt { @apply block w-full resize-none rounded-t-2xl border-0 bg-transparent px-4 py-3 text-sm text-gray-800 outline-none placeholder:text-gray-400 focus:ring-0 dark:text-dark-100; }
.video-controls { @apply space-y-2 border-t border-gray-100 p-2.5 dark:border-dark-700 sm:p-3; }
.video-controls :deep(.select-trigger) { @apply h-8 min-w-0 rounded-lg bg-gray-50 px-2 py-1 text-xs dark:bg-dark-800; }
.video-label { @apply mb-1 block text-[10px] text-gray-500 dark:text-dark-400; }
.video-footnote { @apply mx-auto mt-1.5 max-w-3xl shrink-0 text-center text-[10px] leading-relaxed text-gray-400; }
.video-notice { @apply mb-2 shrink-0 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950 dark:text-amber-200; }
.video-history {
  @apply hidden min-h-0 flex-col border-l lg:flex;
  background: var(--surface-inset);
  border-color: var(--surface-border);
}
.video-result { @apply flex w-full min-w-0 items-center gap-2 rounded-xl border border-transparent p-2 text-left transition hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-800; }
.video-result[aria-pressed='true'] { @apply border-primary-500/60 bg-primary-50 dark:bg-primary-950/40; }
@media (max-width: 1023px) {
  .video-history-visible { @apply flex border-l-0; }
  .video-history-open .video-workspace { @apply hidden; }
}
@media (max-height: 600px) {
  .video-canvas { min-height: 12rem; }
}
</style>
