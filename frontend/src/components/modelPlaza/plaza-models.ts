import type { ModelPlazaGroup, PlazaModel, PlazaOfficialPricing } from '@/api/modelPlaza'
import type { BillingMode } from '@/constants/channel'
import type { GroupPlatform } from '@/types'
import { effectiveGroupRate } from '@/utils/temporary-rate'

export interface PlazaBrandInfo {
  id: string
  name: string
  platform?: GroupPlatform
}

export interface GroupModelVariant {
  group: ModelPlazaGroup
  model: PlazaModel
  effectiveRate: number
}

export interface AggregatedPlazaModel {
  id: string
  name: string
  brand: PlazaBrandInfo
  billingMode: BillingMode
  variants: GroupModelVariant[]
}

export const SUPPORTED_BRANDS: Record<string, { name: string; platform: GroupPlatform }> = {
  claude: { name: 'Claude', platform: 'anthropic' },
  openai: { name: 'OpenAI', platform: 'openai' },
  deepseek: { name: 'DeepSeek', platform: 'deepseek' },
  gemini: { name: 'Gemini', platform: 'gemini' },
  glm: { name: 'GLM', platform: 'zhipu' },
  grok: { name: 'Grok', platform: 'grok' },
  kimi: { name: 'Kimi', platform: 'kimi' }
}

export function inferModelBrand(modelName: string, fallbackPlatform?: string): PlazaBrandInfo {
  const name = (modelName || '').toLowerCase().trim()
  const platform = (fallbackPlatform || '').toLowerCase().trim()

  const patterns: [string, RegExp][] = [
    ['claude', /claude/], ['deepseek', /deepseek/], ['grok', /grok/],
    ['kimi', /kimi|moonshot/], ['glm', /\bglm|chatglm|cogview|cogvideo/],
    ['gemini', /gemini|imagen|veo|nano[-_ ]?banana/],
    ['openai', /gpt|codex|(?:^|\/)o[1-9](?:-|_|\b)|text-embedding|dall-e|whisper|sora/]
  ]
  const id = patterns.find(([, pattern]) => pattern.test(name))?.[0] ??
    Object.keys(SUPPORTED_BRANDS).find(key => SUPPORTED_BRANDS[key].platform === platform)
  return id ? { id, ...SUPPORTED_BRANDS[id] } : { id: platform || 'other', name: platform || 'Other' }
}

export type PlazaModelType = 'text' | 'image' | 'video' | 'other'

export function plazaModelType(model: PlazaModel): PlazaModelType {
  const mode = model.metadata?.mode ?? ''
  if (model.pricing?.billing_mode === 'video' || mode.includes('video')) return 'video'
  if (model.pricing?.billing_mode === 'image' || ['image_generation', 'image', 'image_edit'].includes(mode)) return 'image'
  if (['audio_transcription', 'audio_speech', 'audio'].includes(mode)) return 'other'
  return model.pricing?.billing_mode === 'token' ? 'text' : 'other'
}

export function variantKey(variant: GroupModelVariant): string {
  return `${variant.group.id}:${variant.model.platform}:${variant.model.name}`
}

/** A reference group controls prices only, never filters the all-groups catalog. */
export function selectPriceVariant(model: AggregatedPlazaModel, groupId: number | null, overrideKey?: string | null): GroupModelVariant | null {
  if (overrideKey) {
    const override = model.variants.find(variant => variantKey(variant) === overrideKey)
    if (override) return override
  }
  return model.variants.find(variant => variant.group.id === groupId) ?? null
}

export interface PlazaPriceCell {
  id: string
  labelKey: string
  price: number | null
  original: number | null
  unitKey: string
}

/** The backend resolves official defaults and channel/group overrides before applying multipliers. */
export function modelBasePricing(model: PlazaModel): PlazaOfficialPricing | null {
  return model.pricing?.billing_mode === 'token' ? model.pricing : null
}

export function modelPriceCells(variant: GroupModelVariant): PlazaPriceCell[] {
  const { model, effectiveRate: rate } = variant
  const pricing = model.pricing
  if (!pricing) return []
  const finite = (value: number | null | undefined) => value != null && Number.isFinite(value) ? value : null
  const paid = (value: number | null | undefined, scale = 1) => {
    const price = finite(value)
    return price === null ? null : price * rate * scale
  }
  if (pricing.billing_mode === 'token') {
    const fields = ['input_price', 'output_price', 'cache_write_price', 'cache_write_1h_price', 'cache_read_price'] as const
    const keys = ['inputPrice', 'outputPrice', 'cacheWritePrice', 'cacheWrite1hPrice', 'cacheReadPrice']
    return fields.flatMap((field, index) => {
      const original = finite(modelBasePricing(model)?.[field])
      if (index > 1 && pricing[field] == null && original == null) return []
      return [{
        id: field, labelKey: `modelPlaza.card.${keys[index]}`,
        price: paid(pricing[field], 1e6), original: original === null ? null : original * 1e6,
        unitKey: 'modelPlaza.catalog.perMillion'
      }]
    })
  }
  if (pricing.intervals.length) {
    // Each media/audio tier keeps its own units, never merge different dimensions into a range.
    return pricing.intervals.map((tier, index) => ({
      id: `tier-${index}`, labelKey: tier.tier_label || 'modelPlaza.card.unitPrice',
      price: paid(tier.per_request_price), original: null,
      unitKey: nonTokenUnitKey(pricing.billing_mode, tier.tier_label)
    }))
  }
  return [{ id: 'unit', labelKey: 'modelPlaza.card.unitPrice', price: paid(pricing.per_request_price), original: null, unitKey: nonTokenUnitKey(pricing.billing_mode) }]
}

export function formatTokenLimit(value: number | undefined): string {
  if (!value || !Number.isFinite(value) || value < 0) return '—'
  if (value >= 1e6) return `${Number((value / 1e6).toPrecision(3))}M`
  if (value >= 1000) return `${Number((value / 1000).toPrecision(3))}K`
  return String(value)
}

export function aggregatePlazaModels(groups: ModelPlazaGroup[], now: number | Date = Date.now()): AggregatedPlazaModel[] {
  const models = new Map<string, AggregatedPlazaModel>()
  for (const group of groups) {
    for (const model of group.models) {
      const billingMode = model.pricing?.billing_mode || 'token'
      const effectiveRate = billingMode === 'image' && group.image_rate_independent
        ? group.image_rate_multiplier
        : billingMode === 'video' && group.video_rate_independent
          ? group.video_rate_multiplier ?? 1
          : effectiveGroupRate(group, group.user_rate_multiplier, now)
      const id = `${model.name.trim().toLowerCase()}::${billingMode}`
      const entry = models.get(id) ?? { id, name: model.name, brand: inferModelBrand(model.name, model.platform), billingMode, variants: [] }
      entry.variants.push({ group, model, effectiveRate })
      models.set(id, entry)
    }
  }
  for (const model of models.values()) {
    model.variants.sort((a, b) => a.effectiveRate - b.effectiveRate || a.group.name.localeCompare(b.group.name) || a.model.platform.localeCompare(b.model.platform))
  }
  return [...models.values()].sort((a, b) => a.name.localeCompare(b.name))
}

export function formatUsdPerMillion(pricePerToken: number | null | undefined): string | null {
  if (pricePerToken === null || pricePerToken === undefined || !Number.isFinite(pricePerToken)) {
    return null
  }
  return formatUsdDirect(pricePerToken * 1_000_000)
}

export function formatUsdDirect(price: number | null | undefined): string | null {
  if (price === null || price === undefined || !Number.isFinite(price)) {
    return null
  }
  return `$${Number(price.toPrecision(8))}`
}

export function nonTokenUnitKey(mode: BillingMode, tierLabel?: string): string {
  if (mode === 'image') return 'modelPlaza.table.perUnitImage'
  if (mode === 'video') return 'modelPlaza.card.perSecond'
  if (mode === 'per_request') {
    switch (tierLabel?.toLowerCase()) {
      case 'realtime': return 'modelPlaza.card.perMinute'
      case 'tts': return 'modelPlaza.card.perMillionChars'
      case 'stt': return 'modelPlaza.card.perHour'
    }
  }
  return 'modelPlaza.table.perUnitRequest'
}
