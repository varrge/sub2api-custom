import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UserAllowedGroupsModal from '../UserAllowedGroupsModal.vue'

const { list, update, showError } = vi.hoisted(() => ({ list: vi.fn(), update: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { list }, users: { update } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

async function openModal() {
  const wrapper = mount(UserAllowedGroupsModal, {
    props: { show: false, user: { id: 7, email: 'test@example.test', account_type: 'personal', allowed_groups: [1], group_rates: { 1: 0.8 }, enterprise_group_rates: { '1': 0.6 } } as never },
    global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }, PlatformIcon: true } },
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

describe('enterprise group rates', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    list.mockResolvedValue({ items: [{ id: 1, name: 'Group A', platform: 'openai', subscription_type: 'standard', status: 'active', is_exclusive: true, rate_multiplier: 1 }] })
    update.mockResolvedValue({})
  })

  it('lets a personal user preconfigure an enterprise rate without replacing their ordinary rate', async () => {
    const wrapper = await openModal()
    const enterpriseInput = wrapper.findAll('input[type="number"]').find((input) => (input.element as HTMLInputElement).value === '0.6')!
    expect(enterpriseInput).toBeDefined()
    await enterpriseInput.setValue('0.4')
    await wrapper.findAll('button').find((button) => button.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ group_rates: { 1: 0.8 }, enterprise_group_rates: { '1': 0.4 } }))
    wrapper.unmount()
  })

  it('sends an explicit null when clearing an enterprise rate', async () => {
    const wrapper = await openModal()
    const enterpriseInput = wrapper.findAll('input[type="number"]').find((input) => (input.element as HTMLInputElement).value === '0.6')!
    await enterpriseInput.setValue('')
    await wrapper.findAll('button').find((button) => button.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ group_rates: { 1: 0.8 }, enterprise_group_rates: { '1': null } }))
    wrapper.unmount()
  })

  it('does not copy the previous user configuration after a failed load for another user', async () => {
    const wrapper = await openModal()
    await wrapper.setProps({ show: false })
    list.mockRejectedValueOnce(new Error('network'))
    await wrapper.setProps({ show: true, user: { id: 8, email: 'other@example.test', allowed_groups: [], restrict_public_groups: true, group_rates: { 1: 1 }, enterprise_group_rates: { '1': 0.2 } } as never })
    await flushPromises()
    const save = wrapper.findAll('button').find((button) => button.text() === 'common.save')!
    expect(save.attributes('disabled')).toBeDefined()
    await save.trigger('click')
    expect(update).not.toHaveBeenCalled()
    expect(wrapper.findAll('input[type="number"]')).toHaveLength(0)
    wrapper.unmount()
  })

  it('discards the previous user response after a newer user has loaded', async () => {
    const wrapper = await openModal()
    await wrapper.setProps({ show: false })
    let resolveOld!: (value: unknown) => void
    list.mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
    await wrapper.setProps({ show: true, user: { id: 8, email: 'old@example.test', group_rates: {}, enterprise_group_rates: {} } as never })
    await wrapper.setProps({ user: { id: 9, email: 'new@example.test', group_rates: { 1: 1 }, enterprise_group_rates: { '1': 0.2 } } as never })
    await flushPromises()
    resolveOld({ items: [] })
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(9, expect.objectContaining({ group_rates: { 1: 1 }, enterprise_group_rates: { '1': 0.2 } }))
    wrapper.unmount()
  })

  it('rejects zero without saving any user settings', async () => {
    const wrapper = await openModal()
    const enterpriseInput = wrapper.findAll('input[type="number"]').find((input) => (input.element as HTMLInputElement).value === '0.6')!
    await enterpriseInput.setValue('0')
    await wrapper.findAll('button').find((button) => button.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(update).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalled()
    wrapper.unmount()
  })
})
