<template>
  <div class="video-panel">
    <header class="video-workflow">
      <button type="button" class="video-new-creation" :disabled="!studio.ready" @click="newSession"><Icon name="plus" size="sm" />{{ t('imageGeneration.newCreation') }}</button>
      <ol :aria-label="t('imageGeneration.creationSteps')">
        <li><span>01</span>{{ t('imageGeneration.chooseScene') }}</li>
        <li><span>02</span>{{ t('videoGeneration.describeMotion') }}</li>
        <li><span>03</span>{{ t('videoGeneration.prepareFrame') }}</li>
        <li><Icon name="check" size="sm" />{{ t('imageGeneration.confirmSettings') }}</li>
      </ol>
    </header>
    <nav class="video-panel-tabs" :aria-label="t('imageGeneration.workspacePanels')">
      <button v-for="panel in workspacePanels" :key="panel.value" type="button" :aria-pressed="mobilePanel === panel.value" :aria-controls="panel.control" @click="mobilePanel = panel.value">{{ panel.label }}<span v-if="panel.value === 'result' && results.length">{{ results.length }}</span></button>
    </nav>
    <p v-if="studio.storageError" role="alert" class="video-notice video-storage-notice">{{ storageErrorText }}</p>
    <p v-if="studio.activeSession?.interrupted" role="status" class="video-notice video-storage-notice">{{ t('videoGeneration.sessionInterrupted') }}</p>
    <div class="video-grid" :data-panel="mobilePanel">
      <aside id="video-compose-panel" class="video-compose-panel video-card" :aria-label="t('imageGeneration.createTitle')">
        <slot name="heading"><header class="video-composer-heading">{{ t('imageGeneration.videoTab') }}</header></slot>
        <div v-if="loadingKeys || !studio.ready" class="video-loading">
          <LoadingSpinner size="lg" /><p>{{ t('videoGeneration.loading') }}</p>
        </div>
        <div v-else-if="!videoKeys.length && !studio.sessions.some(session => session.results.length || session.pending)" class="video-loading">
          <Icon name="key" size="lg" /><h2>{{ t('videoGeneration.noKeysTitle') }}</h2><p>{{ t('videoGeneration.noKeysDescription') }}</p>
          <RouterLink to="/keys" class="btn btn-primary btn-sm">{{ t('imageGeneration.manageKeys') }}</RouterLink>
        </div>
        <form v-else class="video-composer" @submit.prevent="submitGeneration" @dragenter.prevent="onDragEnter" @dragover.prevent="onDragOver" @dragleave.prevent="onDragLeave" @drop.prevent="onDrop">
          <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp" class="sr-only" @change="onReferenceChange" />
          <div class="video-composer-scroll">
            <section class="video-form-section">
              <div class="video-section-heading"><span class="video-step">01</span><div><h2>{{ t('imageGeneration.sceneHeading') }}</h2><p>{{ t('imageGeneration.sceneHint') }}</p></div></div>
              <div class="video-scenes">
                <button v-for="scene in promptScenes" :key="scene.id" type="button" :data-video-scene="scene.id" @click="applyScene(scene.prompt)"><span class="video-scene-icon"><Icon :name="scene.icon" size="sm" /></span><span><strong>{{ scene.title }}</strong><small>{{ scene.hint }}</small></span><Icon name="chevronRight" size="xs" /></button>
              </div>
            </section>
            <section class="video-form-section">
              <div class="video-section-heading"><span class="video-step">02</span><div><h2><label for="video-prompt">{{ t('videoGeneration.describeMotion') }}</label></h2><p>{{ t('videoGeneration.promptGuide') }}</p></div></div>
              <div class="video-prompt-area">
                <textarea id="video-prompt" v-model="form.prompt" rows="5" :placeholder="t('videoGeneration.promptPlaceholder')" :aria-label="t('videoGeneration.prompt')" class="video-prompt" @paste="onPaste" @keydown.ctrl.enter.prevent="submitGeneration" @keydown.meta.enter.prevent="submitGeneration"></textarea>
                <div class="video-prompt-meta"><span>{{ t('videoGeneration.promptCharacters', { count: form.prompt.length }) }}</span><kbd>⌘ / Ctrl + Enter</kbd></div>
                <div v-if="dragging" class="video-drop-overlay"><Icon name="upload" size="md" />{{ t('videoGeneration.dropReference') }}</div>
              </div>
            </section>
            <section class="video-form-section video-reference-section">
              <div class="video-section-heading"><span class="video-step">03</span><h2>{{ t('videoGeneration.prepareFrame') }} <small>{{ referenceUrl || form.referenceImageUrl?.trim() ? 1 : 0 }} / 1</small></h2><button v-if="referenceUrl || form.referenceImageUrl" type="button" class="video-icon-button" :aria-label="t('videoGeneration.removeReference')" @click="removeReference"><Icon name="x" size="sm" /></button></div>
              <div class="video-reference-row">
                <img v-if="referenceUrl" :src="referenceUrl" :alt="t('videoGeneration.referenceAlt')" class="video-reference-image" />
                <button type="button" class="video-reference-upload" :title="t('videoGeneration.uploadHint')" @click="fileInput?.click()"><Icon name="upload" size="md" /><span>{{ t('videoGeneration.uploadReference') }}</span></button>
              </div>
              <p class="video-reference-hint">{{ t('videoGeneration.referenceInputHint') }}</p>
              <details class="video-url-details">
                <summary>{{ t('videoGeneration.useImageUrl') }}</summary>
                <label for="video-reference-url" class="video-label">{{ t('videoGeneration.referenceImageUrlLabel') }}</label>
                <input id="video-reference-url" v-model="form.referenceImageUrl" type="url" :placeholder="t('videoGeneration.referenceImageUrlPlaceholder')" class="video-url-input" />
                <p class="video-reference-hint">{{ t('videoGeneration.referenceImageUrlHint') }}</p>
              </details>
              <p v-if="requiresReference && !referenceUrl && !form.referenceImageUrl" role="status" class="video-inline-warning">{{ t('videoGeneration.requiresReference') }}</p>
            </section>
            <section class="video-form-section video-controls">
              <div class="video-section-heading"><h2>{{ t('videoGeneration.videoSettings') }}</h2><span class="video-section-aside">{{ t('videoGeneration.providers') }}</span></div>
              <label for="video-api-key" class="video-label">{{ t('imageGeneration.apiKey') }}</label>
              <Select id="video-api-key" v-model="form.apiKeyId" :options="keyOptions" :aria-label="t('imageGeneration.apiKey')" />
              <label for="video-model" class="video-label">{{ t('imageGeneration.model') }}</label>
              <Select id="video-model" v-model="form.model" :options="modelOptions" searchable creatable :loading="loadingModels" :placeholder="loadingModels ? t('imageGeneration.loadingModels') : t('videoGeneration.modelPlaceholder')" :aria-label="t('imageGeneration.model')" />
              <p v-if="modelError" role="status" class="video-inline-warning">{{ modelError }}</p>
              <label for="video-ratio" class="video-label">{{ t('imageGeneration.aspectRatio') }}</label>
              <Select id="video-ratio" v-model="form.aspectRatio" :options="ratioOptions" :aria-label="t('imageGeneration.aspectRatio')" />
              <div class="video-spec-grid">
                <div><label for="video-duration" class="video-label">{{ t('videoGeneration.duration') }}</label><Select id="video-duration" v-model="form.duration" :options="durationOptions" :aria-label="t('videoGeneration.duration')" /></div>
                <div><label for="video-resolution" class="video-label">{{ t('imageGeneration.resolution') }}</label><Select id="video-resolution" v-model="form.resolution" :options="resolutionOptions" :disabled="Boolean(fixedResolution)" :aria-label="Boolean(fixedResolution) ? t('videoGeneration.resolutionFixed') : t('imageGeneration.resolution')" /></div>
              </div>
              <p v-if="fixedResolution" class="video-reference-hint">{{ t('videoGeneration.resolutionFixed') }}</p>
              <p class="video-reference-hint">{{ t('videoGeneration.requestHint') }}</p>
            </section>
          </div>
          <footer class="video-composer-footer"><div><strong>{{ form.model || t('imageGeneration.selectModel') }}</strong><span>{{ form.aspectRatio }} · {{ form.duration }}s · {{ form.resolution }}</span></div><button type="submit" class="video-generate" :disabled="!canGenerate"><LoadingSpinner v-if="generating" size="sm" /><Icon v-else name="play" size="sm" />{{ t(generating ? 'videoGeneration.generating' : 'videoGeneration.generate') }}</button></footer>
        </form>
      </aside>

      <section id="video-result-panel" class="video-workspace video-card" tabindex="-1" :aria-label="t('videoGeneration.resultsTitle')">
        <header class="video-result-heading"><div><h2>{{ t('videoGeneration.resultsTitle') }}</h2><p>{{ pending ? statusLabel : activeResult ? t('videoGeneration.resultReady') : t('videoGeneration.resultsHint') }}</p></div><span class="video-mode-badge">{{ t((pending?.mode ?? activeResult?.mode ?? (referenceUrl || form.referenceImageUrl ? 'image' : 'text')) === 'image' ? 'videoGeneration.imageToVideo' : 'videoGeneration.textToVideo') }}</span></header>
        <p v-if="sessionError" role="alert" class="video-error">{{ sessionError }}</p>
        <div class="video-canvas">
          <div class="video-viewport" :class="{ 'has-video': activeResult && !pending }">
            <div v-if="pending" class="video-placeholder" role="status" aria-live="polite">
              <span class="video-empty-frame"><LoadingSpinner v-if="generating" size="lg" /><Icon v-else name="clock" size="xl" /></span>
              <h3>{{ statusLabel }}</h3><p>{{ pending.model }} · {{ pending.duration }}s · {{ pending.resolution }}</p>
              <p>{{ t('videoGeneration.backgroundHint') }}</p>
              <button v-if="pending.phase === 'paused'" type="button" class="btn btn-secondary btn-sm" @click="resumePolling"><Icon name="refresh" size="sm" />{{ t('videoGeneration.resume') }}</button>
              <details v-if="pending.task" class="video-task-details"><summary>{{ t('videoGeneration.taskId') }}</summary><code>{{ pending.task.id }}</code></details>
            </div>
            <template v-else-if="activeResult">
              <video v-if="playbackUrl" :key="activeResult.id" :src="playbackUrl" controls playsinline preload="metadata" :aria-label="activeResult.prompt" @error="onPlaybackError"></video>
              <div v-else class="video-placeholder video-playback-placeholder"><LoadingSpinner v-if="loadingPlayback" size="lg" /><Icon v-else name="play" size="xl" /><p>{{ loadingPlayback ? t('videoGeneration.loadingVideo') : playbackError }}</p><button v-if="!loadingPlayback" type="button" class="btn btn-secondary btn-sm" @click="loadPlayback">{{ t('videoGeneration.retryPlayback') }}</button></div>
            </template>
            <div v-else class="video-placeholder"><span class="video-empty-frame"><Icon name="play" size="xl" /></span><h3>{{ t('videoGeneration.emptyTitle') }}</h3><p>{{ t('videoGeneration.emptyDescription') }}</p></div>
          </div>
          <div v-if="activeResult && !pending" class="video-result-summary"><div><strong>{{ activeResult.model }}</strong><span>{{ activeResult.duration }}s · {{ activeResult.resolution }} · {{ activeResult.aspectRatio }}</span></div><p>{{ activeResult.prompt }}</p></div>
          <footer v-if="activeResult && !pending" class="video-result-actions"><button type="button" class="btn btn-secondary btn-sm video-download" :aria-label="t('videoGeneration.download')" :disabled="downloading || loadingPlayback" @click="downloadResult"><Icon name="download" size="xs" />{{ t('videoGeneration.download') }}</button><button type="button" class="btn btn-ghost btn-sm video-continue" @click="revealWorkspacePanel('compose', true)"><Icon name="edit" size="xs" />{{ t('videoGeneration.continueCreating') }}</button></footer>
        </div>
      </section>

      <aside id="video-studio-history" class="video-activity" :aria-label="t('videoGeneration.historyTitle')">
        <section class="video-task-panel video-card"><header><div><h2>{{ t('imageGeneration.runningTasks') }}</h2><p>{{ t('imageGeneration.taskCount', { count: activeTaskSessions.length }) }}</p></div><Icon name="clock" size="sm" /></header><div class="video-task-list"><p v-if="!activeTaskSessions.length" class="video-empty-tasks">{{ t('imageGeneration.noRunningTasks') }}</p><button v-for="session in activeTaskSessions" :key="session.id" type="button" class="video-task-entry" @click="chooseSession(session.id, 'result')"><LoadingSpinner v-if="session.pending && session.pending.phase !== 'paused'" size="sm" /><Icon v-else name="exclamationCircle" size="sm" /><span><strong>{{ session.title || t('imageGeneration.untitledSession') }}</strong><small>{{ session.pending ? jobStatusLabel(session.pending) : t('imageGeneration.batchUncertain') }}</small></span><Icon name="chevronRight" size="xs" /></button></div></section>
        <section class="video-history video-card">
          <header class="video-history-heading"><div><h2>{{ t('videoGeneration.generationHistory') }}</h2><button type="button" class="video-icon-button video-new-session" :disabled="!studio.ready" :aria-label="t('imageGeneration.newSession')" @click="newSession"><Icon name="plus" size="sm" /></button></div><input v-model="historySearch" type="search" class="input video-history-search" :placeholder="t('imageGeneration.searchHistory')" :aria-label="t('imageGeneration.searchHistory')" /></header>
          <div class="video-session-list" data-testid="video-sessions">
            <div v-for="session in filteredSessions" :key="session.id" class="video-session-row" :class="{ 'is-active': session.id === studio.activeId }"><button type="button" class="video-session-select" :aria-pressed="session.id === studio.activeId" @click="chooseSession(session.id)"><span>{{ session.title || t('imageGeneration.untitledSession') }}</span><small>{{ session.pending ? jobStatusLabel(session.pending) : t('videoGeneration.sessionVideoCount', { count: session.results.length }) }}</small></button><button type="button" class="video-icon-button" :disabled="Boolean(session.pending)" :aria-label="t('imageGeneration.deleteSession')" @click="deletingSessionId = session.id"><Icon name="trash" size="xs" /></button></div>
          </div>
          <div class="video-history-filters"><button v-for="filter in historyFilters" :key="filter.value" type="button" :aria-pressed="historyFilter === filter.value" @click="historyFilter = filter.value">{{ filter.label }}</button></div>
          <div class="video-history-results">
            <p v-if="!filteredResults.length" class="video-history-empty">{{ t(historySearch ? 'imageGeneration.noSearchResults' : 'imageGeneration.noHistory') }}</p>
            <button v-for="result in filteredResults" :key="result.id" type="button" class="video-result" :aria-pressed="activeResult?.id === result.id" @click="chooseResult(result)"><span class="video-history-play"><Icon name="play" size="sm" /></span><span class="video-history-info"><strong>{{ result.prompt }}</strong><span>{{ result.model }} · {{ result.duration }}s · {{ result.resolution }}</span><small>{{ result.createdAt }}</small></span></button>
          </div>
          <p class="video-history-hint">{{ t('videoGeneration.historyHint') }}</p>
        </section>
      </aside>
    </div>
    <ConfirmDialog :show="Boolean(deletingSessionId)" :title="t('imageGeneration.deleteSession')" :message="t('videoGeneration.deleteSessionConfirm')" @confirm="deleteSession" @cancel="deletingSessionId = null" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { fetchVideoBlob, getVideoTask, listVideoModels, supportedVideoResolutions, supportsVideoGeneration, videoContentURL, videoNeedsAuthenticatedDownload, videoModelResolution, videoProtocolForModel, videoRequiresReference, type VideoAspectRatio } from '@/api/videoGeneration'
import { ConfirmDialog, LoadingSpinner } from '@/components/common'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useVideoStudioStore, type VideoStudioForm, type VideoStudioJob, type VideoStudioResult } from '@/stores/videoStudio'
import type { ApiKey, GroupPlatform } from '@/types'

const props = defineProps<{ apiKeys: ApiKey[]; loadingKeys: boolean }>()
const { t } = useI18n()
const app = useAppStore()
const studio = useVideoStudioStore()
type WorkspacePanel = 'compose' | 'result' | 'history'
const mobilePanel = ref<WorkspacePanel>('compose')
const historySearch = ref('')
const historyFilter = ref<'all' | 'text' | 'image'>('all')
const workspacePanels = computed(() => [
  { value: 'compose' as const, label: t('imageGeneration.composePanel'), control: 'video-compose-panel' },
  { value: 'result' as const, label: t('imageGeneration.worksPanel'), control: 'video-result-panel' },
  { value: 'history' as const, label: t('imageGeneration.recordsPanel'), control: 'video-studio-history' },
])
const promptScenes = computed(() => [
  { id: 'product', icon: 'grid' as const, title: t('videoGeneration.sceneProduct'), hint: t('videoGeneration.sceneProductHint'), prompt: t('videoGeneration.sceneProductPrompt') },
  { id: 'portrait', icon: 'user' as const, title: t('videoGeneration.scenePortrait'), hint: t('videoGeneration.scenePortraitHint'), prompt: t('videoGeneration.scenePortraitPrompt') },
  { id: 'landscape', icon: 'sparkles' as const, title: t('videoGeneration.sceneLandscape'), hint: t('videoGeneration.sceneLandscapeHint'), prompt: t('videoGeneration.sceneLandscapePrompt') },
])
const historyFilters = computed(() => [
  { value: 'all' as const, label: t('imageGeneration.historyAll') },
  { value: 'text' as const, label: t('videoGeneration.textToVideo') },
  { value: 'image' as const, label: t('videoGeneration.imageToVideo') },
])
const deletingSessionId = ref<string | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const referenceUrl = ref('')
const dragging = ref(false)
const availableModels = ref<string[]>([])
const loadingModels = ref(false)
const modelLoadError = ref('')
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

const emptyForm: VideoStudioForm = { apiKeyId: null, model: '', prompt: '', referenceImageUrl: '', duration: 5, aspectRatio: '16:9', resolution: '720p' }
const form = computed(() => studio.activeSession?.form ?? emptyForm)
const pending = computed(() => studio.activeSession?.pending ?? null)
const generating = computed(() => Boolean(pending.value && pending.value.phase !== 'paused'))
const results = computed(() => studio.activeSession?.results ?? [])
const activeResult = computed(() => results.value.find(result => result.id === studio.activeSession?.activeResultId) ?? null)
const activeTaskSessions = computed(() => studio.sessions.filter(session => session.pending || session.interrupted))
const searchTerm = computed(() => historySearch.value.trim().toLocaleLowerCase())
const filteredSessions = computed(() => studio.sessions.filter(session => !searchTerm.value ||
  [session.title, session.form.prompt, ...session.results.map(result => `${result.prompt} ${result.model}`)].some(text => text.toLocaleLowerCase().includes(searchTerm.value))))
const filteredResults = computed(() => results.value.filter(result =>
  (historyFilter.value === 'all' || result.mode === historyFilter.value) &&
  (!searchTerm.value || `${result.prompt} ${result.model}`.toLocaleLowerCase().includes(searchTerm.value))))
const message = (error: unknown, fallback: string) => {
  const text = error instanceof Error ? error.message : typeof error === 'string' ? error : fallback
  return text.startsWith('videoGeneration.') ? t(text) : text
}
const sessionError = computed(() => message(studio.activeSession?.error || '', 'videoGeneration.generateFailed'))
const storageErrorText = computed(() => t(studio.storageError === 'conflict' ? 'videoGeneration.storageConflict' : 'videoGeneration.storageError'))
const jobStatusLabel = (job: VideoStudioJob) => t(`videoGeneration.${job.phase === 'submitting' ? 'submitting' : job.phase === 'paused' ? 'paused' : job.task?.status === 'queued' ? 'queued' : 'running'}`)
const statusLabel = computed(() => pending.value ? jobStatusLabel(pending.value) : '')

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
const fixedResolution = computed(() => videoModelResolution(form.value.model))
const modelError = computed(() => {
  if (form.value.model && (!protocol.value || !platform.value)) return t('videoGeneration.unsupportedModel')
  if (fixedResolution.value === '4k') return t('videoGeneration.unsupportedResolution')
  return modelLoadError.value
})
const resolutionOptions = computed(() => (fixedResolution.value ? [fixedResolution.value] : supportedVideoResolutions(form.value.model)).map(value => ({ value, label: value === '4k' ? '4K' : value })))
const ratioOptions = (['16:9', '9:16', '1:1', '4:3', '3:4', '21:9'] satisfies VideoAspectRatio[]).map(value => ({ value, label: value }))
const durationOptions = [5, 10].map(value => ({ value, label: `${value}s` }))
const canGenerate = computed(() => studio.ready && !pending.value && Boolean(
  selectedKey.value && platform.value && protocol.value && form.value.prompt.trim() &&
  supportedVideoResolutions(form.value.model).includes(form.value.resolution) &&
  (!requiresReference.value || studio.activeSession?.referenceFile || form.value.referenceImageUrl?.trim()),
))

async function loadModels() {
  const requestId = ++modelRequestId
  const key = selectedKey.value
  availableModels.value = []
  modelLoadError.value = ''
  loadingModels.value = false
  if (!key) return
  loadingModels.value = true
  const groups = eligibleGroups(key)
  try {
    const models = await listVideoModels(key.key, groups.length > 1 ? 'composite' : groups[0]!.platform)
    if (requestId !== modelRequestId || disposed) return
    availableModels.value = models
    if (!form.value.model) form.value.model = models[0] || ''
    if (!models.length) modelLoadError.value = t('videoGeneration.noModels')
  } catch (error) {
    if (requestId === modelRequestId && !disposed) modelLoadError.value = message(error, 'videoGeneration.loadModelsFailed')
  } finally {
    if (requestId === modelRequestId) loadingModels.value = false
  }
}

async function revealWorkspacePanel(panel: WorkspacePanel, focusOnDesktop = false) {
  mobilePanel.value = panel
  await nextTick()
  if (disposed || mobilePanel.value !== panel || (!focusOnDesktop && window.innerWidth > 1199)) return
  const destination = panel === 'compose'
    ? document.getElementById('video-prompt') ?? document.getElementById('studio-video-tab')
    : document.getElementById(panel === 'result' ? 'video-result-panel' : 'video-studio-history')
  destination?.focus({ preventScroll: panel !== 'compose' })
}
function applyScene(prompt: string) {
  form.value.prompt = [form.value.prompt, prompt].filter(Boolean).join('\n')
  void revealWorkspacePanel('compose', true)
}
function newSession() {
  studio.createSession()
  void revealWorkspacePanel('compose')
}
function chooseSession(id: string, panel: WorkspacePanel = 'compose') {
  studio.activeId = id
  void revealWorkspacePanel(panel)
}
function chooseResult(result: VideoStudioResult) {
  if (studio.activeSession) studio.activeSession.activeResultId = result.id
  void revealWorkspacePanel('result')
}
function deleteSession() {
  if (deletingSessionId.value) studio.deleteSession(deletingSessionId.value)
  deletingSessionId.value = null
}
function removeReference() {
  if (studio.activeSession) {
    studio.activeSession.referenceFile = null
    studio.activeSession.form.referenceImageUrl = ''
  }
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
  studio.activeSession.form.referenceImageUrl = ''
  studio.activeSession.referenceFile = file
}
function onReferenceChange(event: Event) {
  const input = event.target as HTMLInputElement
  addReference(input.files?.[0])
  input.value = ''
}
function onDrop(event: DragEvent) {
  dragging.value = false
  addReference(event.dataTransfer?.files[0])
}
function onDragEnter(event: DragEvent) {
  if (event.dataTransfer?.types.includes('Files')) dragging.value = true
}
function onDragOver(event: DragEvent) {
  if (event.dataTransfer?.types.includes('Files')) {
    event.dataTransfer.dropEffect = 'copy'
    dragging.value = true
  }
}
function onDragLeave(event: DragEvent) {
  if (event.relatedTarget instanceof Node && (event.currentTarget as HTMLElement)?.contains(event.relatedTarget)) return
  dragging.value = false
}
function onPaste(event: ClipboardEvent) {
  const file = event.clipboardData?.files[0]
  if (!file) return
  event.preventDefault()
  addReference(file)
}
async function submitGeneration() {
  if (!canGenerate.value || !selectedKey.value || !platform.value) return
  void revealWorkspacePanel('result')
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
    if (!videoNeedsAuthenticatedDownload(task)) {
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
    if (!videoNeedsAuthenticatedDownload(result.task)) {
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
  dragging.value = false
  if (id === previousId && previousKey && previousKey.id !== key?.id) form.value.model = ''
  void loadModels()
}, { immediate: true })
watch([resolutionOptions, () => form.value.resolution], ([options]) => {
  if (options.length && !options.some(option => option.value === form.value.resolution)) form.value.resolution = options[options.length - 1]!.value
}, { immediate: true })
watch(() => form.value.referenceImageUrl, value => {
  if (value?.trim() && studio.activeSession) studio.activeSession.referenceFile = null
})
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
.video-panel { @apply flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-hidden; }
.video-card { @apply min-w-0 overflow-hidden rounded-2xl border; background: var(--surface-card); border-color: var(--surface-border); }
.video-workflow { @apply flex h-12 shrink-0 items-center gap-5 rounded-xl px-4; background: var(--surface-card); }
.video-new-creation { @apply flex shrink-0 items-center gap-1.5 rounded-full bg-primary-50 px-3 py-2 text-xs font-medium text-primary-700 dark:bg-primary-500/10 dark:text-primary-300; }
.video-workflow ol { @apply flex min-w-0 flex-1 items-center gap-4; }
.video-workflow li { @apply flex min-w-0 flex-1 items-center gap-2 whitespace-nowrap text-xs font-medium text-gray-600 dark:text-dark-200; }
.video-workflow li:not(:last-child)::after { content: ''; flex: 1; height: 1px; margin-left: 8px; background: var(--surface-border); }
.video-workflow li > span, .video-step { @apply inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-xs font-semibold text-primary-700 dark:bg-primary-500/10 dark:text-primary-300; }
.video-workflow li:last-child { @apply text-primary-700 dark:text-primary-300; }
.video-grid { @apply grid min-h-0 flex-1 gap-3; grid-template-columns: 350px minmax(0, 1fr) 240px; }
.video-compose-panel, .video-composer { @apply flex min-h-0 flex-col; }
.video-composer { @apply flex-1; }
.video-composer-heading { @apply shrink-0 border-b px-4 py-5 text-sm font-semibold; border-color: var(--surface-border); }
.video-loading { @apply flex flex-1 flex-col items-center justify-center gap-3 p-5 text-center text-sm text-gray-500 dark:text-dark-300; }
.video-composer-scroll { @apply min-h-0 flex-1 overflow-y-auto px-4; scrollbar-width: thin; scrollbar-gutter: stable; }
.video-form-section { @apply min-w-0 py-4; }
.video-form-section + .video-form-section { border-top: 1px solid var(--surface-border); }
.video-section-heading { @apply mb-3 flex min-w-0 items-center gap-2.5; }
.video-section-heading h2 { @apply text-xs font-semibold text-gray-800 dark:text-gray-100; }
.video-section-heading p { @apply mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-dark-400; }
.video-section-heading small { @apply ml-1 text-[11px] font-normal text-gray-400; }
.video-section-aside { @apply ml-auto text-[10px] text-gray-400 dark:text-dark-400; }
.video-scenes { @apply grid gap-1; }
.video-scenes > button { @apply flex min-w-0 items-center gap-2.5 rounded-xl px-2 py-2 text-left transition hover:bg-gray-50 dark:hover:bg-dark-700/50; }
.video-scene-icon { @apply flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300; }
.video-scenes button > span:nth-child(2) { @apply min-w-0 flex-1; }
.video-scenes strong { @apply block text-xs font-medium text-gray-700 dark:text-gray-100; }
.video-scenes small { @apply mt-1 block truncate text-[10px] text-gray-500 dark:text-dark-400; }
.video-scenes button > svg { @apply shrink-0 text-gray-400; }
.video-prompt-area { @apply relative rounded-xl border px-3 pt-3; background: var(--surface-inset); border-color: var(--surface-border); }
.video-prompt-area:focus-within { @apply border-primary-400; }
.video-prompt { @apply block min-h-28 w-full resize-y bg-transparent text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 dark:text-gray-100 dark:placeholder:text-dark-400; max-height: 260px; }
.video-prompt-meta { @apply mt-2 flex items-center justify-between gap-2 border-t py-2 text-[10px] text-gray-400 dark:text-dark-400; border-color: var(--surface-border); }
.video-prompt-meta kbd { @apply font-sans; }
.video-drop-overlay { @apply pointer-events-none absolute inset-0 flex items-center justify-center gap-2 rounded-xl border-2 border-dashed border-primary-400 bg-primary-50/95 p-3 text-xs text-primary-700 dark:bg-dark-800/95 dark:text-primary-300; }
.video-reference-section .video-icon-button { @apply ml-auto; }
.video-reference-row { @apply flex items-center gap-2; }
.video-reference-image { @apply h-24 w-24 rounded-xl border object-cover; border-color: var(--surface-border); }
.video-reference-upload { @apply flex h-24 w-[88px] shrink-0 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-gray-300 text-[11px] text-gray-500 transition hover:border-primary-400 hover:text-primary-700 dark:border-dark-500 dark:text-dark-300 dark:hover:text-primary-300; }
.video-reference-hint { @apply mt-2 text-[10px] leading-relaxed text-gray-400 dark:text-dark-400; }
.video-url-details { @apply mt-3 min-w-0; }
.video-url-details summary { @apply mb-2 cursor-pointer text-[11px] font-medium text-gray-500 dark:text-dark-300; }
.video-url-input { @apply block min-h-9 w-full min-w-0 rounded-lg border px-2.5 py-2 text-xs text-gray-800 placeholder:text-gray-400 focus:border-primary-400 focus:outline-none dark:text-dark-100; background: var(--surface-inset); border-color: var(--surface-border); }
.video-label { @apply mb-1 block pt-1 text-[11px] font-medium text-gray-500 dark:text-dark-300; }
.video-controls { @apply space-y-2; }
.video-controls :deep(.select-trigger) { @apply min-h-9 rounded-lg px-3 py-2 text-xs shadow-none; background: var(--surface-inset); border-color: transparent; }
.video-spec-grid { @apply grid grid-cols-2 gap-2; }
.video-spec-grid > div { @apply min-w-0; }
.video-inline-warning { @apply mt-2 text-[11px] leading-relaxed text-amber-800 dark:text-amber-200; }
.video-composer-footer { @apply flex shrink-0 items-center gap-2 border-t p-3; border-color: var(--surface-border); background: var(--surface-card); }
.video-composer-footer > div { @apply min-w-0 flex-1; }
.video-composer-footer strong { @apply block truncate text-xs font-medium text-gray-700 dark:text-dark-100; }
.video-composer-footer span { @apply mt-1 block truncate text-[10px] text-gray-500 dark:text-dark-400; }
.video-generate { @apply flex h-10 shrink-0 items-center justify-center gap-1.5 rounded-xl bg-primary-500 px-4 text-xs font-semibold text-on-primary transition hover:bg-primary-400 disabled:cursor-not-allowed disabled:opacity-45; }
.video-workspace { @apply flex min-h-0 flex-col p-4; }
.video-result-heading { @apply mb-3 flex shrink-0 items-center justify-between gap-2; }
.video-result-heading h2, .video-task-panel h2, .video-history-heading h2 { @apply text-sm font-semibold text-gray-800 dark:text-gray-100; }
.video-result-heading p, .video-task-panel header p { @apply mt-1 text-[11px] text-gray-500 dark:text-dark-400; }
.video-mode-badge { @apply shrink-0 rounded-full bg-primary-50 px-2 py-1 text-[10px] text-primary-700 dark:bg-primary-500/10 dark:text-primary-300; }
.video-canvas { @apply flex min-h-0 flex-1 flex-col; }
.video-viewport { @apply flex min-h-0 flex-1 items-center justify-center overflow-y-auto rounded-xl; background: var(--surface-inset); }
.video-viewport.has-video { @apply overflow-hidden; background: #18181b; }
.video-viewport video { @apply h-full min-h-0 w-full object-contain; }
.video-placeholder { @apply flex max-w-sm flex-col items-center gap-3 p-5 text-center text-gray-400 dark:text-dark-400; max-height: 100%; }
.video-placeholder h3 { @apply text-sm font-medium text-gray-500 dark:text-dark-300; }
.video-placeholder p { @apply max-w-[280px] text-xs leading-relaxed; }
.video-empty-frame { @apply mb-2 flex h-24 w-28 shrink-0 items-center justify-center rounded-2xl border border-dashed border-gray-300 dark:border-dark-600; }
.video-playback-placeholder { @apply text-gray-300; }
.video-task-details { @apply w-full text-[10px]; }
.video-task-details summary { @apply cursor-pointer; }
.video-task-details code { @apply mt-2 block break-all; }
.video-result-summary { @apply shrink-0 py-3; }
.video-result-summary > div { @apply flex flex-wrap items-center justify-between gap-2; }
.video-result-summary strong { @apply min-w-0 truncate text-xs font-medium text-gray-700 dark:text-dark-100; }
.video-result-summary span { @apply text-[10px] text-gray-500 dark:text-dark-300; }
.video-result-summary p { @apply mt-2 line-clamp-2 text-xs leading-relaxed text-gray-500 dark:text-dark-400; }
.video-result-actions { @apply flex shrink-0 flex-wrap items-center justify-end gap-1 border-t pt-3; border-color: var(--surface-border); }
.video-result-actions .btn { @apply min-h-8 px-2 text-[11px]; }
.video-result-actions .video-download { @apply order-1 border-transparent bg-primary-500 text-on-primary shadow-none hover:bg-primary-400; }
.video-notice { @apply mb-2 shrink-0 rounded-lg bg-amber-50 px-3 py-2 text-xs leading-relaxed text-amber-800 dark:bg-amber-950/40 dark:text-amber-200; }
.video-storage-notice { @apply mb-0; }
.video-error { @apply mb-2 shrink-0 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-950/40 dark:text-red-200; }
.video-activity { @apply flex min-h-0 min-w-0 flex-col gap-3; }
.video-task-panel { @apply flex h-44 shrink-0 flex-col p-3; }
.video-task-panel header { @apply flex items-center justify-between gap-2 text-gray-400; }
.video-task-list { @apply mt-3 min-h-0 flex-1 space-y-2 overflow-y-auto; }
.video-empty-tasks { @apply pt-2 text-[11px] text-gray-400 dark:text-dark-400; }
.video-task-entry { @apply flex w-full items-center gap-2 rounded-lg p-2 text-left; background: var(--surface-inset); }
.video-task-entry > span { @apply min-w-0 flex-1; }
.video-task-entry strong { @apply block truncate text-[11px] font-medium text-gray-700 dark:text-dark-100; }
.video-task-entry small { @apply mt-1 block text-[10px] text-gray-500 dark:text-dark-300; }
.video-task-entry > svg { @apply shrink-0 text-primary-600 dark:text-primary-300; }
.video-history { @apply flex min-h-0 flex-1 flex-col; }
.video-history-heading { @apply space-y-2 p-3 pb-2; }
.video-history-heading > div { @apply flex items-center justify-between; }
.video-history-search { @apply min-w-0 rounded-lg px-2.5 py-2 text-[11px]; }
.video-session-list { @apply max-h-32 shrink-0 overflow-y-auto px-2 pb-2; }
.video-session-row { @apply flex items-center rounded-lg px-1; }
.video-session-row.is-active { @apply bg-primary-50/80 dark:bg-primary-500/10; }
.video-session-select { @apply min-w-0 flex-1 px-1 py-2 text-left; }
.video-session-select span { @apply block truncate text-[11px] font-medium text-gray-700 dark:text-dark-100; }
.video-session-select small { @apply mt-1 block text-[10px] text-gray-500 dark:text-dark-400; }
.video-icon-button { @apply flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 hover:text-gray-800 disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-dark-100; }
.video-history-filters { @apply mx-3 mb-2 grid shrink-0 grid-cols-3 gap-1 rounded-lg p-1; background: var(--surface-inset); }
.video-history-filters button { @apply rounded py-1 text-[10px] text-gray-500 dark:text-dark-300; }
.video-history-filters button[aria-pressed='true'] { @apply bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-gray-100; }
.video-history-results { @apply min-h-0 flex-1 space-y-1 overflow-y-auto p-2; }
.video-history-empty { @apply py-8 text-center text-xs text-gray-400; }
.video-result { @apply flex w-full min-w-0 items-center gap-2 rounded-xl border border-transparent p-2 text-left transition hover:bg-gray-50 dark:hover:bg-dark-700; }
.video-result[aria-pressed='true'] { @apply border-primary-400/50 bg-primary-50/40 dark:bg-primary-500/5; }
.video-history-play { @apply flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300; }
.video-history-info { @apply min-w-0 flex-1; }
.video-history-info strong { @apply line-clamp-2 text-[11px] font-medium text-gray-700 dark:text-dark-100; }
.video-history-info > span, .video-history-info small { @apply mt-1 block truncate text-[9px] text-gray-500 dark:text-dark-400; }
.video-history-hint { @apply shrink-0 px-3 pb-3 pt-1 text-[10px] leading-relaxed text-gray-400 dark:text-dark-400; }
button:focus-visible { @apply outline-none ring-2 ring-primary-500 ring-offset-2 dark:ring-offset-dark-900; }
.video-panel-tabs { display: none; }
@media (min-width: 1200px) and (max-width: 1399px) { .video-grid { grid-template-columns: 320px minmax(0, 1fr) 220px; } }
@media (max-width: 1199px) {
  .video-workflow { display: none; }
  .video-panel-tabs { @apply grid shrink-0 grid-cols-3 gap-1 rounded-xl border p-1; background: var(--surface-card); border-color: var(--surface-border); }
  .video-panel-tabs button { @apply flex h-9 items-center justify-center gap-2 rounded-lg text-xs font-medium text-gray-500 dark:text-dark-300; }
  .video-panel-tabs button[aria-pressed='true'] { @apply bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300; }
  .video-panel-tabs button > span { @apply text-[10px] opacity-70; }
  .video-grid { grid-template-columns: minmax(0, 1fr); }
  .video-grid[data-panel='compose'] > :not(.video-compose-panel),
  .video-grid[data-panel='result'] > :not(.video-workspace),
  .video-grid[data-panel='history'] > :not(.video-activity) { display: none; }
  .video-activity { @apply h-full; }
  .video-task-panel { @apply h-36; }
  .video-result-actions .btn { @apply min-h-9; }
}
@media (max-width: 479px) {
  .video-panel { @apply gap-2; }
  .video-workspace { @apply p-3; }
}
</style>
