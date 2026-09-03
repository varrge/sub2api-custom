import { describe, expect, it } from 'vitest'

import { normalizeTopQuickMenuItems } from '@/utils/topQuickMenu'

describe('normalizeTopQuickMenuItems', () => {
  it('keeps the saved order while removing invalid, duplicate, and excess items', () => {
    expect(normalizeTopQuickMenuItems([
      'usage',
      'unknown',
      'usage',
      'image_generation',
      'api_keys',
      'model_plaza',
    ])).toEqual(['usage', 'image_generation', 'api_keys'])
  })

  it('defaults missing configuration to dashboard-only', () => {
    expect(normalizeTopQuickMenuItems(undefined)).toEqual([])
  })
})
