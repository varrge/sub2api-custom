import { beforeEach, describe, expect, it, vi } from 'vitest'

const appStore = vi.hoisted(() => ({
  cachedPublicSettings: null as null | { support_ticket_enabled?: boolean },
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))

import { FeatureFlags, isFeatureFlagEnabled } from '../featureFlags'

describe('support ticket feature flag', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = null
  })

  it('is opt-in and fails closed for missing values', () => {
    expect(FeatureFlags.supportTicket.mode).toBe('opt-in')
    expect(isFeatureFlagEnabled(FeatureFlags.supportTicket)).toBe(false)

    appStore.cachedPublicSettings = {}
    expect(isFeatureFlagEnabled(FeatureFlags.supportTicket)).toBe(false)
  })

  it('reflects explicit public setting values', () => {
    appStore.cachedPublicSettings = { support_ticket_enabled: true }
    expect(isFeatureFlagEnabled(FeatureFlags.supportTicket)).toBe(true)

    appStore.cachedPublicSettings = { support_ticket_enabled: false }
    expect(isFeatureFlagEnabled(FeatureFlags.supportTicket)).toBe(false)
  })
})
