import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelsView from '../ChannelsView.vue'
import PricingEntryCard from '@/components/admin/channel/PricingEntryCard.vue'

const { list, update, getAll, showError } = vi.hoisted(() => ({
  list: vi.fn(), update: vi.fn(), getAll: vi.fn(), showError: vi.fn()
}))
vi.mock('@/api/admin', () => ({ adminAPI: {
  channels: { list, update }, groups: { getAll },
  settings: { getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false }) }
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

function mountView() {
  return mount(ChannelsView, { global: { stubs: {
    AppLayout: defineComponent({ template: '<main><slot /></main>' }),
    TablePageLayout: defineComponent({ template: '<section><slot name="filters" /><slot name="table" /></section>' }),
    BaseDialog: defineComponent({ props: ['show'], template: '<section v-if="show"><slot /><slot name="footer" /></section>' }),
    DataTable: defineComponent({ props: ['data'], template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>' }),
    Pagination: true, ConfirmDialog: true, EmptyState: true, Select: true,
    Icon: true, PlatformIcon: true, Toggle: true, ModelMappingEditor: true,
    ModelTagInput: true, TimePricingSection: true
  } } })
}

async function editPricing(wrapper: ReturnType<typeof mountView>) {
  await flushPromises()
  await wrapper.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
  await flushPromises()
  await wrapper.findAll('button.channel-tab').find(b => b.text() === 'admin.groups.platforms.openai')!.trigger('click')
  const card = wrapper.findComponent(PricingEntryCard)
  await card.get('.cursor-pointer').trigger('click')
  return card.get<HTMLInputElement>('input[aria-label="admin.channels.form.ultrafastMultiplier"]')
}

describe('ChannelsView Ultrafast pricing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    const channel = {
      id: 3, name: 'OpenAI', description: '', status: 'active', group_ids: [7],
      model_mapping: {}, billing_model_source: 'requested', features_config: {},
      account_stats_pricing_rules: [],
      model_pricing: [{ platform: 'openai', models: ['gpt-6.1-sol'], billing_mode: 'token',
        input_price: 2e-6, output_price: 10e-6, cache_write_price: 2.5e-6, cache_read_price: 0.1e-6,
        fast_multiplier: 2.5, ultrafast_multiplier: 3.5, intervals: [] }]
    }
    list.mockResolvedValue({ items: [channel], total: 1 })
    getAll.mockResolvedValue([{ id: 7, name: 'Codex', platform: 'openai', status: 'active' }])
    update.mockImplementation(async (_id, payload) => {
      const saved = { ...channel, ...payload }
      list.mockResolvedValue({ items: [saved], total: 1 })
      return saved
    })
  })

  it('saves and reloads an independent Ultrafast multiplier and clears it to the default', async () => {
    const wrapper = mountView()
    let input = await editPricing(wrapper)
    expect(input.element.value).toBe('3.5')
    await input.setValue('1')
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenLastCalledWith(3, expect.objectContaining({
      model_pricing: [expect.objectContaining({ ultrafast_multiplier: 1, fast_multiplier: 2.5 })]
    }))
    input = await editPricing(wrapper)
    expect(input.element.value).toBe('1')
    await input.setValue('')
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenLastCalledWith(3, expect.objectContaining({
      model_pricing: [expect.objectContaining({ ultrafast_multiplier: null, fast_multiplier: 2.5 })]
    }))
    input = await editPricing(wrapper)
    expect(input.element.value).toBe('')
    expect(showError).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('rejects zero and negative multipliers before saving', async () => {
    const wrapper = mountView()
    const input = await editPricing(wrapper)
    for (const invalid of ['0', '-1']) {
      await input.setValue(invalid)
      await wrapper.get('#channel-form').trigger('submit')
      await flushPromises()
      expect(update).not.toHaveBeenCalled()
      expect(showError).toHaveBeenLastCalledWith(expect.stringContaining('admin.channels.form.multiplierPositive'))
    }
    wrapper.unmount()
  })
})
