import type { ModelDefaultPricing } from '@/api/admin/channels'
import { createDefaultTimePricingForm, perTokenToMTok, type PricingFormEntry } from './types'

export interface DefaultPricingRow {
  model: string
  pricing: ModelDefaultPricing
}

const priceFields = [
  'input_price', 'output_price', 'cache_write_price', 'cache_write_1h_price',
  'cache_read_price', 'image_input_price', 'image_output_price'
] as const

// Keep aliases identical to normalizeChannelPricingModelName in the billing cache.
function modelKey(model: string): string {
  const name = model.trim().toLowerCase()
  return name.startsWith('claude-') ? name.replace(/\./g, '-') : name
}

function hasOverrides(entry: PricingFormEntry): boolean {
  return entry.billing_mode !== 'token' ||
    [...priceFields, 'per_request_price', 'fast_multiplier', 'flex_multiplier', 'max_reasoning_effort_multiplier'].some((field) => {
      const value = entry[field as keyof PricingFormEntry]
      return value !== null && value !== undefined && value !== ''
    }) || entry.intervals.length > 0 || entry.time_pricing.periods.length > 0
}

/** Do not replace custom prices (including zero), media rules, or wildcard rules. */
export function canImportDefaultPricing(model: string, entries: PricingFormEntry[]): boolean {
  const name = modelKey(model)
  if (!name || name.includes('*')) return false
  return !entries.some(entry => entry.models.some(pattern => {
    const normalized = modelKey(pattern)
    const wildcard = normalized.endsWith('*')
    const matches = wildcard ? name.startsWith(normalized.slice(0, -1)) : name === normalized
    return matches && (wildcard || hasOverrides(entry))
  }))
}

/** This importer only handles catalog token prices, never guesses media/request prices. */
export function hasDefaultTokenPricing(pricing: ModelDefaultPricing): boolean {
  return pricing.found && priceFields.every(field =>
    pricing[field] == null || (Number.isFinite(pricing[field]) && pricing[field]! >= 0)
  ) && ((pricing.input_price ?? 0) > 0 || (pricing.output_price ?? 0) > 0)
}

export function mergeDefaultPricing(entries: PricingFormEntry[], rows: DefaultPricingRow[]) {
  const selected = new Map<string, DefaultPricingRow>()
  for (const row of rows) {
    if (canImportDefaultPricing(row.model, entries) && hasDefaultTokenPricing(row.pricing)) {
      selected.set(modelKey(row.model), row)
    }
  }
  if (!selected.size) return { entries, count: 0 }

  // Split blank multi-model rules: every model keeps its own catalog price.
  const result = entries.flatMap(entry => {
    if (hasOverrides(entry)) return [entry]
    const models = entry.models.filter(model => !selected.has(modelKey(model)))
    return models.length === entry.models.length ? [entry] : models.length ? [{ ...entry, models }] : []
  })
  for (const { model, pricing } of selected.values()) {
    result.push({
      models: [model.trim()],
      billing_mode: 'token',
      input_price: perTokenToMTok(pricing.input_price ?? null),
      output_price: perTokenToMTok(pricing.output_price ?? null),
      cache_write_price: perTokenToMTok(pricing.cache_write_price ?? null),
      cache_write_1h_price: perTokenToMTok(pricing.cache_write_1h_price ?? null),
      cache_read_price: perTokenToMTok(pricing.cache_read_price ?? null),
      image_input_price: perTokenToMTok(pricing.image_input_price ?? null),
      image_output_price: perTokenToMTok(pricing.image_output_price ?? null),
      fast_multiplier: null,
      flex_multiplier: null,
      max_reasoning_effort_multiplier: pricing.max_reasoning_effort_multiplier ?? null,
      per_request_price: null,
      intervals: [],
      time_pricing: createDefaultTimePricingForm()
    })
  }
  return { entries: result, count: selected.size }
}
