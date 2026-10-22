import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UserCreateModal from '../UserCreateModal.vue'

const { create } = vi.hoisted(() => ({ create: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: { create } } }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

function mountModal() {
  return mount(UserCreateModal, {
    props: { show: true },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        TotpStepUpDialog: true,
        Icon: true,
      },
    },
  })
}

describe('enterprise account creation', () => {
  beforeEach(() => create.mockReset().mockResolvedValue({}))

  it('submits the explicitly selected enterprise account type', async () => {
    const wrapper = mountModal()
    await wrapper.get('[data-test="create-account-type"]').setValue('enterprise')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledWith(expect.objectContaining({ role: 'user', account_type: 'enterprise' }))
    expect(wrapper.emitted('success')).toHaveLength(1)
    wrapper.unmount()
  })

  it('resets enterprise type to personal when the role becomes admin', async () => {
    const wrapper = mountModal()
    const accountType = wrapper.get('[data-test="create-account-type"]')
    await accountType.setValue('enterprise')
    await wrapper.findAll('select')[0].setValue('admin')
    expect((accountType.element as HTMLSelectElement).value).toBe('personal')
    expect(accountType.attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledWith(expect.objectContaining({ role: 'admin', account_type: 'personal' }))
    wrapper.unmount()
  })
})
