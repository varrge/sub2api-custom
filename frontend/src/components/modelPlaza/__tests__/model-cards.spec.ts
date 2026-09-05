import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { aggregatePlazaModels, formatUsdPerMillion, inferModelBrand, modelPriceCells, plazaModelType, selectPriceVariant, variantKey } from '../plaza-models'
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
    pricing: { billing_mode: 'token', input_price: 2e-6, output_price: 10e-6, cache_write_price: 3e-6, cache_read_price: 0.2e-6, image_input_price: null, image_output_price: null, per_request_price: null, intervals: [] },
    ...overrides
  }
}
function group(overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id: 1, name: 'Balance', description: '', platform: 'anthropic', subscription_type: 'standard',
    rate_multiplier: 0.5, temporary_rate_enabled: false, temporary_rate_multiplier: 0.2,
    temporary_rate_starts_at: null, temporary_rate_ends_at: null, peak_rate_enabled: false,
    peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: false,
    image_rate_independent: false, image_rate_multiplier: 1, long_context_pricing_enabled: true,
    models: [model()], ...overrides
  }
}
function mountCatalog(groups: ModelPlazaGroup[]) {
  return mount(ModelPlazaContent, {
    props: { loading: false, response: { description: '## Billing\n<img src="x" onerror="alert(1)">', groups } },
    global: { stubs: { BaseDialog: true, PlazaGroupSection: true } }
  })
}

describe('catalog prices', () => {
  it('keeps composite platform variants and different billing modes separate', () => {
    const token = model()
    const request = model({ pricing: { ...token.pricing!, billing_mode: 'per_request', per_request_price: 0.1 } })
    const cards = aggregatePlazaModels([
      group({ platform: 'composite', models: [token, model({ platform: 'antigravity' }), request] }),
      group({ id: 2, models: [model({ name: token.name.toUpperCase() })] })
    ])
    expect(cards).toHaveLength(2)
    expect(cards.find(card => card.billingMode === 'token')!.variants).toHaveLength(3)
  })
  it('calculates from selected custom pricing, preserving nulls, zero and 1h cache rates', () => {
    const entry = model({ pricing: { ...model().pricing!, input_price: 0, output_price: null, cache_write_1h_price: 8e-6 } })
    const card = aggregatePlazaModels([group({ models: [entry] })])[0]
    const cells = modelPriceCells(card.variants[0])
    expect(cells.find(cell => cell.id === 'input_price')).toMatchObject({ price: 0, original: 3 })
    expect(cells.find(cell => cell.id === 'output_price')).toMatchObject({ price: null, original: 15 })
    expect(cells.find(cell => cell.id === 'cache_write_price')).toMatchObject({ price: 1.5, original: null })
    expect(cells.find(cell => cell.id === 'cache_write_1h_price')).toMatchObject({ price: 4 })
    expect(formatUsdPerMillion(null)).toBeNull()
    expect(formatUsdPerMillion(0)).toBe('$0')
  })
  it('resolves temporary and user rates while using independent image/video multipliers only for those modes', () => {
    const now = Date.parse('2026-09-05T12:00:00Z')
    const image = model({ name: 'image', pricing: { ...model().pricing!, billing_mode: 'image' } })
    const video = model({ name: 'video', pricing: { ...model().pricing!, billing_mode: 'video' } })
    const g = group({
      image_rate_independent: true, image_rate_multiplier: 0.8, video_rate_independent: true, video_rate_multiplier: 0.7,
      temporary_rate_enabled: true, temporary_rate_starts_at: '2026-09-01T00:00:00Z', temporary_rate_ends_at: '2026-09-06T00:00:00Z',
      models: [model(), image, video]
    })
    const rates = (entry: ModelPlazaGroup, time = now) => aggregatePlazaModels([entry], time).map(card => card.variants[0].effectiveRate)
    expect(rates(g)).toEqual([0.2, 0.8, 0.7])
    expect(rates({ ...g, user_rate_multiplier: 0 })).toEqual([0, 0.8, 0.7])
    expect(rates(g, Date.parse('2026-09-06T00:00:00Z'))).toEqual([0.5, 0.8, 0.7])
  })
  it('does not substitute another group for an unsupported reference group', () => {
    const card = aggregatePlazaModels([group()])[0]
    expect(selectPriceVariant(card, 999)).toBeNull()
    expect(selectPriceVariant(card, 999, variantKey(card.variants[0]))).toEqual(card.variants[0])
  })
  it('merges supplier families and distinguishes image output from image input', () => {
    expect(inferModelBrand('gpt-5-nano', 'openai').id).toBe('openai')
    expect(inferModelBrand('gpt-5-codex', 'openai').id).toBe('openai')
    expect(inferModelBrand('gemini-3-pro-image-preview', 'gemini').id).toBe('gemini')
    expect(inferModelBrand('claude-sonnet', 'composite').id).toBe('claude')
    expect(plazaModelType(model({ metadata: { supports_vision: true, mode: 'chat' } }))).toBe('text')
    expect(plazaModelType(model({ metadata: { mode: 'image_generation' } }))).toBe('image')
    expect(plazaModelType(model({ metadata: { mode: 'video_generation' } }))).toBe('video')
  })
  it('keeps audio tier dimensions and video per-second units in card and details', () => {
    const video = model({ pricing: { ...model().pricing!, billing_mode: 'video', per_request_price: 0.07 } })
    const card = mount(PlazaModelCard, { props: { model: aggregatePlazaModels([group({ models: [video], rate_multiplier: 1 })])[0], priceGroupId: 1 } })
    expect(card.text()).toContain('modelPlaza.card.perSecond')
    expect(card.text()).toContain('$0.07')
    const tiers = ['realtime', 'tts', 'stt'].map((label, i) => ({
      min_tokens: 0, max_tokens: null, tier_label: label, input_price: null, output_price: null,
      cache_write_price: null, cache_read_price: null, per_request_price: i + 1
    }))
    const audio = model({ name: 'grok-voice', pricing: { ...model().pricing!, billing_mode: 'per_request', intervals: tiers } })
    const audioCard = mount(PlazaModelCard, { props: { model: aggregatePlazaModels([group({ models: [audio] })])[0], priceGroupId: 1 } })
    const details = mount(PlazaModelPricingTable, { props: { models: [video, audio], rateMultiplier: 1 } })
    for (const unit of ['perMinute', 'perHour', 'perMillionChars']) {
      expect(audioCard.text()).toContain(`modelPlaza.card.${unit}`)
      expect(details.text()).toContain(`modelPlaza.card.${unit}`)
    }
    expect(details.text()).toContain('modelPlaza.card.perSecond')
    const videoDetails = mount(PlazaModelPricingTable, { props: { models: [video], rateMultiplier: 0.5, videoRateIndependent: true, videoRateMultiplier: 0.2 } })
    expect(videoDetails.text()).toContain('0.2x')
    expect(videoDetails.text()).toContain('0.014')
    videoDetails.unmount()
    card.unmount(); audioCard.unmount(); details.unmount()
  })
})

describe('catalog selection and interactions', () => {
  it('switches card prices via chips, survives rate refresh and resets when reference changes', async () => {
    const groups = [group(), group({ id: 2, name: 'Subscription', rate_multiplier: 0.2 })]
    const wrapper = mount(PlazaModelCard, { props: { model: aggregatePlazaModels(groups)[0], priceGroupId: 1 } })
    expect(wrapper.get('[data-price="input_price"]').text()).toBe('$1')
    expect(wrapper.find('s').text()).toBe('$3')
    await wrapper.findAll('button[aria-pressed]').find(button => button.text().includes('Subscription'))!.trigger('click')
    expect(wrapper.get('[data-price="input_price"]').text()).toBe('$0.4')
    await wrapper.setProps({ model: aggregatePlazaModels(groups)[0] })
    expect(wrapper.get('[data-price="input_price"]').text()).toBe('$0.4')
    await wrapper.setProps({ priceGroupId: 999 })
    expect(wrapper.find('[data-price]').exists()).toBe(false)
    expect(wrapper.text()).toContain('modelPlaza.catalog.unavailableInReference')
    await wrapper.findAll('button[aria-pressed]').find(button => button.text().includes('Balance'))!.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === 'modelPlaza.pricingDetail')!.trigger('click')
    expect(wrapper.emitted('open-group-detail')?.[0][0]).toMatchObject({ group: { id: 1 } })
    wrapper.unmount()
  })
  it('reference selector changes prices without filtering; group selector narrows models', async () => {
    const extra = model({ name: 'gpt-5-codex', platform: 'openai' })
    const wrapper = mountCatalog([group(), group({ id: 2, name: 'Other', rate_multiplier: 0.2, models: [model(), extra] })])
    expect(wrapper.findAllComponents(PlazaModelCard)).toHaveLength(2)
    expect(wrapper.findAllComponents(PlazaModelCard)[1].text()).toContain('modelPlaza.catalog.unavailableInReference')
    await wrapper.get('#catalog-desktop-reference').setValue('2')
    expect(wrapper.findAllComponents(PlazaModelCard)).toHaveLength(2)
    expect(wrapper.findAllComponents(PlazaModelCard)[1].find('[data-price="input_price"]').text()).toBe('$0.4')
    await wrapper.get('#catalog-desktop-group').setValue('1')
    expect(wrapper.findAllComponents(PlazaModelCard)).toHaveLength(1)
    expect(wrapper.find('#catalog-desktop-reference').exists()).toBe(false)
    await wrapper.get('#catalog-desktop-group').setValue('')
    expect(wrapper.findAllComponents(PlazaModelCard)).toHaveLength(2)
    expect(wrapper.get('#catalog-desktop-reference').element).toHaveProperty('value', '2')
    wrapper.unmount()
  })
  it('handles removed reference groups, supplier filters, search and sanitized Markdown', async () => {
    const groups = [group({ platform: 'composite', models: [model(), model({ name: 'gpt-5-codex', platform: 'openai' })] })]
    const wrapper = mountCatalog(groups)
    expect(wrapper.find('.plaza-description img').attributes('onerror')).toBeUndefined()
    await wrapper.findAll('button').find(button => button.text().startsWith('OpenAI'))!.trigger('click')
    expect(wrapper.findAllComponents(PlazaModelCard)).toHaveLength(1)
    await wrapper.get('input[type="search"]').setValue('missing')
    expect(wrapper.findAllComponents(PlazaModelCard)).toHaveLength(0)
    expect(wrapper.text()).toContain('modelPlaza.noSearchResult')
    await wrapper.setProps({ response: { groups: [], description: '' } })
    expect(wrapper.get('#catalog-desktop-reference').element).toHaveProperty('value', '')
    wrapper.unmount()
  })
  it('only displays known metadata and separates input image capability from generation type', () => {
    const wrapper = mount(PlazaModelCard, { props: {
      model: aggregatePlazaModels([group({ models: [model({ metadata: { context_window: 200000, max_output_tokens: 64000, supports_vision: true } })] })])[0],
      priceGroupId: 1
    } })
    expect(wrapper.text()).toContain('200K')
    expect(wrapper.text()).toContain('64K')
    expect(wrapper.find('[aria-label="modelPlaza.catalog.vision"]').exists()).toBe(true)
    wrapper.unmount()
  })
})
