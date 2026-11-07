<template>
  <AppLayout>
    <div class="studio-shell -m-4 md:-m-6 lg:-m-8">
      <header v-if="activeTab === 'image'" class="workflow-bar">
        <button type="button" class="new-creation" :disabled="!studio.ready" @click="newSession"><Icon name="plus" size="sm" />{{ t('imageGeneration.newCreation') }}</button>
        <ol :aria-label="t('imageGeneration.creationSteps')">
          <li><span>01</span>{{ t('imageGeneration.chooseScene') }}</li>
          <li><span>02</span>{{ t('imageGeneration.describeImage') }}</li>
          <li><span>03</span>{{ t('imageGeneration.prepareReferences') }}</li>
          <li><Icon name="check" size="sm" />{{ t('imageGeneration.confirmSettings') }}</li>
        </ol>
      </header>
      <nav v-if="activeTab === 'image'" class="mobile-panel-tabs" :aria-label="t('imageGeneration.workspacePanels')">
        <button v-for="panel in workspacePanels" :key="panel.value" type="button" :aria-pressed="mobilePanel === panel.value" :aria-controls="panel.control" @click="mobilePanel = panel.value">{{ panel.label }}<span v-if="panel.value === 'result' && results.length">{{ results.length }}</span></button>
      </nav>
      <p v-if="activeTab === 'image' && studio.storageError" role="alert" class="storage-notice status-banner status-warning">{{ t(studio.storageError === 'conflict' ? 'imageGeneration.sessionStorageConflict' : 'imageGeneration.sessionStorageError') }}</p>
      <div v-if="activeTab === 'image'" class="studio-grid" :data-panel="mobilePanel">
        <aside class="composer-panel studio-panel" :aria-label="t('imageGeneration.createTitle')">
          <header class="composer-heading">
        <div class="studio-tabs" role="tablist" :aria-label="t('imageGeneration.studioTitle')">
          <button id="studio-image-tab" ref="imageTab" type="button" role="tab" aria-selected="true" tabindex="0" aria-controls="studio-image-panel" @click="selectTab('image')" @keydown.right.prevent="selectTab('video')" @keydown.left.prevent="selectTab('video')">{{ t('imageGeneration.imageTab') }}</button>
          <button v-if="videoEnabled" id="studio-video-tab" ref="videoTab" type="button" role="tab" aria-selected="false" tabindex="-1" aria-controls="studio-video-panel" @click="selectTab('video')" @keydown.right.prevent="selectTab('image')" @keydown.left.prevent="selectTab('image')"><Icon name="play" size="xs" />{{ t('imageGeneration.videoTab') }}</button>
        </div>

            <span class="composer-heading-mark"><span></span>{{ t('imageGeneration.studioTitle') }}</span>
          </header>
          <div v-if="activeTab === 'image'" id="studio-image-panel" role="tabpanel" aria-labelledby="studio-image-tab" class="composer-content">
            <div v-if="loadingKeys || !studio.ready" class="loading-panel"><LoadingSpinner size="lg" /><p>{{ t('imageGeneration.loadingKeys') }}</p></div>
            <div v-else-if="!imageKeys.length && !studio.sessions.some(session => session.results.length || session.runs.length)" class="loading-panel">
              <Icon name="key" size="lg" /><h2>{{ t('imageGeneration.noKeysTitle') }}</h2><p>{{ t('imageGeneration.noKeysDescription') }}</p><RouterLink to="/keys" class="btn btn-primary btn-sm">{{ t('imageGeneration.manageKeys') }}</RouterLink>
            </div>
            <form v-else class="composer-card" @submit.prevent="submitGeneration()" @dragenter.prevent="onDragEnter" @dragover.prevent="onDragOver" @dragleave.prevent="onDragLeave" @drop.prevent="onDrop">
              <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp" multiple class="sr-only" @change="onFileChange" />
              <div class="composer-scroll">
                <section class="composer-section scene-section">
                  <div class="section-heading"><span class="step-number">01</span><div><h2>{{ t('imageGeneration.sceneHeading') }}</h2><p>{{ t('imageGeneration.sceneHint') }}</p></div></div>
                  <div class="scene-options">
                    <button v-for="scene in promptScenes" :key="scene.id" type="button" :data-scene="scene.id" @click="applyScene(scene.prompt)"><span class="scene-symbol"><Icon :name="scene.icon" size="sm" /></span><span><strong>{{ scene.title }}</strong><small>{{ scene.hint }}</small></span><Icon name="chevronRight" size="xs" /></button>
                  </div>
                </section>
                <section class="composer-section prompt-section">
                  <div class="section-heading"><span class="step-number">02</span><div><h2><label for="image-prompt">{{ t('imageGeneration.describeImage') }}</label></h2><p>{{ t('imageGeneration.promptGuide') }}</p></div></div>
                  <div class="prompt-area">
                    <textarea id="image-prompt" v-model="form.prompt" :maxlength="capabilities.maxPromptLength" rows="5" class="image-prompt-input" :placeholder="t('imageGeneration.workbenchPromptPlaceholder')" :aria-label="t('imageGeneration.prompt')" @paste="onPaste" @keydown.ctrl.enter.prevent="submitGeneration()" @keydown.meta.enter.prevent="submitGeneration()"></textarea>
                    <div class="prompt-meta"><span>{{ form.prompt.length }} / {{ capabilities.maxPromptLength }}</span><kbd>⌘ / Ctrl + Enter</kbd></div>
                    <div v-if="dragging" class="drop-overlay"><Icon name="upload" size="md" />{{ t('imageGeneration.dropReference') }}</div>
                  </div>
                </section>
                <section class="composer-section reference-panel">
                  <div class="reference-heading"><div class="section-heading"><span class="step-number">03</span><h2>{{ t('imageGeneration.referenceImages') }} <small>{{ referenceImages.length }} / {{ capabilities.maxReferenceImages }}</small></h2></div><button v-if="referenceImages.length" type="button" @click="clearAllReferences">{{ t('imageGeneration.clear') }}</button></div>
                  <div class="reference-strip">
                    <div v-for="(image, index) in referenceImages" :key="image.url" class="reference-card">
                      <button type="button" class="reference-preview" :aria-label="t('imageGeneration.referenceAlt', { index: index + 1 })" @click="preview = { src: image.url }"><img :src="image.url" :alt="t('imageGeneration.referenceAlt', { index: index + 1 })" /></button>
                      <div class="reference-actions">
                        <button type="button" :disabled="index === 0" :aria-label="t('imageGeneration.moveReferenceLeft', { index: index + 1 })" @click="moveReference(index, -1)"><Icon name="chevronLeft" size="xs" /></button>
                        <button type="button" :disabled="index === referenceImages.length - 1" :aria-label="t('imageGeneration.moveReferenceRight', { index: index + 1 })" @click="moveReference(index, 1)"><Icon name="chevronRight" size="xs" /></button>
                        <button type="button" :aria-label="t('imageGeneration.removeReference')" @click="removeReference(index)"><Icon name="x" size="xs" /></button>
                      </div>
                    </div>
                    <button v-if="referenceImages.length < capabilities.maxReferenceImages" type="button" class="composer-icon-button reference-upload" :aria-label="t('imageGeneration.uploadReference')" @click="fileInput?.click()"><Icon name="upload" size="md" /><span>{{ t('imageGeneration.addReference') }}</span></button>
                  </div>
                  <p class="reference-input-hint">{{ t('imageGeneration.referenceInputHint') }}</p>
                </section>
                <section class="composer-section settings-section">
                  <div class="section-heading"><h2>{{ t('imageGeneration.imageSettings') }}</h2><span class="section-aside">{{ t('imageGeneration.settingsFollowModel') }}</span></div>
                  <label for="image-api-key" class="field-label">{{ t('imageGeneration.apiKey') }}</label>
                  <Select id="image-api-key" :model-value="form.apiKeyId" @update:model-value="selectApiKey" :options="keyOptions" :placeholder="t('imageGeneration.selectKey')" :aria-label="t('imageGeneration.apiKey')" class="composer-select composer-key-select" />
                  <p v-if="form.apiKeyId !== null && !selectedKey" role="alert" class="inline-warning">{{ t('imageGeneration.savedKeyUnavailable') }}</p>
                  <label for="image-model" class="field-label">{{ t('imageGeneration.model') }}</label>
                  <Select id="image-model" v-model="form.model" :options="modelOptions" :placeholder="t('imageGeneration.modelPlaceholder')" :empty-text="modelError || t('imageGeneration.noModels')" :disabled="loadingModels || !selectedKey" :loading="loadingModels" :creatable="true" searchable :aria-label="t('imageGeneration.model')" class="composer-select composer-model-select" />
                  <fieldset><legend>{{ t('imageGeneration.aspectRatio') }}</legend><div class="ratio-options"><button v-for="ratio in ratioOptions" :key="ratio.value" type="button" class="ratio-card" :aria-pressed="form.aspectRatio === ratio.value" @click="form.aspectRatio = ratio.value"><span class="ratio-shape" :style="{ aspectRatio: ratio.cssRatio }"></span>{{ ratio.value === 'auto' ? t('imageGeneration.auto') : ratio.value }}</button></div></fieldset>
                  <fieldset><legend>{{ t('imageGeneration.resolution') }}</legend><div class="choice-options"><button v-for="resolution in resolutions" :key="resolution" type="button" class="resolution-card quality-card" :aria-pressed="form.resolution === resolution" @click="form.resolution = resolution">{{ resolution }}</button></div></fieldset>
                  <fieldset v-if="showQuality"><legend>{{ t('imageGeneration.quality') }}</legend><div class="choice-options"><button v-for="quality in qualities" :key="quality.value" type="button" class="quality-card" :aria-pressed="form.quality === quality.value" @click="form.quality = quality.value">{{ quality.label }}</button></div></fieldset>
                  <div class="count-row"><label for="image-count">{{ t('imageGeneration.count') }}</label><Select id="image-count" v-model="form.count" :options="countOptions" :aria-label="t('imageGeneration.count')" class="composer-select composer-count-select" /></div>
                  <p class="size-note">{{ displaySize }}</p>
                </section>
              </div>
              <footer class="composer-footer"><div><strong>{{ form.model || t('imageGeneration.selectModel') }}</strong><span>{{ ratioLabel }} · {{ form.resolution }} · ×{{ form.count }}</span></div><button type="submit" class="generate-button" :disabled="generating || !selectedKey || !form.model || !form.prompt.trim()"><LoadingSpinner v-if="generating" size="sm" /><Icon v-else name="sparkles" size="sm" />{{ generating ? t('imageGeneration.generating') : t('imageGeneration.generate') }}</button></footer>
            </form>
          </div>
        </aside>
        <template v-if="activeTab === 'image'">
          <section id="image-result-panel" class="stage-panel studio-panel" tabindex="-1" :aria-label="t('imageGeneration.resultsTitle')">
            <header class="canvas-header"><div><h2>{{ t('imageGeneration.resultsTitle') }}</h2><p>{{ generating ? t('imageGeneration.generating') : activeResult ? t('imageGeneration.resultReady') : t('imageGeneration.resultsHint') }}</p></div><button v-if="activeResult" type="button" class="history-icon" :aria-label="t('imageGeneration.previewTitle')" @click="preview = activeResult"><Icon name="search" size="sm" /></button></header>
            <div class="studio-notices">
              <div v-if="recoverableRun && !generating" class="status-banner status-warning"><span>{{ t(recoverableRun.error || 'imageGeneration.taskRecoverable') }}</span><button type="button" class="btn btn-secondary btn-sm" @click="recoverSession(studio.activeId!)">{{ t('imageGeneration.queryTask') }}</button></div>
              <p v-else-if="studio.activeSession?.interrupted && !generating" role="status" class="status-banner status-warning">{{ t('imageGeneration.submitUnknown') }}</p>
              <p v-if="sessionError && !studio.activeSession?.interrupted" role="alert" class="status-banner status-error">{{ sessionError }}</p>
            </div>
            <div class="canvas-viewport" :class="{ 'has-image': activeResult }" @dragenter.prevent="onDragEnter" @dragover.prevent="onDragOver" @dragleave.prevent="onDragLeave" @drop.prevent="onDrop">
              <div v-if="activeResult && imageLoadError" class="stage-placeholder" role="status"><Icon name="exclamationCircle" size="lg" /><p>{{ t('imageGeneration.imageLoadFailed') }}</p><button type="button" class="btn btn-secondary btn-sm" @click="retryImage">{{ t('imageGeneration.reloadImage') }}</button></div>
              <button v-else-if="activeResult" type="button" class="result-preview" :aria-label="t('imageGeneration.previewTitle')" @click="preview = activeResult"><img :key="imageLoadAttempt" :src="activeResult.src" :alt="activeResult.prompt || t('imageGeneration.previewTitle')" @error="imageLoadError = true" /></button>
              <div v-else class="stage-placeholder"><div class="empty-image-frame"><LoadingSpinner v-if="generating" size="lg" /><Icon v-else name="grid" size="xl" /></div><h3>{{ generating ? t('imageGeneration.generating') : t('imageGeneration.resultPlaceholder') }}</h3><p>{{ t('imageGeneration.workbenchEmptyHint') }}</p></div>
              <div v-if="activeResult" class="image-caption"><span>{{ activeResult.model }}</span><button type="button" @click="preview = activeResult">{{ t('imageGeneration.previewTitle') }}<Icon name="search" size="xs" /></button></div>
            </div>
            <div v-if="batchResults.length > 1" class="batch-strip" :aria-label="t('imageGeneration.batchResults')"><button v-for="(result, index) in batchResults" :key="result.id" type="button" class="batch-thumb" :aria-pressed="activeResult?.id === result.id" :aria-label="t('imageGeneration.imageNumber', { index: index + 1 })" @click="activeResult = result"><img :src="result.src" alt="" /></button></div>
            <div v-if="generating || latestRun" class="task-status" role="status" aria-live="polite"><LoadingSpinner v-if="generating" size="sm" /><span>{{ t(generating ? 'imageGeneration.progressCount' : 'imageGeneration.completedCount', { completed: generating ? pendingGeneration?.completed : latestRun?.completed, count: generating ? pendingGeneration?.count : latestRun?.requested }) }}</span><span v-if="generating" aria-live="off">{{ t('imageGeneration.elapsed', { seconds: elapsedSeconds }) }}</span><span v-else-if="latestRun && latestRun.completed < latestRun.requested">{{ t(latestRun.status === 'failed' ? 'imageGeneration.batchFailed' : latestRun.status === 'completed' ? 'imageGeneration.fewerImagesReturned' : 'imageGeneration.batchUncertain') }}</span></div>
            <footer v-if="activeResult" class="result-actions"><button type="button" class="btn btn-ghost btn-sm reuse-result" @click="reuseResult(activeResult)"><Icon name="refresh" size="xs" />{{ t('imageGeneration.reuseParameters') }}</button><button type="button" class="btn btn-secondary btn-sm reference-result" :disabled="importingReference || referenceImages.length >= capabilities.maxReferenceImages" @click="useResultAsReference(activeResult)"><Icon name="plus" size="xs" />{{ t('imageGeneration.useAsReference') }}</button><button type="button" class="btn btn-primary btn-sm" @click="downloadResult(activeResult)"><Icon name="download" size="xs" />{{ t('imageGeneration.download') }}</button></footer>
          </section>
          <aside id="image-studio-history" class="activity-rail" tabindex="-1" :aria-label="t('imageGeneration.historyTitle')">
            <section class="task-panel studio-panel"><header><div><h2>{{ t('imageGeneration.runningTasks') }}</h2><p>{{ t('imageGeneration.taskCount', { count: activeTaskSessions.length }) }}</p></div><Icon name="clock" size="sm" /></header><div class="task-list"><p v-if="!activeTaskSessions.length" class="no-tasks">{{ t('imageGeneration.noRunningTasks') }}</p><button v-for="session in activeTaskSessions" :key="session.id" type="button" class="task-entry" @click="selectSession(session.id, 'result')"><LoadingSpinner v-if="session.pending" size="sm" /><Icon v-else name="exclamationCircle" size="sm" /><span><strong>{{ session.title || t('imageGeneration.untitledSession') }}</strong><small>{{ session.pending ? t('imageGeneration.progressCount', { completed: session.pending.completed, count: session.pending.count }) : t('imageGeneration.batchUncertain') }}</small></span><Icon name="chevronRight" size="xs" /></button></div></section>
            <section class="history-sidebar studio-panel"><div class="history-heading"><div class="history-title"><h2>{{ t('imageGeneration.generationHistory') }}</h2><button type="button" class="history-icon" :aria-label="t('imageGeneration.newSession')" @click="newSession"><Icon name="plus" size="sm" /></button></div><input v-model="historySearch" type="search" class="input history-search" :placeholder="t('imageGeneration.searchHistory')" :aria-label="t('imageGeneration.searchHistory')" /></div>
              <div class="session-list" data-testid="image-sessions">
                <div v-for="session in filteredSessions" :key="session.id" class="session-row" :class="{ 'is-active': session.id === studio.activeId }">
                  <button type="button" class="session-select" :aria-pressed="session.id === studio.activeId" @click="selectSession(session.id)"><span>{{ session.title || t('imageGeneration.untitledSession') }}</span><small>{{ session.pending ? t('imageGeneration.generating') : t('imageGeneration.sessionImageCount', { count: session.results.length }) }}</small></button>
                  <button type="button" class="history-icon" :aria-label="t('imageGeneration.renameSession')" @click="startRename(session.id, session.title)"><Icon name="edit" size="xs" /></button>
                  <button type="button" class="history-icon" :disabled="Boolean(session.pending)" :aria-label="t('imageGeneration.deleteSession')" @click="deletingSessionId = session.id"><Icon name="trash" size="xs" /></button>
                </div>
              </div>
              <div class="history-filters"><button v-for="filter in historyFilters" :key="filter.value" type="button" :aria-pressed="historyFilter === filter.value" @click="historyFilter = filter.value">{{ filter.label }}</button></div>
              <div class="history-results">
                <p v-if="!filteredResults.length" class="history-empty">{{ t(historySearch ? 'imageGeneration.noSearchResults' : 'imageGeneration.noHistory') }}</p>
                <article v-for="(result, index) in filteredResults" :key="result.id" class="history-item-card" :class="{ 'is-active': activeResult?.id === result.id }">
                  <button type="button" class="history-result-select" :aria-pressed="activeResult?.id === result.id" @click="selectResult(result)">
                    <img :src="result.src" :alt="t('imageGeneration.resultAlt', { index: index + 1 })" loading="lazy" />
                    <span class="history-result-info"><strong>{{ result.prompt || result.model }}</strong><span>{{ result.model }} · {{ result.aspectRatio }}</span><small>{{ result.createdAt }}</small></span>
                  </button>
                  <button type="button" class="history-icon" :aria-label="t('imageGeneration.download')" @click="downloadResult(result)"><Icon name="download" size="xs" /></button>
                </article>
              </div>

            </section>
          </aside>
        </template>
      </div>
      <VideoGenerationPanel v-else-if="videoEnabled" id="studio-video-panel" role="tabpanel" aria-labelledby="studio-video-tab" :api-keys="apiKeys" :loading-keys="loadingKeys">
        <template #heading>
          <header class="composer-heading">
        <div class="studio-tabs" role="tablist" :aria-label="t('imageGeneration.studioTitle')">
          <button id="studio-image-tab" ref="imageTab" type="button" role="tab" aria-selected="false" tabindex="-1" aria-controls="studio-image-panel" @click="selectTab('image')" @keydown.right.prevent="selectTab('video')" @keydown.left.prevent="selectTab('video')">{{ t('imageGeneration.imageTab') }}</button>
          <button v-if="videoEnabled" id="studio-video-tab" ref="videoTab" type="button" role="tab" aria-selected="true" tabindex="0" aria-controls="studio-video-panel" @click="selectTab('video')" @keydown.right.prevent="selectTab('image')" @keydown.left.prevent="selectTab('image')"><Icon name="play" size="xs" />{{ t('imageGeneration.videoTab') }}</button>
        </div>

            <span class="composer-heading-mark"><span></span>{{ t('imageGeneration.studioTitle') }}</span>
          </header>
        </template>
      </VideoGenerationPanel>
        <ConfirmDialog :show="Boolean(deletingSessionId)" :title="t('imageGeneration.deleteSession')" :message="t('imageGeneration.deleteSessionConfirm')" @confirm="deleteSession" @cancel="deletingSessionId = null" />
        <ConfirmDialog :show="Boolean(resubmitSessionId)" :title="t('imageGeneration.confirmNewGeneration')" :message="t('imageGeneration.confirmNewGenerationMessage')" :danger="false" @confirm="confirmResubmit" @cancel="resubmitSessionId = null" />
        <BaseDialog :show="Boolean(renamingSessionId)" :title="t('imageGeneration.renameSession')" width="narrow" @close="renamingSessionId = null">
          <form id="rename-image-session" @submit.prevent="renameSession"><label for="session-name" class="sr-only">{{ t('imageGeneration.sessionName') }}</label><input id="session-name" v-model="sessionName" class="input" maxlength="120" autofocus /></form>
          <template #footer><button type="submit" form="rename-image-session" class="btn btn-primary" :disabled="!sessionName.trim()">{{ t('common.save') }}</button></template>
        </BaseDialog>
        <BaseDialog :show="Boolean(preview)" :title="t('imageGeneration.previewTitle')" width="full" close-on-click-outside @close="preview = null">
          <div v-if="preview" class="space-y-3"><div class="image-preview-frame"><img :src="preview.src" :alt="t('imageGeneration.previewTitle')" /></div><p v-if="preview.revisedPrompt" class="text-sm text-gray-500">{{ preview.revisedPrompt }}</p></div>
          <template #footer><button type="button" class="btn btn-secondary btn-sm" @click="preview = null">{{ t('common.close') }}</button><button type="button" class="btn btn-primary btn-sm" @click="preview && downloadResult(preview)"><Icon name="download" size="xs" />{{ t('imageGeneration.download') }}</button></template>
        </BaseDialog>

    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { keysAPI } from '@/api/keys'
import {
  listImageModels,
  openAIImageSize,
  resolveImageCapabilities,
  supportsImageGeneration,
  type ImageAspectRatio,
  type ImageQuality,
} from '@/api/imageGeneration'
import { BaseDialog, ConfirmDialog, LoadingSpinner } from '@/components/common'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import VideoGenerationPanel from '@/components/image-generation/VideoGenerationPanel.vue'
import { resultImageFile } from '@/components/image-generation/image-result-file'
import { useAppStore } from '@/stores/app'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { useImageStudioStore, type ImageStudioForm, type ImageStudioResult } from '@/stores/imageStudio'
import type { ApiKey } from '@/types'

interface ReferenceImage {
  file: File
  url: string
}

type GenerationMode = 'text' | 'image'
type HistoryFilter = 'all' | GenerationMode

const { t } = useI18n()
const appStore = useAppStore()
const studio = useImageStudioStore()
const route = useRoute()
const router = useRouter()
const videoEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.videoGeneration))
const activeTab = ref<'image' | 'video'>(route.query.tab === 'video' && videoEnabled.value ? 'video' : 'image')
const imageTab = ref<HTMLButtonElement | null>(null)
const videoTab = ref<HTMLButtonElement | null>(null)
async function selectTab(tab: 'image' | 'video') {
  if (tab === 'video' && !videoEnabled.value) tab = 'image'
  activeTab.value = tab
  if (tab === 'image') mobilePanel.value = 'compose'
  await nextTick()
  const button = tab === 'image' ? imageTab.value : videoTab.value
  button?.focus()
}
watch(activeTab, tab => {
  void router.replace({ query: { ...route.query, tab: tab === 'video' ? 'video' : undefined } })
})
watch([() => route.query.tab, videoEnabled], ([tab, enabled]) => {
  const nextTab = tab === 'video' && enabled ? 'video' : 'image'
  if (activeTab.value !== nextTab) {
    activeTab.value = nextTab
  } else if (tab === 'video' && !enabled) {
    // Also normalize a disabled video link on the initial page load.
    void router.replace({ query: { ...route.query, tab: undefined } })
  }
}, { immediate: true })
const apiKeys = ref<ApiKey[]>([])
const loadingKeys = ref(true)
const loadingModels = ref(false)
const modelError = ref('')
const availableModels = ref<string[]>([])
const referenceImages = ref<ReferenceImage[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
type WorkspacePanel = 'compose' | 'result' | 'history'
const mobilePanel = ref<WorkspacePanel>('compose')
const workspacePanels = computed(() => [
  { value: 'compose' as const, label: t('imageGeneration.composePanel'), control: 'studio-image-panel' },
  { value: 'result' as const, label: t('imageGeneration.worksPanel'), control: 'image-result-panel' },
  { value: 'history' as const, label: t('imageGeneration.recordsPanel'), control: 'image-studio-history' },
])
const historySearch = ref('')
const renamingSessionId = ref<string | null>(null)
const sessionName = ref('')
const resubmitSessionId = ref<string | null>(null)
const importingReference = ref(false)
const imageLoadError = ref(false)
const imageLoadAttempt = ref(0)
const now = ref(Date.now())
let clockTimer: ReturnType<typeof setInterval> | undefined
const attemptedRecovery = new Set<string>()
async function revealWorkspacePanel(panel: WorkspacePanel) {
  mobilePanel.value = panel
  await nextTick()
  if (window.innerWidth > 1199 || activeTab.value !== 'image' || mobilePanel.value !== panel) return
  const destination = panel === 'compose'
    ? document.getElementById('image-prompt') ?? imageTab.value
    : document.getElementById(panel === 'result' ? 'image-result-panel' : 'image-studio-history')
  destination?.focus({ preventScroll: panel !== 'compose' })
}
function newSession() {
  studio.createSession()
  void revealWorkspacePanel('compose')
}
function selectSession(id: string, panel: WorkspacePanel = 'compose') {
  studio.activeId = id
  void revealWorkspacePanel(panel)
}
function selectResult(result: ImageStudioResult) {
  activeResult.value = result
  void revealWorkspacePanel('result')
}
function retryImage() { imageLoadError.value = false; imageLoadAttempt.value++ }
function startRename(id: string, title: string) { renamingSessionId.value = id; sessionName.value = title }
function renameSession() {
  if (renamingSessionId.value) studio.renameSession(renamingSessionId.value, sessionName.value)
  renamingSessionId.value = null
}

const dragging = ref(false)
const pendingGeneration = computed(() => studio.activeSession?.pending ?? null)
const generating = computed(() => pendingGeneration.value !== null)
const results = computed(() => studio.activeSession?.results ?? [])
const activeResult = computed({
  get: () => results.value.find(result => result.id === studio.activeSession?.activeResultId) ?? null,
  set: (result: ImageStudioResult | null) => { if (studio.activeSession) studio.activeSession.activeResultId = result?.id ?? null },
})
const preview = ref<{ src: string; revisedPrompt?: string } | null>(null)
const latestRun = computed(() => studio.activeSession?.runs[0])
const recoverableRun = computed(() => studio.activeSession?.runs.find(run => run.taskId && run.status === 'processing'))
const activeTaskSessions = computed(() => studio.sessions.filter(session => session.pending || session.interrupted || session.runs.some(run => run.taskId && run.status === 'processing')))
const batchResults = computed(() => activeResult.value?.runId
  ? results.value.filter(result => result.runId === activeResult.value!.runId)
  : activeResult.value ? [activeResult.value] : [])
const elapsedSeconds = computed(() => Math.max(0, Math.floor((now.value - (pendingGeneration.value?.startedAt ?? now.value)) / 1000)))
const deletingSessionId = ref<string | null>(null)
const sessionError = computed(() => {
  const error = studio.activeSession?.error || ''
  return error.startsWith('imageGeneration.') ? t(error) : error
})
const historyFilter = ref<HistoryFilter>('all')
let modelRequestID = 0

const emptyForm: ImageStudioForm = {
  apiKeyId: null as number | null,
  model: '',
  prompt: '',
  aspectRatio: 'auto' as ImageAspectRatio,
  quality: 'standard' as ImageQuality,
  resolution: '1K',
  count: 1,
}
const form = computed(() => studio.activeSession?.form ?? emptyForm)
function selectApiKey(value: string | number | boolean | null | undefined) {
  const id = typeof value === 'number' ? value : null
  if (form.value.apiKeyId === id) return
  form.value.model = ''
  form.value.apiKeyId = id
}

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
  { value: 'standard' as const, label: t('imageGeneration.qualityStandard') },
  { value: 'high' as const, label: t('imageGeneration.qualityHigh') },
])
const countOptions = [1, 2, 4].map(value => ({ value, label: `×${value}` }))
const promptScenes = computed(() => [
  { id: 'product', icon: 'grid' as const, title: t('imageGeneration.exampleProduct'), hint: t('imageGeneration.sceneProductHint'), prompt: t('imageGeneration.sceneProductPrompt') },
  { id: 'portrait', icon: 'user' as const, title: t('imageGeneration.examplePortrait'), hint: t('imageGeneration.scenePortraitHint'), prompt: t('imageGeneration.scenePortraitPrompt') },
  { id: 'poster', icon: 'sparkles' as const, title: t('imageGeneration.examplePoster'), hint: t('imageGeneration.scenePosterHint'), prompt: t('imageGeneration.scenePosterPrompt') },
])
async function applyScene(prompt: string) {
  form.value.prompt = [form.value.prompt.trim(), prompt].filter(Boolean).join('\n').slice(0, capabilities.value.maxPromptLength)
  await nextTick()
  document.getElementById('image-prompt')?.focus()
}

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
const selectedKey = computed(() => imageKeys.value.find(key => key.id === form.value.apiKeyId) || null)
const keyOptions = computed(() => imageKeys.value.map(key => ({
  value: key.id,
  label: `${key.name} · ${key.group?.name || ''}`,
})))
const modelOptions = computed(() => availableModels.value.map(model => ({ value: model, label: model })))
const capabilities = computed(() => resolveImageCapabilities(selectedKey.value?.group?.platform, form.value.model, form.value.resolution))
const resolutions = computed(() => capabilities.value.resolutions)
const showQuality = computed(() => capabilities.value.supportsQuality)
const ratioOptions = computed(() => capabilities.value.aspectRatios.map(value => ({ value, cssRatio: ratioShapes[value] })))
const ratioLabel = computed(() => form.value.aspectRatio === 'auto' ? t('imageGeneration.autoRatio') : form.value.aspectRatio)
const searchTerm = computed(() => historySearch.value.trim().toLocaleLowerCase())
const filteredSessions = computed(() => studio.sessions.filter(session => !searchTerm.value ||
  [session.title, ...session.results.map(result => `${result.prompt} ${result.model}`)].some(text => text.toLocaleLowerCase().includes(searchTerm.value))))
const filteredResults = computed(() => results.value.filter(result =>
  (historyFilter.value === 'all' || result.mode === historyFilter.value) &&
  (!searchTerm.value || `${result.prompt} ${result.model}`.toLocaleLowerCase().includes(searchTerm.value))))
const displaySize = computed(() => {
  if (!showQuality.value) return t('imageGeneration.sizeByModel')
  const size = openAIImageSize(form.value.aspectRatio, form.value.resolution, form.value.model)
  return size === 'auto' ? t('imageGeneration.auto') : `${size.replace('x', ' × ')} px`
})

async function loadKeys() {
  loadingKeys.value = true
  try {
    const keys: ApiKey[] = []
    let page = 1
    while (true) {
      const response = await keysAPI.list(page, 100, {
        sort_by: 'created_at',
        sort_order: 'desc',
      })
      keys.push(...(response.items || []))
      if (page >= response.pages || !response.items?.length) break
      page += 1
    }
    apiKeys.value = keys
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
  modelError.value = ''
  if (!key?.group) { loadingModels.value = false; return }

  loadingModels.value = true
  try {
    const models = await listImageModels(key.key, key.group.platform)
    if (requestID !== modelRequestID) return
    availableModels.value = models
    if (!form.value.model) form.value.model = models[0] || ''
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
    if (file.size > capabilities.value.maxReferenceImageBytes) {
      appStore.showError(t('imageGeneration.fileTooLarge', { name: file.name }))
      continue
    }
    valid.push(file)
  }

  const slots = Math.max(0, capabilities.value.maxReferenceImages - referenceImages.value.length)
  if (valid.length > slots) appStore.showError(t('imageGeneration.tooManyReferences', { count: capabilities.value.maxReferenceImages }))
  studio.activeSession?.referenceFiles.push(...valid.slice(0, slots))
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
  const files = Array.from(event.clipboardData?.files || [])
  if (!files.length) return
  event.preventDefault()
  addReferenceImages(files)
}

function removeReference(index: number) {
  studio.activeSession?.referenceFiles.splice(index, 1)
}

function moveReference(index: number, offset: number) {
  const files = studio.activeSession?.referenceFiles
  const target = index + offset
  if (!files || target < 0 || target >= files.length) return
  const [file] = files.splice(index, 1)
  if (file) files.splice(target, 0, file)
}
function clearAllReferences() { if (studio.activeSession) studio.activeSession.referenceFiles = [] }

async function reuseResult(result: ImageStudioResult) {
  studio.restoreResult(result.id)
  mobilePanel.value = 'compose'
  await nextTick()
  document.getElementById('image-prompt')?.focus()
}

async function useResultAsReference(result: ImageStudioResult) {
  const session = studio.activeSession
  if (!session || importingReference.value) return
  importingReference.value = true
  try {
    const file = await resultImageFile(result.src, capabilities.value.maxReferenceImageBytes)
    // A slow image download must not modify another account's or session's draft.
    if (studio.activeSession !== session) return
    addReferenceImages([file])
    void revealWorkspacePanel('compose')
  } catch (error) {
    if (studio.activeSession === session) {
      const message = errorMessage(error, 'imageGeneration.referenceLoadFailed')
      appStore.showError(message.startsWith('imageGeneration.') ? t(message) : t('imageGeneration.referenceLoadFailed'))
    }
  } finally { importingReference.value = false }
}

async function recoverSession(sessionId: string, automatic = false) {
  const session = studio.sessions.find(item => item.id === sessionId)
  const run = session?.runs.find(item => item.taskId && item.status === 'processing')
  if (!session || !run || session.pending) return
  const key = apiKeys.value.find(item => item.id === run.form.apiKeyId)
  if (!key) {
    if (!automatic) appStore.showError(t('imageGeneration.taskKeyUnavailable'))
    return
  }
  attemptedRecovery.add(run.id)
  try { await studio.recover(sessionId, key.key) }
  catch { /* The store retains the task and shows a recoverable query error. */ }
}
function confirmResubmit() {
  const sessionId = resubmitSessionId.value
  resubmitSessionId.value = null
  if (sessionId === studio.activeId) void submitGeneration(true)
}


function deleteSession() {
  if (deletingSessionId.value) studio.deleteSession(deletingSessionId.value)
  deletingSessionId.value = null
}

async function submitGeneration(confirmed = false) {
  if (generating.value) return

  const key = selectedKey.value
  const prompt = form.value.prompt.trim()
  if (!key?.group) {
    appStore.showError(t('imageGeneration.selectKey'))
    return
  }
  if (!form.value.model.trim()) {
    appStore.showError(t('imageGeneration.selectModel'))
    return
  }
  if (!prompt) {
    appStore.showError(t('imageGeneration.promptRequired'))
    return
  }

  if (!confirmed && studio.activeSession?.interrupted) {
    resubmitSessionId.value = studio.activeId
    return
  }
  const submittedSession = studio.activeSession
  void revealWorkspacePanel('result')
  try {
    const count = await studio.generate(key.key, key.group.platform)
    if (count) appStore.showSuccess(t('imageGeneration.generated', { count }))
  } catch (error) {
    const message = submittedSession?.error || errorMessage(error, t('imageGeneration.generateFailed'))
    appStore.showError(message.startsWith('imageGeneration.') ? t(message) : message)
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

async function downloadResult(result: { src: string }) {
  if (result.src.startsWith('data:')) {
    clickDownload(result.src, result.src.startsWith('data:image/jpeg') ? 'jpg' : result.src.startsWith('data:image/webp') ? 'webp' : 'png')
    return
  }

  try {
    const response = await fetch(result.src, { credentials: 'omit', referrerPolicy: 'no-referrer' })
    if (!response.ok) throw new Error(response.statusText)
    const blob = await response.blob()
    const url = URL.createObjectURL(blob)
    clickDownload(url, blob.type === 'image/jpeg' ? 'jpg' : blob.type === 'image/webp' ? 'webp' : 'png')
    setTimeout(() => URL.revokeObjectURL(url), 0)
  } catch {
    clickDownload(result.src, '', true)
  }
}

watch([() => studio.activeId, imageKeys, loadingKeys], () => {
  if (!studio.activeSession || loadingKeys.value) return
  if (form.value.apiKeyId === null && !studio.activeSession.runs.length && !studio.activeSession.results.length) {
    form.value.apiKeyId = imageKeys.value[0]?.id ?? null
  }
}, { immediate: true })
watch([() => studio.activeId, selectedKey], () => {
  preview.value = null
  dragging.value = false
  void loadModels()
}, { immediate: true })
watch([resolutions, selectedKey, loadingModels], ([options, key, loading]) => {
  if (key && !loading && form.value.model && !options.includes(form.value.resolution)) {
    form.value.resolution = options[options.length - 1]!
  }
})
watch([ratioOptions, selectedKey], ([options, key]) => {
  if (key && !options.some(option => option.value === form.value.aspectRatio)) {
    form.value.aspectRatio = options.some(option => option.value === 'auto') ? 'auto' : '1:1'
  }
})
watch(() => studio.activeSession?.referenceFiles, files => {
  clearReferences()
  referenceImages.value = (files || []).map(file => ({ file, url: URL.createObjectURL(file) }))
}, { immediate: true, deep: true })

watch(() => activeResult.value?.id, () => { imageLoadError.value = false; imageLoadAttempt.value = 0 })
watch([() => studio.ready, loadingKeys, () => studio.sessions.map(session =>
  `${session.id}:${Boolean(session.pending)}:${session.runs.filter(run => run.taskId && run.status === 'processing').map(run => run.id).join(',')}`).join('|')], () => {
  if (!studio.ready || loadingKeys.value) return
  for (const session of studio.sessions) {
    const run = session.runs.find(item => item.taskId && item.status === 'processing')
    if (run && !session.pending && !attemptedRecovery.has(run.id)) void recoverSession(session.id, true)
  }
}, { immediate: true })
onMounted(() => {
  void loadKeys()
  clockTimer = setInterval(() => { now.value = Date.now() }, 1000)
})
onBeforeUnmount(() => {
  modelRequestID++
  clearReferences()
  clearInterval(clockTimer)
  void studio.flush()
})
</script>

<style scoped>
.studio-shell { @apply flex min-h-0 flex-col gap-3 overflow-hidden p-3; height: calc(100dvh - 4rem - 1px); background: var(--surface-page); }
.studio-panel { @apply min-w-0 overflow-hidden rounded-2xl border; background: var(--surface-card); border-color: var(--surface-border); }
.workflow-bar { @apply flex h-12 shrink-0 items-center gap-5 rounded-xl px-4; background: var(--surface-card); }
.new-creation { @apply flex shrink-0 items-center gap-1.5 rounded-full bg-primary-50 px-3 py-2 text-xs font-medium text-primary-700 dark:bg-primary-500/10 dark:text-primary-400; }
.workflow-bar ol { @apply flex min-w-0 flex-1 items-center gap-4; }
.workflow-bar li { @apply flex min-w-0 flex-1 items-center gap-2 whitespace-nowrap text-xs font-medium text-gray-600 dark:text-dark-200; }
.workflow-bar li:not(:last-child)::after { content: ''; flex: 1; height: 1px; margin-left: 8px; background: var(--surface-border); }
.workflow-bar li > span, .step-number { @apply inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-xs font-semibold text-primary-700 dark:bg-primary-500/10 dark:text-primary-400; }
.workflow-bar li:last-child { @apply text-primary-700 dark:text-primary-400; }
.studio-grid { @apply grid min-h-0 flex-1 gap-3; grid-template-columns: 350px minmax(0, 1fr) 240px; }
.composer-panel, .composer-content, .composer-card { @apply flex min-h-0 flex-col; }
.composer-content, .composer-card { @apply flex-1; }
.composer-heading { @apply flex shrink-0 items-center justify-between gap-2 border-b px-3 py-3; border-color: var(--surface-border); }
.studio-tabs { @apply flex gap-1 rounded-lg p-1; background: var(--surface-inset); }
.studio-tabs button { @apply flex h-8 items-center gap-1.5 rounded-md px-3 text-xs font-medium text-gray-500 dark:text-dark-300; }
.studio-tabs button[aria-selected='true'] { @apply bg-white text-primary-700 shadow-sm dark:bg-dark-600 dark:text-primary-300; }
.composer-heading-mark { @apply flex items-center gap-1.5 text-[11px] text-gray-500 dark:text-dark-300; }
.composer-heading-mark > span { @apply h-1.5 w-1.5 rounded-full bg-primary-500; }
.loading-panel { @apply flex flex-1 flex-col items-center justify-center gap-3 p-5 text-center text-sm text-gray-500 dark:text-dark-300; }
.composer-scroll { @apply min-h-0 flex-1 overflow-y-auto px-4; scrollbar-width: thin; scrollbar-gutter: stable; }
.composer-section { @apply min-w-0 py-4; }
.composer-section + .composer-section { border-top: 1px solid var(--surface-border); }
.section-heading { @apply mb-3 flex min-w-0 items-center gap-2.5; }
.section-heading h2, .reference-heading h2 { @apply text-xs font-semibold text-gray-800 dark:text-gray-100; }
.section-heading p { @apply mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-dark-400; }
.section-aside { @apply ml-auto text-[10px] text-gray-400 dark:text-dark-400; }
.scene-options { @apply grid gap-1; }
.scene-options > button { @apply flex min-w-0 items-center gap-2.5 rounded-xl px-2 py-2 text-left transition hover:bg-gray-50 dark:hover:bg-dark-700/50; }
.scene-symbol { @apply flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-400; }
.scene-options button > span:nth-child(2) { @apply min-w-0 flex-1; }
.scene-options strong { @apply block text-xs font-medium text-gray-700 dark:text-gray-100; }
.scene-options small { @apply mt-1 block truncate text-[10px] text-gray-500 dark:text-dark-400; }
.scene-options button > svg { @apply shrink-0 text-gray-400; }
.prompt-area { @apply relative rounded-xl border px-3 pt-3; background: var(--surface-inset); border-color: var(--surface-border); }
.prompt-area:focus-within { @apply border-primary-400; }
.image-prompt-input { @apply block min-h-28 w-full resize-y bg-transparent text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 dark:text-gray-100 dark:placeholder:text-dark-400; max-height: 260px; }
.prompt-meta { @apply mt-2 flex items-center justify-between gap-2 border-t py-2 text-[10px] text-gray-400 dark:text-dark-400; border-color: var(--surface-border); }
.prompt-meta kbd { @apply font-sans; }
.drop-overlay { @apply pointer-events-none absolute inset-0 flex items-center justify-center gap-2 rounded-xl border-2 border-dashed border-primary-400 bg-primary-50/95 p-3 text-xs text-primary-700 dark:bg-dark-800/95 dark:text-primary-300; }
.reference-heading { @apply mb-3 flex items-center justify-between gap-2; }
.reference-heading .section-heading { @apply mb-0; }
.reference-heading h2 small { @apply ml-1 text-[11px] font-normal text-gray-400; }
.reference-heading > button { @apply min-h-7 px-1 text-[11px] text-gray-500 dark:text-dark-300; }
.reference-strip { @apply flex flex-wrap gap-2; }
.reference-upload { @apply flex h-24 w-[88px] shrink-0 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-gray-300 text-[11px] text-gray-500 transition hover:border-primary-400 hover:text-primary-700 dark:border-dark-500 dark:text-dark-300 dark:hover:text-primary-300; }
.reference-card { @apply w-[88px] shrink-0 rounded-xl border p-1; border-color: var(--surface-border); }
.reference-preview { @apply block h-16 w-full overflow-hidden rounded-lg; }
.reference-preview img { @apply h-full w-full object-cover; }
.reference-actions { @apply flex justify-between pt-1; }
.reference-actions button { @apply flex h-7 w-6 items-center justify-center rounded text-gray-500 disabled:opacity-30 dark:text-dark-300; }
.reference-input-hint { @apply mt-2 text-[10px] leading-relaxed text-gray-400 dark:text-dark-400; }
.settings-section { @apply space-y-2; }
.field-label, fieldset legend, .count-row > label { @apply text-[11px] font-medium text-gray-500 dark:text-dark-300; }
.field-label { @apply block pt-1; }
.composer-select { @apply min-w-0; }
.composer-select :deep(.select-trigger) { @apply min-h-9 rounded-lg px-3 py-2 text-xs shadow-none; background: var(--surface-inset); border-color: transparent; }
fieldset { @apply min-w-0 pt-2; }
fieldset legend { @apply mb-2; }
.ratio-options { @apply grid grid-cols-4 gap-1.5; }
.ratio-card { @apply flex min-h-14 min-w-0 flex-col items-center justify-center gap-1.5 rounded-lg border border-gray-200 py-2 text-[10px] text-gray-600 dark:border-dark-600 dark:text-dark-200; }
.ratio-shape { @apply block max-h-4 w-4 rounded-sm border border-current; }
.choice-options { @apply flex gap-1.5; }
.quality-card { @apply min-h-8 flex-1 rounded-lg border border-gray-200 px-2 text-xs text-gray-600 dark:border-dark-600 dark:text-dark-200; }
.ratio-card[aria-pressed='true'], .quality-card[aria-pressed='true'] { @apply border-primary-400 bg-primary-50 text-primary-700 dark:border-primary-500/60 dark:bg-primary-500/10 dark:text-primary-300; }
.count-row { @apply flex items-center justify-between gap-2 pt-2; }
.composer-count-select { @apply w-24; }
.size-note { @apply text-[10px] text-gray-400 dark:text-dark-400; }
.composer-footer { @apply flex shrink-0 items-center gap-2 border-t p-3; border-color: var(--surface-border); background: var(--surface-card); }
.composer-footer > div { @apply min-w-0 flex-1; }
.composer-footer strong { @apply block truncate text-xs font-medium text-gray-700 dark:text-dark-100; }
.composer-footer span { @apply mt-1 block truncate text-[10px] text-gray-500 dark:text-dark-400; }
.generate-button { @apply flex h-10 shrink-0 items-center justify-center gap-1.5 rounded-xl bg-primary-500 px-4 text-xs font-semibold text-on-primary transition hover:bg-primary-400 disabled:cursor-not-allowed disabled:opacity-45; }
.stage-panel { @apply flex min-h-0 flex-col p-4; }
.canvas-header { @apply mb-3 flex shrink-0 items-center justify-between gap-2; }
.canvas-header h2, .task-panel h2, .history-title h2 { @apply text-sm font-semibold text-gray-800 dark:text-gray-100; }
.canvas-header p, .task-panel header p { @apply mt-1 text-[11px] text-gray-500 dark:text-dark-400; }
.studio-notices { @apply shrink-0; }
.status-banner { @apply mb-2 flex flex-wrap items-center gap-2 rounded-lg px-3 py-2 text-xs leading-relaxed; }
.storage-notice { @apply mb-0 shrink-0; }
.status-warning, .inline-warning { @apply text-amber-800 dark:text-amber-200; }
.status-warning { @apply bg-amber-50 dark:bg-amber-950/40; }
.inline-warning { @apply text-[11px] leading-relaxed; }
.status-error { @apply bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-200; }
.canvas-viewport { @apply relative flex min-h-0 flex-1 items-center justify-center overflow-hidden rounded-xl; background: var(--surface-inset); }
.canvas-viewport.has-image { background: #18181b; }
.result-preview { @apply flex h-full min-h-0 w-full min-w-0 items-center justify-center pb-9; }
.result-preview img { @apply h-full min-h-0 w-full min-w-0 object-contain; }
.image-caption { @apply absolute bottom-0 left-0 right-0 flex h-9 items-center justify-between gap-2 px-3 text-[10px] text-white/80; background: #18181b; }
.image-caption > span { @apply truncate; }
.image-caption button { @apply flex shrink-0 items-center gap-1 text-white/60; }
.stage-placeholder { @apply flex flex-col items-center justify-center gap-3 p-5 text-center text-gray-400 dark:text-dark-400; }
.stage-placeholder h3 { @apply max-w-xs text-sm font-medium text-gray-500 dark:text-dark-300; }
.stage-placeholder p { @apply max-w-[240px] text-xs leading-relaxed; }
.empty-image-frame { @apply mb-2 flex h-24 w-28 items-center justify-center rounded-2xl border border-dashed border-gray-300 dark:border-dark-600; }
.batch-strip { @apply flex shrink-0 gap-2 overflow-x-auto py-3; }
.batch-thumb { @apply h-12 w-12 shrink-0 overflow-hidden rounded-lg border-2 border-transparent p-0.5; }
.batch-thumb img { @apply h-full w-full rounded object-cover; }
.batch-thumb[aria-pressed='true'] { @apply border-primary-500; }
.task-status { @apply flex shrink-0 flex-wrap items-center gap-2 py-2 text-[11px] text-gray-500 dark:text-dark-300; }
.result-actions { @apply flex shrink-0 flex-wrap items-center justify-end gap-1 border-t pt-3; border-color: var(--surface-border); }
.result-actions .btn { @apply min-h-8 px-2 text-[11px]; }
.result-actions .btn-primary { @apply bg-primary-500 shadow-none; background-image: none; }
.activity-rail { @apply flex min-h-0 min-w-0 flex-col gap-3; }
.task-panel { @apply flex h-44 shrink-0 flex-col p-3; }
.task-panel header { @apply flex items-center justify-between gap-2 text-gray-400; }
.task-list { @apply mt-3 min-h-0 flex-1 space-y-2 overflow-y-auto; }
.no-tasks { @apply pt-2 text-[11px] text-gray-400 dark:text-dark-400; }
.task-entry { @apply flex w-full items-center gap-2 rounded-lg p-2 text-left; background: var(--surface-inset); }
.task-entry > span { @apply min-w-0 flex-1; }
.task-entry strong { @apply block truncate text-[11px] font-medium text-gray-700 dark:text-dark-100; }
.task-entry small { @apply mt-1 block text-[10px] text-gray-500 dark:text-dark-300; }
.task-entry > svg { @apply shrink-0 text-primary-600 dark:text-primary-400; }
.history-sidebar { @apply flex min-h-0 flex-1 flex-col; }
.history-heading { @apply space-y-2 p-3 pb-2; }
.history-title { @apply flex items-center justify-between; }
.history-search { @apply min-w-0 rounded-lg px-2.5 py-2 text-[11px]; }
.session-list { @apply max-h-32 shrink-0 overflow-y-auto px-2 pb-2; }
.session-row { @apply flex items-center rounded-lg px-1; }
.session-row.is-active { @apply bg-primary-50/80 dark:bg-primary-500/10; }
.session-select { @apply min-w-0 flex-1 px-1 py-2 text-left; }
.session-select span { @apply block truncate text-[11px] font-medium text-gray-700 dark:text-dark-100; }
.session-select small { @apply mt-1 block text-[10px] text-gray-500 dark:text-dark-400; }
.history-icon { @apply flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 hover:text-gray-800 disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-dark-100; }
.history-filters { @apply mx-3 mb-2 grid shrink-0 grid-cols-3 gap-1 rounded-lg p-1; background: var(--surface-inset); }
.history-filters button { @apply rounded py-1 text-[10px] text-gray-500 dark:text-dark-300; }
.history-filters button[aria-pressed='true'] { @apply bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-gray-100; }
.history-results { @apply min-h-0 flex-1 space-y-1 overflow-y-auto p-2; }
.history-empty { @apply py-8 text-center text-xs text-gray-400; }
.history-item-card { @apply flex gap-1 rounded-xl border border-transparent p-1.5; }
.history-item-card.is-active { @apply border-primary-400/50 bg-primary-50/40 dark:bg-primary-500/5; }
.history-result-select { @apply flex min-w-0 flex-1 items-center gap-2 text-left; }
.history-result-select > img { @apply h-11 w-11 shrink-0 rounded-lg object-cover; }
.history-result-info { @apply min-w-0 flex-1; }
.history-result-info strong { @apply line-clamp-2 text-[11px] font-medium text-gray-700 dark:text-dark-100; }
.history-result-info > span, .history-result-info small { @apply mt-1 block truncate text-[9px] text-gray-500 dark:text-dark-400; }
.image-preview-frame { @apply flex items-center justify-center; height: min(70dvh, 800px); }
.image-preview-frame img { @apply h-full w-full object-contain; }
button:focus-visible { @apply outline-none ring-2 ring-primary-500 ring-offset-2 dark:ring-offset-dark-900; }
.mobile-panel-tabs { display: none; }
@media (min-width: 1200px) and (max-width: 1399px) { .studio-grid { grid-template-columns: 320px minmax(0, 1fr) 220px; } }
@media (max-width: 1199px) {
  .workflow-bar { display: none; }
  .mobile-panel-tabs { @apply grid shrink-0 grid-cols-3 gap-1 rounded-xl border p-1; background: var(--surface-card); border-color: var(--surface-border); }
  .mobile-panel-tabs button { @apply flex h-9 items-center justify-center gap-2 rounded-lg text-xs font-medium text-gray-500 dark:text-dark-300; }
  .mobile-panel-tabs button[aria-pressed='true'] { @apply bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300; }
  .mobile-panel-tabs button > span { @apply text-[10px] opacity-70; }
  .studio-grid { grid-template-columns: minmax(0, 1fr); }
  .studio-grid[data-panel='compose'] > :not(.composer-panel),
  .studio-grid[data-panel='result'] > :not(.stage-panel),
  .studio-grid[data-panel='history'] > :not(.activity-rail) { display: none; }
  .activity-rail { @apply h-full; }
  .composer-scroll { @apply px-4; }
  .task-panel { @apply h-36; }
  .result-actions .btn { @apply min-h-9; }
  .reference-actions button { @apply h-8; }
}
@media (max-width: 479px) {
  .studio-shell { @apply gap-2 p-2; }
  .stage-panel { @apply p-3; }
  .composer-footer { @apply p-3; }
  .workflow-bar { @apply hidden; }
}
</style>
