import { createPinia, setActivePinia } from 'pinia'
import { reactive, nextTick, toRaw } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { useImageStudioStore } from '../imageStudio'
import { ImageGenerationError, type GenerateImageOptions, type ImageTask } from '@/api/imageGeneration'

const { load, save, generate, capability, submit, query } = vi.hoisted(() => ({
  load: vi.fn(), save: vi.fn(), generate: vi.fn(), capability: vi.fn(), submit: vi.fn(), query: vi.fn(),
}))
const auth = reactive({ isAuthenticated: true, user: { id: 1 } })
vi.mock('../auth', () => ({ useAuthStore: () => auth }))
vi.mock('../imageStudioStorage', () => ({ loadImageStudio: load, saveImageStudio: save }))
vi.mock('@/api/imageGeneration', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/imageGeneration')>(),
  generateImages: generate, readImageAsyncCapability: capability, submitImageTask: submit, getImageTask: query,
}))

let store: ReturnType<typeof useImageStudioStore>
beforeEach(async () => {
  vi.resetAllMocks()
  capability.mockResolvedValue(false)
  auth.isAuthenticated = true
  auth.user = { id: 1 }
  load.mockResolvedValue(null)
  save.mockImplementation(async (_userId, snapshot) => (snapshot.revision ?? 0) + 1)
  setActivePinia(createPinia())
  store = useImageStudioStore()
  await flushPromises()
  store.activeSession!.form.prompt = 'A forest'
  store.activeSession!.form.model = 'gpt-image-2'
})
afterEach(async () => { await store.flush(); store.$dispose(); await flushPromises() })

describe('image studio sessions', () => {
  it('records generation time in Beijing time without a timezone suffix', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-26T16:30:45Z'))
    generate.mockResolvedValue([{ src: 'data:image/png;base64,QUJD' }])
    try {
      await store.generate('secret', 'openai')
      expect(store.activeSession!.results[0]?.createdAt).toBe('2026/10/27 00:30:45')
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps background results in the originating session and excludes the API key from storage', async () => {
    let complete!: (images: { src: string }[]) => void
    generate.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    const original = store.activeSession!
    const pending = store.generate('never-persist-this-secret', 'openai')
    await store.generate('never-persist-this-secret', 'openai')
    await flushPromises()
    expect(generate).toHaveBeenCalledTimes(1)
    store.createSession()
    store.activeSession!.form.prompt = 'Next draft'
    complete([{ src: 'data:image/png;base64,QUJD' }])
    await pending
    await store.flush()
    expect(original.results[0]?.prompt).toBe('A forest')
    expect(original.pending).toBeNull()
    expect(store.activeSession!.results).toEqual([])
    expect(store.activeSession!.form.prompt).toBe('Next draft')
    expect(JSON.stringify(save.mock.calls)).not.toContain('never-persist-this-secret')
  })

  it('restores completed sessions and marks refreshed pending requests without resubmitting', async () => {
    const session = structuredClone(toRaw(store.activeSession!))
    session.pending = { count: 1, aspectRatio: 'auto', completed: 0, startedAt: Date.now() }
    load.mockResolvedValue({ sessions: [session], activeId: session.id })
    auth.user = { id: 2 }
    await flushPromises()
    expect(store.activeSession!.form.prompt).toBe('A forest')
    expect(store.activeSession!.interrupted).toBe(true)
    expect(store.activeSession!.pending).toBeNull()
    expect(generate).not.toHaveBeenCalled()
  })

  it.each(['standard', 'high'] as const)('migrates legacy %s drafts without changing their quality', async quality => {
    const session = structuredClone(toRaw(store.activeSession!))
    session.form.quality = quality
    Reflect.deleteProperty(session.form, 'resolution')
    load.mockResolvedValue({ sessions: [session], activeId: session.id })
    auth.user = { id: 2 }
    await flushPromises()
    expect(store.activeSession!.form.quality).toBe(quality)
    expect(store.activeSession!.form.resolution).toBe(quality === 'high' ? '2K' : '1K')
  })

  it('persists 4K and snapshots it independently of the next draft and quality', async () => {
    let complete!: (images: { src: string }[]) => void
    generate.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    store.activeSession!.form.resolution = '4K'
    const pending = store.generate('secret', 'openai')
    store.activeSession!.form.resolution = '1K'
    await flushPromises()
    expect(generate).toHaveBeenCalledWith(expect.objectContaining({ resolution: '4K', quality: 'standard' }))
    complete([{ src: 'data:image/png;base64,QUJD' }])
    await pending
    store.activeSession!.form.resolution = '4K'
    await store.flush()
    const snapshot = structuredClone(save.mock.calls.at(-1)![1])
    load.mockResolvedValue(snapshot)
    auth.user = { id: 2 }
    await flushPromises()
    expect(store.activeSession!.form.resolution).toBe('4K')
    expect(store.activeSession!.form.quality).toBe('standard')
    expect(store.activeSession!.results).toHaveLength(1)
  })

  it('does not rewrite unmodified history just by opening another tab', async () => {
    const session = structuredClone(toRaw(store.activeSession!))
    await store.flush()
    load.mockResolvedValue({ sessions: [session], activeId: session.id, revision: 4 })
    auth.user = { id: 2 }
    await flushPromises()
    await store.flush()
    expect(save.mock.calls.some(([id]) => id === 2)).toBe(false)
    store.activeSession!.form.prompt = 'Changed'
    await store.flush()
    expect(save).toHaveBeenLastCalledWith(2, expect.objectContaining({ revision: 4 }))
  })

  it('isolates accounts and ignores a response after logout', async () => {
    let complete!: (images: { src: string }[]) => void
    generate.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    const pending = store.generate('secret', 'openai')
    await flushPromises()
    auth.isAuthenticated = false
    expect(store.sessions).toEqual([])
    auth.user = { id: 2 }
    auth.isAuthenticated = true
    await flushPromises()
    complete([{ src: 'data:image/png;base64,QUJD' }])
    await pending
    await store.flush()
    expect(store.activeSession!.results).toEqual([])
    const accountTwoWrites = save.mock.calls.filter(([id]) => id === 2)
    expect(JSON.stringify(accountTwoWrites)).not.toContain('QUJD')
    expect(load).toHaveBeenLastCalledWith(2)
  })

  it('does not overwrite unread history when storage loading fails', async () => {
    await store.flush()
    load.mockRejectedValueOnce(new Error('storage unavailable'))
    auth.user = { id: 2 }
    await flushPromises()
    expect(store.ready).toBe(true)
    expect(store.storageError).toBe('load')
    store.activeSession!.form.prompt = 'Unsaved draft'
    await store.flush()
    expect(save.mock.calls.some(([id]) => id === 2)).toBe(false)
  })

  it('reports failed persistence without discarding the current draft', async () => {
    save.mockRejectedValueOnce(new Error('quota exceeded'))
    await store.flush()
    expect(store.storageError).toBe('save')
    expect(store.activeSession!.form.prompt).toBe('A forest')
  })

  it('does not silently overwrite another tab after a version conflict', async () => {
    save.mockRejectedValueOnce(new Error('image-studio-conflict'))
    const first = store.flush()
    const queued = store.flush()
    await Promise.all([first, queued])
    expect(store.storageError).toBe('conflict')
    const count = save.mock.calls.length
    store.activeSession!.form.prompt = 'Keep unsaved work visible'
    await store.flush()
    expect(save).toHaveBeenCalledTimes(count)
    expect(store.activeSession!.form.prompt).toBe('Keep unsaved work visible')
  })

  it('deletes only the selected session and keeps a usable workspace', async () => {
    const first = store.activeId!
    store.createSession()
    const second = store.activeId!
    store.deleteSession(first)
    await nextTick()
    expect(store.sessions.map(session => session.id)).toEqual([second])
    await store.flush()
    expect(store.storageError).toBeNull()
    expect(save).toHaveBeenLastCalledWith(1, expect.objectContaining({ activeId: second }))
    store.deleteSession(second)
    expect(store.sessions).toHaveLength(1)
    expect(store.activeId).not.toBe(second)
  })
})


describe('image studio run records', () => {
  it('persists each successful image before a later confirmed failure and retains unsubmitted progress', async () => {
    store.activeSession!.form.count = 4
    generate.mockImplementation(async (options: GenerateImageOptions) => {
      options.onRequest?.(1)
      await options.onImages?.([{ src: 'data:image/png;base64,FIRST' }])
      expect(save.mock.calls.at(-1)?.[1].sessions[0].results).toHaveLength(1)
      options.onRequest?.(1)
      throw new ImageGenerationError('second request failed', 'confirmed', 400)
    })
    expect(await store.generate('secret', 'gemini')).toBe(1)
    const session = store.activeSession!
    expect(session.results).toHaveLength(1)
    expect(session.runs[0]).toMatchObject({ status: 'failed', requested: 4, submitted: 2, completed: 1, errorKind: 'confirmed' })
    expect(session.error).toBe('second request failed')
    expect(session.pending).toBeNull()
    expect(generate).toHaveBeenCalledTimes(1)
  })

  it('keeps partial results when the next submitted request has an unknown outcome', async () => {
    generate.mockImplementation(async (options: GenerateImageOptions) => {
      options.onRequest?.(1)
      await options.onImages?.([{ src: 'first' }])
      options.onRequest?.(1)
      throw new ImageGenerationError('offline', 'unknown')
    })
    expect(await store.generate('secret', 'gemini')).toBe(1)
    expect(store.activeSession!.runs[0]).toMatchObject({ status: 'interrupted', errorKind: 'unknown', completed: 1 })
    expect(store.activeSession!.interrupted).toBe(true)
    expect(store.activeSession!.error).toBe('imageGeneration.submitUnknown')
  })

  it('restores the normalized immutable run inputs and its original key without submitting', async () => {
    const session = store.activeSession!
    session.form = { ...session.form, apiKeyId: 73, model: ' models/gpt-image-2 ', prompt: '  A forest  ', aspectRatio: '16:9' }
    const reference = new File(['source'], 'source.png', { type: 'image/png' })
    session.referenceFiles = [reference]
    generate.mockResolvedValue([{ src: 'first' }, { src: 'second' }])
    await store.generate('must-not-be-saved', 'openai')
    const run = session.runs[0]!
    expect(run.form).toMatchObject({ apiKeyId: 73, model: 'gpt-image-2', prompt: 'A forest', aspectRatio: '1:1', resolution: '1K' })
    expect(Object.isFrozen(run.form)).toBe(true)
    expect(Object.isFrozen(run.referenceFiles)).toBe(true)
    expect(session.results.every(result => result.runId === run.id && result.aspectRatio === '1:1')).toBe(true)
    session.form.prompt = 'new draft'
    session.form.apiKeyId = 999
    session.referenceFiles = []
    expect(store.restoreResult(session.results[1]!.id)).toBe(true)
    expect(session.form).toEqual(run.form)
    expect(session.form).not.toBe(run.form)
    expect(session.referenceFiles).toEqual([reference])
    expect(session.referenceFiles).not.toBe(run.referenceFiles)
    expect(generate).toHaveBeenCalledTimes(1)
    await store.flush()
    expect(JSON.stringify(save.mock.calls)).not.toContain('must-not-be-saved')
    expect(session.results[0]).not.toHaveProperty('referenceFiles')
  })

  it('loads legacy results and only reuses their recorded parameters', async () => {
    const saved = structuredClone(toRaw(store.activeSession!))
    Reflect.deleteProperty(saved, 'runs')
    saved.results.push({ id: 'legacy', src: 'old', model: 'old-model', prompt: 'old prompt', aspectRatio: '3:2', createdAt: 'old', mode: 'text' })
    load.mockResolvedValue({ sessions: [saved], activeId: saved.id })
    auth.user = { id: 2 }
    await flushPromises()
    const session = store.activeSession!
    session.form.apiKeyId = 11
    session.form.resolution = '4K'
    session.form.quality = 'high'
    expect(store.restoreResult('legacy')).toBe(true)
    expect(session.runs).toEqual([])
    expect(session.form).toMatchObject({ model: 'old-model', prompt: 'old prompt', aspectRatio: '3:2', apiKeyId: 11, resolution: '4K', quality: 'high' })
    expect(generate).not.toHaveBeenCalled()
  })

  it('does not submit after a capability check finishes for a different owner', async () => {
    let complete!: (enabled: boolean) => void
    capability.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    const pending = store.generate('secret', 'openai')
    await flushPromises()
    const signal = capability.mock.calls[0]![2] as AbortSignal
    auth.user = { id: 2 }
    await flushPromises()
    expect(signal.aborted).toBe(true)
    complete(true)
    await pending
    expect(submit).not.toHaveBeenCalled()
    expect(generate).not.toHaveBeenCalled()
    expect(store.activeSession!.runs).toEqual([])
  })

  it('renames a session without changing its generation inputs', () => {
    const session = store.activeSession!
    store.renameSession(session.id, '  Forest study  ')
    expect(session.title).toBe('Forest study')
    expect(session.form.prompt).toBe('A forest')
  })
})

describe('image studio asynchronous recovery', () => {
  const task = (status: ImageTask['status'] = 'processing'): ImageTask => ({
    id: 'imgtask_original', status, images: status === 'completed' ? [{ src: 'stored-result' }] : [], retryAfterMs: 0,
  })

  function mockSubmission() {
    capability.mockResolvedValue(true)
    submit.mockImplementation(async (options: GenerateImageOptions) => {
      options.onRequest?.(options.count)
      return task()
    })
  }

  it('persists the accepted task before polling and deduplicates completed recovery', async () => {
    mockSubmission()
    query.mockImplementation(async (key, id) => {
      expect(key).toBe('original-key')
      expect(id).toBe('imgtask_original')
      expect(save.mock.calls.at(-1)?.[1].sessions[0].runs[0].taskId).toBe(id)
      return task('completed')
    })
    expect(await store.generate('original-key', 'openai')).toBe(1)
    const session = store.activeSession!
    expect(session.runs[0]).toMatchObject({ taskId: 'imgtask_original', status: 'completed', completed: 1 })
    expect(await store.recover(session.id, 'original-key')).toBe(0)
    expect(query).toHaveBeenCalledTimes(1)
    expect(submit).toHaveBeenCalledTimes(1)
    expect(generate).not.toHaveBeenCalled()
    expect(session.results).toHaveLength(1)
  })

  it('keeps task identity after a query network failure and resumes after reload with GET only', async () => {
    mockSubmission()
    query.mockRejectedValueOnce(new TypeError('offline'))
    await expect(store.generate('original-key', 'openai')).rejects.toThrow('offline')
    expect(store.activeSession!.runs[0]).toMatchObject({ taskId: 'imgtask_original', status: 'processing', errorKind: 'query' })
    expect(store.activeSession!.pending).toBeNull()
    await store.flush()
    const snapshot = structuredClone(save.mock.calls.at(-1)![1])
    load.mockResolvedValue(snapshot)
    auth.user = { id: 2 }
    await flushPromises()
    const session = store.activeSession!
    query.mockResolvedValue(task('completed'))
    expect(await store.recover(session.id, 'original-key')).toBe(1)
    expect(submit).toHaveBeenCalledTimes(1)
    expect(capability).toHaveBeenCalledTimes(1)
    expect(generate).not.toHaveBeenCalled()
    expect(session.results).toHaveLength(1)
  })

  it('never falls back or retries when the async submission response is lost', async () => {
    mockSubmission()
    submit.mockImplementation(async (options: GenerateImageOptions) => {
      options.onRequest?.(options.count)
      throw new ImageGenerationError('connection lost', 'unknown')
    })
    await expect(store.generate('key', 'openai')).rejects.toThrow('connection lost')
    const session = store.activeSession!
    expect(session.runs[0]).toMatchObject({ status: 'interrupted', errorKind: 'unknown' })
    expect(session.runs[0]!.taskId).toBeUndefined()
    expect(await store.recover(session.id, 'key')).toBe(0)
    expect(submit).toHaveBeenCalledTimes(1)
    expect(generate).not.toHaveBeenCalled()
    expect(query).not.toHaveBeenCalled()
  })

  it('does not submit when the capability check fails', async () => {
    capability.mockRejectedValue(new Error('capability offline'))
    await expect(store.generate('key', 'openai')).rejects.toThrow('capability offline')
    expect(store.activeSession!.runs[0]).toMatchObject({ submitted: 0, errorKind: 'confirmed' })
    expect(generate).not.toHaveBeenCalled()
    expect(submit).not.toHaveBeenCalled()
  })

  it.each([[404, 'taskExpired'], [401, 'taskKeyUnavailable'], [403, 'taskKeyUnavailable']] as const)(
    'stops polling on HTTP %s without losing the task or submitting again', async (status, key) => {
      mockSubmission()
      query.mockRejectedValue(new ImageGenerationError('query rejected', 'confirmed', status))
      await expect(store.generate('original-key', 'openai')).rejects.toThrow('query rejected')
      expect(store.activeSession!.runs[0]).toMatchObject({ status: 'processing', taskId: 'imgtask_original', errorKind: 'query', error: `imageGeneration.${key}` })
      expect(query).toHaveBeenCalledTimes(1)
      expect(submit).toHaveBeenCalledTimes(1)
      expect(generate).not.toHaveBeenCalled()
    },
  )

  it('aborts a pending task query when the owner changes and ignores late results', async () => {
    mockSubmission()
    let complete!: (result: ImageTask) => void
    query.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    const pending = store.generate('key', 'openai')
    await vi.waitFor(() => expect(query).toHaveBeenCalledTimes(1))
    const signal = query.mock.calls[0]![2] as AbortSignal
    auth.user = { id: 2 }
    await flushPromises()
    expect(signal.aborted).toBe(true)
    complete(task('completed'))
    await pending
    expect(store.activeSession!.results).toEqual([])
    expect(store.activeSession!.runs).toEqual([])
  })
})
