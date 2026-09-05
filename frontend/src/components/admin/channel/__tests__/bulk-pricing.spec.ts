import { describe, expect, it } from 'vitest'
import { canImportDefaultPricing, hasDefaultTokenPricing, mergeDefaultPricing } from '../bulk-pricing'
import { createDefaultTimePricingForm, formIntervalsToAPI, mTokToPerToken, type PricingFormEntry } from '../types'

function blank(models: string[], overrides: Partial<PricingFormEntry> = {}): PricingFormEntry {
  return {
    models, billing_mode: 'token', input_price: null, output_price: null,
    cache_write_price: null, cache_read_price: null, image_input_price: null,
    image_output_price: null, per_request_price: null, intervals: [],
    time_pricing: createDefaultTimePricingForm(), ...overrides
  }
}

describe('bulk default prices', () => {
  it('splits blank rules by model and preserves the unselected names without mutating the draft', () => {
    const entries = [blank(['claude-a', 'claude-b', 'unknown'])]
    const before = structuredClone(entries)
    const result = mergeDefaultPricing(entries, [
      { model: 'claude-a', pricing: { found: true, input_price: 3e-6, output_price: 15e-6, cache_write_1h_price: 6e-6 } },
      { model: 'claude-b', pricing: { found: true, input_price: 12e-6, output_price: 60e-6, cache_read_price: 1.2e-6 } }
    ])
    expect(entries).toEqual(before)
    expect(result.count).toBe(2)
    expect(result.entries.map(entry => entry.models)).toEqual([['unknown'], ['claude-a'], ['claude-b']])
    expect(result.entries[1].input_price).toBe(3)
    expect(result.entries[1].cache_write_1h_price).toBe(6)
    expect(result.entries[2].input_price).toBe(12)
    expect(result.entries[2].cache_read_price).toBe(1.2)
    expect(mTokToPerToken(result.entries[2].input_price)).toBe(12e-6)
    expect(formIntervalsToAPI(result.entries[2].intervals)).toEqual([])
  })

  it('preserves custom zero prices, case-insensitive matches, wildcard and media rules', () => {
    const entries = [
      blank(['CLAUDE-A'], { input_price: 0 }), blank(['claude-b*']),
      blank(['image-a'], { billing_mode: 'image', per_request_price: 0.1 }),
      blank(['claude-c'], { fast_multiplier: 2 })
    ]
    for (const model of ['claude-a', 'claude-b-2026', 'image-a', 'claude-c', '*']) {
      expect(canImportDefaultPricing(model, entries)).toBe(false)
    }
    expect(canImportDefaultPricing('claude-d', entries)).toBe(true)
    const result = mergeDefaultPricing(entries, entries.map(entry => ({
      model: entry.models[0], pricing: { found: true, input_price: 3e-6 }
    })))
    expect(result.entries).toBe(entries)
    expect(result.count).toBe(0)
  })

  it('ignores missing, non-token and invalid catalog prices instead of importing zeros', () => {
    for (const pricing of [
      { found: false }, { found: true, input_price: 0, output_price: 0 },
      { found: true, input_price: -1 }, { found: true, input_price: NaN },
      { found: true, input_price: 1e-6, cache_read_price: Infinity }
    ]) expect(hasDefaultTokenPricing(pricing)).toBe(false)
    const original = [blank(['unknown'])]
    expect(mergeDefaultPricing(original, [{ model: 'unknown', pricing: { found: false } }]).entries).toBe(original)
  })

  it('deduplicates case variants and rechecks rules added while catalog prices were loading', () => {
    const rows = ['Model-A', 'model-a'].map(model => ({ model, pricing: { found: true, input_price: 1e-6 } }))
    const result = mergeDefaultPricing([], rows)
    expect(result.count).toBe(1)
    expect(mergeDefaultPricing(result.entries, rows).count).toBe(0)
  })

  it('uses the same Claude dot/hyphen aliases as the billing cache', () => {
    expect(canImportDefaultPricing('claude-sonnet-4-5', [blank(['claude-sonnet-4.5'], { input_price: 0 })])).toBe(false)
    expect(canImportDefaultPricing('claude-sonnet-4-5-2026', [blank(['claude-sonnet-4.5*'])])).toBe(false)
    const result = mergeDefaultPricing([blank(['claude-sonnet-4.5'])], [
      { model: 'claude-sonnet-4-5', pricing: { found: true, input_price: 3e-6 } }
    ])
    expect(result.count).toBe(1)
    expect(result.entries).toHaveLength(1)
    expect(result.entries[0].models).toEqual(['claude-sonnet-4-5'])
  })
})
