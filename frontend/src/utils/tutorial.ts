import DOMPurify from 'dompurify'
import { marked } from 'marked'
import type { ApiKey, ApiKeyModelOptions, GroupPlatform } from '@/types'
import { modelPolicyAllows } from '@/utils/modelKeyPolicy'

export const tutorialPlatforms: GroupPlatform[] = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'typesafe', 'command_code', 'cline', 'composite']
export const tutorialClients = ['claude', 'codex', 'codex-ws', 'gemini', 'grok', 'opencode', 'systemone']
export const safeConfigModel = (value: string) => /^[a-zA-Z0-9][a-zA-Z0-9._:/+-]{0,255}$/.test(value)
export const safeTutorialEndpoint = (value: string) => /^https?:\/\/[a-zA-Z0-9.\-:[\]]+(\/[a-zA-Z0-9._~%/-]*)?$/.test(value)

// The permission catalog also contains models served by media/audio endpoints.
// Those cannot be used as the conversation model in a coding client.
export function tutorialClientModelAllowed(model: string, client: string, platform: string): boolean {
  const id = model.split('/').pop()!.toLowerCase()
  if (/^(gpt-image-|grok-imagine|imagen[-_]|dall-e|sora|veo|doubao-seedance|seedance|text-embedding-|whisper|tts[-_])/.test(id) ||
    /gemini.*image|embedding|rerank/.test(id) ||
    (id.startsWith('gpt-') && /audio|realtime|transcribe|tts/.test(id)) ||
    id.startsWith('grok-voice')) return false
  if (platform === 'antigravity') {
    if (client === 'gemini') return id.startsWith('gemini')
    if (client === 'claude') return !id.startsWith('gemini')
  }
  return true
}

export function tutorialKeyUsable(key: { status: string; expires_at: string | null } & Partial<Pick<ApiKey, 'quota' | 'quota_used'>>): boolean {
  return key.status === 'active' && (!key.expires_at || Date.parse(key.expires_at) > Date.now()) &&
    (!key.quota || (key.quota_used ?? 0) < key.quota)
}
export function tutorialModels(catalog: ApiKeyModelOptions, groupID: number, key?: ApiKey | null): string[] {
  if (key && (!tutorialKeyUsable(key) || !(key.group_ids ?? [key.group_id]).includes(groupID))) return []
  return [...new Set(catalog.models.filter(m => m.group_ids.includes(groupID) && safeConfigModel(m.id) &&
    (!key || ((!key.model_allowlist || modelPolicyAllows(key.model_allowlist, m.id)) &&
      (!key.model_group_routes || !Object.prototype.hasOwnProperty.call(key.model_group_routes, m.id) || key.model_group_routes[m.id].includes(groupID)))))
    .map(m => m.id))].sort()
}
export interface TutorialPreset { platform: string; group: number | null; model: string; client: string; os: string; route: string }
export function tutorialShareURL(origin: string, preset: TutorialPreset): string {
  const url = new URL('/tutorial', origin)
  // Explicit allowlist: never copy the page query, a selected key ID, or a secret.
  for (const name of ['platform', 'group', 'model', 'client', 'os', 'route'] as const) {
    if (preset[name] !== null && preset[name] !== '') url.searchParams.set(name, String(preset[name]))
  }
  return url.toString()
}
export function renderTutorialMarkdown(markdown: string): string {
  const clean = DOMPurify.sanitize(marked.parse(markdown, { async: false }) as string, {
    ALLOWED_TAGS: ['h1','h2','h3','h4','p','br','strong','em','del','ul','ol','li','blockquote','pre','code','hr','a','img','table','thead','tbody','tr','th','td'],
    ALLOWED_ATTR: ['href','src','alt','title']
  })
  const document = new DOMParser().parseFromString(clean, 'text/html')
  document.querySelectorAll('a').forEach(node => {
    const href = node.getAttribute('href') || ''
    if (!/^https?:\/\//i.test(href) && !href.startsWith('#')) node.removeAttribute('href')
    node.setAttribute('target', '_blank'); node.setAttribute('rel', 'noopener noreferrer')
  })
  document.querySelectorAll('img').forEach(node => {
    const src = node.getAttribute('src') || ''
    if (!/^https?:\/\//i.test(src) && !/^data:image\/(png|jpeg|webp|gif);base64,[A-Za-z0-9+/=]+$/.test(src)) node.remove()
    else { node.setAttribute('loading', 'lazy'); node.setAttribute('referrerpolicy', 'no-referrer') }
  })
  return document.body.innerHTML
}
