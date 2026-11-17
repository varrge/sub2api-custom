import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import AdminOrdersView from '../AdminOrdersView.vue'
import MonthCardRefundConfirmDialog from '@/features/group-buy/MonthCardRefundConfirmDialog.vue'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { PaymentOrder } from '@/types/payment'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-router', async () => ({ ...await vi.importActual<typeof import('vue-router')>('vue-router'), useRoute: () => ({ query: {} }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
const { showSuccess } = vi.hoisted(() => ({ showSuccess: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError: vi.fn() }) }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getOrders: vi.fn(), getOrder: vi.fn(), confirmMonthCardRefund: vi.fn(), confirmBalanceRefund: vi.fn(), queryRefund: vi.fn() } }))

describe('admin order recovery action', () => {
  beforeEach(() => { vi.clearAllMocks(); vi.useFakeTimers(); vi.setSystemTime(new Date('2026-11-17T09:00:00Z')) })
  afterEach(() => vi.useRealTimers())
  it('keeps the existing month-card confirmation action and refreshes on success', async () => {
    const base = { id: 1, user_id: 10, order_type: 'month_card', amount: 198, pay_amount: 198, refund_amount: 198, refund_reason: 'Provider transport failure', status: 'COMPLETED', created_at: '2026-09-01T00:00:00Z', expires_at: '2026-09-01T00:30:00Z', fee_rate: 0, payment_type: 'wxpay', out_trade_no: 'payment-1' } as PaymentOrder
    const rows = [base, { ...base, id: 2, refund_amount: 0, refund_reason: undefined }]
    vi.mocked(adminPaymentAPI.getOrders).mockResolvedValue({ data: { items: rows, total: rows.length } } as never)
    const wrapper = shallowMount(AdminOrdersView, { global: { stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      OrderTable: { props: ['orders'], template: '<div><div v-for="row in orders" :key="row.id" :data-order="row.id"><slot name="actions" :row="row" /></div></div>' },
      BaseDialog: true, RouterLink: true,
    } } })
    try {
      await flushPromises()
      const action = wrapper.findAll('button').filter(button => button.text() === 'groupBuy.confirmRefund')
      expect(action).toHaveLength(1)
      await action[0].trigger('click')
      const dialog = wrapper.getComponent(MonthCardRefundConfirmDialog)
      expect(dialog.props('order')?.id).toBe(1)
      expect(adminPaymentAPI.confirmMonthCardRefund).not.toHaveBeenCalled()
      dialog.vm.$emit('confirmed')
      await flushPromises()
      expect(dialog.props('order')).toBeNull()
      expect(adminPaymentAPI.getOrders).toHaveBeenCalledTimes(2)
      expect(showSuccess).toHaveBeenCalledWith('payment.admin.refundSuccess')
    } finally { wrapper.unmount() }
  })

  it('offers balance verification and query only for pending or stale refunds, then refreshes after resolution', async () => {
    const base: PaymentOrder = { id: 1, user_id: 10, order_type: 'balance', amount: 100, pay_amount: 80, currency: 'EUR', refund_amount: 0, status: 'REFUND_PENDING', created_at: '2026-11-17T08:00:00Z', updated_at: new Date(Date.now()).toISOString(), expires_at: '2026-11-17T08:30:00Z', fee_rate: 0, payment_type: 'wxpay', out_trade_no: 'payment-1' }
    const rows: PaymentOrder[] = [base,
      { ...base, id: 2, status: 'REFUNDING', updated_at: new Date(Date.now() - 300_001).toISOString() },
      { ...base, id: 3, status: 'REFUNDING', updated_at: new Date(Date.now() - 300_000).toISOString() },
      { ...base, id: 4, status: 'REFUNDING', updated_at: 'invalid' },
      { ...base, id: 5, status: 'COMPLETED' },
      { ...base, id: 6, order_type: 'subscription' },
    ]
    vi.mocked(adminPaymentAPI.getOrders).mockResolvedValue({ data: { items: rows, total: rows.length } } as never)
    vi.mocked(adminPaymentAPI.queryRefund).mockResolvedValue({ data: { success: true } } as never)
    const wrapper = shallowMount(AdminOrdersView, { global: { stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      OrderTable: { props: ['orders'], template: '<div><div v-for="row in orders" :key="row.id" :data-order="row.id"><slot name="actions" :row="row" /></div></div>' },
      BaseDialog: true, RouterLink: true,
    } } })
    try {
      await flushPromises()
      const actions = (id: number, label: string) => wrapper.get(`[data-order="${id}"]`).findAll('button').filter(button => button.text() === label)
      expect(actions(1, 'payment.admin.verifyRefund')).toHaveLength(1)
      expect(actions(2, 'payment.admin.verifyRefund')).toHaveLength(1)
      for (const id of [3, 4, 5, 6]) expect(actions(id, 'payment.admin.verifyRefund')).toHaveLength(0)
      for (const id of [1, 2]) expect(actions(id, 'payment.admin.queryRefundStatus')).toHaveLength(1)
      for (const id of [3, 4, 5]) expect(actions(id, 'payment.admin.queryRefundStatus')).toHaveLength(0)
      await actions(2, 'payment.admin.queryRefundStatus')[0].trigger('click')
      await flushPromises()
      expect(adminPaymentAPI.queryRefund).toHaveBeenCalledWith(2)
      expect(adminPaymentAPI.getOrders).toHaveBeenCalledTimes(2)
      await actions(1, 'payment.admin.verifyRefund')[0].trigger('click')
      const dialog = wrapper.getComponent(MonthCardRefundConfirmDialog)
      expect(dialog.props('order')?.id).toBe(1)
      dialog.vm.$emit('confirmed')
      await flushPromises()
      expect(dialog.props('order')).toBeNull()
      expect(adminPaymentAPI.getOrders).toHaveBeenCalledTimes(3)
      expect(showSuccess).toHaveBeenLastCalledWith('payment.admin.refundVerifiedComplete')
    } finally { wrapper.unmount() }
  })
})
