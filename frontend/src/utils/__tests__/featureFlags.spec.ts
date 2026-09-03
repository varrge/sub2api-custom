import { beforeEach, describe, expect, it, vi } from 'vitest'

const appStore = vi.hoisted(() => ({
  cachedPublicSettings: null as null | { top_quick_bar_enabled?: boolean },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

import { FeatureFlags, isFeatureFlagEnabled } from '../featureFlags'

describe('top quick bar feature flag', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = null
  })

  it('defaults on and honors an explicit off value', () => {
    expect(isFeatureFlagEnabled(FeatureFlags.topQuickBar)).toBe(true)
    appStore.cachedPublicSettings = { top_quick_bar_enabled: false }
    expect(isFeatureFlagEnabled(FeatureFlags.topQuickBar)).toBe(false)
  })
})
