import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import MonthCardRefundConfirmDialog from '../MonthCardRefundConfirmDialog.vue'
import { canConfirmMonthCardRefund, refundReferenceBytes, validRefundReference } from '../refundConfirmation'
import type { PaymentOrder } from '@/types/payment'
import { adminPaymentAPI } from '@/api/admin/payment'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { confirmMonthCardRefund: vi.fn(), refundOrder: vi.fn(), queryRefund: vi.fn() } }))
const now = Date.parse('2026-09-10T09:00:00Z')
const order: PaymentOrder = { id: 18, user_id: 2, amount: 198, pay_amount: 198, currency: 'CNY', fee_rate: 0, payment_type: 'wxpay', out_trade_no: 'pay-18', status: 'REFUND_PENDING', order_type: 'month_card', created_at: '2026-09-01T00:00:00Z', updated_at: new Date(now - 300_001).toISOString(), expires_at: '2026-09-01T00:30:00Z', refund_amount: 198, refund_reason: 'Verified by support' }

describe('month-card refund confirmation eligibility', () => {
  it.each(['REFUND_PENDING', 'REFUND_FAILED', 'REFUNDING', 'COMPLETED'] as const)('accepts a prior full refund in %s', status => {
    expect(canConfirmMonthCardRefund({ ...order, status }, now)).toBe(true)
  })
  it.each([
    { order_type: 'subscription' }, { order_type: 'balance' }, { refund_amount: 197.99 }, { refund_reason: undefined }, { refund_reason: '' }, { refund_reason: '  ' },
    { status: 'REFUNDED' }, { status: 'REFUND_REQUESTED' }, { status: 'PARTIALLY_REFUNDED' }, { status: 'PENDING' },
    { status: 'REFUNDING', updated_at: new Date(now - 300_000).toISOString() },
    { status: 'REFUNDING', updated_at: new Date(now + 1000).toISOString() },
    { status: 'REFUNDING', updated_at: 'invalid' }, { status: 'REFUNDING', updated_at: undefined },
    { status: 'COMPLETED', refund_amount: 0, refund_reason: undefined }
  ] as Partial<PaymentOrder>[])('rejects an ineligible refund %#', change => {
    expect(canConfirmMonthCardRefund({ ...order, ...change }, now)).toBe(false)
  })
  it('counts trimmed UTF-8 bytes, including multibyte verification details', () => {
    expect(refundReferenceBytes('  核实  ')).toBe(6)
    expect(validRefundReference('ab')).toBe(false)
    expect(validRefundReference('abc')).toBe(true)
    expect(validRefundReference('a'.repeat(500))).toBe(true)
    expect(validRefundReference('a'.repeat(501))).toBe(false)
    expect(validRefundReference('中'.repeat(166))).toBe(true)
    expect(validRefundReference('中'.repeat(167))).toBe(false)
  })
})

describe('verified refund confirmation dialog', () => {
  const wrappers: ReturnType<typeof mount>[] = []
  function render(current = order) {
    const wrapper = mount(MonthCardRefundConfirmDialog, { props: { order: current }, global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' } } } })
    wrappers.push(wrapper)
    return wrapper
  }
  beforeEach(() => { vi.clearAllMocks() })
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

  it('requires provider reference and explicit full-refund verification before calling the confirmation endpoint', async () => {
    vi.mocked(adminPaymentAPI.confirmMonthCardRefund).mockResolvedValue({ data: { success: true } } as never)
    const wrapper = render()
    expect(wrapper.text()).toContain('groupBuy.confirmRefundHint')
    expect(wrapper.text()).toContain('¥198.00')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('textarea').setValue('  provider-refund-18  ')
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.confirmMonthCardRefund).not.toHaveBeenCalled()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(adminPaymentAPI.confirmMonthCardRefund).toHaveBeenCalledTimes(1)
    expect(adminPaymentAPI.confirmMonthCardRefund).toHaveBeenCalledWith(18, { reference: 'provider-refund-18', confirmed: true })
    expect(adminPaymentAPI.refundOrder).not.toHaveBeenCalled()
    expect(adminPaymentAPI.queryRefund).not.toHaveBeenCalled()
    expect(wrapper.emitted('confirmed')).toHaveLength(1)
  })
  it('rejects overlength Unicode references and an order that becomes ineligible', async () => {
    const wrapper = render()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('textarea').setValue('中'.repeat(167))
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.confirmMonthCardRefund).not.toHaveBeenCalled()
    await wrapper.get('textarea').setValue('provider-refund')
    await wrapper.setProps({ order: { ...order, status: 'REFUNDED' } })
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.confirmMonthCardRefund).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('groupBuy.refundConfirmationUnavailable')
  })
  it('keeps the dialog open and displays a server conflict without claiming success', async () => {
    vi.mocked(adminPaymentAPI.confirmMonthCardRefund).mockRejectedValue({ message: 'Order status changed', status: 409 })
    const wrapper = render()
    await wrapper.get('textarea').setValue('provider-reference')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('Order status changed')
    expect(wrapper.emitted('confirmed')).toBeUndefined()
    expect(wrapper.emitted('close')).toBeUndefined()
  })
  it('prevents repeated submits while the confirmation is in progress', async () => {
    let resolve!: (value: never) => void
    vi.mocked(adminPaymentAPI.confirmMonthCardRefund).mockReturnValue(new Promise(done => { resolve = done }))
    const wrapper = render()
    await wrapper.get('textarea').setValue('provider-reference')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.confirmMonthCardRefund).toHaveBeenCalledTimes(1)
    resolve({ data: { success: true } } as never)
    await flushPromises()
  })
})
