import type { ModelPlazaGroup, PlazaModel, PlazaOfficialPricing } from '@/api/modelPlaza'
import type { BillingMode } from '@/constants/channel'
import type { UserSupportedModelPricing } from '@/api/channels'
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
  officialPricing: PlazaOfficialPricing | null
  /** 多个分组下基准价范围 (USD per token 或 USD per unit) */
  priceRanges: {
    input?: [number, number]
    output?: [number, number]
    cacheWrite?: [number, number]
    cacheWrite1h?: [number, number]
    cacheRead?: [number, number]
    perRequest?: [number, number]
    imageInput?: [number, number]
    imageOutput?: [number, number]
  }
  hasPriceDifferences: boolean
  variants: GroupModelVariant[]
}

export const SUPPORTED_BRANDS: Record<string, { name: string; platform: GroupPlatform }> = {
  claude: { name: 'CLAUDE', platform: 'anthropic' },
  codex: { name: 'CODEX', platform: 'openai' },
  deepseek: { name: 'DEEPSEEK', platform: 'deepseek' },
  gemini: { name: 'GEMINI', platform: 'gemini' },
  glm: { name: 'GLM', platform: 'zhipu' },
  gpt: { name: 'GPT', platform: 'openai' },
  grok: { name: 'GROK', platform: 'grok' },
  kimi: { name: 'KIMI', platform: 'kimi' },
  nano: { name: 'NANO', platform: 'gemini' },
  antigravity: { name: 'ANTIGRAVITY', platform: 'antigravity' }
}

export function inferModelBrand(modelName: string, fallbackPlatform?: string): PlazaBrandInfo {
  const name = (modelName || '').toLowerCase().trim()
  const platform = (fallbackPlatform || '').toLowerCase().trim()

  if (/nano[-_ ]?banana|gemini.*image/i.test(name)) {
    return { id: 'nano', name: 'NANO', platform: 'gemini' }
  }
  if (/claude/i.test(name)) {
    return { id: 'claude', name: 'CLAUDE', platform: 'anthropic' }
  }
  if (/codex/i.test(name)) {
    return { id: 'codex', name: 'CODEX', platform: 'openai' }
  }
  if (/deepseek/i.test(name)) {
    return { id: 'deepseek', name: 'DEEPSEEK', platform: 'deepseek' }
  }
  if (/gemini|imagen|veo/i.test(name)) {
    return { id: 'gemini', name: 'GEMINI', platform: 'gemini' }
  }
  if (/\bglm|chatglm|cogview|cogvideo/i.test(name) || platform === 'zhipu') {
    return { id: 'glm', name: 'GLM', platform: 'zhipu' }
  }
  if (/\bgpt|chatgpt|^o[1-9](?:-|_|\b)|text-embedding|dall-e|whisper|tts|sora/i.test(name)) {
    return { id: 'gpt', name: 'GPT', platform: 'openai' }
  }
  if (/grok/i.test(name)) {
    return { id: 'grok', name: 'GROK', platform: 'grok' }
  }
  if (/kimi|moonshot/i.test(name)) {
    return { id: 'kimi', name: 'KIMI', platform: 'kimi' }
  }

  // Fallback by platform
  if (platform === 'anthropic') return { id: 'claude', name: 'CLAUDE', platform: 'anthropic' }
  if (platform === 'openai') return { id: 'gpt', name: 'GPT', platform: 'openai' }
  if (platform === 'gemini') return { id: 'gemini', name: 'GEMINI', platform: 'gemini' }
  if (platform === 'deepseek') return { id: 'deepseek', name: 'DEEPSEEK', platform: 'deepseek' }
  if (platform === 'zhipu') return { id: 'glm', name: 'GLM', platform: 'zhipu' }
  if (platform === 'kimi') return { id: 'kimi', name: 'KIMI', platform: 'kimi' }
  if (platform === 'grok') return { id: 'grok', name: 'GROK', platform: 'grok' }
  if (platform === 'antigravity') return { id: 'antigravity', name: 'ANTIGRAVITY', platform: 'antigravity' }

  const cleanId = platform || 'other'
  const cleanPlatform: GroupPlatform | undefined =
    platform === 'anthropic' ||
    platform === 'openai' ||
    platform === 'gemini' ||
    platform === 'grok' ||
    platform === 'deepseek' ||
    platform === 'kimi' ||
    platform === 'zhipu' ||
    platform === 'antigravity'
      ? platform
      : undefined

  return {
    id: cleanId,
    name: cleanId.toUpperCase(),
    platform: cleanPlatform
  }
}

function calculateRange(values: (number | null | undefined)[]): [number, number] | undefined {
  const valid = values.filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
  if (valid.length === 0) return undefined
  const min = Math.min(...valid)
  const max = Math.max(...valid)
  return [min, max]
}

export function aggregatePlazaModels(groups: ModelPlazaGroup[], now: number | Date = Date.now()): AggregatedPlazaModel[] {
  const map = new Map<string, {
    name: string
    brand: PlazaBrandInfo
    billingMode: BillingMode
    officialPricing: PlazaOfficialPricing | null
    variants: GroupModelVariant[]
  }>()

  for (const group of groups) {
    for (const model of group.models || []) {
      const billingMode = model.pricing?.billing_mode || 'token'
      const effectiveRate = billingMode === 'image' && group.image_rate_independent
        ? group.image_rate_multiplier
        : effectiveGroupRate(group, group.user_rate_multiplier, now)
      const key = `${model.name.trim().toLowerCase()}::${billingMode}`

      const brand = inferModelBrand(model.name, model.platform || group.platform)
      if (!map.has(key)) {
        map.set(key, {
          name: model.name,
          brand,
          billingMode,
          officialPricing: model.official_pricing || null,
          variants: []
        })
      }

      const entry = map.get(key)!
      if (!entry.officialPricing && model.official_pricing) {
        entry.officialPricing = model.official_pricing
      }

      entry.variants.push({
        group,
        model,
        effectiveRate
      })
    }
  }

  const result: AggregatedPlazaModel[] = []

  for (const [key, entry] of map.entries()) {
    const basePricings = entry.variants
      .map((v) => v.model.pricing)
      .filter((p): p is UserSupportedModelPricing => p !== null && p !== undefined)

    const inputRanges = calculateRange(basePricings.map((p) => p.input_price))
    const outputRanges = calculateRange(basePricings.map((p) => p.output_price))
    const cacheWriteRanges = calculateRange(basePricings.map((p) => p.cache_write_price))
    const cacheWrite1hRanges = calculateRange(basePricings.map((p) => p.cache_write_1h_price))
    const cacheReadRanges = calculateRange(basePricings.map((p) => p.cache_read_price))
    const perRequestRanges = calculateRange(basePricings.flatMap((p) => [
      p.per_request_price, ...p.intervals.map(tier => tier.per_request_price)
    ]))
    const imageInputRanges = calculateRange(basePricings.map((p) => p.image_input_price))
    const imageOutputRanges = calculateRange(basePricings.map((p) => p.image_output_price))

    const hasPriceDifferences = Boolean(
      (inputRanges && inputRanges[0] !== inputRanges[1]) ||
      (outputRanges && outputRanges[0] !== outputRanges[1]) ||
      (cacheWriteRanges && cacheWriteRanges[0] !== cacheWriteRanges[1]) ||
      (cacheWrite1hRanges && cacheWrite1hRanges[0] !== cacheWrite1hRanges[1]) ||
      (cacheReadRanges && cacheReadRanges[0] !== cacheReadRanges[1]) ||
      (imageInputRanges && imageInputRanges[0] !== imageInputRanges[1]) ||
      (imageOutputRanges && imageOutputRanges[0] !== imageOutputRanges[1]) ||
      (perRequestRanges && perRequestRanges[0] !== perRequestRanges[1])
    )

    result.push({
      id: key,
      name: entry.name,
      brand: entry.brand,
      billingMode: entry.billingMode,
      officialPricing: entry.officialPricing,
      priceRanges: {
        input: inputRanges,
        output: outputRanges,
        cacheWrite: cacheWriteRanges,
        cacheWrite1h: cacheWrite1hRanges,
        cacheRead: cacheReadRanges,
        perRequest: perRequestRanges,
        imageInput: imageInputRanges,
        imageOutput: imageOutputRanges
      },
      hasPriceDifferences,
      variants: entry.variants.sort((a, b) => a.effectiveRate - b.effectiveRate || a.group.name.localeCompare(b.group.name))
    })
  }

  // 默认按模型名升序排序
  return result.sort((a, b) => a.name.localeCompare(b.name))
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
