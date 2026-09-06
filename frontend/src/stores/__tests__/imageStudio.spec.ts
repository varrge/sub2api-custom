import { createPinia, setActivePinia } from 'pinia'
import { reactive, nextTick, toRaw } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { useImageStudioStore } from '../imageStudio'

const { load, save, generate } = vi.hoisted(() => ({ load: vi.fn(), save: vi.fn(), generate: vi.fn() }))
const auth = reactive({ isAuthenticated: true, user: { id: 1 } })
vi.mock('../auth', () => ({ useAuthStore: () => auth }))
vi.mock('../imageStudioStorage', () => ({ loadImageStudio: load, saveImageStudio: save }))
vi.mock('@/api/imageGeneration', () => ({ generateImages: generate }))

let store: ReturnType<typeof useImageStudioStore>
beforeEach(async () => {
  vi.clearAllMocks()
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
  it('keeps background results in the originating session and excludes the API key from storage', async () => {
    let complete!: (images: { src: string }[]) => void
    generate.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    const original = store.activeSession!
    const pending = store.generate('never-persist-this-secret', 'openai')
    await store.generate('never-persist-this-secret', 'openai')
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
    session.pending = { count: 1, aspectRatio: 'auto' }
    load.mockResolvedValue({ sessions: [session], activeId: session.id })
    auth.user = { id: 2 }
    await flushPromises()
    expect(store.activeSession!.form.prompt).toBe('A forest')
    expect(store.activeSession!.interrupted).toBe(true)
    expect(store.activeSession!.pending).toBeNull()
    expect(generate).not.toHaveBeenCalled()
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
