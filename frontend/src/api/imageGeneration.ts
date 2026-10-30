import { buildGatewayUrl } from './client'
import type { GroupPlatform } from '@/types'

export type ImageQuality = 'standard' | 'high'
export type ImageResolution = '1K' | '2K' | '4K'
export type ImageAspectRatio = 'auto' | '21:9' | '16:9' | '3:2' | '4:3' | '1:1' | '3:4' | '2:3' | '9:16'

export interface GeneratedImage {
  src: string
  revisedPrompt?: string
}

export interface GenerateImageOptions {
  apiKey: string
  platform: GroupPlatform
  model: string
  prompt: string
  aspectRatio: ImageAspectRatio
  quality: ImageQuality
  resolution: ImageResolution
  count: number
  referenceImages: File[]
}

const supportedPlatforms = new Set<GroupPlatform>([
  'openai',
  'grok',
  'gemini',
  'antigravity',
  'composite',
])

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

export function supportsImageGeneration(platform: GroupPlatform | undefined): boolean {
  return platform ? supportedPlatforms.has(platform) : false
}

const fullAspectRatios: ImageAspectRatio[] = ['auto', '21:9', '16:9', '3:2', '4:3', '1:1', '3:4', '2:3', '9:16']

function supportsCustomImageSize(model: string): boolean {
  return /^gpt-image-2(?:[.-]|$)/i.test(model)
}

export function supportsImageQuality(platform: GroupPlatform | undefined, model: string): boolean {
  return platform === 'openai' || platform === 'composite' && !/gemini|imagen|^grok-/i.test(model)
}

export function supportedImageResolutions(platform: GroupPlatform | undefined, model: string): ImageResolution[] {
  if (!platform) return ['1K']
  if (usesGrokProtocol(platform, model)) return ['1K', '2K']
  if (usesGeminiProtocol(platform, model)) {
    return /gemini-2[.-]|flash-lite-image/i.test(model) ? ['1K'] : ['1K', '2K', '4K']
  }
  return supportsCustomImageSize(model) ? ['1K', '2K', '4K'] : ['1K']
}

export function supportedImageAspectRatios(
  platform: GroupPlatform | undefined,
  model: string,
  resolution: ImageResolution = '1K',
): ImageAspectRatio[] {
  if (supportsImageQuality(platform, model) && !supportsCustomImageSize(model)) {
    return ['auto', '3:2', '1:1', '2:3']
  }
  if (supportsImageQuality(platform, model) && supportsCustomImageSize(model)) {
    // Auto cannot preserve a chosen resolution; wide 1K ratios fall below the minimum pixel count.
    return fullAspectRatios.filter(ratio => ratio !== 'auto' &&
      (resolution !== '1K' || !['21:9', '16:9', '9:16'].includes(ratio)))
  }
  return fullAspectRatios
}

export function isLikelyImageModel(model: string): boolean {
  const value = model.trim().toLowerCase()
  if (!value || value.includes('video')) return false
  return value.startsWith('gpt-image-') ||
    value.startsWith('grok-imagine') ||
    value.includes('gemini') && value.includes('image')
}

function usesGeminiProtocol(platform: GroupPlatform, model: string): boolean {
  return platform === 'gemini' || platform === 'antigravity' ||
    platform === 'composite' && /gemini|imagen/i.test(model)
}

function usesGrokProtocol(platform: GroupPlatform, model: string): boolean {
  return platform === 'grok' || platform === 'composite' && /^grok-/i.test(model)
}

function authHeaders(apiKey: string): HeadersInit {
  return {
    Accept: 'application/json',
    Authorization: `Bearer ${apiKey}`,
  }
}

async function responseJSON(response: Response): Promise<unknown> {
  let payload: unknown = null
  try {
    payload = await response.json()
  } catch {
    // Keep the HTTP status as the useful fallback for non-JSON proxy errors.
  }
  if (response.ok) return payload

  const body = asRecord(payload)
  const error = asRecord(body?.error)
  const message = asString(error?.message) || asString(body?.message) || response.statusText
  throw new Error(message || `HTTP ${response.status}`)
}

function modelIDs(payload: unknown): string[] {
  const body = asRecord(payload)
  const rows = asArray(body?.data).length ? asArray(body?.data) : asArray(body?.models)
  const seen = new Set<string>()
  return rows.flatMap((row) => {
    const item = asRecord(row)
    const value = (typeof row === 'string' ? row : asString(item?.id) || asString(item?.name))
      .replace(/^models\//, '')
      .trim()
    if (!value || seen.has(value)) return []
    seen.add(value)
    return [value]
  })
}

export async function listImageModels(apiKey: string, platform: GroupPlatform): Promise<string[]> {
  const path = platform === 'antigravity' ? '/antigravity/models' : '/v1/models'
  const response = await fetch(buildGatewayUrl(path), { headers: authHeaders(apiKey) })
  const models = modelIDs(await responseJSON(response))
  return models.filter(isLikelyImageModel)
}

export function parseGeneratedImages(payload: unknown): GeneratedImage[] {
  const root = asRecord(payload)
  if (!root) return []

  const openAIImages = asArray(root.data).flatMap((row): GeneratedImage[] => {
    const item = asRecord(row)
    if (!item) return []
    const url = asString(item.url)
    const base64 = asString(item.b64_json)
    const src = url || (base64 ? `data:image/png;base64,${base64}` : '')
    return src ? [{ src, revisedPrompt: asString(item.revised_prompt) || undefined }] : []
  })
  if (openAIImages.length) return openAIImages

  const geminiRoot = asRecord(root.response) || root
  const text = asArray(geminiRoot.candidates)
    .flatMap(candidate => asArray(asRecord(asRecord(candidate)?.content)?.parts))
    .map(part => asString(asRecord(part)?.text))
    .filter(Boolean)
    .join('\n')

  return asArray(geminiRoot.candidates)
    .flatMap(candidate => asArray(asRecord(asRecord(candidate)?.content)?.parts))
    .flatMap((part): GeneratedImage[] => {
      const item = asRecord(part)
      const inline = asRecord(item?.inlineData) || asRecord(item?.inline_data)
      const data = asString(inline?.data)
      if (!data) return []
      const mimeType = asString(inline?.mimeType) || asString(inline?.mime_type) || 'image/png'
      return [{ src: `data:${mimeType};base64,${data}`, revisedPrompt: text || undefined }]
    })
}

function fileBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error || new Error('Failed to read image'))
    reader.onload = () => {
      const value = String(reader.result || '')
      resolve(value.includes(',') ? value.slice(value.indexOf(',') + 1) : value)
    }
    reader.readAsDataURL(file)
  })
}

async function generateGeminiImages(options: GenerateImageOptions): Promise<GeneratedImage[]> {
  const model = options.model.replace(/^models\//, '')
  const prefix = options.platform === 'antigravity' ? '/antigravity/v1beta' : '/v1beta'
  const parts: Array<Record<string, unknown>> = [{ text: options.prompt }]
  for (const file of options.referenceImages) {
    parts.push({
      inlineData: {
        mimeType: file.type,
        data: await fileBase64(file),
      },
    })
  }

  const images: GeneratedImage[] = []
  for (let index = 0; index < options.count; index += 1) {
    const response = await fetch(buildGatewayUrl(`${prefix}/models/${encodeURIComponent(model)}:generateContent`), {
      method: 'POST',
      headers: { ...authHeaders(options.apiKey), 'Content-Type': 'application/json' },
      body: JSON.stringify({
        contents: [{ role: 'user', parts }],
        generationConfig: {
          responseModalities: ['TEXT', 'IMAGE'],
          imageConfig: {
            ...(options.aspectRatio === 'auto' ? {} : { aspectRatio: options.aspectRatio }),
            ...(/gemini-2[.-]/i.test(model) ? {} : { imageSize: options.resolution }),
          },
        },
      }),
    })
    images.push(...parseGeneratedImages(await responseJSON(response)))
  }
  return images
}

export function openAIImageSize(aspectRatio: ImageAspectRatio, resolution: ImageResolution, model: string): string {
  if (supportsCustomImageSize(model)) {
    const [width, height] = (aspectRatio === 'auto' ? '1:1' : aspectRatio).split(':').map(Number)
    const ratio = Math.max(width!, height!) / Math.min(width!, height!)
    const edge = resolution === '4K' ? 3840 : resolution === '2K' ? 2048 : 1024
    // GPT Image caps both edges at 3840 and total pixels at 8,294,400.
    const longEdge = Math.floor(Math.min(edge, Math.sqrt(8_294_400 * ratio)) / 16) * 16
    const shortEdge = Math.floor(longEdge / ratio / 16) * 16
    return width! >= height! ? `${longEdge}x${shortEdge}` : `${shortEdge}x${longEdge}`
  }
  if (aspectRatio === 'auto') return 'auto'
  if (aspectRatio === '3:2') return '1536x1024'
  if (aspectRatio === '2:3') return '1024x1536'
  return '1024x1024'
}

async function generateOpenAIImages(options: GenerateImageOptions): Promise<GeneratedImage[]> {
  const grok = usesGrokProtocol(options.platform, options.model)
  const values: Record<string, string | number> = {
    model: options.model,
    prompt: options.prompt,
    n: options.count,
  }
  if (grok) {
    if (options.aspectRatio !== 'auto') values.aspect_ratio = options.aspectRatio
    values.resolution = options.resolution.toLowerCase()
  } else {
    values.size = openAIImageSize(options.aspectRatio, options.resolution, options.model)
    values.quality = options.quality === 'high' ? 'high' : 'medium'
  }

  let body: BodyInit
  let headers = authHeaders(options.apiKey)
  if (options.referenceImages.length) {
    const form = new FormData()
    Object.entries(values).forEach(([key, value]) => form.append(key, String(value)))
    options.referenceImages.forEach(file => form.append('image', file, file.name))
    body = form
  } else {
    headers = { ...headers, 'Content-Type': 'application/json' }
    body = JSON.stringify(values)
  }

  const endpoint = options.referenceImages.length ? '/v1/images/edits' : '/v1/images/generations'
  const response = await fetch(buildGatewayUrl(endpoint), { method: 'POST', headers, body })
  return parseGeneratedImages(await responseJSON(response))
}

export async function generateImages(options: GenerateImageOptions): Promise<GeneratedImage[]> {
  const resolutions = supportedImageResolutions(options.platform, options.model)
  const resolution = resolutions.includes(options.resolution) ? options.resolution : resolutions[resolutions.length - 1]!
  const ratios = supportedImageAspectRatios(options.platform, options.model, resolution)
  const request = {
    ...options,
    resolution,
    aspectRatio: ratios.includes(options.aspectRatio) ? options.aspectRatio : '1:1' as const,
  }
  return usesGeminiProtocol(request.platform, request.model)
    ? generateGeminiImages(request)
    : generateOpenAIImages(request)
}
