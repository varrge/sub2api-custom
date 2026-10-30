import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import type { VideoTask } from '@/api/videoGeneration'
import { useVideoStudioStore, type VideoStudioResult } from '@/stores/videoStudio'
import type { SavedVideoStudio } from '@/stores/videoStudioStorage'
import type { ApiKey, Group } from '@/types'
import VideoGenerationPanel from '../VideoGenerationPanel.vue'

const { load, save, createTask, getTask, listModels, fetchBlob, createObjectURL, revokeObjectURL, showError } = vi.hoisted(() => ({
  load: vi.fn(), save: vi.fn(), createTask: vi.fn(), getTask: vi.fn(), listModels: vi.fn(),
  fetchBlob: vi.fn(), createObjectURL: vi.fn(), revokeObjectURL: vi.fn(), showError: vi.fn(),
}))

vi.mock('@/api/videoGeneration', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/videoGeneration')>(),
  createVideoTask: createTask, getVideoTask: getTask, listVideoModels: listModels, fetchVideoBlob: fetchBlob,
}))
vi.mock('@/stores/videoStudioStorage', () => ({ loadVideoStudio: load, saveVideoStudio: save }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: true, user: { id: 42 } }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn(), showInfo: vi.fn() }) }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: `<select :value="modelValue" @change="$emit('update:modelValue', typeof modelValue === 'number' ? Number($event.target.value) : $event.target.value)">
    <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
  </select>`,
}
const grokGroup = { id: 1, name: 'Grok media', platform: 'grok', allow_image_generation: true } as Group
const keys = [
  { id: 1, name: 'Original', key: 'original-test-key', status: 'active', group: grokGroup },
  { id: 2, name: 'Next draft', key: 'next-draft-test-key', status: 'active', group: grokGroup },
] as ApiKey[]
const queued: VideoTask = { id: 'task-1', protocol: 'grok', status: 'queued' }
let view: VueWrapper | undefined
let pinia: ReturnType<typeof createPinia>
let studio: ReturnType<typeof useVideoStudioStore> | undefined

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(done => { resolve = done })
  return { promise, resolve }
}

function mountPanel(props = { apiKeys: keys, loadingKeys: false }) {
  studio = useVideoStudioStore(pinia)
  view = mount(VideoGenerationPanel, { props, global: {
    plugins: [pinia],
    stubs: { Select: SelectStub, ConfirmDialog: true, RouterLink: true, Icon: true, LoadingSpinner: true },
  } })
  return view
}

function result(task: VideoTask): VideoStudioResult {
  return {
    id: `result-${task.id}`, task, apiKeyId: 1, platform: task.protocol === 'grok' ? 'grok' : 'openai',
    model: task.protocol === 'grok' ? 'grok-imagine-video' : 'doubao-seedance-1-5-pro',
    prompt: `Video ${task.id}`, duration: 5, aspectRatio: '16:9', resolution: '720p',
    mode: 'text', createdAt: '2026/10/30 08:30:45',
  }
}

function restoredJob(): SavedVideoStudio {
  return { activeId: 'restored', sessions: [{
    id: 'restored', title: 'Restored video',
    form: { apiKeyId: 2, model: 'grok-imagine-video', prompt: 'Next draft', duration: 10, aspectRatio: '9:16', resolution: '720p' },
    referenceFile: null, results: [], activeResultId: null, interrupted: false, error: '',
    pending: {
      apiKeyId: 1, platform: 'grok', model: 'grok-imagine-video', prompt: 'Original video',
      duration: 5, aspectRatio: '16:9', resolution: '720p', mode: 'text', createdAt: '2026/10/30 08:30:45',
      phase: 'polling', task: queued,
    },
  }] }
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
  vi.resetAllMocks()
  pinia = createPinia()
  studio = undefined
  view = undefined
  load.mockResolvedValue(null)
  save.mockImplementation(async (_userId, state) => (state.revision ?? 0) + 1)
  createTask.mockResolvedValue(queued)
  getTask.mockImplementation(async (_key, protocol, id) => ({ id, protocol, status: 'succeeded' }))
  listModels.mockImplementation(async (_key, platform) => platform === 'openai'
    ? ['doubao-seedance-1-5-pro'] : ['grok-imagine-video', 'grok-imagine-video-1.5'])
  fetchBlob.mockResolvedValue(new Blob(['video'], { type: 'video/mp4' }))
  createObjectURL.mockImplementation(() => `blob:video-${createObjectURL.mock.calls.length}`)
  const NativeURL = URL
  vi.stubGlobal('URL', class extends NativeURL {
    static createObjectURL = createObjectURL
    static revokeObjectURL = revokeObjectURL
  })
})

afterEach(async () => {
  view?.unmount()
  await studio?.flush()
  studio?.$dispose()
  await flushPromises()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('video generation panel', () => {
  it('uses a later Grok group and requires the first frame without changing Grok 1.5 settings', async () => {
    const nonMedia = { id: 3, name: 'Text only', platform: 'anthropic', allow_image_generation: false } as Group
    const key = { ...keys[0]!, group: nonMedia, groups: [nonMedia, grokGroup] }
    const panel = mountPanel({ apiKeys: [key], loadingKeys: false })
    await flushPromises()
    expect(listModels).toHaveBeenCalledWith(key.key, 'grok')
    expect(panel.get('#video-api-key').text()).toContain('Grok media')
    await panel.get('#video-model').setValue('grok-imagine-video-1.5')
    await panel.get('#video-prompt').setValue('A fox at sunrise')
    await panel.get('#video-duration').setValue('10')
    await panel.get('#video-ratio').setValue('9:16')
    await panel.get('#video-resolution').setValue('1080p')
    expect(panel.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    expect(panel.text()).toContain('videoGeneration.requiresReference')
    await panel.get('form').trigger('submit')
    expect(createTask).not.toHaveBeenCalled()

    const reference = new File(['first frame'], 'first.webp', { type: 'image/webp' })
    Object.defineProperty(panel.get('input[type="file"]').element, 'files', { value: [reference] })
    await panel.get('input[type="file"]').trigger('change')
    expect(panel.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    await panel.get('form').trigger('submit')
    await flushPromises()
    expect(createTask).toHaveBeenCalledTimes(1)
    expect(createTask).toHaveBeenCalledWith({
      apiKey: key.key, platform: 'grok', model: 'grok-imagine-video-1.5', prompt: 'A fox at sunrise',
      duration: 10, aspectRatio: '9:16', resolution: '1080p', referenceImage: reference,
      signal: expect.any(AbortSignal),
    })
  })

  it('shows submitted settings while editing and plays the result with its original key through a blob URL', async () => {
    const creation = deferred<VideoTask>()
    createTask.mockReturnValueOnce(creation.promise)
    const panel = mountPanel()
    await flushPromises()
    await panel.get('#video-prompt').setValue('Original video')
    await panel.get('form').trigger('submit')
    await flushPromises()
    await panel.get('#video-api-key').setValue('2')
    await flushPromises()
    await panel.get('#video-model').setValue('grok-imagine-video-1.5')
    await panel.get('#video-duration').setValue('10')
    await panel.get('#video-resolution').setValue('1080p')
    await panel.get('#video-prompt').setValue('Next video')
    expect(panel.get('.video-canvas').text()).toContain('grok-imagine-video · 5s · 720p')
    expect(panel.get('.video-canvas').text()).not.toContain('grok-imagine-video-1.5')
    expect(createTask).toHaveBeenCalledTimes(1)

    creation.resolve(queued)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(getTask).toHaveBeenCalledTimes(2)
    for (const call of getTask.mock.calls) expect(call).toEqual([keys[0]!.key, 'grok', 'task-1', expect.any(AbortSignal)])
    expect(fetchBlob).toHaveBeenCalledTimes(1)
    expect(fetchBlob).toHaveBeenCalledWith(keys[0]!.key, expect.objectContaining({ id: 'task-1' }), expect.any(AbortSignal))
    expect(panel.get('video').attributes('src')).toBe('blob:video-1')
    expect(panel.get('video').attributes('src')).not.toContain(keys[0]!.key)
    expect(panel.get('.video-canvas').text()).toContain('grok-imagine-video')
    expect(studio!.activeSession!.results[0]).toMatchObject({ apiKeyId: 1, model: 'grok-imagine-video', duration: 5, resolution: '720p' })
    expect(studio!.activeSession!.form).toMatchObject({ apiKeyId: 2, model: 'grok-imagine-video-1.5', prompt: 'Next video', duration: 10, resolution: '1080p' })
  })

  it.each(['expired', 'quota_exhausted'] as const)('excludes %s keys from generation while restoring playback with the original Bearer', async status => {
    const original = { ...keys[0]!, status }
    const saved = restoredJob()
    const completed = result({ id: 'restored-task', protocol: 'grok', status: 'succeeded' })
    Object.assign(saved.sessions[0]!, {
      pending: null, results: [completed], activeResultId: completed.id,
      form: { ...saved.sessions[0]!.form, apiKeyId: original.id },
    })
    load.mockResolvedValueOnce(saved)
    const actual = await vi.importActual<typeof import('@/api/videoGeneration')>('@/api/videoGeneration')
    fetchBlob.mockImplementationOnce(actual.fetchVideoBlob)
    const fetcher = vi.fn().mockResolvedValue({
      ok: true, blob: async () => new Blob(['saved video'], { type: 'video/mp4' }),
    })
    vi.stubGlobal('fetch', fetcher)
    const panel = mountPanel({ apiKeys: [original, keys[1]!], loadingKeys: false })
    await flushPromises()

    expect(panel.findAll('#video-api-key option').map(option => option.attributes('value'))).toEqual(['2'])
    expect(studio!.activeSession!.form.apiKeyId).toBe(2)
    expect(listModels).toHaveBeenCalledWith(keys[1]!.key, 'grok')
    expect(getTask).toHaveBeenCalledWith(original.key, 'grok', completed.task.id, expect.any(AbortSignal))
    expect(fetcher).toHaveBeenCalledTimes(1)
    const contentUrl = new URL('/v1/videos/restored-task/content', window.location.origin).href
    expect(fetcher).toHaveBeenCalledWith(contentUrl, expect.objectContaining({
      headers: expect.objectContaining({ Authorization: `Bearer ${original.key}` }),
      credentials: 'omit',
    }))
    expect(panel.get('video').attributes('src')).toBe('blob:video-1')
    expect(panel.get('video').attributes('src')).not.toContain(original.key)
    expect(createTask).not.toHaveBeenCalled()
  })

  it('plays the refreshed Seedance URL natively and fetches a blob only for download', async () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const panel = mountPanel()
    await flushPromises()
    const saved = result({ id: 'seedance-task', protocol: 'seedance', status: 'succeeded', videoUrl: 'https://cdn.example/expired.mp4' })
    getTask.mockResolvedValueOnce({ ...saved.task, videoUrl: 'https://cdn.example/fresh.mp4?signature=safe' })
    studio!.activeSession!.results.push(saved)
    studio!.activeSession!.activeResultId = saved.id
    await flushPromises()
    expect(panel.get('video').attributes('src')).toBe('https://cdn.example/fresh.mp4?signature=safe')
    expect(fetchBlob).not.toHaveBeenCalled()
    expect(createObjectURL).not.toHaveBeenCalled()

    await panel.findAll('button').find(button => button.text() === 'videoGeneration.download')!.trigger('click')
    await flushPromises()
    expect(fetchBlob).toHaveBeenCalledTimes(1)
    expect(fetchBlob).toHaveBeenCalledWith(keys[0]!.key, expect.objectContaining({ protocol: 'seedance', videoUrl: 'https://cdn.example/fresh.mp4?signature=safe' }), expect.any(AbortSignal))
    const link = click.mock.instances[0] as HTMLAnchorElement
    expect(link.href).toBe('blob:video-1')
    expect(link.download).toBe('sub2api-video-seedance-task.mp4')
    expect(panel.get('video').attributes('src')).toBe('https://cdn.example/fresh.mp4?signature=safe')
  })

  it('rejects an unsafe Seedance playback URL before creating a video element', async () => {
    const panel = mountPanel()
    await flushPromises()
    const saved = result({ id: 'unsafe-task', protocol: 'seedance', status: 'succeeded' })
    getTask.mockResolvedValueOnce({ ...saved.task, videoUrl: 'javascript:alert(1)' })
    studio!.activeSession!.results.push(saved)
    studio!.activeSession!.activeResultId = saved.id
    await flushPromises()
    expect(panel.find('video').exists()).toBe(false)
    expect(panel.get('.video-canvas').text()).toContain('unsupported download URL')
    expect(fetchBlob).not.toHaveBeenCalled()
    expect(createObjectURL).not.toHaveBeenCalled()
  })

  it('aborts playback when switching sessions and ignores a late blob from the previous session', async () => {
    const oldBlob = deferred<Blob>()
    fetchBlob.mockReturnValueOnce(oldBlob.promise)
    const panel = mountPanel()
    await flushPromises()
    const first = studio!.activeSession!
    const second = studio!.createSession()!
    const oldResult = result({ id: 'old-task', protocol: 'grok', status: 'succeeded' })
    const newResult = result({ id: 'new-task', protocol: 'grok', status: 'succeeded' })
    first.results.push(oldResult)
    first.activeResultId = oldResult.id
    second.results.push(newResult)
    second.activeResultId = newResult.id
    studio!.activeId = first.id
    await flushPromises()
    const oldSignal = fetchBlob.mock.calls[0]![2] as AbortSignal
    expect(oldSignal.aborted).toBe(false)
    await panel.get('[data-testid="video-sessions"] button[aria-pressed="false"]').trigger('click')
    await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    expect(panel.get('video').attributes('aria-label')).toBe('Video new-task')
    expect(panel.get('video').attributes('src')).toBe('blob:video-1')

    oldBlob.resolve(new Blob(['old video'], { type: 'video/mp4' }))
    await flushPromises()
    expect(createObjectURL).toHaveBeenCalledTimes(1)
    expect(panel.get('video').attributes('src')).toBe('blob:video-1')
    expect(panel.get('video').attributes('aria-label')).toBe('Video new-task')
  })

  it('aborts a stalled Seedance download on session change and keeps the next download independent', async () => {
    const oldDownload = deferred<Blob>()
    const newDownload = deferred<Blob>()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    fetchBlob.mockReturnValueOnce(oldDownload.promise).mockReturnValueOnce(newDownload.promise)
    getTask.mockImplementation(async (_key, protocol, id) => ({
      id, protocol, status: 'succeeded', videoUrl: `https://cdn.example/${id}.mp4`,
    }))
    const panel = mountPanel()
    await flushPromises()
    const first = studio!.activeSession!
    const second = studio!.createSession()!
    const oldResult = result({ id: 'old-download', protocol: 'seedance', status: 'succeeded' })
    const newResult = result({ id: 'new-download', protocol: 'seedance', status: 'succeeded' })
    first.results.push(oldResult)
    first.activeResultId = oldResult.id
    second.results.push(newResult)
    second.activeResultId = newResult.id
    studio!.activeId = first.id
    await flushPromises()
    await panel.get('.video-canvas button').trigger('click')
    await flushPromises()
    const oldSignal = fetchBlob.mock.calls[0]![2] as AbortSignal
    expect(oldSignal).toBeInstanceOf(AbortSignal)
    expect(oldSignal.aborted).toBe(false)
    expect(panel.get('.video-canvas button').attributes('disabled')).toBeDefined()

    await panel.get('[data-testid="video-sessions"] button[aria-pressed="false"]').trigger('click')
    await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    expect(panel.get('video').attributes('src')).toBe('https://cdn.example/new-download.mp4')
    expect(panel.get('.video-canvas button').attributes('disabled')).toBeUndefined()
    await panel.get('.video-canvas button').trigger('click')
    await flushPromises()
    const newSignal = fetchBlob.mock.calls[1]![2] as AbortSignal
    expect(newSignal).toBeInstanceOf(AbortSignal)
    expect(newSignal.aborted).toBe(false)
    expect(panel.get('.video-canvas button').attributes('disabled')).toBeDefined()

    oldDownload.resolve(new Blob(['old download'], { type: 'video/mp4' }))
    await flushPromises()
    expect(click).not.toHaveBeenCalled()
    expect(createObjectURL).not.toHaveBeenCalled()
    expect(panel.get('.video-canvas button').attributes('disabled')).toBeDefined()
    newDownload.resolve(new Blob(['new download'], { type: 'video/mp4' }))
    await flushPromises()
    expect(click).toHaveBeenCalledTimes(1)
    expect((click.mock.instances[0] as HTMLAnchorElement).download).toBe('sub2api-video-new-download.mp4')
    expect(panel.get('.video-canvas button').attributes('disabled')).toBeUndefined()
  })

  it('resumes restored jobs only after storage and API keys have both finished loading', async () => {
    const loading = deferred<SavedVideoStudio | null>()
    load.mockReturnValueOnce(loading.promise)
    studio = useVideoStudioStore(pinia)
    const resume = vi.spyOn(studio, 'resumeJobs')
    getTask.mockResolvedValue({ ...queued, status: 'running' })
    const panel = mountPanel({ apiKeys: keys, loadingKeys: true })
    await flushPromises()
    await panel.setProps({ loadingKeys: false })
    expect(resume).not.toHaveBeenCalled()
    await panel.setProps({ loadingKeys: true })
    loading.resolve(restoredJob())
    await flushPromises()
    expect(studio.ready).toBe(true)
    expect(resume).not.toHaveBeenCalled()
    await panel.setProps({ loadingKeys: false })
    await flushPromises()
    expect(resume).toHaveBeenCalledTimes(1)
    expect(resume).toHaveBeenCalledWith(keys)
    expect(getTask).toHaveBeenCalledTimes(1)
    expect(getTask).toHaveBeenCalledWith(keys[0]!.key, 'grok', 'task-1', expect.any(AbortSignal))
    expect(createTask).not.toHaveBeenCalled()
  })

  it('offers resume after a polling failure and refreshes the saved task without creating another job', async () => {
    load.mockResolvedValueOnce(restoredJob())
    getTask.mockRejectedValueOnce(new Error('Network unavailable'))
    const panel = mountPanel()
    await flushPromises()
    expect(studio!.activeSession!.pending?.phase).toBe('paused')
    expect(panel.get('.video-canvas').text()).toContain('videoGeneration.paused')
    const resume = panel.findAll('button').find(button => button.text() === 'videoGeneration.resume')!
    expect(resume.exists()).toBe(true)
    await resume.trigger('click')
    await flushPromises()
    expect(studio!.activeSession!.pending).toBeNull()
    expect(panel.find('video').exists()).toBe(true)
    expect(getTask).toHaveBeenCalledTimes(3)
    for (const call of getTask.mock.calls) expect(call[0]).toBe(keys[0]!.key)
    expect(createTask).not.toHaveBeenCalled()
    expect(showError).not.toHaveBeenCalled()
  })

  it('keeps store polling after the panel unmounts and displays its completed result on remount', async () => {
    const panel = mountPanel()
    await flushPromises()
    await panel.get('#video-prompt').setValue('Continue in the background')
    await panel.get('form').trigger('submit')
    await flushPromises()
    const signal = createTask.mock.calls[0]![0].signal as AbortSignal
    expect(getTask).not.toHaveBeenCalled()
    panel.unmount()
    view = undefined
    expect(signal.aborted).toBe(false)
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(studio!.activeSession!.pending).toBeNull()
    expect(studio!.activeSession!.results).toHaveLength(1)
    expect(getTask).toHaveBeenCalledTimes(1)
    const reopened = mountPanel()
    await flushPromises()
    expect(reopened.get('video').attributes('src')).toBe('blob:video-1')
    expect(createTask).toHaveBeenCalledTimes(1)
  })
})
