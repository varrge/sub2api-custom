import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaGroupSection from '../PlazaGroupSection.vue'
import PlazaFilterBar from '../PlazaFilterBar.vue'
import type { ModelPlazaGroup, ModelPlazaResponse } from '@/api/modelPlaza'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({
    t: (key: string, params?: { selected: number; total: number }) =>
      params ? `${key}: ${params.selected}/${params.total}` : key
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: true })
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: null }) }))

function group(id: number, name: string, platform: string, rate: number): ModelPlazaGroup {
  return {
    id,
    name,
    platform,
    description: '',
    subscription_type: 'standard',
    rate_multiplier: rate,
    temporary_rate_enabled: false,
    temporary_rate_multiplier: 1,
    temporary_rate_starts_at: null,
    temporary_rate_ends_at: null,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    long_context_pricing_enabled: true,
    models: ['shared-model', `${platform}-model`].map((name) => ({
      name, platform, pricing: null, official_pricing: null
    }))
  }
}

function response(): ModelPlazaResponse {
  return {
    description: '',
    groups: [
      group(1, 'Alpha', 'openai', 0.5),
      group(2, 'Beta', 'openai', 1),
      group(3, 'Gamma', 'anthropic', 0.5),
      { ...group(4, 'Delta', 'anthropic', 2), user_rate_multiplier: 1 }
    ]
  }
}

let wrapper: VueWrapper

afterEach(() => wrapper?.unmount())

function mountPlaza(data = response()) {
  wrapper = mount(ModelPlazaContent, {
    props: { response: data, loading: false },
    global: { stubs: { Icon: true, PlatformIcon: true, PlazaGroupSection: true } }
  })
}

function button(text: string) {
  const found = wrapper.findAll('button').find((node) => node.text().trim() === text)
  if (!found) throw new Error(`Button not found: ${text}`)
  return found
}

function shownGroups(): ModelPlazaGroup[] {
  return wrapper.findAllComponents(PlazaGroupSection).map((section) => section.props('group'))
}

const selectAll = 'modelPlaza.filters.selectAll'
const clear = 'modelPlaza.filters.clearGroups'

describe('ModelPlazaContent group multi-selection', () => {
  it('renders top filters followed by group pricing tables, with sanitized page description', () => {
    const data = response()
    data.description = '## Prices\n<img src="x" onerror="alert(1)">'
    data.groups[0].models[0].pricing = {
      billing_mode: 'token', input_price: 2e-6, output_price: 10e-6,
      cache_write_price: null, cache_read_price: null, image_input_price: null,
      image_output_price: null, per_request_price: null, intervals: []
    }
    wrapper = mount(ModelPlazaContent, {
      props: { response: data, loading: false },
      global: { stubs: { Icon: true, PlatformIcon: true } }
    })
    const filters = wrapper.getComponent(PlazaFilterBar).element
    const section = wrapper.get('section').element
    expect(filters.compareDocumentPosition(section) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(wrapper.find('aside').exists()).toBe(false)
    expect(wrapper.find('[data-model]').exists()).toBe(false)
    expect(wrapper.findAll('section')).toHaveLength(4)
    expect(wrapper.get('section table').text()).toContain('shared-model')
    expect(wrapper.get('section table').text()).toContain('$1.00')
    expect(wrapper.get('.plaza-description img').attributes('onerror')).toBeUndefined()
  })

  it('shows all group prices initially, sorted by effective personal rate', () => {
    mountPlaza()
    expect(shownGroups().map((g) => g.name)).toEqual(['Alpha', 'Gamma', 'Beta', 'Delta'])
    expect(button(selectAll).attributes('aria-pressed')).toBe('true')
    expect(wrapper.text()).toContain('modelPlaza.filters.selectedGroups: 4/4')
  })

  it('selects several groups, removes one, and restores every group with select all', async () => {
    mountPlaza()
    await button(clear).trigger('click')
    await button('Alpha').trigger('click')
    expect(shownGroups().map((g) => g.id)).toEqual([1])
    await button('Gamma').trigger('click')
    expect(shownGroups().map((g) => g.id)).toEqual([1, 3])
    expect(button('Alpha').attributes('aria-pressed')).toBe('true')
    expect(button('Gamma').attributes('aria-pressed')).toBe('true')
    await button('Alpha').trigger('click')
    expect(shownGroups().map((g) => g.id)).toEqual([3])
    await button(selectAll).trigger('click')
    expect(shownGroups()).toHaveLength(4)
  })

  it('unchecks only the clicked group after selecting all', async () => {
    mountPlaza()
    await button('Alpha').trigger('click')
    expect(shownGroups().map((g) => g.id)).toEqual([3, 2, 4])
    expect(button('Alpha').attributes('aria-pressed')).toBe('false')
    await button('Alpha').trigger('click')
    expect(shownGroups()).toHaveLength(4)
  })

  it('keeps an intentional empty selection and permits choosing a different platform', async () => {
    mountPlaza()
    await button(clear).trigger('click')
    expect(shownGroups()).toHaveLength(0)
    expect(wrapper.text()).toContain('modelPlaza.selectGroupsHint')
    expect(button('openai').attributes('disabled')).toBeUndefined()
    await button('openai').trigger('click')
    await button('Beta').trigger('click')
    expect(shownGroups().map((g) => g.id)).toEqual([2])
    await button('Beta').trigger('click')
    expect(shownGroups()).toHaveLength(0)
    await button(selectAll).trigger('click')
    expect(shownGroups().map((g) => g.id)).toEqual([1, 2])
  })

  it('uses the union of selected groups for platform and rate availability', async () => {
    mountPlaza()
    await button(clear).trigger('click')
    await button('Alpha').trigger('click')
    expect(button('anthropic').attributes('disabled')).toBeDefined()
    expect(button('1x').attributes('disabled')).toBeDefined()
    await button('Delta').trigger('click')
    expect(button('anthropic').attributes('disabled')).toBeUndefined()
    expect(button('1x').attributes('disabled')).toBeUndefined()
    await button('1x').trigger('click')
    expect(shownGroups().map((g) => g.id)).toEqual([4])
    await button(selectAll).trigger('click')
    expect(shownGroups().map((g) => g.id)).toEqual([2, 4])
    expect(wrapper.text()).toContain('modelPlaza.filters.selectedGroups: 2/2')
  })

  it('counts only available selections and allows removing a selection hidden by another filter', async () => {
    mountPlaza()
    await button(clear).trigger('click')
    await button('Alpha').trigger('click')
    await button('Gamma').trigger('click')
    await button('openai').trigger('click')
    expect(shownGroups().map((g) => g.id)).toEqual([1])
    expect(wrapper.text()).toContain('modelPlaza.filters.selectedGroups: 1/2')
    expect(button('Gamma').attributes('disabled')).toBeUndefined()
    await button('Gamma').trigger('click')
    expect(button('Gamma').attributes('aria-pressed')).toBe('false')
    expect(shownGroups().map((g) => g.id)).toEqual([1])
  })

  it('searches model names across all selected groups without changing the original pricing data', async () => {
    const data = response()
    mountPlaza(data)
    await button(clear).trigger('click')
    await button('Alpha').trigger('click')
    await button('Gamma').trigger('click')
    await wrapper.get('input').setValue('  SHARED  ')
    expect(shownGroups().map((g) => g.id)).toEqual([1, 3])
    expect(shownGroups().every((g) => g.models.length === 1 && g.models[0]?.name === 'shared-model')).toBe(true)
    expect(data.groups.every((g) => g.models.length === 2)).toBe(true)
    await wrapper.get('input').setValue('anthropic')
    expect(shownGroups().map((g) => g.id)).toEqual([3])
    await wrapper.get('input').setValue('no-such-model')
    expect(shownGroups()).toHaveLength(0)
    expect(wrapper.text()).toContain('modelPlaza.noSearchResult')
  })

  it('preserves surviving selections after a refresh and recovers when every selected group disappears', async () => {
    mountPlaza()
    await button(clear).trigger('click')
    await button('Alpha').trigger('click')
    await button('Gamma').trigger('click')
    const data = response()
    data.groups = data.groups.filter((g) => g.id !== 1)
    await wrapper.setProps({ response: data })
    expect(shownGroups().map((g) => g.id)).toEqual([3])
    await wrapper.setProps({ response: { ...data, groups: data.groups.filter((g) => g.id !== 3) } })
    expect(shownGroups().map((g) => g.id)).toEqual([2, 4])
    expect(button(selectAll).attributes('aria-pressed')).toBe('true')
  })

  it('includes newly loaded groups in select-all mode but preserves a cleared selection on refresh', async () => {
    mountPlaza()
    const data = response()
    data.groups.push(group(5, 'Epsilon', 'openai', 3))
    await wrapper.setProps({ response: data })
    expect(shownGroups()).toHaveLength(5)
    await button(clear).trigger('click')
    await wrapper.setProps({ response: response() })
    expect(shownGroups()).toHaveLength(0)
    expect(wrapper.text()).toContain('modelPlaza.selectGroupsHint')
  })
})
