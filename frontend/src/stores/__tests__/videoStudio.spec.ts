import { createPinia, setActivePinia } from 'pinia'
import { reactive, toRaw } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '@/types'
import type { VideoTask } from '@/api/videoGeneration'
import type { SavedVideoStudio } from '../videoStudioStorage'
import { useVideoStudioStore } from '../videoStudio'

const { load, save, createTask, getTask } = vi.hoisted(() => ({
  load: vi.fn(), save: vi.fn(), createTask: vi.fn(), getTask: vi.fn(),
}))
const auth = reactive({ isAuthenticated: true, user: { id: 1 } })
vi.mock('../auth', () => ({ useAuthStore: () => auth }))
vi.mock('../videoStudioStorage', () => ({ loadVideoStudio: load, saveVideoStudio: save }))
vi.mock('@/api/videoGeneration', () => ({ createVideoTask: createTask, getVideoTask: getTask }))

const queued: VideoTask = { id: 'task-1', protocol: 'grok', status: 'queued' }
const completed: VideoTask = { ...queued, status: 'succeeded', videoUrl: 'https://example.test/video.mp4' }
const apiKeys = [{ id: 7, key: 'never-save-this-key', status: 'active' }] as ApiKey[]
let store: ReturnType<typeof useVideoStudioStore>

function snapshot(): SavedVideoStudio {
  return structuredClone({ sessions: toRaw(store.sessions), activeId: store.activeId })
}

async function startPending() {
  const pending = store.generate(apiKeys[0]!.key, 'grok')
  await flushPromises()
  expect(store.activeSession!.pending?.task?.id).toBe('task-1')
  return { pending }
}

beforeEach(async () => {
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
  vi.setSystemTime(new Date('2026-10-30T00:30:45Z'))
  vi.clearAllMocks()
  auth.isAuthenticated = true
  auth.user = { id: 1 }
  load.mockResolvedValue(null)
  save.mockImplementation(async (_userId, state) => (state.revision ?? 0) + 1)
  createTask.mockResolvedValue(queued)
  getTask.mockResolvedValue(completed)
  setActivePinia(createPinia())
  store = useVideoStudioStore()
  await flushPromises()
  Object.assign(store.activeSession!.form, { apiKeyId: 7, model: 'grok-imagine-video', prompt: ' A forest in the wind ' })
})

afterEach(async () => {
  await store.flush()
  store.$dispose()
  await flushPromises()
  vi.useRealTimers()
})

describe('video studio sessions', () => {
  it('uses video defaults and snapshots the submitted settings independently of the next draft', async () => {
    expect(store.activeSession!.form).toMatchObject({ duration: 5, aspectRatio: '16:9', resolution: '720p' })
    const { pending } = await startPending()
    Object.assign(store.activeSession!.form, { model: 'grok-imagine-video-1.5', prompt: 'Next idea', duration: 8 })
    await vi.advanceTimersByTimeAsync(5000)
    expect(await pending).toBe(1)
    expect(createTask).toHaveBeenCalledWith(expect.objectContaining({
      apiKey: apiKeys[0]!.key, model: 'grok-imagine-video', prompt: 'A forest in the wind',
      duration: 5, aspectRatio: '16:9', resolution: '720p', signal: expect.any(AbortSignal),
    }))
    expect(store.activeSession!.results[0]).toMatchObject({
      task: completed, model: 'grok-imagine-video', prompt: 'A forest in the wind',
      duration: 5, createdAt: '2026/10/30 08:30:45', apiKeyId: 7,
    })
    expect(store.activeSession!.form.prompt).toBe('Next idea')
    expect(store.activeSession!.pending).toBeNull()
    await store.flush()
    expect(JSON.stringify(save.mock.calls)).not.toContain(apiKeys[0]!.key)
  })

  it('persists the submitting snapshot before creation and the task ID before polling', async () => {
    let finishCreation!: (task: VideoTask) => void
    createTask.mockImplementation(() => new Promise(resolve => { finishCreation = resolve }))
    const pending = store.generate(apiKeys[0]!.key, 'grok')
    await flushPromises()
    expect(save).toHaveBeenCalledWith(1, expect.objectContaining({
      sessions: [expect.objectContaining({
        pending: expect.objectContaining({ apiKeyId: 7, phase: 'submitting', task: null, duration: 5 }),
      })],
    }))
    finishCreation(queued)
    await flushPromises()
    expect(save).toHaveBeenLastCalledWith(1, expect.objectContaining({
      sessions: [expect.objectContaining({ pending: expect.objectContaining({ task: queued }) })],
    }))
    expect(getTask).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(5000)
    expect(await pending).toBe(1)
  })

  it('keeps background work in its originating session and blocks pending-session deletion', async () => {
    const original = store.activeSession!
    const { pending } = await startPending()
    store.createSession()
    store.activeSession!.form.prompt = 'Second draft'
    store.deleteSession(original.id)
    expect(store.sessions).toHaveLength(2)
    await vi.advanceTimersByTimeAsync(5000)
    expect(await pending).toBe(1)
    expect(original.results[0]?.prompt).toBe('A forest in the wind')
    expect(store.activeSession!.results).toEqual([])
    expect(store.activeSession!.form.prompt).toBe('Second draft')
    store.deleteSession(original.id)
    expect(store.sessions).toHaveLength(1)
  })

  it('ignores duplicate submissions and resume calls while the same job is running', async () => {
    const { pending } = await startPending()
    expect(await store.generate(apiKeys[0]!.key, 'grok')).toBe(0)
    store.resumeJobs(apiKeys)
    store.resumeJobs(apiKeys)
    const refreshed = store.refreshJob(apiKeys[0]!.key)
    expect(createTask).toHaveBeenCalledTimes(1)
    expect(getTask).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(5000)
    expect(await pending).toBe(1)
    expect(await refreshed).toBe(1)
    expect(getTask).toHaveBeenCalledTimes(1)
    expect(store.activeSession!.results).toHaveLength(1)
  })

  it('resumes a persisted request using GET only and only once per job', async () => {
    const { pending } = await startPending()
    load.mockResolvedValue(snapshot())
    auth.user = { id: 2 }
    await flushPromises()
    expect(await pending).toBe(0)
    expect(store.activeSession!.pending?.phase).toBe('paused')
    store.resumeJobs(apiKeys)
    store.resumeJobs(apiKeys)
    await flushPromises()
    expect(createTask).toHaveBeenCalledTimes(1)
    expect(getTask).toHaveBeenCalledTimes(1)
    expect(getTask).toHaveBeenCalledWith(apiKeys[0]!.key, 'grok', 'task-1', expect.any(AbortSignal))
    expect(store.activeSession!.results[0]?.task.id).toBe('task-1')
  })

  it('marks creation interrupted on reload without automatically creating another paid task', async () => {
    let finishCreation!: (task: VideoTask) => void
    createTask.mockImplementation(() => new Promise(resolve => { finishCreation = resolve }))
    const pending = store.generate(apiKeys[0]!.key, 'grok')
    await flushPromises()
    load.mockResolvedValue(snapshot())
    auth.user = { id: 2 }
    await flushPromises()
    store.resumeJobs(apiKeys)
    expect(store.activeSession!.interrupted).toBe(true)
    expect(store.activeSession!.pending).toBeNull()
    expect(createTask).toHaveBeenCalledTimes(1)
    expect(getTask).not.toHaveBeenCalled()
    finishCreation(queued)
    expect(await pending).toBe(0)
    expect(store.activeSession!.results).toEqual([])
  })

  it('clears runtime bookkeeping when a restored task is already terminal', async () => {
    const { pending } = await startPending()
    const saved = snapshot()
    saved.sessions[0]!.pending!.task = completed
    load.mockResolvedValue(saved)
    auth.user = { id: 2 }
    await flushPromises()
    await pending
    store.resumeJobs(apiKeys)
    await flushPromises()
    expect(store.activeSession!.results).toHaveLength(1)
    expect(getTask).not.toHaveBeenCalled()
    createTask.mockResolvedValueOnce({ ...completed, id: 'task-2' })
    expect(await store.generate(apiKeys[0]!.key, 'grok')).toBe(1)
    expect(createTask).toHaveBeenCalledTimes(2)
    expect(store.activeSession!.results.map(result => result.task.id)).toEqual(['task-2', 'task-1'])
  })

  it('pauses after a polling network error and retries status without resubmitting', async () => {
    getTask.mockRejectedValueOnce(new Error('Network unavailable'))
    const { pending } = await startPending()
    const rejected = expect(pending).rejects.toThrow('Network unavailable')
    await vi.advanceTimersByTimeAsync(5000)
    await rejected
    expect(store.activeSession!.pending).toMatchObject({ phase: 'paused', task: queued })
    expect(store.activeSession!.error).toBe('Network unavailable')
    store.deleteSession(store.activeId!)
    expect(store.activeSession!.pending?.task?.id).toBe('task-1')
    await vi.advanceTimersByTimeAsync(60000)
    expect(getTask).toHaveBeenCalledTimes(1)
    expect(await store.refreshJob(apiKeys[0]!.key)).toBe(1)
    expect(createTask).toHaveBeenCalledTimes(1)
    expect(getTask).toHaveBeenCalledTimes(2)
    expect(store.activeSession!.error).toBe('')
  })

  it('bounds polling and leaves a resumable task after the polling window ends', async () => {
    getTask.mockResolvedValue({ ...queued, status: 'running' })
    const { pending } = await startPending()
    await vi.advanceTimersByTimeAsync(600000)
    expect(await pending).toBe(0)
    expect(getTask).toHaveBeenCalledTimes(120)
    expect(store.activeSession!.pending).toMatchObject({ phase: 'paused', task: { id: 'task-1' } })
    expect(store.activeSession!.error).toBe('videoGeneration.pollingTimeout')
    await vi.advanceTimersByTimeAsync(60000)
    expect(getTask).toHaveBeenCalledTimes(120)
    getTask.mockResolvedValue(completed)
    expect(await store.refreshJob(apiKeys[0]!.key)).toBe(1)
    expect(createTask).toHaveBeenCalledTimes(1)
  })

  it('waits for the original credential when restoring an unfinished job', async () => {
    const { pending } = await startPending()
    load.mockResolvedValue(snapshot())
    auth.user = { id: 2 }
    await flushPromises()
    await pending
    store.resumeJobs([{ ...apiKeys[0]!, status: 'inactive' }])
    expect(store.activeSession!.error).toBe('videoGeneration.missingKey')
    expect(store.activeSession!.pending?.phase).toBe('paused')
    expect(getTask).not.toHaveBeenCalled()
    store.resumeJobs(apiKeys)
    await flushPromises()
    expect(store.activeSession!.results).toHaveLength(1)
  })

  it.each(['expired', 'quota_exhausted'] as const)('resumes historical status reads with a %s key without creating another task', async status => {
    const { pending } = await startPending()
    load.mockResolvedValue(snapshot())
    auth.user = { id: 2 }
    await flushPromises()
    await pending
    const creationCount = createTask.mock.calls.length
    store.resumeJobs([{ ...apiKeys[0]!, status }])
    await flushPromises()
    expect(createTask).toHaveBeenCalledTimes(creationCount)
    expect(getTask).toHaveBeenCalledTimes(1)
    expect(getTask).toHaveBeenCalledWith(apiKeys[0]!.key, 'grok', 'task-1', expect.any(AbortSignal))
    expect(store.activeSession!.results[0]?.task).toEqual(completed)
    expect(store.activeSession!.error).toBe('')
  })

  it('retains successful history when a subsequent task fails', async () => {
    createTask.mockResolvedValueOnce(completed)
    expect(await store.generate(apiKeys[0]!.key, 'grok')).toBe(1)
    createTask.mockResolvedValueOnce({ ...queued, id: 'failed-task', status: 'failed', error: 'Prompt rejected' })
    expect(await store.generate(apiKeys[0]!.key, 'grok')).toBe(0)
    expect(store.activeSession!.error).toBe('Prompt rejected')
    expect(store.activeSession!.pending).toBeNull()
    expect(store.activeSession!.results.map(result => result.task.id)).toEqual(['task-1'])
  })

  it('allows an explicit retry after a creation failure', async () => {
    createTask.mockRejectedValueOnce(new Error('Invalid model'))
    await expect(store.generate(apiKeys[0]!.key, 'grok')).rejects.toThrow('Invalid model')
    expect(store.activeSession!.pending).toBeNull()
    expect(store.activeSession!.error).toBe('Invalid model')
    createTask.mockResolvedValueOnce(completed)
    expect(await store.generate(apiKeys[0]!.key, 'grok')).toBe(1)
    expect(createTask).toHaveBeenCalledTimes(2)
  })

  it('aborts status requests and ignores late results after logout', async () => {
    let finishStatus!: (task: VideoTask) => void
    getTask.mockImplementation(() => new Promise(resolve => { finishStatus = resolve }))
    const { pending } = await startPending()
    await vi.advanceTimersByTimeAsync(5000)
    const signal = getTask.mock.calls[0]![3] as AbortSignal
    auth.isAuthenticated = false
    expect(signal.aborted).toBe(true)
    expect(store.sessions).toEqual([])
    auth.user = { id: 2 }
    auth.isAuthenticated = true
    await flushPromises()
    finishStatus(completed)
    expect(await pending).toBe(0)
    await store.flush()
    expect(store.activeSession!.results).toEqual([])
    expect(JSON.stringify(save.mock.calls.filter(([id]) => id === 2))).not.toContain('task-1')
    expect(load).toHaveBeenLastCalledWith(2)
  })

  it('aborts creation requests when the store is disposed', async () => {
    let finishCreation!: (task: VideoTask) => void
    createTask.mockImplementation(() => new Promise(resolve => { finishCreation = resolve }))
    const pending = store.generate(apiKeys[0]!.key, 'grok')
    await flushPromises()
    const signal = createTask.mock.calls[0]![0].signal as AbortSignal
    store.$dispose()
    expect(signal.aborted).toBe(true)
    finishCreation(completed)
    expect(await pending).toBe(0)
    expect(store.activeSession!.results).toEqual([])
  })

  it('does not overwrite unread history after a storage load failure', async () => {
    await store.flush()
    load.mockRejectedValueOnce(new Error('Storage unavailable'))
    auth.user = { id: 2 }
    await flushPromises()
    expect(store.ready).toBe(true)
    expect(store.storageError).toBe('load')
    store.activeSession!.form.prompt = 'An unsaved draft'
    await store.flush()
    expect(save.mock.calls.some(([id]) => id === 2)).toBe(false)
  })

  it('reports persistence failures without discarding the draft', async () => {
    save.mockRejectedValueOnce(new Error('Quota exceeded'))
    await store.flush()
    expect(store.storageError).toBe('save')
    expect(store.activeSession!.form.prompt).toBe(' A forest in the wind ')
  })

  it('stops overwriting storage after a version conflict', async () => {
    save.mockRejectedValueOnce(new Error('video-studio-conflict'))
    await store.flush()
    expect(store.storageError).toBe('conflict')
    const count = save.mock.calls.length
    store.activeSession!.form.prompt = 'Keep this draft visible'
    await store.flush()
    expect(save).toHaveBeenCalledTimes(count)
    expect(store.activeSession!.form.prompt).toBe('Keep this draft visible')
  })

  it('preserves the saved revision without writing unmodified history', async () => {
    const saved = snapshot()
    await store.flush()
    load.mockResolvedValue({ ...saved, revision: 4 })
    auth.user = { id: 2 }
    await flushPromises()
    await store.flush()
    expect(save.mock.calls.some(([id]) => id === 2)).toBe(false)
    store.activeSession!.form.prompt = 'Changed'
    await store.flush()
    expect(save).toHaveBeenLastCalledWith(2, expect.objectContaining({ revision: 4 }))
  })

  it('deletes only the chosen session and retains a usable workspace', async () => {
    const first = store.activeId!
    store.createSession()
    const second = store.activeId!
    store.deleteSession(first)
    expect(store.sessions.map(session => session.id)).toEqual([second])
    store.deleteSession(second)
    expect(store.sessions).toHaveLength(1)
    expect(store.activeId).not.toBe(second)
  })
})
