import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  createVideoTask,
  fetchVideoBlob,
  getVideoTask,
  listVideoModels,
  supportedVideoResolutions,
  supportsVideoGeneration,
  videoContentURL,
  videoProtocolForModel,
  videoRequiresReference,
  type GenerateVideoOptions,
  type VideoTask,
} from '../videoGeneration'

vi.mock('../client', () => ({ buildGatewayUrl: (path: string) => path }))

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

function jsonResponse(payload: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: '',
    json: async () => payload,
  }
}

const grokOptions: GenerateVideoOptions = {
  apiKey: 'test-only', platform: 'grok', model: 'grok-imagine-video',
  prompt: 'Waves at sunset', duration: 8, aspectRatio: '16:9', resolution: '720p',
}

describe('video capabilities and discovery', () => {
  it('recognizes supported providers and requires an image only for Grok 1.5', () => {
    expect(videoProtocolForModel('grok-imagine-video')).toBe('grok')
    expect(videoProtocolForModel('x-ai/grok-imagine-video-1.5-preview')).toBe('grok')
    expect(videoProtocolForModel('doubao-seedance-1-5-pro')).toBe('seedance')
    expect(videoProtocolForModel('seedance-video')).toBe('seedance')
    expect(videoProtocolForModel('grok-imagine-image')).toBeNull()
    expect(videoProtocolForModel('gemini-3-pro-image')).toBeNull()
    expect(supportsVideoGeneration('openai')).toBe(true)
    expect(supportsVideoGeneration('composite')).toBe(true)
    expect(supportsVideoGeneration('gemini')).toBe(false)
    expect(supportsVideoGeneration(undefined)).toBe(false)
    expect(videoRequiresReference('grok-imagine-video-1.5-preview')).toBe(true)
    expect(videoRequiresReference('grok-imagine-video')).toBe(false)
    expect(videoRequiresReference('doubao-seedance-1-5-pro')).toBe(false)
    expect(supportedVideoResolutions('grok-imagine-video')).toEqual(['480p', '720p'])
    expect(supportedVideoResolutions('grok-imagine-video-1.5')).toEqual(['480p', '720p', '1080p'])
    expect(supportedVideoResolutions('gpt-image-2')).toEqual([])
  })

  it('filters and deduplicates each group platform without inventing models', async () => {
    const fetcher = vi.fn().mockImplementation(async () => jsonResponse({
      data: [
        { id: 'grok-imagine-video' }, { id: 'grok-imagine-video' },
        { id: 'grok-imagine-video-1.5' }, { id: 'doubao-seedance-1-5-pro' },
        { id: 'grok-imagine-image' }, { id: 'gpt-4.1' },
      ],
    }))
    vi.stubGlobal('fetch', fetcher)
    expect(await listVideoModels('test-only', 'grok')).toEqual(['grok-imagine-video', 'grok-imagine-video-1.5'])
    expect(await listVideoModels('test-only', 'openai')).toEqual(['doubao-seedance-1-5-pro'])
    expect(await listVideoModels('test-only', 'composite')).toEqual([
      'grok-imagine-video', 'grok-imagine-video-1.5', 'doubao-seedance-1-5-pro',
    ])
    expect(await listVideoModels('test-only', 'gemini')).toEqual([])
    expect(fetcher).toHaveBeenCalledTimes(3)
    expect(fetcher).toHaveBeenCalledWith('/v1/models', {
      headers: { Accept: 'application/json', Authorization: 'Bearer test-only' },
    })
  })
})

describe('video submission', () => {
  it('submits one Grok job with explicit duration, ratio, and resolution', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ request_id: 'grok-task' }, 202))
    vi.stubGlobal('fetch', fetcher)
    expect(await createVideoTask(grokOptions)).toEqual({ id: 'grok-task', protocol: 'grok', status: 'queued' })
    expect(fetcher).toHaveBeenCalledTimes(1)
    const [path, request] = fetcher.mock.calls[0]!
    expect(path).toBe('/v1/videos/generations')
    expect(request.headers.Authorization).toBe('Bearer test-only')
    expect(request.redirect).toBe('error')
    expect(JSON.parse(request.body)).toEqual({
      model: 'grok-imagine-video', prompt: 'Waves at sunset',
      duration: 8, aspect_ratio: '16:9', resolution: '720p',
    })
  })

  it('preserves Grok 1.5 and embeds the selected first frame as a data URL', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ task_id: 'image-task' }))
    vi.stubGlobal('fetch', fetcher)
    const referenceImage = new File(['reference'], 'reference.webp', { type: 'image/webp' })
    await createVideoTask({ ...grokOptions, platform: 'composite', model: 'grok-imagine-video-1.5', resolution: '1080p', referenceImage })
    const body = JSON.parse(fetcher.mock.calls[0]![1].body)
    expect(body.model).toBe('grok-imagine-video-1.5')
    expect(body.image).toEqual({ url: 'data:image/webp;base64,cmVmZXJlbmNl', type: 'image_url' })
    expect(body.resolution).toBe('1080p')
  })

  it('uses native Seedance content and ratio fields with an optional first frame', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ id: 'ark-task' }))
    vi.stubGlobal('fetch', fetcher)
    await createVideoTask({
      ...grokOptions, platform: 'openai', model: 'doubao-seedance-1-5-pro', duration: 5,
      referenceImage: new File(['png'], 'first.png', { type: 'image/png' }),
    })
    const [path, request] = fetcher.mock.calls[0]!
    expect(path).toBe('/api/v3/contents/generations/tasks')
    expect(JSON.parse(request.body)).toEqual({
      model: 'doubao-seedance-1-5-pro',
      content: [
        { type: 'text', text: 'Waves at sunset' },
        { type: 'image_url', image_url: { url: 'data:image/png;base64,cG5n' }, role: 'first_frame' },
      ],
      duration: 5, ratio: '16:9', resolution: '720p',
    })
  })

  it('leaves Seedance model duration limits to Ark instead of applying the Grok tariff bounds', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ id: 'ark-task' }))
    vi.stubGlobal('fetch', fetcher)
    await createVideoTask({ ...grokOptions, platform: 'openai', model: 'seedance-video', duration: 20 })
    expect(JSON.parse(fetcher.mock.calls[0]![1].body)).toEqual({
      model: 'seedance-video', content: [{ type: 'text', text: 'Waves at sunset' }],
      duration: 20, ratio: '16:9', resolution: '720p',
    })
  })

  it.each([
    { model: 'grok-imagine-video-1.5' },
    { duration: 16 },
    { duration: 1.5 },
    { duration: 0 },
    { resolution: '1080p' as const },
    { prompt: ' ' },
    { platform: 'openai' as const },
    { referenceImage: new File(['gif'], 'first.gif', { type: 'image/gif' }) },
  ])('rejects invalid submission options before a paid request %#', async override => {
    const fetcher = vi.fn()
    vi.stubGlobal('fetch', fetcher)
    await expect(createVideoTask({ ...grokOptions, ...override })).rejects.toThrow()
    expect(fetcher).not.toHaveBeenCalled()
  })

  it('rejects oversized references before reading or submitting them', async () => {
    const fetcher = vi.fn()
    vi.stubGlobal('fetch', fetcher)
    const referenceImage = new File(['small'], 'first.png', { type: 'image/png' })
    Object.defineProperty(referenceImage, 'size', { value: 10 * 1024 * 1024 + 1 })
    await expect(createVideoTask({ ...grokOptions, referenceImage })).rejects.toThrow('10 MB')
    expect(fetcher).not.toHaveBeenCalled()
  })

  it('does not retry a rejected create or silently replace the requested model', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ error: { message: 'No eligible Grok media accounts' } }, 503))
    vi.stubGlobal('fetch', fetcher)
    await expect(createVideoTask(grokOptions)).rejects.toThrow('No eligible Grok media accounts')
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(JSON.parse(fetcher.mock.calls[0]![1].body).model).toBe(grokOptions.model)
  })

  it('passes abort signals through submission and skips already aborted work', async () => {
    const controller = new AbortController()
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ request_id: 'task' }))
    vi.stubGlobal('fetch', fetcher)
    await createVideoTask({ ...grokOptions, signal: controller.signal })
    expect(fetcher.mock.calls[0]![1].signal).toBe(controller.signal)
    controller.abort()
    await expect(createVideoTask({ ...grokOptions, signal: controller.signal })).rejects.toMatchObject({ name: 'AbortError' })
    expect(fetcher).toHaveBeenCalledTimes(1)
  })
})

describe('video task polling', () => {
  it.each([
    ['pending', 'queued'], ['running', 'running'], ['done', 'succeeded'],
    ['failed', 'failed'], ['expired', 'failed'], ['cancelled', 'cancelled'],
  ] as const)('normalizes Grok %s to %s', async (status, normalized) => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({
      status, video: { url: '/v1/videos/task/content', duration: 8 },
    }))
    vi.stubGlobal('fetch', fetcher)
    const controller = new AbortController()
    const task = await getVideoTask('original-key', 'grok', 'task', controller.signal)
    expect(task).toMatchObject({ id: 'task', protocol: 'grok', status: normalized })
    expect(fetcher).toHaveBeenCalledWith('/v1/videos/task', {
      headers: { Accept: 'application/json', Authorization: 'Bearer original-key' },
      signal: expect.any(AbortSignal),
    })
  })

  it('reads Seedance native success and error responses without exposing the internal task prefix', async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ id: 'ark-task', status: 'succeeded', content: { video_url: 'https://cdn.example/video.mp4' } }))
      .mockResolvedValueOnce(jsonResponse({ id: 'ark-task', status: 'failed', error: { code: 'ContentFilter', message: 'Blocked by provider' } }))
    vi.stubGlobal('fetch', fetcher)
    expect(await getVideoTask('test-only', 'seedance', 'ark-task')).toEqual({
      id: 'ark-task', protocol: 'seedance', status: 'succeeded', videoUrl: 'https://cdn.example/video.mp4',
    })
    expect(await getVideoTask('test-only', 'seedance', 'ark-task')).toMatchObject({
      status: 'failed', error: 'Blocked by provider',
    })
    expect(fetcher.mock.calls[0]![0]).toBe('/api/v3/contents/generations/tasks/ark-task')
  })

  it('rejects malformed task responses instead of polling forever', async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(jsonResponse({}))
      .mockResolvedValueOnce(jsonResponse({ status: 'unexpected' }))
      .mockResolvedValueOnce(jsonResponse({}))
    vi.stubGlobal('fetch', fetcher)
    await expect(createVideoTask(grokOptions)).rejects.toThrow('task ID')
    await expect(getVideoTask('test-only', 'grok', 'task')).rejects.toThrow('Unknown video task status')
    await expect(getVideoTask('test-only', 'grok', 'task')).rejects.toThrow('task status')
  })

  it('aborts a stalled poll after 60 seconds with a recoverable timeout error', async () => {
    vi.useFakeTimers()
    let requestSignal: AbortSignal | undefined
    const fetcher = vi.fn((_url: string, options: RequestInit) => new Promise<Response>((_resolve, reject) => {
      const signal = options.signal
      requestSignal = signal || undefined
      signal?.addEventListener('abort', () => reject(signal.reason), { once: true })
    }))
    vi.stubGlobal('fetch', fetcher)
    const result = getVideoTask('test-only', 'grok', 'task').catch(error => error)
    await vi.advanceTimersByTimeAsync(59_999)
    expect(requestSignal?.aborted).not.toBe(true)
    await vi.advanceTimersByTimeAsync(1)
    expect(requestSignal?.aborted).toBe(true)
    await expect(result).resolves.toEqual(new Error('videoGeneration.pollingTimeout'))
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('keeps the poll deadline active while reading a stalled response body', async () => {
    vi.useFakeTimers()
    const fetcher = vi.fn(async (_url: string, options: RequestInit) => ({
      ...jsonResponse({ status: 'pending' }),
      json: () => new Promise((_resolve, reject) => {
        const signal = options.signal
        signal?.addEventListener('abort', () => reject(signal.reason), { once: true })
      }),
    }))
    vi.stubGlobal('fetch', fetcher)
    const result = getVideoTask('test-only', 'seedance', 'task').catch(error => error)
    await vi.advanceTimersByTimeAsync(60_000)
    await expect(result).resolves.toEqual(new Error('videoGeneration.pollingTimeout'))
    expect(vi.getTimerCount()).toBe(0)
  })

  it('propagates caller cancellation during a pending poll and clears its deadline', async () => {
    vi.useFakeTimers()
    const controller = new AbortController()
    const reason = new DOMException('User cancelled polling', 'AbortError')
    let requestSignal: AbortSignal | undefined
    const fetcher = vi.fn((_url: string, options: RequestInit) => new Promise<Response>((_resolve, reject) => {
      const signal = options.signal
      requestSignal = signal || undefined
      signal?.addEventListener('abort', () => reject(signal.reason), { once: true })
    }))
    vi.stubGlobal('fetch', fetcher)
    const result = getVideoTask('test-only', 'grok', 'task', controller.signal).catch(error => error)
    controller.abort(reason)
    expect(requestSignal?.aborted).toBe(true)
    await expect(result).resolves.toBe(reason)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('skips already cancelled polls without issuing a request or starting a deadline', async () => {
    vi.useFakeTimers()
    const controller = new AbortController()
    controller.abort()
    const fetcher = vi.fn()
    vi.stubGlobal('fetch', fetcher)
    await expect(getVideoTask('test-only', 'grok', 'task', controller.signal)).rejects.toBe(controller.signal.reason)
    expect(fetcher).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('cleans up the deadline and caller abort listener after a fast response', async () => {
    vi.useFakeTimers()
    const controller = new AbortController()
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ status: 'pending' }))
    vi.stubGlobal('fetch', fetcher)
    await getVideoTask('test-only', 'grok', 'task', controller.signal)
    const requestSignal = fetcher.mock.calls[0]![1].signal as AbortSignal
    expect(vi.getTimerCount()).toBe(0)
    controller.abort()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(requestSignal.aborted).toBe(false)
  })
})

describe('video downloads', () => {
  const task: VideoTask = { id: 'task', protocol: 'grok', status: 'succeeded' }

  it('builds the canonical Grok media URL with an escaped task ID', () => {
    expect(videoContentURL({
      ...task, id: 'task/1?#', videoUrl: 'https://attacker.example/collect',
    })).toBe('/v1/videos/task%2F1%3F%23/content')
  })

  it('exposes safe absolute or relative Seedance URLs for native playback', () => {
    expect(videoContentURL({
      ...task, protocol: 'seedance', videoUrl: 'https://cdn.example/video.mp4?signature=abc',
    })).toBe('https://cdn.example/video.mp4?signature=abc')
    expect(videoContentURL({
      ...task, protocol: 'seedance', videoUrl: '/media/video.mp4?signature=abc',
    })).toBe(new URL('/media/video.mp4?signature=abc', window.location.href).href)
    expect(videoContentURL({
      ...task, protocol: 'seedance', videoUrl: 'http://cdn.example/video.mp4',
    })).toBe('http://cdn.example/video.mp4')
  })

  it.each([
    'javascript:alert(1)',
    'data:video/mp4;base64,dmlkZW8=',
    'blob:https://cdn.example/video',
    'ftp://cdn.example/video.mp4',
    'https://user:secret@cdn.example/video.mp4',
    '//user@cdn.example/video.mp4',
    'https://:secret@cdn.example/video.mp4',
    'https://[invalid/video.mp4',
  ])('rejects unsafe or malformed Seedance media URLs: %s', videoUrl => {
    expect(() => videoContentURL({ ...task, protocol: 'seedance', videoUrl })).toThrow('unsupported download URL')
  })

  it('requires a completed task, task ID, and Seedance media URL', () => {
    expect(() => videoContentURL({ ...task, status: 'queued' })).toThrow('not ready')
    expect(() => videoContentURL({ ...task, id: ' ' })).toThrow('not ready')
    expect(() => videoContentURL({ ...task, protocol: 'seedance' })).toThrow('missing a download URL')
    expect(() => videoContentURL({ ...task, protocol: 'seedance', videoUrl: ' ' })).toThrow('missing a download URL')
  })

  it('fetches protected Grok content from its fixed endpoint and ignores returned URLs', async () => {
    const blob = new Blob(['video'], { type: 'video/webm' })
    const fetcher = vi.fn().mockResolvedValue({ ok: true, blob: async () => blob })
    vi.stubGlobal('fetch', fetcher)
    expect(await fetchVideoBlob('original-key', { ...task, videoUrl: 'https://attacker.example/collect' })).toBe(blob)
    expect(fetcher).toHaveBeenCalledWith('/v1/videos/task/content', {
      headers: { Accept: 'video/*, application/octet-stream', Authorization: 'Bearer original-key' },
      credentials: 'omit', redirect: 'error', referrerPolicy: 'no-referrer', signal: undefined,
    })
  })

  it('downloads remote Seedance media without an Authorization header or cookies', async () => {
    const fetcher = vi.fn().mockResolvedValue({
      ok: true, blob: async () => new Blob(['video'], { type: 'application/octet-stream' }),
    })
    vi.stubGlobal('fetch', fetcher)
    const blob = await fetchVideoBlob('private-key', {
      ...task, protocol: 'seedance', videoUrl: 'https://cdn.example/video.mp4?signature=abc',
    })
    expect(blob.type).toBe('video/mp4')
    const [url, request] = fetcher.mock.calls[0]!
    expect(url).toBe('https://cdn.example/video.mp4?signature=abc')
    expect(request.headers).not.toHaveProperty('Authorization')
    expect(request.credentials).toBe('omit')
    expect(JSON.stringify(request)).not.toContain('private-key')
  })

  it('rejects unsafe media schemes and non-video or empty responses', async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce({ ok: true, blob: async () => new Blob(['<html>oops</html>'], { type: 'text/html' }) })
      .mockResolvedValueOnce({ ok: true, blob: async () => new Blob([], { type: 'video/mp4' }) })
    vi.stubGlobal('fetch', fetcher)
    await expect(fetchVideoBlob('key', { ...task, protocol: 'seedance', videoUrl: 'javascript:alert(1)' })).rejects.toThrow('unsupported download URL')
    expect(fetcher).not.toHaveBeenCalled()
    await expect(fetchVideoBlob('key', task)).rejects.toThrow('did not return a video')
    await expect(fetchVideoBlob('key', task)).rejects.toThrow('empty')
  })

  it('keeps not-yet-completed tasks from downloading and reports native HTTP errors', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ error: { message: 'Video request not found' } }, 404))
    vi.stubGlobal('fetch', fetcher)
    await expect(fetchVideoBlob('key', { ...task, status: 'running' })).rejects.toThrow('not ready')
    expect(fetcher).not.toHaveBeenCalled()
    await expect(fetchVideoBlob('key', task)).rejects.toThrow('Video request not found')
  })
})
