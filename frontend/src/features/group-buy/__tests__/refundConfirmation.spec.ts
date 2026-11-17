import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import MonthCardRefundConfirmDialog from '../MonthCardRefundConfirmDialog.vue'
import { canConfirmBalanceRefund, canConfirmMonthCardRefund, canConfirmRefund, canQueryRefund, refundReferenceBytes, validRefundReference } from '../refundConfirmation'
import type { PaymentOrder } from '@/types/payment'
import { adminPaymentAPI } from '@/api/admin/payment'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key, locale: 'en-US' }) }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { confirmMonthCardRefund: vi.fn(), confirmBalanceRefund: vi.fn(), refundOrder: vi.fn(), queryRefund: vi.fn() } }))
const now = Date.parse('2026-09-10T09:00:00Z')
const order: PaymentOrder = { id: 18, user_id: 2, amount: 198, pay_amount: 198, currency: 'CNY', fee_rate: 0, payment_type: 'wxpay', out_trade_no: 'pay-18', status: 'REFUND_PENDING', order_type: 'month_card', created_at: '2026-09-01T00:00:00Z', updated_at: new Date(now - 300_001).toISOString(), expires_at: '2026-09-01T00:30:00Z', refund_amount: 198, refund_reason: 'Verified by support' }
const balanceOrder: PaymentOrder = { ...order, order_type: 'balance', amount: 100, pay_amount: 80, refund_amount: 25, currency: 'EUR' }

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

describe('balance refund confirmation and query eligibility', () => {
  it.each(['REFUND_PENDING', 'REFUNDING'] as const)('accepts %s without month-card full-refund requirements', status => {
    const current = { ...balanceOrder, status, refund_amount: 0, refund_reason: undefined }
    expect(canConfirmBalanceRefund(current, now)).toBe(true)
    expect(canConfirmRefund(current, now)).toBe(true)
    expect(canQueryRefund(current, now)).toBe(true)
  })
  it.each([
    { status: 'COMPLETED' }, { status: 'REFUND_FAILED' }, { status: 'REFUNDED' }, { status: 'REFUND_REQUESTED' },
    { status: 'REFUNDING', updated_at: new Date(now - 300_000).toISOString() },
    { status: 'REFUNDING', updated_at: new Date(now - 299_999).toISOString() },
    { status: 'REFUNDING', updated_at: new Date(now + 1000).toISOString() },
    { status: 'REFUNDING', updated_at: 'invalid' }, { status: 'REFUNDING', updated_at: undefined },
  ] as Partial<PaymentOrder>[])('rejects unsafe confirmation and query %#', change => {
    const current = { ...balanceOrder, ...change }
    expect(canConfirmBalanceRefund(current, now)).toBe(false)
    expect(canQueryRefund(current, now)).toBe(false)
  })
  it('does not expand confirmation to subscription orders', () => {
    expect(canConfirmRefund({ ...balanceOrder, order_type: 'subscription' }, now)).toBe(false)
  })
  it.each(['month_card', 'subscription'] as const)('only offers queries for pending %s refunds, not stale REFUNDING', order_type => {
    expect(canQueryRefund({ ...order, order_type, status: 'REFUND_PENDING' }, now)).toBe(true)
    expect(canQueryRefund({ ...order, order_type, status: 'REFUNDING' }, now)).toBe(false)
  })
})

describe('verified refund confirmation dialog', () => {
  const wrappers: ReturnType<typeof mount>[] = []
  function render(current = order) {
    const wrapper = mount(MonthCardRefundConfirmDialog, { props: { order: current }, global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' } } } })
    wrappers.push(wrapper)
    return wrapper
  }
  beforeEach(() => { vi.clearAllMocks(); vi.useFakeTimers(); vi.setSystemTime(now) })
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers() })

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

  it.each([
    { outcome: 'success', result: { success: true } },
    { outcome: 'failed', result: { success: false, resolved: true, warning: 'Verified failed; balance restored' } },
  ])('requires explicit outcome, evidence and verification for a $outcome balance refund', async ({ outcome, result }) => {
    vi.mocked(adminPaymentAPI.confirmBalanceRefund).mockResolvedValue({ data: result } as never)
    const wrapper = render(balanceOrder)
    expect(wrapper.text()).toContain('payment.admin.verifyBalanceRefundHint')
    expect(wrapper.text()).toContain('€80.00')
    expect(wrapper.text()).toContain('€20.00')
    expect(wrapper.text()).not.toContain('groupBuy.confirmRefundHint')
    expect(wrapper.findAll('input[type="radio"]').every(input => !(input.element as HTMLInputElement).checked)).toBe(true)
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    await wrapper.get('textarea').setValue('  Verified provider evidence  ')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.confirmBalanceRefund).not.toHaveBeenCalled()
    await wrapper.get(`input[value="${outcome}"]`).setValue(true)
    await wrapper.get('input[type="checkbox"]').setValue(false)
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.confirmBalanceRefund).not.toHaveBeenCalled()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(adminPaymentAPI.confirmBalanceRefund).toHaveBeenCalledTimes(1)
    expect(adminPaymentAPI.confirmBalanceRefund).toHaveBeenCalledWith(balanceOrder.id, {
      reference: 'Verified provider evidence', confirmed: true, outcome, updated_at: balanceOrder.updated_at,
    })
    expect(adminPaymentAPI.confirmMonthCardRefund).not.toHaveBeenCalled()
    expect(wrapper.emitted('confirmed')).toHaveLength(1)
  })

  it('keeps unresolved balance confirmations open without refreshing', async () => {
    vi.mocked(adminPaymentAPI.confirmBalanceRefund).mockResolvedValue({ data: { success: false, warning: 'Still unresolved' } } as never)
    const wrapper = render(balanceOrder)
    await wrapper.get('input[value="failed"]').setValue(true)
    await wrapper.get('textarea').setValue('provider evidence')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('Still unresolved')
    expect(wrapper.emitted('confirmed')).toBeUndefined()
  })

  it('keeps month-card success semantics when resolved is returned without success', async () => {
    vi.mocked(adminPaymentAPI.confirmMonthCardRefund).mockResolvedValue({ data: { success: false, resolved: true, warning: 'Unconfirmed full refund' } } as never)
    const wrapper = render()
    await wrapper.get('textarea').setValue('provider evidence')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('confirmed')).toBeUndefined()
    expect(wrapper.find('input[type="radio"]').exists()).toBe(false)
  })

  it('requires explicitly reconciled legacy amounts, including a manually entered zero deduction', async () => {
    vi.mocked(adminPaymentAPI.confirmBalanceRefund)
      .mockRejectedValueOnce({ reason: 'REFUND_RECONCILIATION_REQUIRED', message: 'Verify legacy balance deductions' })
      .mockResolvedValueOnce({ data: { success: false, resolved: true } } as never)
    const wrapper = render({ ...balanceOrder, status: 'REFUNDING' })
    expect(wrapper.find('input[type="number"]').exists()).toBe(false)
    await wrapper.get('input[value="failed"]').setValue(true)
    await wrapper.get('textarea').setValue('provider and ledger evidence')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect((wrapper.get('#refund-reconciliation-amount').element as HTMLInputElement).value).toBe('25')
    expect((wrapper.get('#refund-reconciliation-deducted').element as HTMLInputElement).value).toBe('')
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.confirmBalanceRefund).toHaveBeenCalledTimes(1)
    for (const [amount, deducted] of [[0, 0], [-1, 0], [101, 0], [25, -1], [25, 26]]) {
      await wrapper.get('#refund-reconciliation-amount').setValue(amount)
      await wrapper.get('#refund-reconciliation-deducted').setValue(deducted)
      await wrapper.get('form').trigger('submit')
      expect(adminPaymentAPI.confirmBalanceRefund).toHaveBeenCalledTimes(1)
    }
    await wrapper.get('#refund-reconciliation-amount').setValue(50)
    await wrapper.get('#refund-reconciliation-deducted').setValue(0)
    expect(wrapper.text()).toContain('€40.00')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(adminPaymentAPI.confirmBalanceRefund).toHaveBeenLastCalledWith(balanceOrder.id, {
      reference: 'provider and ledger evidence', confirmed: true, outcome: 'failed', updated_at: balanceOrder.updated_at,
      refund_amount: 50, balance_deducted: 0,
    })
    expect(wrapper.emitted('confirmed')).toHaveLength(1)
  })

  it('leaves unknown legacy refund amounts empty and clears all acknowledgements for another order', async () => {
    vi.mocked(adminPaymentAPI.confirmBalanceRefund).mockRejectedValue({ code: 'REFUND_RECONCILIATION_REQUIRED' })
    const wrapper = render({ ...balanceOrder, refund_amount: 0 })
    expect(wrapper.text()).toContain('payment.admin.refundAmountUnknown')
    await wrapper.get('input[value="success"]').setValue(true)
    await wrapper.get('textarea').setValue('provider evidence')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect((wrapper.get('#refund-reconciliation-amount').element as HTMLInputElement).value).toBe('')
    await wrapper.setProps({ order: { ...balanceOrder, id: 19 } })
    expect(wrapper.find('input[type="number"]').exists()).toBe(false)
    expect(wrapper.findAll('input[type="radio"]').every(input => !(input.element as HTMLInputElement).checked)).toBe(true)
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
  })

  it('blocks fresh REFUNDING orders until more than five minutes have elapsed', async () => {
    vi.mocked(adminPaymentAPI.confirmBalanceRefund).mockResolvedValue({ data: { success: true } } as never)
    const wrapper = render({ ...balanceOrder, status: 'REFUNDING', updated_at: new Date(now - 300_000).toISOString() })
    await wrapper.get('input[value="success"]').setValue(true)
    await wrapper.get('textarea').setValue('provider evidence')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.confirmBalanceRefund).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1000)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('confirmed')).toHaveLength(1)
  })
})
