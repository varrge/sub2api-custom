import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ResetPasswordView from '../ResetPasswordView.vue'
const mocks = vi.hoisted(() => ({ reset: vi.fn(), error: vi.fn(), success: vi.fn() }))
vi.mock('@/api/auth', () => ({ resetPassword: mocks.reset }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError: mocks.error, showSuccess: mocks.success }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: { email: 'test@example.test', token: 'fixture-token' } }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/layout', () => ({ AuthLayout: { template: '<div><slot /></div>' } }))
const setup = async () => {
 const wrapper = mount(ResetPasswordView, { global: { stubs: { Icon: true, RouterLink: true } } })
 await flushPromises()
 await wrapper.get('#password').setValue('a-test-password')
 await wrapper.get('#confirmPassword').setValue('a-test-password')
 return wrapper
}
beforeEach(() => vi.resetAllMocks())
describe('administrator recovery', () => {
 it('submits the independent factor and preserves it after a rejected attempt', async () => {
  mocks.reset.mockRejectedValueOnce({ message: 'Existing authenticator required' })
  const wrapper = await setup()
  await wrapper.get('#recovery-totp').setValue('123456')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(mocks.reset).toHaveBeenCalledWith({ email: 'test@example.test', token: 'fixture-token', new_password: 'a-test-password', totp_code: '123456' })
  expect(mocks.error).toHaveBeenCalledWith('Existing authenticator required')
  expect(wrapper.find('form').exists()).toBe(true)
 })
 it('keeps ordinary email recovery available without an authenticator field value', async () => {
  mocks.reset.mockResolvedValue({})
  const wrapper = await setup(); await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(mocks.reset.mock.calls[0][0].totp_code).toBe('')
  expect(mocks.success).toHaveBeenCalled()
 })
})
