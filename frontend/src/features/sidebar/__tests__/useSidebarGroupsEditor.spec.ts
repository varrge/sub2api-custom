import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, ref, type Ref } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { CustomMenuItem, PublicSettings, SidebarGroup, SidebarGroupsConfig } from '@/types'
import { useSidebarGroupsEditor } from '../useSidebarGroupsEditor'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  locale: { value: 'en' },
  app: {
    cachedPublicSettings: null as PublicSettings | null,
    showSuccess: vi.fn(),
  },
  admin: { sidebarGroups: { groups: [] } as SidebarGroupsConfig },
}))

vi.mock('@/api/client', () => ({ apiClient: { get: mocks.get, put: mocks.put } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks.app }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => mocks.admin }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: mocks.locale }) }))

const endpoint = '/admin/settings/sidebar-groups'
const group = (id: string, items: string[] = [], visibility: 'user' | 'admin' = 'user'): SidebarGroup => ({
  id, label: `Group ${id}`, visibility, items,
})
const wrappers: VueWrapper[] = []

function mountEditor(customMenus: Ref<CustomMenuItem[]> = ref([])) {
  let editor!: ReturnType<typeof useSidebarGroupsEditor>
  wrappers.push(mount(defineComponent({
    setup() {
      editor = useSidebarGroupsEditor(customMenus)
      return () => null
    },
  })))
  return editor
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((onResolve, onReject) => { resolve = onResolve; reject = onReject })
  return { promise, resolve, reject }
}

beforeEach(() => {
  vi.resetAllMocks()
  mocks.locale.value = 'en'
  mocks.get.mockResolvedValue({ data: { groups: [] } })
  mocks.put.mockImplementation(async (_path: string, data: SidebarGroupsConfig) => ({ data }))
  mocks.app.cachedPublicSettings = { site_name: 'Existing site', sidebar_groups: { groups: [] } } as unknown as PublicSettings
  mocks.admin.sidebarGroups = { groups: [] }
  delete window.__APP_CONFIG__
})

afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount())
  delete window.__APP_CONFIG__
})

describe('useSidebarGroupsEditor loading and saving', () => {
  it('loads through the dedicated endpoint and keeps edits separate from the response', async () => {
    const server = { groups: [group('tools', ['/keys'])] }
    mocks.get.mockResolvedValue({ data: server })
    const editor = mountEditor()
    await flushPromises()

    expect(mocks.get).toHaveBeenCalledOnce()
    expect(mocks.get).toHaveBeenCalledWith(endpoint)
    expect(editor.loading.value).toBe(false)
    expect(editor.draft.value).toEqual(server)
    editor.draft.value.groups[0].label = 'Edited'
    editor.draft.value.groups[0].items.push('/usage')
    expect(server.groups[0]).toEqual(group('tools', ['/keys']))
    expect(mocks.admin.sidebarGroups).toEqual({ groups: [] })
  })

  it('cannot save while loading or after a failed load, and supports retrying the load', async () => {
    const pending = deferred<{ data: SidebarGroupsConfig }>()
    mocks.get.mockReturnValueOnce(pending.promise)
    const editor = mountEditor()

    expect(await editor.save()).toBe(false)
    pending.reject(new Error('Unavailable'))
    await flushPromises()
    expect(editor.loading.value).toBe(false)
    expect(editor.error.value).toContain('Could not load groups')
    expect(await editor.save()).toBe(false)
    expect(mocks.put).not.toHaveBeenCalled()

    mocks.get.mockResolvedValueOnce({ data: { groups: [group('loaded', ['/keys'])] } })
    await editor.load()
    expect(editor.error.value).toBe('')
    expect(await editor.save()).toBe(true)
    expect(mocks.put).toHaveBeenCalledWith(endpoint, { groups: [group('loaded', ['/keys'])] })
  })

  it('saves only group settings and updates admin and public caches with separate copies', async () => {
    const editor = mountEditor()
    await flushPromises()
    const groups = [group('user', ['/keys']), group('admin', ['/admin/users'], 'admin')]
    editor.draft.value = { groups }
    editor.draft.value.groups[0].label = '  My tools  '
    const saved = { groups: [{ ...groups[0], label: 'My tools' }, groups[1]] }
    mocks.put.mockResolvedValueOnce({ data: saved })
    window.__APP_CONFIG__ = { site_name: 'Injected site', sidebar_groups: { groups: [] } } as unknown as PublicSettings

    expect(await editor.save()).toBe(true)

    expect(mocks.put).toHaveBeenCalledOnce()
    expect(mocks.put).toHaveBeenCalledWith(endpoint, saved)
    expect(mocks.admin.sidebarGroups).toEqual(saved)
    const publicGroups = { groups: [saved.groups[0]] }
    expect(mocks.app.cachedPublicSettings).toMatchObject({ site_name: 'Existing site', sidebar_groups: publicGroups })
    expect(window.__APP_CONFIG__).toMatchObject({ site_name: 'Injected site', sidebar_groups: publicGroups })
    expect(editor.draft.value).toEqual(saved)
    expect(mocks.app.showSuccess).toHaveBeenCalledOnce()
    expect(mocks.app.showSuccess).toHaveBeenCalledWith('Sidebar groups saved')

    editor.draft.value.groups[0].items.push('/usage')
    expect(saved.groups[0].items).toEqual(['/keys'])
    expect(mocks.admin.sidebarGroups.groups[0].items).toEqual(['/keys'])
    expect(mocks.app.cachedPublicSettings?.sidebar_groups?.groups[0].items).toEqual(['/keys'])
    mocks.admin.sidebarGroups.groups[0].items.push('/tickets')
    expect(window.__APP_CONFIG__?.sidebar_groups?.groups[0].items).toEqual(['/keys'])
    expect(mocks.app.cachedPublicSettings?.sidebar_groups?.groups[0].items).toEqual(['/keys'])
  })

  it('retains the draft and existing caches when saving fails, then retries successfully', async () => {
    const editor = mountEditor()
    await flushPromises()
    editor.draft.value = { groups: [group('tools', ['/keys', '/usage'])] }
    editor.draft.value.groups[0].label = '  Work  '
    mocks.admin.sidebarGroups = { groups: [group('previous', ['/tickets'])] }
    const originalPublic = mocks.app.cachedPublicSettings
    mocks.put.mockRejectedValueOnce(new Error('Unavailable'))

    expect(await editor.save()).toBe(false)
    expect(editor.saving.value).toBe(false)
    expect(editor.error.value).toContain('Your changes are retained')
    expect(editor.draft.value.groups[0]).toEqual({ ...group('tools', ['/keys', '/usage']), label: '  Work  ' })
    expect(mocks.admin.sidebarGroups).toEqual({ groups: [group('previous', ['/tickets'])] })
    expect(mocks.app.cachedPublicSettings).toBe(originalPublic)
    expect(mocks.app.showSuccess).not.toHaveBeenCalled()

    expect(await editor.save()).toBe(true)
    expect(editor.error.value).toBe('')
    expect(editor.draft.value.groups[0].label).toBe('Work')
    expect(mocks.put).toHaveBeenCalledTimes(2)
  })

  it('prevents duplicate saves and reloads while a save is pending', async () => {
    const editor = mountEditor()
    await flushPromises()
    editor.draft.value = { groups: [group('tools', ['/keys'])] }
    const pending = deferred<{ data: SidebarGroupsConfig }>()
    mocks.put.mockReturnValueOnce(pending.promise)

    const firstSave = editor.save()
    expect(editor.saving.value).toBe(true)
    expect(await editor.save()).toBe(false)
    await editor.load()
    expect(mocks.get).toHaveBeenCalledTimes(1)
    expect(mocks.put).toHaveBeenCalledTimes(1)

    pending.resolve({ data: { groups: [group('tools', ['/keys'])] } })
    expect(await firstSave).toBe(true)
    expect(editor.saving.value).toBe(false)
  })

  it('supports saving when public caches have not been initialized', async () => {
    mocks.app.cachedPublicSettings = null
    const editor = mountEditor()
    await flushPromises()
    editor.draft.value = { groups: [group('admin', ['/admin/users'], 'admin')] }

    expect(await editor.save()).toBe(true)
    expect(mocks.app.cachedPublicSettings).toBeNull()
    expect(window.__APP_CONFIG__).toBeUndefined()
    expect(mocks.admin.sidebarGroups).toEqual(editor.draft.value)
  })

  it('retains newer edits when an earlier save finishes', async () => {
    const editor = mountEditor()
    await flushPromises()
    editor.draft.value = { groups: [group('tools', ['/keys'])] }
    const pending = deferred<{ data: SidebarGroupsConfig }>()
    mocks.put.mockReturnValueOnce(pending.promise)
    const saving = editor.save()
    editor.draft.value.groups[0].label = 'Newer name'
    editor.addItem('tools', '/usage')
    pending.resolve({ data: { groups: [group('tools', ['/keys'])] } })
    expect(await saving).toBe(true)
    expect(editor.draft.value.groups[0]).toEqual({ ...group('tools', ['/keys', '/usage']), label: 'Newer name' })
    expect(mocks.admin.sidebarGroups.groups[0]).toEqual(group('tools', ['/keys']))
    expect(mocks.app.showSuccess).toHaveBeenCalledWith('Saved. Your newer edits are still unsaved.')
  })

  it.each(['   ', 'a'.repeat(65)])('rejects invalid group labels before writing: %j', async (label) => {
    const editor = mountEditor()
    await flushPromises()
    editor.draft.value = { groups: [{ ...group('tools'), label }] }

    expect(await editor.save()).toBe(false)
    expect(editor.error.value).toContain('1–64 characters')
    expect(mocks.put).not.toHaveBeenCalled()
  })

  it('counts Unicode characters when validating names', async () => {
    const editor = mountEditor()
    await flushPromises()
    editor.draft.value = { groups: [{ ...group('tools'), label: '🔑'.repeat(64) }] }

    expect(await editor.save()).toBe(true)
    expect(mocks.put).toHaveBeenCalledOnce()
  })

  it('rejects duplicate assignments within a scope but accepts the same page across scopes', async () => {
    const editor = mountEditor()
    await flushPromises()
    editor.draft.value = { groups: [group('first', ['/keys']), group('second', ['/keys'])] }

    expect(await editor.save()).toBe(false)
    expect(editor.error.value).toContain('only one group')
    expect(mocks.put).not.toHaveBeenCalled()

    editor.draft.value.groups[1].visibility = 'admin'
    expect(await editor.save()).toBe(true)
    expect(mocks.put).toHaveBeenCalledOnce()
  })
})

describe('useSidebarGroupsEditor assignments', () => {
  it('isolates available pages by scope and prevents duplicate or unknown assignments', async () => {
    const editor = mountEditor()
    await flushPromises()
    editor.draft.value = { groups: [group('first'), group('second'), group('admin', [], 'admin')] }

    editor.addItem('first', '/keys')
    editor.addItem('first', '/keys')
    editor.addItem('second', '/keys')
    editor.addItem('first', '/admin/users')
    editor.addItem('first', '/missing')
    editor.addItem('admin', '/dashboard')
    editor.addItem('admin', '/keys')
    editor.addItem('admin', '/admin/users')

    expect(editor.draft.value.groups.map(item => item.items)).toEqual([['/keys'], [], ['/keys', '/admin/users']])
    editor.removeItem('first', '/keys')
    expect(editor.availableItems(editor.draft.value.groups[1]).some(item => item.path === '/keys')).toBe(true)
    expect(editor.availableItems(editor.draft.value.groups[2]).some(item => item.path === '/keys')).toBe(false)
  })

  it('updates custom page choices reactively and preserves unknown existing page labels', async () => {
    const menus = ref<CustomMenuItem[]>([])
    const editor = mountEditor(menus)
    await flushPromises()
    editor.draft.value = { groups: [group('user'), group('admin', [], 'admin')] }
    menus.value.push({ id: 'help', label: 'Help center', visibility: 'user', sort_order: 0, icon_svg: '', url: '/help' })

    editor.addItem('user', '/custom/help')
    editor.addItem('admin', '/custom/help')

    expect(editor.draft.value.groups[0].items).toEqual(['/custom/help'])
    expect(editor.draft.value.groups[1].items).toEqual([])
    expect(editor.itemLabel('/custom/help')).toBe('Help center')
    expect(editor.itemLabel('/custom/deleted')).toBe('/custom/deleted')
  })

  it('reorders groups and pages, ignores boundary moves, and releases removed assignments', async () => {
    const editor = mountEditor()
    await flushPromises()
    editor.draft.value = { groups: [group('first', ['/keys', '/usage']), group('second', ['/tickets'])] }

    editor.moveGroup('first', -1)
    editor.moveGroup('missing', 1)
    expect(editor.draft.value.groups.map(item => item.id)).toEqual(['first', 'second'])
    editor.moveGroup('second', -1)
    editor.moveItem('first', '/usage', -1)
    editor.moveItem('first', '/usage', -1)
    editor.moveItem('first', '/missing', 1)
    expect(editor.draft.value.groups.map(item => item.id)).toEqual(['second', 'first'])
    expect(editor.draft.value.groups[1].items).toEqual(['/usage', '/keys'])

    editor.removeGroup('first')
    expect(editor.availableItems(editor.draft.value.groups[0]).map(item => item.path)).toContain('/keys')
    expect(editor.draft.value.groups.map(item => item.id)).toEqual(['second'])
  })
})
