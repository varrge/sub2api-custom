import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import { FeatureFlags, isFeatureFlagEnabled, makeSidebarFlag, resolveFeatureFlag } from '@/utils/featureFlags'
import type { PublicSettings } from '@/types'

vi.mock('@/api/admin/system', () => ({
  checkUpdates: vi.fn(),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: vi.fn(),
}))

describe('FeatureFlags.subscription', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    delete (window as any).__APP_CONFIG__
  })

  it('reads subscription_enabled as an opt-out flag: visible before settings load', () => {
    expect(FeatureFlags.subscription.key).toBe('subscription_enabled')
    expect(FeatureFlags.subscription.mode).toBe('opt-out')
    expect(useAppStore().cachedPublicSettings).toBeNull()
    expect(isFeatureFlagEnabled(FeatureFlags.subscription)).toBe(true)
  })

  it('hides only when the backend explicitly sends false', () => {
    const store = useAppStore()
    const sidebarFlag = makeSidebarFlag(FeatureFlags.subscription)

    store.cachedPublicSettings = { subscription_enabled: false } as PublicSettings
    expect(sidebarFlag()).toBe(false)

    store.cachedPublicSettings = { subscription_enabled: true } as PublicSettings
    expect(sidebarFlag()).toBe(true)

    store.cachedPublicSettings = {} as PublicSettings
    expect(sidebarFlag()).toBe(true)
  })
})

describe('resolveFeatureFlag', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('reads an explicit boolean from the given settings object', () => {
    expect(resolveFeatureFlag({ subscription_enabled: false } as PublicSettings, FeatureFlags.subscription)).toBe(false)
    expect(resolveFeatureFlag({ subscription_enabled: true } as PublicSettings, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag({ available_channels_enabled: true } as PublicSettings, FeatureFlags.availableChannels)).toBe(true)
  })

  it('falls back to the declared mode when settings are missing or the key is absent', () => {
    expect(resolveFeatureFlag(undefined, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag(null, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.availableChannels)).toBe(false)
  })

  it('backs isFeatureFlagEnabled with the same resolution', () => {
    useAppStore().cachedPublicSettings = { subscription_enabled: false } as PublicSettings
    expect(isFeatureFlagEnabled(FeatureFlags.subscription)).toBe(false)
  })
})

describe('FeatureFlags.nativeBoxEntry', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    delete (window as any).__APP_CONFIG__
  })

  it('hides the native box entry unless it is explicitly enabled', () => {
    const flag = FeatureFlags.nativeBoxEntry
    expect(flag.key).toBe('native_box_entry_enabled')
    expect(flag.mode).toBe('opt-in')
    expect(resolveFeatureFlag(null, flag)).toBe(false)
    expect(resolveFeatureFlag({}, flag)).toBe(false)
    expect(resolveFeatureFlag({ native_box_entry_enabled: false }, flag)).toBe(false)
    expect(resolveFeatureFlag({ native_box_entry_enabled: true }, flag)).toBe(true)
  })

  it('uses the injected value after refresh and follows newly saved public settings', () => {
    (window as any).__APP_CONFIG__ = { native_box_entry_enabled: true }
    const store = useAppStore()
    expect(store.initFromInjectedConfig()).toBe(true)
    const visible = makeSidebarFlag(FeatureFlags.nativeBoxEntry)
    expect(visible()).toBe(true)
    store.cachedPublicSettings = { native_box_entry_enabled: false } as PublicSettings
    expect(visible()).toBe(false)
  })
})
