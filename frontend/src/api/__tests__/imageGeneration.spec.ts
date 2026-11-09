import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  generateImages,
  getImageTask,
  ImageGenerationError,
  normalizeImageRequest,
  readImageAsyncCapability,
  resolveImageCapabilities,
  submitImageTask,
  isLikelyImageModel,
  openAIImageSize,
  parseGeneratedImages,
  supportedImageAspectRatios,
  supportedImageResolutions,
  supportsImageGeneration,
  type GenerateImageOptions,
} from '../imageGeneration'

vi.mock('../client', () => ({ buildGatewayUrl: (path: string) => path }))

afterEach(() => vi.unstubAllGlobals())

describe('image generation helpers', () => {
  it('keeps supported image models and excludes video models', () => {
    expect(isLikelyImageModel('gemini-3-pro-image-preview')).toBe(true)
    expect(isLikelyImageModel('gpt-image-2')).toBe(true)
    expect(isLikelyImageModel('grok-imagine-video-1.5')).toBe(false)
    expect(isLikelyImageModel('imagen-3')).toBe(false)
    expect(supportsImageGeneration('anthropic')).toBe(false)
    expect(supportedImageAspectRatios('openai', 'gpt-image-1.5')).toEqual(['auto', '3:2', '1:1', '2:3'])
    expect(supportedImageAspectRatios('openai', 'gpt-image-2', '4K')).toContain('16:9')
    expect(supportedImageAspectRatios('openai', 'gpt-image-2', '4K')).not.toContain('auto')
    expect(supportedImageAspectRatios('gemini', 'gemini-3-pro-image-preview')).toContain('21:9')
  })

  it('offers only the resolutions each model supports, including composite groups', () => {
    expect(supportedImageResolutions('openai', 'gpt-image-2')).toEqual(['1K', '2K', '4K'])
    expect(supportedImageResolutions('openai', 'gpt-image-2.5-sunburst')).toEqual(['1K', '2K', '4K'])
    expect(supportedImageResolutions('openai', 'gpt-image-1.5')).toEqual(['1K'])
    expect(supportedImageResolutions('gemini', 'gemini-2.5-flash-image')).toEqual(['1K'])
    expect(supportedImageResolutions('gemini', 'gemini-3.1-flash-lite-image')).toEqual(['1K'])
    expect(supportedImageResolutions('antigravity', 'gemini-3-pro-image-preview')).toEqual(['1K', '2K', '4K'])
    expect(supportedImageResolutions('grok', 'grok-imagine-image')).toEqual(['1K', '2K'])
    expect(supportedImageResolutions('composite', 'grok-imagine-image')).toEqual(['1K', '2K'])
    expect(supportedImageResolutions('composite', 'gemini-3-pro-image')).toEqual(['1K', '2K', '4K'])
  })

  it('keeps custom sizes inside OpenAI pixel and edge limits at every ratio and resolution', () => {
    expect(openAIImageSize('auto', '1K', 'gpt-image-2')).toBe('1024x1024')
    expect(openAIImageSize('4:3', '1K', 'gpt-image-2')).toBe('1024x768')
    expect(openAIImageSize('3:4', '1K', 'gpt-image-2')).toBe('768x1024')
    expect(openAIImageSize('1:1', '2K', 'gpt-image-2')).toBe('2048x2048')
    expect(openAIImageSize('16:9', '4K', 'gpt-image-2')).toBe('3840x2160')
    expect(openAIImageSize('1:1', '4K', 'gpt-image-2')).toBe('2880x2880')
    expect(openAIImageSize('2:3', '1K', 'gpt-image-1.5')).toBe('1024x1536')
    for (const resolution of ['1K', '2K', '4K'] as const) {
      for (const ratio of supportedImageAspectRatios('openai', 'gpt-image-2', resolution)) {
        const size = openAIImageSize(ratio, resolution, 'gpt-image-2')
        if (size === 'auto') continue
        const [width, height] = size.split('x').map(Number) as [number, number]
        expect(width % 16).toBe(0)
        expect(height % 16).toBe(0)
        expect(Math.max(width, height)).toBeLessThanOrEqual(3840)
        if (resolution === '1K') expect(Math.max(width, height)).toBeLessThanOrEqual(1024)
        expect(width * height).toBeGreaterThanOrEqual(655_360)
        expect(width * height).toBeLessThanOrEqual(8_294_400)
        expect(Math.max(width, height) / Math.min(width, height)).toBeLessThanOrEqual(3)
      }
    }
  })

  it('normalizes OpenAI and Gemini image responses', () => {
    expect(parseGeneratedImages({ data: [{ b64_json: 'QUJD', revised_prompt: 'cat' }] })).toEqual([
      { src: 'data:image/png;base64,QUJD', revisedPrompt: 'cat' },
    ])
    expect(parseGeneratedImages({
      candidates: [{ content: { parts: [{ text: 'done' }, { inlineData: { mimeType: 'image/webp', data: 'REVG' } }] } }],
    })).toEqual([{ src: 'data:image/webp;base64,REVG', revisedPrompt: 'done' }])
  })
})

describe('image generation requests', () => {
  const options: GenerateImageOptions = {
    apiKey: 'test-only', platform: 'openai', model: 'gpt-image-2',
    prompt: 'A forest', aspectRatio: '16:9', resolution: '1K', quality: 'standard',
    count: 1, referenceImages: [],
  }

  function mockFetch() {
    const fetcher = vi.fn().mockResolvedValue({
      ok: true, json: async () => ({ data: [{ b64_json: 'QUJD' }] }),
    })
    vi.stubGlobal('fetch', fetcher)
    return fetcher
  }

  it.each(['1K', '2K', '4K'] as const)('sends Gemini %s independently of quality', async resolution => {
    const fetcher = mockFetch()
    await generateImages({ ...options, platform: 'gemini', model: 'gemini-3-pro-image', resolution, quality: 'high' })
    expect(fetcher).toHaveBeenCalledWith('/v1beta/models/gemini-3-pro-image:generateContent', expect.anything())
    expect(JSON.parse(fetcher.mock.calls[0]![1].body).generationConfig.imageConfig).toEqual({
      aspectRatio: '16:9', imageSize: resolution,
    })
  })

  it('sends OpenAI 4K dimensions and preserves the independently selected quality', async () => {
    const fetcher = mockFetch()
    await generateImages({ ...options, resolution: '4K', quality: 'standard' })
    expect(JSON.parse(fetcher.mock.calls[0]![1].body)).toMatchObject({
      size: '3840x2160', quality: 'medium',
    })
    await generateImages({ ...options, resolution: '2K', quality: 'high' })
    expect(JSON.parse(fetcher.mock.calls[1]![1].body)).toMatchObject({
      size: '2048x1152', quality: 'high',
    })
  })

  it('keeps 1K requests inside the 1K billing tier and uses a valid ratio for stale drafts', async () => {
    const fetcher = mockFetch()
    await generateImages({ ...options, resolution: '1K', aspectRatio: '4:3' })
    expect(JSON.parse(fetcher.mock.calls[0]![1].body).size).toBe('1024x768')
    await generateImages({ ...options, resolution: '1K', aspectRatio: '16:9' })
    expect(JSON.parse(fetcher.mock.calls[1]![1].body).size).toBe('1024x1024')
  })

  it('uses the same resolution and quality fields for image editing', async () => {
    const fetcher = mockFetch()
    const file = new File(['reference'], 'reference.png', { type: 'image/png' })
    await generateImages({ ...options, resolution: '4K', referenceImages: [file] })
    const [path, request] = fetcher.mock.calls[0]!
    expect(path).toBe('/v1/images/edits')
    expect(request.body).toBeInstanceOf(FormData)
    expect(request.body.get('size')).toBe('3840x2160')
    expect(request.body.get('quality')).toBe('medium')
    expect(request.body.get('image').name).toBe('reference.png')
  })

  it('does not send unsupported 4K to Grok or configurable size to legacy Gemini', async () => {
    const fetcher = mockFetch()
    await generateImages({ ...options, platform: 'grok', model: 'grok-imagine-image', resolution: '4K' })
    expect(JSON.parse(fetcher.mock.calls[0]![1].body)).toMatchObject({ resolution: '2k', aspect_ratio: '16:9' })
    await generateImages({ ...options, platform: 'gemini', model: 'gemini-2.5-flash-image', resolution: '4K' })
    expect(JSON.parse(fetcher.mock.calls[1]![1].body).generationConfig.imageConfig).toEqual({ aspectRatio: '16:9' })
  })
})


describe('image generation progress and task protocol', () => {
  const options: GenerateImageOptions = {
    apiKey: 'original-key', platform: 'openai', model: 'gpt-image-2',
    prompt: 'A forest', aspectRatio: '1:1', resolution: '2K', quality: 'high', count: 3, referenceImages: [],
  }
  const jsonResponse = (body: unknown, status = 200, headers: HeadersInit = {}) => new Response(JSON.stringify(body), { status, headers })

  const moderationError = {
    message: 'Your request was rejected by the safety system. safety_violations=[sexual].',
    type: 'image_generation_user_error', code: 'moderation_blocked',
    moderation_details: { moderation_stage: 'output', categories: ['sexual'] },
  }

  it('surfaces a synchronous HTTP 200 moderation error instead of an empty-image message', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ error: moderationError }))
    vi.stubGlobal('fetch', fetcher)
    await expect(generateImages(options)).rejects.toMatchObject({ message: moderationError.message, kind: 'confirmed' })
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('recognizes an error inside a legacy completed task as a failed generation', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({
      id: 'imgtask_blocked', status: 'completed', http_status: 200, result: { error: moderationError },
    }))
    vi.stubGlobal('fetch', fetcher)
    expect(await getImageTask('original-key', 'imgtask_blocked')).toMatchObject({
      id: 'imgtask_blocked', status: 'failed', images: [], error: moderationError.message,
    })
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('returns a failed task with its error instead of treating it as a failed poll', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      id: 'imgtask_blocked', status: 'failed', http_status: 400, error: moderationError,
    })))
    expect(await getImageTask('original-key', 'imgtask_blocked')).toMatchObject({
      id: 'imgtask_blocked', status: 'failed', images: [], error: moderationError.message,
    })
  })

  it('exposes the existing capability policies and normalizes the same values used by snapshots', () => {
    expect(resolveImageCapabilities('grok', 'grok-imagine-image')).toMatchObject({
      resolutions: ['1K', '2K'], supportsQuality: false, knownModel: true,
      maxReferenceImages: 4, maxReferenceImageBytes: 10 * 1024 * 1024, maxPromptLength: 4000,
    })
    expect(resolveImageCapabilities('openai', 'my-custom-alias').knownModel).toBe(false)
    expect(normalizeImageRequest({ ...options, platform: 'grok', model: ' models/grok-imagine-image ', resolution: '4K', prompt: '  forest  ' }))
      .toMatchObject({ model: 'grok-imagine-image', resolution: '2K', prompt: 'forest', quality: 'high' })
  })

  it('publishes each Gemini response before making another POST and retains it on later failure', async () => {
    const received: string[] = []
    const requests: number[] = []
    const fetcher = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ data: [{ b64_json: 'FIRST' }] }))
      .mockImplementationOnce(async () => {
        expect(received).toEqual(['data:image/png;base64,FIRST'])
        return jsonResponse({ error: { message: 'second failed' } }, 400)
      })
    vi.stubGlobal('fetch', fetcher)
    await expect(generateImages({
      ...options, platform: 'gemini', model: 'gemini-3-pro-image',
      onRequest: count => { requests.push(count) },
      onImages: async images => { received.push(...images.map(image => image.src)) },
    })).rejects.toMatchObject({ message: 'second failed', kind: 'confirmed' })
    expect(fetcher).toHaveBeenCalledTimes(2)
    expect(requests).toEqual([1, 1])
    expect(received).toEqual(['data:image/png;base64,FIRST'])
  })

  it('stops a serial Gemini batch on empty images without losing the preceding callback', async () => {
    const onImages = vi.fn()
    const fetcher = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ data: [{ b64_json: 'FIRST' }] }))
      .mockResolvedValueOnce(jsonResponse({ candidates: [] }))
    vi.stubGlobal('fetch', fetcher)
    await expect(generateImages({ ...options, platform: 'gemini', model: 'gemini-3-pro-image', onImages }))
      .rejects.toMatchObject({ message: 'imageGeneration.noImageReturned', kind: 'confirmed' })
    expect(onImages).toHaveBeenCalledTimes(1)
    expect(fetcher).toHaveBeenCalledTimes(2)
  })

  it('does not start another serial request after its listener is aborted', async () => {
    const controller = new AbortController()
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ data: [{ b64_json: 'FIRST' }] }))
    vi.stubGlobal('fetch', fetcher)
    await expect(generateImages({
      ...options, platform: 'gemini', model: 'gemini-3-pro-image', signal: controller.signal,
      onImages: () => { controller.abort() },
    })).rejects.toMatchObject({ name: 'AbortError' })
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('keeps OpenAI multiple output generation in one POST', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ data: [{ b64_json: 'ONE' }, { b64_json: 'TWO' }] }))
    vi.stubGlobal('fetch', fetcher)
    const onImages = vi.fn()
    const onRequest = vi.fn()
    const result = await generateImages({ ...options, count: 2, onImages, onRequest })
    expect(result).toHaveLength(2)
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(JSON.parse(fetcher.mock.calls[0]![1].body).n).toBe(2)
    expect(onImages).toHaveBeenCalledTimes(1)
    expect(onRequest).toHaveBeenCalledWith(2)
  })

  it('classifies a lost synchronous response as an unknown outcome without retrying', async () => {
    const fetcher = vi.fn().mockRejectedValue(new TypeError('network lost'))
    vi.stubGlobal('fetch', fetcher)
    await expect(generateImages(options)).rejects.toMatchObject({ kind: 'unknown' })
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('treats a successful HTTP response with a lost body as unknown instead of confirmed failure', async () => {
    const fetcher = vi.fn().mockResolvedValue({ ok: true, json: async () => { throw new TypeError('body lost') } })
    vi.stubGlobal('fetch', fetcher)
    await expect(generateImages(options)).rejects.toMatchObject({ message: 'imageGeneration.invalidResponse', kind: 'unknown' })
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it.each([false, true])('uses explicit async capability %s from a read-only request', async enabled => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ async_tasks: enabled }))
    vi.stubGlobal('fetch', fetcher)
    expect(await readImageAsyncCapability('original-key', 'openai')).toBe(enabled)
    expect(fetcher).toHaveBeenCalledWith('/v1/images/capabilities', expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer original-key' }),
    }))
    expect(fetcher.mock.calls[0]![1].method).toBeUndefined()
  })

  it('treats a missing capability route as sync and never probes composite or Gemini groups', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ message: 'not found' }, 404))
    vi.stubGlobal('fetch', fetcher)
    expect(await readImageAsyncCapability('key', 'openai')).toBe(false)
    for (const platform of ['composite', 'gemini', 'antigravity'] as const) {
      expect(await readImageAsyncCapability('key', platform)).toBe(false)
    }
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('propagates capability authorization/network errors instead of choosing another submission path', async () => {
    const fetcher = vi.fn().mockResolvedValueOnce(jsonResponse({ error: { message: 'invalid key' } }, 401))
      .mockRejectedValueOnce(new TypeError('offline'))
    vi.stubGlobal('fetch', fetcher)
    await expect(readImageAsyncCapability('key', 'grok')).rejects.toBeInstanceOf(ImageGenerationError)
    await expect(readImageAsyncCapability('key', 'grok')).rejects.toThrow('offline')
  })

  it('uses the same normalized JSON payload for async submission and keeps task identity and Retry-After', async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ data: [{ b64_json: 'ONE' }] }))
      .mockResolvedValueOnce(jsonResponse({ task_id: 'imgtask_one', status: 'processing' }, 202, { 'Retry-After': '7' }))
    vi.stubGlobal('fetch', fetcher)
    await generateImages(options)
    const task = await submitImageTask(options)
    expect(fetcher.mock.calls[1]![0]).toBe('/v1/images/generations/async')
    expect(fetcher.mock.calls[1]![1].body).toBe(fetcher.mock.calls[0]![1].body)
    expect(task).toMatchObject({ id: 'imgtask_one', status: 'processing', retryAfterMs: 7000 })
  })

  it('preserves multipart reference order in async edits', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse({ id: 'imgtask_one', status: 'processing' }, 202))
    vi.stubGlobal('fetch', fetcher)
    await submitImageTask({ ...options, referenceImages: [new File(['a'], 'a.png'), new File(['b'], 'b.png')] })
    expect(fetcher.mock.calls[0]![0]).toBe('/v1/images/edits/async')
    const body = fetcher.mock.calls[0]![1].body as FormData
    expect(body.get('quality')).toBe('high')
    expect(body.get('size')).toBe('2048x2048')
    expect(body.getAll('image').map(file => (file as File).name)).toEqual(['a.png', 'b.png'])
  })

  it('queries only the identified task using the original key and accepts only real server statuses', async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ task_id: 'imgtask_one', status: 'completed', result: { data: [{ url: 'https://image.example/result.png' }] } }))
      .mockResolvedValueOnce(jsonResponse({ task_id: 'imgtask_one', status: 'saving' }))
      .mockResolvedValueOnce(jsonResponse({ task_id: 'another-task', status: 'processing' }))
    vi.stubGlobal('fetch', fetcher)
    expect(await getImageTask('original-key', 'imgtask_one')).toMatchObject({ status: 'completed', images: [{ src: 'https://image.example/result.png' }] })
    expect(fetcher.mock.calls[0]).toEqual(['/v1/images/tasks/imgtask_one', expect.objectContaining({ headers: expect.objectContaining({ Authorization: 'Bearer original-key' }) })])
    await expect(getImageTask('original-key', 'imgtask_one')).rejects.toMatchObject({ message: 'imageGeneration.invalidTaskResponse' })
    await expect(getImageTask('original-key', 'imgtask_one')).rejects.toMatchObject({ message: 'imageGeneration.invalidTaskResponse' })
    expect(fetcher.mock.calls.every(([, init]) => !init.method)).toBe(true)
  })

  it('does not repeat a POST or fall back to sync after losing the async response', async () => {
    const fetcher = vi.fn().mockRejectedValue(new TypeError('lost response'))
    vi.stubGlobal('fetch', fetcher)
    await expect(submitImageTask(options)).rejects.toMatchObject({ kind: 'unknown' })
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(fetcher.mock.calls[0]![0]).toBe('/v1/images/generations/async')
  })
})
