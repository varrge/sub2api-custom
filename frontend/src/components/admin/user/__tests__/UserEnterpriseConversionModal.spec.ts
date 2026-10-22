import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserEnterpriseConversionModal from '../UserEnterpriseConversionModal.vue'
import type { EnterpriseConversionPreview } from '@/api/admin/users'

const { previewEnterpriseConversion, convertEnterpriseAccount, showSuccess, showError } =
  vi.hoisted(() => ({
    previewEnterpriseConversion: vi.fn(),
    convertEnterpriseAccount: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { previewEnterpriseConversion, convertEnterpriseAccount }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key })
}))

const makePreview = (
  overrides: Partial<EnterpriseConversionPreview> = {}
): EnterpriseConversionPreview => ({
  user_id: 7,
  account_type: 'personal',
  balance: 12,
  cards: [
    {
      id: 1,
      code: 'CARD-1',
      group_id: 1,
      group_name: 'Group A',
      total_quota_usd: 200,
      total_used_usd: 50,
      remaining_usd: 150,
      expires_at: null
    }
  ],
  remaining_usd: 150,
  conversion_rate: null,
  credited_usd: 0,
  balance_after: 12,
  preview_token: 'token-1',
  blockers: [],
  ...overrides
})

const mountModal = async (
  user: Record<string, unknown> = { id: 7, email: 'u@example.test', role: 'user', account_type: 'personal' }
) => {
  const wrapper = mount(UserEnterpriseConversionModal, {
    // 组件通过 show 的 false → true 变化触发打开逻辑（watch 非 immediate），测试需复现该过程。
    props: { show: false, user: user as never },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show', 'title'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>'
        }
      }
    }
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

const deferred = <T>() => {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('UserEnterpriseConversionModal', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    previewEnterpriseConversion.mockReset()
    convertEnterpriseAccount.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    previewEnterpriseConversion.mockResolvedValue(makePreview())
    convertEnterpriseAccount.mockResolvedValue({})
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('parses the rate input safely when v-model yields a number (type=number)', async () => {
    const wrapper = await mountModal()
    await flushPromises()

    const input = wrapper.get('[data-test="conversion-rate-input"]')
    await input.setValue('0.5')
    // v-model on type="number" auto-casts to number; parsing must not crash.
    await vi.advanceTimersByTimeAsync(400)
    await flushPromises()

    expect(previewEnterpriseConversion).toHaveBeenLastCalledWith(7, 0.5)

    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    const confirmBtn = wrapper.get('[data-test="conversion-confirm"]')
    expect(confirmBtn.attributes('disabled')).toBeUndefined()

    await confirmBtn.trigger('click')
    await flushPromises()
    expect(convertEnterpriseAccount).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ conversion_rate: 0.5, preview_token: 'token-1' })
    )
  })

  it('does not require a rate when cards exist but remaining total is zero', async () => {
    previewEnterpriseConversion.mockResolvedValue(
      makePreview({
        cards: [
          {
            id: 1,
            code: 'CARD-1',
            group_id: 1,
            group_name: 'Group A',
            total_quota_usd: 100,
            total_used_usd: 100,
            remaining_usd: 0,
            expires_at: null
          }
        ],
        remaining_usd: 0,
        credited_usd: 0
      })
    )
    const wrapper = await mountModal()
    await flushPromises()

    expect(wrapper.find('[data-test="conversion-rate-input"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="zero-credit-hint"]').exists()).toBe(true)

    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    const confirmBtn = wrapper.get('[data-test="conversion-confirm"]')
    expect(confirmBtn.attributes('disabled')).toBeUndefined()

    await confirmBtn.trigger('click')
    await flushPromises()
    const payload = convertEnterpriseAccount.mock.calls[0]?.[1]
    expect(payload).not.toHaveProperty('conversion_rate')
  })

  it('does not require a rate when there are no cards', async () => {
    previewEnterpriseConversion.mockResolvedValue(
      makePreview({ cards: [], remaining_usd: 0 })
    )
    const wrapper = await mountModal()
    await flushPromises()

    expect(wrapper.find('[data-test="conversion-rate-input"]').exists()).toBe(false)
    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    expect(wrapper.get('[data-test="conversion-confirm"]').attributes('disabled')).toBeUndefined()
  })

  it('invalidates the old preview immediately when the rate changes (before the debounce fires)', async () => {
    const wrapper = await mountModal()
    await flushPromises()

    await wrapper.get('[data-test="conversion-rate-input"]').setValue('0.5')
    await vi.advanceTimersByTimeAsync(400)
    await flushPromises()
    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    expect(wrapper.get('[data-test="conversion-confirm"]').attributes('disabled')).toBeUndefined()

    // Change the rate again: submit must be disabled synchronously, not after 400ms.
    await wrapper.get('[data-test="conversion-rate-input"]').setValue('0.8')
    expect(wrapper.get('[data-test="conversion-confirm"]').attributes('disabled')).toBeDefined()
    expect(
      (wrapper.get('[data-test="conversion-confirm-checkbox"]').element as HTMLInputElement).checked
    ).toBe(false)

    // Debounce only re-fetches the preview; until it resolves the button stays disabled.
    await vi.advanceTimersByTimeAsync(400)
    await flushPromises()
    expect(previewEnterpriseConversion).toHaveBeenLastCalledWith(7, 0.8)
    // Still disabled: the confirmation checkbox was cleared by the rate change.
    expect(wrapper.get('[data-test="conversion-confirm"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    expect(wrapper.get('[data-test="conversion-confirm"]').attributes('disabled')).toBeUndefined()
  })

  it('discards stale preview responses that arrive after a newer request was issued', async () => {
    const initial = deferred<EnterpriseConversionPreview>()
    previewEnterpriseConversion.mockReset()
    previewEnterpriseConversion.mockImplementationOnce(() => initial.promise)

    const wrapper = await mountModal()

    // 首次预览在途时组件仍在加载态，先让它落地。
    initial.resolve(makePreview())
    await flushPromises()

    const raceA = deferred<EnterpriseConversionPreview>()
    const raceB = deferred<EnterpriseConversionPreview>()
    previewEnterpriseConversion
      .mockImplementationOnce(() => raceA.promise)
      .mockImplementationOnce(() => raceB.promise)

    const input = wrapper.get('[data-test="conversion-rate-input"]')
    await input.setValue('0.5')
    await vi.advanceTimersByTimeAsync(400)
    await input.setValue('0.8')
    await vi.advanceTimersByTimeAsync(400)

    // 新请求先返回，旧请求后返回；旧响应必须被丢弃，不能覆盖新结果。
    raceB.resolve(makePreview({ preview_token: 'token-new', conversion_rate: 0.8 }))
    await flushPromises()
    raceA.resolve(makePreview({ preview_token: 'token-stale', conversion_rate: 0.5 }))
    await flushPromises()

    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    await wrapper.get('[data-test="conversion-confirm"]').trigger('click')
    await flushPromises()

    expect(convertEnterpriseAccount).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ preview_token: 'token-new', conversion_rate: 0.8 })
    )
  })

  it('ignores an old response arriving during the new rate debounce', async () => {
    const wrapper = await mountModal()
    const old = deferred<EnterpriseConversionPreview>()
    previewEnterpriseConversion.mockImplementationOnce(() => old.promise)
    const input = wrapper.get('[data-test="conversion-rate-input"]')
    await input.setValue('0.5')
    await vi.advanceTimersByTimeAsync(400)
    await input.setValue('0.8')
    old.resolve(makePreview({ preview_token: 'old-token', conversion_rate: 0.5 }))
    await flushPromises()
    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    expect(wrapper.get('[data-test="conversion-confirm"]').attributes('disabled')).toBeDefined()
    expect(convertEnterpriseAccount).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('retries a failed preview with the entered conversion rate', async () => {
    const wrapper = await mountModal()
    previewEnterpriseConversion.mockRejectedValueOnce(new Error('network'))
    await wrapper.get('[data-test="conversion-rate-input"]').setValue('0.5')
    await vi.advanceTimersByTimeAsync(400)
    await flushPromises()
    previewEnterpriseConversion.mockResolvedValueOnce(makePreview({ conversion_rate: 0.5, credited_usd: 75, balance_after: 87 }))
    await wrapper.get('[data-test="retry-preview"]').trigger('click')
    await flushPromises()
    expect(previewEnterpriseConversion).toHaveBeenLastCalledWith(7, 0.5)
    expect(wrapper.text()).toContain('$75.00')
    expect(wrapper.text()).toContain('$87.00')
    wrapper.unmount()
  })

  it('cancels a pending rate preview when unmounted', async () => {
    const wrapper = await mountModal()
    await wrapper.get('[data-test="conversion-rate-input"]').setValue('0.5')
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(500)
    expect(previewEnterpriseConversion).toHaveBeenCalledTimes(1)
  })

  it('requires a fresh preview and re-confirmation after a 409, reusing the same operation_id', async () => {
    // 无月卡（剩余为 0）时不要求折算倍率，专注于 409 重试流程本身。
    previewEnterpriseConversion.mockReset()
    previewEnterpriseConversion
      .mockResolvedValueOnce(makePreview({ cards: [], remaining_usd: 0, preview_token: 'token-1' }))
      .mockResolvedValueOnce(makePreview({ cards: [], remaining_usd: 0, preview_token: 'token-2' }))

    const wrapper = await mountModal()

    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    convertEnterpriseAccount.mockRejectedValueOnce({ status: 409 })

    await wrapper.get('[data-test="conversion-confirm"]').trigger('click')
    await flushPromises()

    // 409 must clear the confirmation, trigger a re-preview and show the conflict notice.
    expect(showError).toHaveBeenCalledWith('enterpriseAdmin.conversion.conflict')
    expect(wrapper.find('[data-test="conversion-conflict"]').exists()).toBe(true)
    expect(previewEnterpriseConversion).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-test="conversion-confirm"]').attributes('disabled')).toBeDefined()

    // Must not retry convert with the old preview token.
    expect(convertEnterpriseAccount).toHaveBeenCalledTimes(1)
    const firstOperationId = convertEnterpriseAccount.mock.calls[0]?.[1]?.operation_id
    expect(firstOperationId).toBeTruthy()
    expect(convertEnterpriseAccount.mock.calls[0]?.[1]?.preview_token).toBe('token-1')

    // Re-confirm against the new preview and submit again with the same operation_id.
    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    await wrapper.get('[data-test="conversion-confirm"]').trigger('click')
    await flushPromises()

    expect(convertEnterpriseAccount).toHaveBeenCalledTimes(2)
    expect(convertEnterpriseAccount).toHaveBeenLastCalledWith(
      7,
      expect.objectContaining({
        preview_token: 'token-2',
        operation_id: firstOperationId
      })
    )
  })

  it('regenerates the operation_id only when the dialog is reopened', async () => {
    // 无月卡预览：不要求折算倍率，直接聚焦 operation_id 的生命周期。
    previewEnterpriseConversion.mockReset()
    previewEnterpriseConversion.mockResolvedValue(
      makePreview({ cards: [], remaining_usd: 0 })
    )
    const wrapper = await mountModal()
    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    await wrapper.get('[data-test="conversion-confirm"]').trigger('click')
    await flushPromises()
    const firstOperationId = convertEnterpriseAccount.mock.calls[0]?.[1]?.operation_id

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await flushPromises()
    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    await wrapper.get('[data-test="conversion-confirm"]').trigger('click')
    await flushPromises()

    const secondOperationId = convertEnterpriseAccount.mock.calls[1]?.[1]?.operation_id
    expect(secondOperationId).toBeTruthy()
    expect(secondOperationId).not.toBe(firstOperationId)
  })

  it('converts enterprise back to personal without requiring a rate', async () => {
    const wrapper = await mountModal({ id: 9, email: 'e@example.test', role: 'user', account_type: 'enterprise' })
    await flushPromises()

    // No rate input even though remaining > 0.
    expect(wrapper.find('[data-test="conversion-rate-input"]').exists()).toBe(false)

    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    await wrapper.get('[data-test="conversion-confirm"]').trigger('click')
    await flushPromises()

    const payload = convertEnterpriseAccount.mock.calls[0]?.[1]
    expect(payload.account_type).toBe('personal')
    expect(payload).not.toHaveProperty('conversion_rate')
    expect(showSuccess).toHaveBeenCalled()
    expect(wrapper.emitted('success')).toBeTruthy()
  })

  it('keeps the submit button disabled while blockers are present', async () => {
    previewEnterpriseConversion.mockResolvedValue(
      makePreview({ blockers: ['unsettled requests in flight'] })
    )
    const wrapper = await mountModal()
    await flushPromises()

    expect(wrapper.find('[data-test="conversion-blockers"]').exists()).toBe(true)
    await wrapper.get('[data-test="conversion-confirm-checkbox"]').setValue(true)
    expect(wrapper.get('[data-test="conversion-confirm"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="conversion-confirm"]').trigger('click')
    await flushPromises()
    expect(convertEnterpriseAccount).not.toHaveBeenCalled()
  })
})
