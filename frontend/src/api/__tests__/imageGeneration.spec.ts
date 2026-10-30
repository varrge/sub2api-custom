import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  generateImages,
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
