import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BulkPricingImport from '../BulkPricingImport.vue'

const api = vi.hoisted(() => ({ syncPricingModels: vi.fn(), getModelDefaultPricing: vi.fn() }))
vi.mock('@/api/admin/channels', () => ({ default: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

function create() {
  return mount(BulkPricingImport, {
    props: { platform: 'anthropic', entries: [] },
    global: { stubs: {
      Icon: true,
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }
    } }
  })
}

function button(wrapper: ReturnType<typeof create>, label: string) {
  return wrapper.findAll('button').find(item => item.text() === `admin.channels.bulkPricing.${label}`)!
}

beforeEach(() => vi.clearAllMocks())

describe('bulk price import form', () => {
  it('previews catalog prices, selects filtered models and only updates the draft after confirmation', async () => {
    api.syncPricingModels.mockResolvedValue({ models: ['claude-a', 'claude-b', 'missing'] })
    api.getModelDefaultPricing.mockImplementation(async (model: string) => ({
      found: model !== 'missing', input_price: model === 'claude-a' ? 3e-6 : 12e-6
    }))
    const wrapper = create()
    await button(wrapper, 'open').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('$3')
    expect(wrapper.text()).toContain('$12')
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
    expect(wrapper.emitted('update')).toBeUndefined()
    await wrapper.get('input[type="search"]').setValue('claude-b')
    await button(wrapper, 'selectVisible').trigger('click')
    await button(wrapper, 'apply').trigger('click')
    expect(wrapper.emitted('update')?.[0]?.[0]).toEqual([expect.objectContaining({ models: ['claude-b'], input_price: 12 })])
    wrapper.unmount()
  })

  it('discarding the dialog prevents a delayed catalog response from updating a new dialog', async () => {
    let finish!: (result: { models: string[] }) => void
    api.syncPricingModels.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const wrapper = create()
    await button(wrapper, 'open').trigger('click')
    await wrapper.findAll('button').find(item => item.text() === 'common.cancel')!.trigger('click')
    finish({ models: ['stale-model'] })
    await flushPromises()
    expect(api.getModelDefaultPricing).not.toHaveBeenCalled()
    expect(wrapper.emitted('update')).toBeUndefined()
    wrapper.unmount()
  })

  it('a failed catalog request offers retry and leaves the form unchanged', async () => {
    api.syncPricingModels.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create()
    await button(wrapper, 'open').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('admin.channels.bulkPricing.failed')
    expect(button(wrapper, 'apply').attributes('disabled')).toBeDefined()
    expect(wrapper.emitted('update')).toBeUndefined()
    wrapper.unmount()
  })
})
