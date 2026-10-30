import { defineStore } from 'pinia'
import { computed, onScopeDispose, ref, toRaw, watch } from 'vue'
import { createVideoTask, getVideoTask, type GenerateVideoOptions, type VideoAspectRatio, type VideoResolution, type VideoTask } from '@/api/videoGeneration'
import type { ApiKey, GroupPlatform } from '@/types'
import { formatBeijingDateTime } from '@/utils/beijingDateTime'
import { useAuthStore } from './auth'
import { loadVideoStudio, saveVideoStudio, type SavedVideoStudio } from './videoStudioStorage'

export interface VideoStudioForm {
  apiKeyId: number | null
  model: string
  prompt: string
  duration: number
  aspectRatio: VideoAspectRatio
  resolution: VideoResolution
}

interface VideoStudioRequest extends VideoStudioForm {
  platform: GroupPlatform
  mode: 'text' | 'image'
  createdAt: string
}

export interface VideoStudioJob extends VideoStudioRequest {
  phase: 'submitting' | 'polling' | 'paused'
  task: VideoTask | null
}

export interface VideoStudioResult extends VideoStudioRequest {
  id: string
  task: VideoTask
}

export interface VideoStudioSession {
  id: string
  title: string
  form: VideoStudioForm
  referenceFile: File | null
  results: VideoStudioResult[]
  activeResultId: string | null
  pending: VideoStudioJob | null
  interrupted: boolean
  error: string
}

const pollInterval = 5000
const maxPolls = 120

export const useVideoStudioStore = defineStore('videoStudio', () => {
  const auth = useAuthStore()
  const sessions = ref<VideoStudioSession[]>([])
  const activeId = ref<string | null>(null)
  const ready = ref(false)
  const storageError = ref<'load' | 'save' | 'conflict' | null>(null)
  const activeSession = computed(() => sessions.value.find(session => session.id === activeId.value) ?? null)
  const running = new Map<string, { controller: AbortController; promise: Promise<number> }>()
  let owner: number | null = null
  let generation = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let writes = Promise.resolve()
  let canSave = false
  let dirty = false
  let revision = { value: 0 }
  let disposed = false

  function createSession() {
    if (!ready.value) return null
    const session: VideoStudioSession = {
      id: crypto.randomUUID(), title: '',
      form: { apiKeyId: null, model: '', prompt: '', duration: 5, aspectRatio: '16:9', resolution: '720p' },
      referenceFile: null, results: [], activeResultId: null, pending: null, interrupted: false, error: '',
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
    let snapshot: SavedVideoStudio
    try {
      // Only key IDs, request metadata and task URLs belong in browser history.
      snapshot = structuredClone({ sessions: toRaw(sessions.value), activeId: activeId.value })
    } catch {
      storageError.value = 'save'
      return writes
    }
    dirty = false
    writes = writes.then(async () => {
      if (epoch === generation && !canSave) return
      version.value = await saveVideoStudio(userId, { ...snapshot, revision: version.value })
    }).then(() => {
      if (epoch === generation && canSave) storageError.value = null
    }).catch(error => {
      if (epoch !== generation) return
      dirty = true
      if (error instanceof Error && error.message === 'video-studio-conflict') {
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

  function abortJobs() {
    running.forEach(job => job.controller.abort())
    running.clear()
  }

  watch(() => auth.isAuthenticated ? auth.user?.id ?? null : null, async userId => {
    void flush()
    const epoch = ++generation
    abortJobs()
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
      const saved = await loadVideoStudio(userId)
      if (epoch !== generation || disposed) return
      if (saved) {
        revision.value = saved.revision ?? 0
        if (!Array.isArray(saved.sessions)) throw new Error('Invalid video studio history')
        sessions.value = saved.sessions.map(session => ({
          ...session,
          // A returned task can resume GET polling; an unknown creation must never be retried automatically.
          pending: session.pending?.task ? { ...session.pending, phase: 'paused' } : null,
          interrupted: Boolean(session.pending && !session.pending.task) || session.interrupted,
        }))
        activeId.value = saved.activeId
      }
      canSave = true
    } catch {
      if (epoch !== generation || disposed) return
      storageError.value = 'load'
    }
    if (epoch !== generation || disposed) return
    ready.value = true
    if (!sessions.value.length) createSession()
    if (!activeSession.value) activeId.value = sessions.value[0]!.id
  }, { immediate: true, flush: 'sync' })

  function waitForPoll(signal: AbortSignal): Promise<boolean> {
    return new Promise(resolve => {
      if (signal.aborted) { resolve(false); return }
      const timeout = setTimeout(() => {
        signal.removeEventListener('abort', abort)
        resolve(true)
      }, pollInterval)
      const abort = () => {
        clearTimeout(timeout)
        signal.removeEventListener('abort', abort)
        resolve(false)
      }
      signal.addEventListener('abort', abort, { once: true })
    })
  }

  function finishTask(session: VideoStudioSession, job: VideoStudioJob): number | null {
    const task = job.task
    if (!task) return null
    if (task.status === 'succeeded') {
      const result: VideoStudioResult = {
        id: crypto.randomUUID(), task: { ...task },
        apiKeyId: job.apiKeyId, platform: job.platform, model: job.model, prompt: job.prompt,
        duration: job.duration, aspectRatio: job.aspectRatio, resolution: job.resolution,
        mode: job.mode, createdAt: job.createdAt,
      }
      session.results.unshift(result)
      session.activeResultId = result.id
      session.pending = null
      session.error = ''
      return 1
    }
    if (task.status === 'failed' || task.status === 'cancelled') {
      session.error = task.error || 'videoGeneration.generateFailed'
      session.pending = null
      return 0
    }
    return null
  }

  function runJob(session: VideoStudioSession, apiKey: string): Promise<number> {
    const existing = running.get(session.id)
    if (existing) return existing.promise
    const job = session.pending
    if (!job || owner === null || disposed) return Promise.resolve(0)
    const epoch = generation
    const controller = new AbortController()
    const referenceImage = session.referenceFile
    const isCurrent = () => !disposed && !controller.signal.aborted && epoch === generation &&
      sessions.value.some(item => item.id === session.id) && session.pending === job
    const promise = Promise.resolve().then(async () => {
      try {
        if (!isCurrent()) return 0
        let waitBeforePoll = false
        if (!job.task) {
          await flush()
          if (!isCurrent()) return 0
          const request: GenerateVideoOptions = {
            apiKey, platform: job.platform, model: job.model, prompt: job.prompt,
            duration: job.duration, aspectRatio: job.aspectRatio, resolution: job.resolution,
            referenceImage, signal: controller.signal,
          }
          const task = await createVideoTask(request)
          if (!isCurrent()) return 0
          job.task = task
          job.phase = 'polling'
          await flush()
          if (!isCurrent()) return 0
          waitBeforePoll = true
        }
        const completed = finishTask(session, job)
        if (completed !== null) return completed
        for (let count = 0; count < maxPolls; count += 1) {
          if (waitBeforePoll && !await waitForPoll(controller.signal)) return 0
          if (!isCurrent() || !job.task) return 0
          const task = await getVideoTask(apiKey, job.task.protocol, job.task.id, controller.signal)
          if (!isCurrent()) return 0
          job.task = task
          const completed = finishTask(session, job)
          if (completed !== null) return completed
          void flush()
          waitBeforePoll = true
        }
        job.phase = 'paused'
        session.error = 'videoGeneration.pollingTimeout'
        return 0
      } catch (error) {
        if (!isCurrent()) return 0
        session.error = error instanceof Error ? error.message : job.task ? 'videoGeneration.pollingPaused' : 'videoGeneration.generateFailed'
        if (job.task) job.phase = 'paused'
        else session.pending = null
        throw error
      } finally {
        if (running.get(session.id)?.controller === controller) running.delete(session.id)
        if (epoch === generation && !disposed) void flush()
      }
    })
    running.set(session.id, { controller, promise })
    return promise
  }

  async function generate(apiKey: string, platform: GroupPlatform): Promise<number> {
    const session = activeSession.value
    if (!session || session.pending || owner === null) return 0
    const form = session.form
    session.title ||= form.prompt.trim().slice(0, 48)
    session.error = ''
    session.interrupted = false
    session.pending = {
      ...form, platform, model: form.model.trim(), prompt: form.prompt.trim(),
      mode: session.referenceFile ? 'image' : 'text',
      createdAt: formatBeijingDateTime(new Date(), undefined, 'zh-CN'),
      phase: 'submitting', task: null,
    }
    return runJob(session, apiKey)
  }

  function refreshJob(apiKey: string, sessionId = activeId.value): Promise<number> {
    const session = sessions.value.find(item => item.id === sessionId)
    if (!session?.pending?.task || owner === null) return Promise.resolve(0)
    if (running.has(session.id)) return running.get(session.id)!.promise
    session.pending.phase = 'polling'
    session.interrupted = false
    session.error = ''
    return runJob(session, apiKey)
  }

  function resumeJobs(apiKeys: ApiKey[]) {
    if (!ready.value || owner === null) return
    for (const session of sessions.value) {
      if (!session.pending?.task || running.has(session.id)) continue
      const key = apiKeys.find(key => key.id === session.pending!.apiKeyId &&
        (key.status === 'active' || key.status === 'expired' || key.status === 'quota_exhausted'))
      if (!key) {
        session.pending.phase = 'paused'
        session.error = 'videoGeneration.missingKey'
        continue
      }
      void refreshJob(key.key, session.id).catch(() => undefined)
    }
  }

  const onPageHide = () => void flush()
  window.addEventListener('pagehide', onPageHide)
  onScopeDispose(() => {
    void flush()
    disposed = true
    abortJobs()
    window.removeEventListener('pagehide', onPageHide)
  })

  return { sessions, activeId, activeSession, ready, storageError, createSession, deleteSession, generate, resumeJobs, refreshJob, flush }
})
