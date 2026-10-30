import { buildGatewayUrl } from './client'
import type { GroupPlatform } from '@/types'

export type VideoProtocol = 'grok' | 'seedance'
export type VideoAspectRatio = '16:9' | '9:16' | '1:1' | '4:3' | '3:4' | '21:9'
export type VideoResolution = '480p' | '720p' | '1080p'

export interface GenerateVideoOptions {
  apiKey: string
  platform: GroupPlatform
  model: string
  prompt: string
  duration: number
  aspectRatio: VideoAspectRatio
  resolution: VideoResolution
  referenceImage?: File | null
  signal?: AbortSignal
}

export interface VideoTask {
  id: string
  protocol: VideoProtocol
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  videoUrl?: string
  error?: string
}

const videoRatios: VideoAspectRatio[] = ['16:9', '9:16', '1:1', '4:3', '3:4', '21:9']
const imageTypes = new Set(['image/png', 'image/jpeg', 'image/webp'])

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function normalizedModel(model: string): string {
  return model.trim().toLowerCase().replace(/^(?:xai|x-ai|grok)\//, '')
}

export function videoProtocolForModel(model: string): VideoProtocol | null {
  const value = normalizedModel(model)
  if (/^grok-(?:imagine-)?video(?:[.-]|$)/.test(value)) return 'grok'
  if (/^(?:doubao-)?seedance(?:[.-]|$)/.test(value)) return 'seedance'
  return null
}

export function supportsVideoGeneration(platform: GroupPlatform | undefined): boolean {
  return platform === 'grok' || platform === 'openai' || platform === 'composite'
}

function supportsProtocol(platform: GroupPlatform, protocol: VideoProtocol): boolean {
  return platform === 'composite' || platform === (protocol === 'grok' ? 'grok' : 'openai')
}

export function supportedVideoResolutions(model: string): VideoResolution[] {
  const protocol = videoProtocolForModel(model)
  if (protocol === 'grok') {
    return videoRequiresReference(model) ? ['480p', '720p', '1080p'] : ['480p', '720p']
  }
  // Ark validates the native options against the selected Seedance model.
  return protocol === 'seedance' ? ['480p', '720p', '1080p'] : []
}

export function videoRequiresReference(model: string): boolean {
  return /^grok-(?:imagine-)?video-1\.5(?:[.-]|$)/.test(normalizedModel(model))
}

function authHeaders(apiKey: string): Record<string, string> {
  return { Accept: 'application/json', Authorization: `Bearer ${apiKey}` }
}

async function responseJSON(response: Response): Promise<unknown> {
  let payload: unknown = null
  try {
    payload = await response.json()
  } catch {
    // Non-JSON proxy errors still expose their HTTP status.
  }
  if (response.ok) return payload

  const body = asRecord(payload)
  const error = asRecord(body?.error)
  throw new Error(asString(error?.message) || asString(body?.message) ||
    asString(body?.error) || response.statusText || `HTTP ${response.status}`)
}

export async function listVideoModels(apiKey: string, platform: GroupPlatform): Promise<string[]> {
  if (!supportsVideoGeneration(platform)) return []
  const response = await fetch(buildGatewayUrl('/v1/models'), { headers: authHeaders(apiKey) })
  const body = asRecord(await responseJSON(response))
  const rows = Array.isArray(body?.data) ? body.data : Array.isArray(body?.models) ? body.models : []
  const seen = new Set<string>()
  return rows.flatMap(row => {
    const item = asRecord(row)
    const model = (asString(row) || asString(item?.id) || asString(item?.name)).replace(/^models\//, '')
    const protocol = videoProtocolForModel(model)
    if (!protocol || !supportsProtocol(platform, protocol) || seen.has(model)) return []
    seen.add(model)
    return [model]
  })
}

function taskFromResponse(payload: unknown, protocol: VideoProtocol, id = '', creating = false): VideoTask {
  const root = asRecord(payload)
  const body = asRecord(root?.data) || root
  const video = asRecord(body?.video)
  const content = asRecord(body?.content)
  const taskID = asString(body?.request_id) || asString(body?.id) || asString(video?.request_id) ||
    asString(video?.id) || asString(body?.task_id) || asString(video?.task_id) || id
  if (!taskID) throw new Error('Video response is missing a task ID')

  const videoUrl = protocol === 'seedance'
    ? asString(content?.video_url)
    : asString(video?.url) || asString(body?.url) || asString(body?.video_url) || asString(body?.download_url)
  const rawStatus = asString(body?.status).toLowerCase()
  let status: VideoTask['status']
  switch (rawStatus) {
    case 'pending':
    case 'queued':
      status = 'queued'
      break
    case 'running':
    case 'processing':
    case 'in_progress':
      status = 'running'
      break
    case 'done':
    case 'completed':
    case 'succeeded':
    case 'success':
      status = 'succeeded'
      break
    case 'failed':
    case 'error':
    case 'expired':
      status = 'failed'
      break
    case 'cancelled':
    case 'canceled':
      status = 'cancelled'
      break
    default:
      if (!rawStatus && creating) status = 'queued'
      else throw new Error(rawStatus ? `Unknown video task status: ${rawStatus}` : 'Video response is missing a task status')
  }
  const error = asString(asRecord(body?.error)?.message) || asString(body?.error) ||
    (status === 'failed' ? asString(body?.message) || (rawStatus === 'expired' ? 'Video generation expired' : 'Video generation failed') : '')
  return {
    id: taskID,
    protocol,
    status,
    ...(videoUrl ? { videoUrl } : {}),
    ...(error ? { error } : {}),
  }
}

function fileDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error || new Error('Failed to read reference image'))
    reader.onload = () => resolve(String(reader.result || ''))
    reader.readAsDataURL(file)
  })
}

export async function createVideoTask(options: GenerateVideoOptions): Promise<VideoTask> {
  options.signal?.throwIfAborted()
  const protocol = videoProtocolForModel(options.model)
  if (!protocol || !supportsProtocol(options.platform, protocol)) {
    throw new Error('This model does not support video generation in the selected group')
  }
  if (!options.apiKey.trim()) throw new Error('An API key is required')
  if (!options.prompt.trim()) throw new Error('A video prompt is required')
  if (!Number.isInteger(options.duration) || options.duration <= 0 || protocol === 'grok' && options.duration > 15) {
    throw new Error(protocol === 'grok' ? 'Grok video duration must be 1–15 seconds' : 'Video duration must be a positive whole number')
  }
  if (!supportedVideoResolutions(options.model).includes(options.resolution)) {
    throw new Error('The selected resolution is not supported by this video model')
  }
  if (!videoRatios.includes(options.aspectRatio)) throw new Error('The selected video aspect ratio is not supported')
  const image = options.referenceImage
  if (image && (!imageTypes.has(image.type) || image.size > 10 * 1024 * 1024)) {
    throw new Error('Use a PNG, JPG, or WebP reference image no larger than 10 MB')
  }
  if (videoRequiresReference(options.model) && !image) {
    throw new Error('Grok video 1.5 requires a reference image')
  }
  const imageURL = image ? await fileDataURL(image) : undefined
  options.signal?.throwIfAborted()
  const body = protocol === 'grok'
    ? {
      model: options.model.trim(),
      prompt: options.prompt.trim(),
      duration: options.duration,
      aspect_ratio: options.aspectRatio,
      resolution: options.resolution,
      ...(imageURL ? { image: { url: imageURL, type: 'image_url' } } : {}),
    }
    : {
      model: options.model.trim(),
      content: [
        { type: 'text', text: options.prompt.trim() },
        ...(imageURL ? [{ type: 'image_url', image_url: { url: imageURL }, role: 'first_frame' }] : []),
      ],
      duration: options.duration,
      ratio: options.aspectRatio,
      resolution: options.resolution,
    }
  const path = protocol === 'grok' ? '/v1/videos/generations' : '/api/v3/contents/generations/tasks'
  const response = await fetch(buildGatewayUrl(path), {
    method: 'POST',
    headers: { ...authHeaders(options.apiKey), 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    redirect: 'error',
    signal: options.signal,
  })
  return taskFromResponse(await responseJSON(response), protocol, '', true)
}

export async function getVideoTask(apiKey: string, protocol: VideoProtocol, id: string, signal?: AbortSignal): Promise<VideoTask> {
  if (!id.trim()) throw new Error('A video task ID is required')
  signal?.throwIfAborted()
  const controller = new AbortController()
  const abort = () => controller.abort(signal?.reason)
  signal?.addEventListener('abort', abort, { once: true })
  const timeout = setTimeout(() => controller.abort(new Error('videoGeneration.pollingTimeout')), 60_000)
  const prefix = protocol === 'grok' ? '/v1/videos' : '/api/v3/contents/generations/tasks'
  try {
    const response = await fetch(buildGatewayUrl(`${prefix}/${encodeURIComponent(id)}`), {
      headers: authHeaders(apiKey),
      signal: controller.signal,
    })
    const payload = await responseJSON(response)
    controller.signal.throwIfAborted()
    return taskFromResponse(payload, protocol, id)
  } catch (error) {
    controller.signal.throwIfAborted()
    throw error
  } finally {
    clearTimeout(timeout)
    signal?.removeEventListener('abort', abort)
  }
}

export function videoContentURL(task: VideoTask): string {
  if (task.status !== 'succeeded' || !task.id.trim()) throw new Error('The video is not ready for download')
  if (task.protocol === 'grok') {
    // Only our fixed task endpoint receives the key; returned URLs are untrusted.
    return buildGatewayUrl(`/v1/videos/${encodeURIComponent(task.id)}/content`)
  }
  const videoURL = task.videoUrl?.trim()
  if (!videoURL) throw new Error('The video response is missing a download URL')
  let mediaURL: URL
  try {
    const gatewayURL = new URL(buildGatewayUrl('/'), window.location.href)
    mediaURL = new URL(videoURL, gatewayURL)
  } catch {
    throw new Error('The video response contains an unsupported download URL')
  }
  if (!['https:', 'http:'].includes(mediaURL.protocol) || mediaURL.username || mediaURL.password) {
    throw new Error('The video response contains an unsupported download URL')
  }
  return mediaURL.href
}

export async function fetchVideoBlob(apiKey: string, task: VideoTask, signal?: AbortSignal): Promise<Blob> {
  const url = videoContentURL(task)
  const headers: Record<string, string> = { Accept: 'video/*, application/octet-stream' }
  if (task.protocol === 'grok') headers.Authorization = `Bearer ${apiKey}`
  const response = await fetch(url, {
    headers,
    credentials: 'omit',
    redirect: 'error',
    referrerPolicy: 'no-referrer',
    signal,
  })
  if (!response.ok) await responseJSON(response)
  const blob = await response.blob()
  if (!blob.size) throw new Error('The downloaded video is empty')
  const mimeType = blob.type.split(';')[0]?.toLowerCase() || ''
  if (mimeType.startsWith('video/')) return blob
  if (!mimeType || ['application/octet-stream', 'binary/octet-stream', 'application/mp4'].includes(mimeType)) {
    return new Blob([blob], { type: /\.webm(?:[?#]|$)/i.test(task.videoUrl || '') ? 'video/webm' : 'video/mp4' })
  }
  throw new Error('The download did not return a video')
}
