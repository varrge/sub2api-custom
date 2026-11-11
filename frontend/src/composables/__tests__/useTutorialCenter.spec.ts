import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, reactive } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { ApiKey, Group } from '@/types'
const mocks = vi.hoisted(() => ({ get: vi.fn(), groups: vi.fn(), list: vi.fn(), key: vi.fn(), models: vi.fn(), copy: vi.fn(), save: vi.fn(), query: {} as Record<string,string> }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: mocks.query }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: { api_base_url: 'https://api.example.com' } }) }))
vi.mock('@/api/tutorial', () => ({ tutorialAPI: { get: mocks.get } }))
vi.mock('@/api/keys', () => ({ keysAPI: { getById: mocks.key, getModelOptions: mocks.models } }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getAvailable: mocks.groups } }))
vi.mock('@/api/modelKeyAccess', () => ({ modelKeyAccessAPI: { list: mocks.list } }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: mocks.copy }) }))
vi.mock('file-saver', () => ({ saveAs: mocks.save }))
import { useTutorialCenter } from '../useTutorialCenter'
const groups = [{ id: 1, name: 'OpenAI', platform: 'openai' }, { id: 2, name: 'Gemini', platform: 'gemini' }] as Group[]
const key = (id: number) => ({ id, name: 'test', key: `sk-test-${id}`, status: 'active', expires_at: null, group_ids: [1,2], groups } as ApiKey)
let wrapper: VueWrapper, d: ReturnType<typeof setup>
function setup() { return reactive(useTutorialCenter()) }
async function start() {
  wrapper = mount(defineComponent({ setup() { d = setup(); return () => null } }))
  await flushPromises()
}
afterEach(() => { wrapper?.unmount() })
beforeEach(() => {
  vi.clearAllMocks(); mocks.query = {}
  mocks.get.mockResolvedValue({ revision: 1, articles: [], routes: [] })
  mocks.groups.mockResolvedValue(groups)
  mocks.list.mockResolvedValue({ keys: [key(1),key(2)] })
  mocks.key.mockImplementation(async (id: number) => key(id))
  mocks.models.mockImplementation(async (ids: number[]) => ({ models: [{ id: ids[0] === 1 ? 'gpt-test' : 'gemini-test', group_ids: ids }, { id: 'gpt-image-2', group_ids: ids }] }))
  mocks.copy.mockResolvedValue(true)
})
describe('tutorial workbench', () => {
  it('restores a shared preset and never includes key material in a shared link', async () => {
    mocks.query = { key_id: '1', platform: 'gemini', group: '2', client: 'gemini', os: 'powershell', model: 'gemini-test' }
    await start()
    expect(d.groupID).toBe(2); expect(d.model).toBe('gemini-test'); expect(d.config.activeTab).toBe('powershell')
    expect(d.ready).toBe(true); expect(d.files[0].content).toContain('YOUR_API_KEY')
    d.revealKey = true
    expect(d.files[0].content).toContain('sk-test-1')
    await d.share()
    expect(mocks.copy.mock.calls[0][0]).not.toMatch(/sk-test|key_id/)
    expect(mocks.copy.mock.calls[0][0]).toContain('os=powershell')
    d.download(); expect(mocks.save.mock.calls[0][1]).toBe('environment.ps1')
  })
  it('keeps a requested nondefault model while a key loads slowly', async () => {
    mocks.query = { key_id: '1', platform: 'gemini', group: '2', model: 'gemini-z' }
    let finish!: (key: ApiKey) => void
    mocks.key.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    mocks.models.mockImplementation(async (ids: number[]) => ({ models: ids[0] === 1
      ? [{ id: 'gpt-test', group_ids: [1] }]
      : [{ id: 'gemini-a', group_ids: [2] }, { id: 'gemini-z', group_ids: [2] }] }))
    await start()
    finish(key(1)); await flushPromises()
    expect(d.model).toBe('gemini-z')
  })
  it('ignores an old key response when another key is selected', async () => {
    await start()
    let finish!: (key: ApiKey) => void
    mocks.key.mockImplementation((id: number) => id === 1 ? new Promise(resolve => { finish = resolve }) : Promise.resolve(key(id)))
    d.selectedKeyId = 1; await flushPromises()
    expect(d.files).toEqual([])
    d.selectedKeyId = 2; await flushPromises()
    finish(key(1)); await flushPromises()
    expect(d.selectedKey?.id).toBe(2)
    d.revealKey = true
    expect(d.files.map(f => f.content).join()).toContain('sk-test-2')
    expect(d.files.map(f => f.content).join()).not.toContain('sk-test-1')
  })
  it('ignores stale group catalogs and excludes media models', async () => {
    await start(); expect(d.models).toEqual(['gpt-test'])
    let finish!: (value: unknown) => void
    mocks.models.mockImplementation((ids: number[]) => ids[0] === 2 ? new Promise(resolve => { finish = resolve }) : Promise.resolve({ models: [{ id: 'gpt-new', group_ids: [1] }] }))
    d.platform = 'gemini'; await flushPromises(); expect(d.files).toEqual([])
    d.platform = 'openai'; await flushPromises()
    finish({ models: [{ id: 'gemini-old', group_ids: [2] }] }); await flushPromises()
    expect(d.models).toEqual(['gpt-new']); expect(d.ready).toBe(true)
  })
})
