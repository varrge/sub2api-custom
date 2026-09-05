import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { aggregatePlazaModels, formatUsdPerMillion, inferModelBrand } from '../plaza-models'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaModelCard from '../PlazaModelCard.vue'
import PlazaModelPricingTable from '../PlazaModelPricingTable.vue'

vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: true }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: null }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: object) => `${key}${params ? JSON.stringify(params) : ''}` }) }))

function model(overrides: Partial<PlazaModel> = {}): PlazaModel {
  return {
    name: 'claude-sonnet-4-5', platform: 'anthropic',
    official_pricing: { input_price: 3e-6, output_price: 15e-6, cache_write_price: null, cache_read_price: null },
    pricing: {
      billing_mode: 'token', input_price: 2e-6, output_price: 10e-6,
      cache_write_price: 3e-6, cache_read_price: 0.2e-6, image_input_price: null,
      image_output_price: null, per_request_price: null, intervals: []
    }, ...overrides
  }
}

function group(overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id: 1, name: 'Balance', description: '', platform: 'anthropic', subscription_type: 'standard',
    rate_multiplier: 0.5, temporary_rate_enabled: false, temporary_rate_multiplier: 0.2,
    temporary_rate_starts_at: null, temporary_rate_ends_at: null,
    peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1,
    is_exclusive: false, image_rate_independent: false, image_rate_multiplier: 1,
    long_context_pricing_enabled: true, models: [model()], ...overrides
  }
}

describe('model card aggregation and rates', () => {
  it('shows one reference price across groups, retaining different modes and composite variants', () => {
    const token = model()
    const perRequest = model({ pricing: { ...token.pricing!, billing_mode: 'per_request', per_request_price: 0.1 } })
    const models = aggregatePlazaModels([
      group({ platform: 'composite', models: [token, model({ platform: 'antigravity' }), perRequest] }),
      group({ id: 2, models: [model({ name: token.name.toUpperCase() })] })
    ])
    expect(models).toHaveLength(2)
    const card = models.find(item => item.billingMode === 'token')!
    expect(card.variants).toHaveLength(3)
    expect(card.officialPricing?.input_price).toBe(3e-6)
    expect(card.priceRanges.input).toEqual([2e-6, 2e-6])
  })

  it('uses user and temporary rates for tokens, and independent image rates only for images', () => {
    const now = Date.parse('2026-09-05T12:00:00Z')
    const image = model({ name: 'gemini-image', pricing: { ...model().pricing!, billing_mode: 'image' } })
    const g = group({
      image_rate_independent: true, image_rate_multiplier: 0.8,
      temporary_rate_enabled: true, temporary_rate_starts_at: '2026-09-01T00:00:00Z',
      temporary_rate_ends_at: '2026-09-06T00:00:00Z', models: [model(), image]
    })
    const rates = (entry: ModelPlazaGroup, time = now) => aggregatePlazaModels([entry], time).map(card => card.variants[0].effectiveRate)
    expect(rates(g)).toEqual([0.2, 0.8])
    expect(rates({ ...g, user_rate_multiplier: 0 })).toEqual([0, 0.8])
    expect(rates(g, Date.parse('2026-09-06T00:00:00Z'))).toEqual([0.5, 0.8])
  })

  it('shows base price ranges and image tiers without inventing missing prices', () => {
    const models = aggregatePlazaModels([
      group({ models: [model({ official_pricing: null })] }),
      group({ id: 2, models: [model({ official_pricing: null, pricing: { ...model().pricing!, input_price: 4e-6 } })] })
    ])
    expect(models[0].hasPriceDifferences).toBe(true)
    expect(models[0].priceRanges.input).toEqual([2e-6, 4e-6])
    expect(formatUsdPerMillion(null)).toBeNull()
    expect(formatUsdPerMillion(0)).toBe('$0')
    const image = model({ pricing: { ...model().pricing!, billing_mode: 'image', per_request_price: null, intervals: [
      { min_tokens: 0, max_tokens: null, tier_label: '1K', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0.1 },
      { min_tokens: 0, max_tokens: null, tier_label: '4K', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0.4 }
    ] } })
    expect(aggregatePlazaModels([group({ models: [image] })])[0].priceRanges.perRequest).toEqual([0.1, 0.4])
  })

  it('classifies brands by model instead of a composite group, without treating GPT nano as Nano Banana', () => {
    expect(inferModelBrand('claude-sonnet-4-5', 'composite').id).toBe('claude')
    expect(inferModelBrand('gpt-5-nano', 'openai').id).toBe('gpt')
    expect(inferModelBrand('gemini-3-pro-image-preview', 'gemini').id).toBe('nano')
    expect(inferModelBrand('gpt-5-codex', 'openai').id).toBe('codex')
  })
})

describe('model card interactions', () => {
  it('labels video prices per second and keeps audio units separate in group details', () => {
    const video = model({ pricing: { ...model().pricing!, billing_mode: 'video', per_request_price: 0.07 } })
    const card = mount(PlazaModelCard, { props: { model: aggregatePlazaModels([group({ models: [video] })])[0] } })
    expect(card.text()).toContain('modelPlaza.card.perSecond')
    expect(card.text()).toContain('$0.07')
    const tiers = ['realtime', 'tts', 'stt'].map((label, i) => ({
      min_tokens: 0, max_tokens: null, tier_label: label,
      input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: i + 1
    }))
    const audio = model({ name: 'grok-voice', pricing: { ...model().pricing!, billing_mode: 'per_request', intervals: tiers } })
    const audioCard = mount(PlazaModelCard, { props: { model: aggregatePlazaModels([group({ models: [audio] })])[0] } })
    expect(audioCard.text()).not.toContain('$1 ~ $3')
    expect(audioCard.text()).toContain('modelPlaza.card.tierDetails')
    const details = mount(PlazaModelPricingTable, { props: { models: [video, audio], rateMultiplier: 1 } })
    for (const unit of ['perSecond', 'perMinute', 'perHour', 'perMillionChars']) {
      expect(details.text()).toContain(`modelPlaza.card.${unit}`)
    }
    card.unmount()
    audioCard.unmount()
    details.unmount()
  })

  it('does not label a group cache price as an official price when the official field is absent', async () => {
    const card = aggregatePlazaModels([group()])[0]
    const wrapper = mount(PlazaModelCard, { props: { model: card } })
    expect(wrapper.text()).toContain('modelPlaza.card.referencePricing')
    expect(wrapper.text()).toContain('$15')
    expect(wrapper.text()).not.toContain('$0.2')
    await wrapper.get('button[aria-expanded]').trigger('click')
    expect(wrapper.get('button[aria-expanded]').attributes('aria-expanded')).toBe('true')
    await wrapper.findAll('button').find(button => button.text() === 'modelPlaza.card.details')!.trigger('click')
    expect(wrapper.emitted('open-group-detail')?.[0][0]).toEqual(card.variants[0])
    wrapper.unmount()
  })

  it('filters a composite catalog by brand and search, and preserves sanitized descriptions', async () => {
    const wrapper = mount(ModelPlazaContent, {
      props: { loading: false, response: { description: '## Billing\n<img src="x" onerror="alert(1)">', groups: [group({
        platform: 'composite', models: [model(), model({ name: 'gpt-5-codex', platform: 'openai' })]
      })] } },
      global: { stubs: { BaseDialog: true, PlazaGroupSection: true } }
    })
    expect(wrapper.findAllComponents(PlazaModelCard)).toHaveLength(2)
    expect(wrapper.find('.plaza-description img').attributes('onerror')).toBeUndefined()
    await wrapper.findAll('button').find(button => button.text() === 'CODEX')!.trigger('click')
    expect(wrapper.findAllComponents(PlazaModelCard)).toHaveLength(1)
    expect(wrapper.findComponent(PlazaModelCard).props('model').name).toBe('gpt-5-codex')
    await wrapper.get('input[type="search"]').setValue('missing')
    expect(wrapper.findAllComponents(PlazaModelCard)).toHaveLength(0)
    expect(wrapper.text()).toContain('modelPlaza.noSearchResult')
    wrapper.unmount()
  })
})
