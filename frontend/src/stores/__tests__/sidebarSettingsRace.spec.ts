import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAdminSettingsStore } from '../adminSettings'
import { useAppStore } from '../app'
import type { PublicSettings, SidebarGroupsConfig } from '@/types'

const requests = vi.hoisted(() => ({ settings: vi.fn(), payment: vi.fn(), public: vi.fn() }))
vi.mock('@/api', () => ({ adminAPI: { settings: { getSettings: requests.settings }, payment: { getConfig: requests.payment } } }))
vi.mock('@/api/auth', () => ({ getPublicSettings: requests.public }))
vi.mock('@/api/admin/system', () => ({ checkUpdates: vi.fn() }))

const saved: SidebarGroupsConfig = { groups: [{ id: 'tools', label: 'Tools', visibility: 'user', items: ['/keys'] }] }
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(done => { resolve = done })
  return { promise, resolve }
}
beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
  vi.resetAllMocks()
  delete window.__APP_CONFIG__
})
afterEach(() => { delete window.__APP_CONFIG__ })

it('keeps an independent save when an earlier admin fetch is waiting for payment config', async () => {
  const payment = deferred<{ data: { enabled: boolean } }>()
  requests.settings.mockResolvedValue({ sidebar_groups: { groups: [] } })
  requests.payment.mockReturnValue(payment.promise)
  const store = useAdminSettingsStore()
  const fetching = store.fetch()
  store.sidebarGroups = saved
  payment.resolve({ data: { enabled: true } })
  await fetching
  expect(store.sidebarGroups).toEqual(saved)
  expect(store.paymentEnabled).toBe(true)

  requests.settings.mockResolvedValue({ sidebar_groups: { groups: [] } })
  await store.fetch(true)
  expect(store.sidebarGroups).toEqual({ groups: [] })
})

it('keeps saved public groups when an older public response arrives, but allows a later refresh', async () => {
  const response = deferred<PublicSettings>()
  requests.public.mockReturnValueOnce(response.promise)
  const store = useAppStore()
  store.cachedPublicSettings = { site_name: 'Site', sidebar_groups: { groups: [] } } as PublicSettings
  const fetching = store.fetchPublicSettings(true)
  store.cachedPublicSettings = { ...store.cachedPublicSettings, sidebar_groups: saved }
  response.resolve({ site_name: 'Refreshed site', sidebar_groups: { groups: [] } } as PublicSettings)
  const result = await fetching
  expect(result?.sidebar_groups).toEqual(saved)
  expect(store.cachedPublicSettings?.sidebar_groups).toEqual(saved)
  expect(store.siteName).toBe('Refreshed site')
  expect(window.__APP_CONFIG__?.sidebar_groups).toEqual(saved)

  requests.public.mockResolvedValue({ site_name: 'Site', sidebar_groups: { groups: [] } })
  await store.fetchPublicSettings(true)
  expect(store.cachedPublicSettings?.sidebar_groups).toEqual({ groups: [] })
})
