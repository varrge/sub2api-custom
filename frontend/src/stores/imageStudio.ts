import { defineStore } from 'pinia'
import { computed, onScopeDispose, ref, toRaw, watch } from 'vue'
import {
  generateImages, getImageTask, ImageGenerationError, normalizeImageRequest, readImageAsyncCapability, submitImageTask,
  type GeneratedImage, type GenerateImageOptions, type ImageAspectRatio, type ImageQuality, type ImageResolution, type ImageTask,
} from '@/api/imageGeneration'
import type { GroupPlatform } from '@/types'
import { formatBeijingDateTime } from '@/utils/beijingDateTime'
import { useAuthStore } from './auth'
import { loadImageStudio, saveImageStudio, type SavedImageStudio } from './imageStudioStorage'

export interface ImageStudioForm {
  apiKeyId: number | null
  model: string
  prompt: string
  aspectRatio: ImageAspectRatio
  quality: ImageQuality
  resolution: ImageResolution
  count: number
}

export interface ImageStudioRun {
  readonly id: string
  readonly form: Readonly<ImageStudioForm>
  readonly platform: GroupPlatform
  readonly referenceFiles: readonly File[]
  readonly createdAt: string
  status: 'submitting' | 'processing' | 'completed' | 'failed' | 'interrupted'
  requested: number
  submitted: number
  completed: number
  taskId?: string
  error?: string
  errorKind?: 'confirmed' | 'unknown' | 'query'
}

export interface ImageStudioResult extends GeneratedImage {
  id: string
  runId?: string
  model: string
  createdAt: string
  aspectRatio: string
  mode: 'text' | 'image'
  prompt: string
}

export interface ImageStudioSession {
  id: string
  title: string
  form: ImageStudioForm
  referenceFiles: File[]
  results: ImageStudioResult[]
  runs: ImageStudioRun[]
  activeResultId: string | null
  pending: { count: number; aspectRatio: ImageAspectRatio; completed: number; startedAt: number } | null
  interrupted: boolean
  error: string
}

export const useImageStudioStore = defineStore('imageStudio', () => {
  const auth = useAuthStore()
  const sessions = ref<ImageStudioSession[]>([])
  const activeId = ref<string | null>(null)
  const ready = ref(false)
  const storageError = ref<'load' | 'save' | 'conflict' | null>(null)
  const activeSession = computed(() => sessions.value.find(session => session.id === activeId.value) ?? null)
  let owner: number | null = null
  let generation = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let writes = Promise.resolve()
  let canSave = false
  let dirty = false
  let revision = { value: 0 }
  const listeners = new Map<string, AbortController>()

  function stopListeners() {
    listeners.forEach(controller => controller.abort())
    listeners.clear()
  }

  function createSession() {
    if (!ready.value) return null
    const session: ImageStudioSession = {
      id: crypto.randomUUID(), title: '',
      form: { apiKeyId: null, model: '', prompt: '', aspectRatio: 'auto', quality: 'standard', resolution: '1K', count: 1 },
      referenceFiles: [], results: [], runs: [], activeResultId: null, pending: null, interrupted: false, error: '',
    }
    sessions.value.unshift(session)
    activeId.value = session.id
    return session
  }

  function deleteSession(id: string) {
    if (sessions.value.find(session => session.id === id)?.pending) return
    const index = sessions.value.findIndex(session => session.id === id)
    if (index < 0) return
    sessions.value.splice(index, 1)
    if (activeId.value === id) activeId.value = sessions.value[0]?.id ?? null
    if (!sessions.value.length) createSession()
    void flush()
  }

  function flush(): Promise<void> {
    clearTimeout(timer)
    if (!ready.value || !canSave || !dirty || owner === null) return writes
    const userId = owner
    const epoch = generation
    const version = revision
    // IndexedDB keeps large image data and File references out of localStorage.
    // A snapshot excludes API key secrets: only the selected key ID is retained.
    let snapshot: SavedImageStudio
    try {
      snapshot = structuredClone({ sessions: toRaw(sessions.value), activeId: activeId.value })
    } catch {
      storageError.value = 'save'
      return writes
    }
    dirty = false
    writes = writes.then(async () => {
      if (epoch === generation && !canSave) return
      version.value = await saveImageStudio(userId, { ...snapshot, revision: version.value })
    }).then(() => {
      if (epoch === generation && canSave) storageError.value = null
    }).catch(error => {
      if (epoch !== generation) return
      dirty = true
      if (error instanceof Error && error.message === 'image-studio-conflict') {
        canSave = false
        storageError.value = 'conflict'
      } else storageError.value = 'save'
    })
    return writes
  }

  watch([sessions, activeId], () => {
    if (!ready.value || !canSave) return
    dirty = true
    clearTimeout(timer)
    timer = setTimeout(() => void flush(), 250)
  }, { deep: true, flush: 'sync' })

  watch(() => auth.isAuthenticated ? auth.user?.id ?? null : null, async userId => {
    void flush()
    const epoch = ++generation
    stopListeners()
    owner = userId
    ready.value = false
    canSave = false
    dirty = false
    sessions.value = []
    activeId.value = null
    storageError.value = null
    revision = { value: 0 }
    if (userId === null) return
    try {
      await writes
      const saved = await loadImageStudio(userId)
      if (epoch !== generation) return
      if (saved) {
        revision.value = saved.revision ?? 0
        if (!Array.isArray(saved.sessions)) throw new Error('Invalid image studio history')
        sessions.value = saved.sessions.map(session => ({
          ...session,
          form: {
            ...session.form,
            resolution: session.form.resolution ?? (session.form.quality === 'high' ? '2K' : '1K'),
          },
          runs: (session.runs ?? []).map(run => ({
            ...run,
            form: Object.freeze({ ...run.form }),
            referenceFiles: Object.freeze([...run.referenceFiles]),
            status: !run.taskId && ['submitting', 'processing'].includes(run.status) ? 'interrupted' as const : run.status,
            errorKind: !run.taskId && ['submitting', 'processing'].includes(run.status) ? 'unknown' as const : run.errorKind,
            error: !run.taskId && ['submitting', 'processing'].includes(run.status) ? 'imageGeneration.submitUnknown' : run.error,
          })),
          interrupted: Boolean(session.pending) || session.interrupted,
          pending: null,
        }))
        activeId.value = saved.activeId
      }
      canSave = true
    } catch {
      if (epoch !== generation) return
      storageError.value = 'load'
    }
    if (epoch !== generation) return
    ready.value = true
    if (!sessions.value.length) createSession()
    if (!activeSession.value) activeId.value = sessions.value[0]!.id
  }, { immediate: true, flush: 'sync' })

  function renameSession(id: string, title: string) {
    const session = sessions.value.find(item => item.id === id)
    if (!session || !title.trim()) return
    session.title = title.trim().slice(0, 120)
    void flush()
  }

  function restoreResult(resultId: string): boolean {
    const session = activeSession.value
    const result = session?.results.find(item => item.id === resultId)
    if (!session || !result) return false
    const run = session.runs.find(item => item.id === result.runId)
    if (run) {
      session.form = { ...run.form }
      session.referenceFiles = [...run.referenceFiles]
    } else {
      // Legacy history only recorded these fields; leave unrecorded draft choices alone.
      session.form.prompt = result.prompt
      session.form.model = result.model
      session.form.aspectRatio = result.aspectRatio as ImageAspectRatio
    }
    void flush()
    return true
  }

  function addImages(session: ImageStudioSession, run: ImageStudioRun, images: GeneratedImage[]) {
    const results = images.map((image): ImageStudioResult => ({
      ...image, id: crypto.randomUUID(), runId: run.id, model: run.form.model,
      createdAt: formatBeijingDateTime(new Date(), undefined, 'zh-CN'),
      aspectRatio: run.form.aspectRatio, mode: run.referenceFiles.length ? 'image' : 'text', prompt: run.form.prompt,
    }))
    session.results.unshift(...results)
    if (results.length) session.activeResultId = results[0]!.id
    run.completed += results.length
    if (session.pending) session.pending.completed = run.completed
  }

  function waitForPoll(milliseconds: number, signal: AbortSignal): Promise<void> {
    return new Promise((resolve, reject) => {
      const abort = () => {
        clearTimeout(timeout)
        signal.removeEventListener('abort', abort)
        reject(new DOMException('Image task listener stopped', 'AbortError'))
      }
      const timeout = setTimeout(() => {
        signal.removeEventListener('abort', abort)
        resolve()
      }, milliseconds)
      signal.addEventListener('abort', abort, { once: true })
      if (signal.aborted) abort()
    })
  }

  async function pollTask(
    session: ImageStudioSession, run: ImageStudioRun, apiKey: string,
    signal: AbortSignal, isCurrent: () => boolean, initial?: ImageTask,
  ) {
    let task = initial
    while (isCurrent()) {
      task ??= await getImageTask(apiKey, run.taskId!, signal)
      if (!isCurrent()) return
      run.status = task.status
      run.error = undefined
      run.errorKind = undefined
      if (task.status === 'failed') {
        throw new ImageGenerationError(task.error || 'imageGeneration.generateFailed', 'confirmed')
      }
      if (task.status === 'completed') {
        if (!task.images.length) throw new ImageGenerationError('imageGeneration.noImageReturned', 'confirmed')
        const alreadyRecorded = session.runs.some(other => other.taskId === run.taskId &&
          session.results.some(result => result.runId === other.id))
        if (!alreadyRecorded) addImages(session, run, task.images)
        session.interrupted = false
        await flush()
        return
      }
      await waitForPoll(task.retryAfterMs, signal)
      task = undefined
    }
  }

  function setRunError(session: ImageStudioSession, run: ImageStudioRun, error: unknown) {
    const message = error instanceof Error ? error.message : 'imageGeneration.generateFailed'
    if (run.taskId && run.status !== 'failed' && run.status !== 'completed') {
      // Query failures say nothing about whether the server's task completed.
      run.errorKind = 'query'
      run.error = error instanceof ImageGenerationError && [404, 410].includes(error.status ?? 0)
        ? 'imageGeneration.taskExpired'
        : error instanceof ImageGenerationError && [401, 403].includes(error.status ?? 0)
          ? 'imageGeneration.taskKeyUnavailable' : 'imageGeneration.taskQueryFailed'
    } else {
      run.errorKind = run.submitted > 0 && (!(error instanceof ImageGenerationError) || error.kind === 'unknown')
        ? 'unknown' : 'confirmed'
      run.status = run.errorKind === 'unknown' ? 'interrupted' : run.status === 'completed' ? 'completed' : 'failed'
      run.error = run.errorKind === 'unknown' ? 'imageGeneration.submitUnknown' : message
    }
    session.error = run.error
    session.interrupted = run.errorKind === 'unknown' || run.errorKind === 'query'
  }

  async function generate(apiKey: string, platform: GroupPlatform): Promise<number> {
    const session = activeSession.value
    if (!session || session.pending || owner === null) return 0
    const epoch = generation
    const controller = new AbortController()
    listeners.set(session.id, controller)
    const request = normalizeImageRequest({
      ...session.form, apiKey, platform, referenceImages: [...session.referenceFiles], signal: controller.signal,
    })
    const { apiKeyId, model, prompt, aspectRatio, quality, resolution, count } = request
    session.runs.unshift({
      id: crypto.randomUUID(), form: Object.freeze({ apiKeyId, model, prompt, aspectRatio, quality, resolution, count }),
      platform, referenceFiles: Object.freeze([...request.referenceImages]),
      createdAt: formatBeijingDateTime(new Date(), undefined, 'zh-CN'),
      status: 'submitting', requested: count, submitted: 0, completed: 0,
    })
    const run = session.runs[0]!
    session.title ||= prompt.slice(0, 48)
    session.error = ''
    session.interrupted = false
    session.pending = { count, aspectRatio, completed: 0, startedAt: Date.now() }
    const isCurrent = () => epoch === generation && !controller.signal.aborted && sessions.value.some(item => item.id === session.id)
    const options: GenerateImageOptions = {
      ...request,
      onRequest: submitted => { if (isCurrent()) run.submitted += submitted },
      onImages: async images => {
        if (!isCurrent()) return
        addImages(session, run, images)
        await flush()
      },
    }
    try {
      await flush()
      if (!isCurrent()) return 0
      const useAsync = await readImageAsyncCapability(apiKey, platform, controller.signal)
      if (!isCurrent()) return 0
      if (useAsync) {
        const task = await submitImageTask(options)
        if (!isCurrent()) return 0
        run.taskId = task.id
        run.status = task.status
        await flush()
        if (!isCurrent()) return 0
        await pollTask(session, run, apiKey, controller.signal, isCurrent, task)
      } else {
        run.status = 'processing'
        const images = await generateImages(options)
        if (!isCurrent()) return 0
        // Array-only clients remain supported; callbacks have already recorded normal API results.
        if (!run.completed) addImages(session, run, images)
        if (!run.completed) throw new ImageGenerationError('imageGeneration.noImageReturned', 'confirmed')
        run.status = 'completed'
      }
      return run.completed
    } catch (error) {
      if (!isCurrent()) return 0
      setRunError(session, run, error)
      if (run.completed) return run.completed
      throw error
    } finally {
      if (isCurrent()) {
        listeners.delete(session.id)
        session.pending = null
        await flush()
      }
    }
  }

  async function recover(sessionId: string, apiKey: string): Promise<number> {
    const session = sessions.value.find(item => item.id === sessionId)
    if (!session || session.pending || owner === null) return 0
    const run = session.runs.find(item => item.taskId && item.status === 'processing')
    if (!run) return 0
    const epoch = generation
    const controller = new AbortController()
    listeners.set(session.id, controller)
    const isCurrent = () => epoch === generation && !controller.signal.aborted && sessions.value.some(item => item.id === session.id)
    session.pending = { count: run.requested, aspectRatio: run.form.aspectRatio, completed: run.completed, startedAt: Date.now() }
    session.error = ''
    const before = run.completed
    try {
      await pollTask(session, run, apiKey, controller.signal, isCurrent)
      return isCurrent() ? run.completed - before : 0
    } catch (error) {
      if (!isCurrent()) return 0
      setRunError(session, run, error)
      throw error
    } finally {
      if (isCurrent()) {
        listeners.delete(session.id)
        session.pending = null
        await flush()
      }
    }
  }

  const onPageHide = () => void flush()
  window.addEventListener('pagehide', onPageHide)
  onScopeDispose(() => {
    void flush()
    generation += 1
    stopListeners()
    window.removeEventListener('pagehide', onPageHide)
  })

  return { sessions, activeId, activeSession, ready, storageError, createSession, deleteSession, renameSession, restoreResult, generate, recover, flush }
})
