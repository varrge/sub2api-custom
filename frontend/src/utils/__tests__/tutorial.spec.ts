import { describe, expect, it } from 'vitest'
import { renderTutorialMarkdown, safeConfigModel, safeTutorialEndpoint, tutorialClientModelAllowed, tutorialKeyUsable, tutorialModels, tutorialShareURL } from '../tutorial'
import type { ApiKey } from '@/types'
const key = { id: 1, status: 'active', expires_at: null, group_ids: [1, 2], model_allowlist: { enabled: false } } as ApiKey
const catalog = { models: [{ id: 'model-a', group_ids: [1,2] }, { id: 'model-b', group_ids: [2] }] }
describe('tutorial permissions and sharing', () => {
  it('intersects group models, key bindings, allow/deny policy and per-model routes', () => {
    expect(tutorialModels(catalog,1,key)).toEqual(['model-a'])
    expect(tutorialModels(catalog,3,key)).toEqual([])
    expect(tutorialModels(catalog,2,{ ...key, model_allowlist: { enabled: true, mode: 'allow', models: ['model-b'] } })).toEqual(['model-b'])
    expect(tutorialModels(catalog,2,{ ...key, model_allowlist: { enabled: true, mode: 'deny', models: ['model-b'] } })).toEqual(['model-a'])
    expect(tutorialModels(catalog,2,{ ...key, model_group_routes: { 'model-a': [], 'model-b': [1] } })).toEqual([])
    expect(tutorialModels(catalog,2,{ ...key, status: 'inactive' })).toEqual([])
    expect(tutorialKeyUsable({ ...key, expires_at: '2001-01-01' })).toBe(false)
    expect(tutorialKeyUsable({ ...key, quota: 1, quota_used: 1 })).toBe(false)
    expect(tutorialKeyUsable({ ...key, expires_at: 'invalid' })).toBe(false)
  })
  it('excludes dedicated media models and respects mixed-platform client protocols', () => {
    for (const model of ['gpt-image-2', 'models/gemini-3-pro-image-preview', 'grok-imagine-video', 'doubao-seedance-1-5-pro', 'text-embedding-3-large', 'gpt-4o-mini-tts', 'gpt-4o-realtime-preview', 'gpt-audio', 'gpt-4o-transcribe', 'grok-voice-latest']) {
      expect(tutorialClientModelAllowed(model, 'codex', 'openai')).toBe(false)
    }
    expect(tutorialClientModelAllowed('gpt-6.1-sol', 'codex', 'openai')).toBe(true)
    expect(tutorialClientModelAllowed('claude-sonnet-4-6', 'gemini', 'antigravity')).toBe(false)
    expect(tutorialClientModelAllowed('gemini-2.5-pro', 'claude', 'antigravity')).toBe(false)
    expect(tutorialClientModelAllowed('gemini-2.5-pro', 'gemini', 'antigravity')).toBe(true)
  })
  it('shares only selections, never a key or an inherited query string', () => {
    const preset = { platform: 'openai', group: 2, model: 'model-b', client: 'codex', os: 'windows', route: 'cn', key_id: 123, apiKey: 'sk-secret' }
    const url = tutorialShareURL('https://example.com/tutorial?apiKey=sk-other', preset)
    expect(url).toContain('group=2')
    expect(url).not.toMatch(/sk-secret|sk-other|key_id|apiKey/)
  })
  it('rejects command metacharacters in model IDs and endpoints', () => {
    for (const value of ['a"b', 'a`whoami`', 'a$(id)', 'a\nb', 'a;b']) expect(safeConfigModel(value)).toBe(false)
    expect(safeConfigModel('org/model-v1.2:preview')).toBe(true)
    for (const value of ['javascript:foo','https://u:p@x.com','https://x.com?key=sk','https://x.com/$(id)']) expect(safeTutorialEndpoint(value)).toBe(false)
    expect(safeTutorialEndpoint('https://api.example.com/v1')).toBe(true)
  })
  it('renders Markdown and raster images without scripts, iframe, SVG or active links', () => {
    const html = renderTutorialMarkdown('# Guide\n\n![ok](data:image/png;base64,aA==)\n\n<img src="x" onerror="alert(1)"><script>alert(1)</script><iframe src="https://x"></iframe><svg onload="alert(1)"></svg><a href="javascript:alert(1)">bad</a>\n\n[docs](https://example.com)')
    expect(html).toContain('<h1>Guide</h1>')
    expect(html).toContain('data:image/png')
    expect(html).toContain('noopener noreferrer')
    expect(html).not.toMatch(/<script|<iframe|<svg|onerror|onload|javascript:/)
  })
})
