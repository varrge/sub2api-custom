import { describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import AdminOrdersView from '../AdminOrdersView.vue'
import MonthCardRefundConfirmDialog from '@/features/group-buy/MonthCardRefundConfirmDialog.vue'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { PaymentOrder } from '@/types/payment'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-router', async () => ({ ...await vi.importActual<typeof import('vue-router')>('vue-router'), useRoute: () => ({ query: {} }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getOrders: vi.fn(), confirmMonthCardRefund: vi.fn() } }))

describe('admin order recovery action', () => {
  it('shows the confirmation action only for existing full month-card refunds and refreshes on success', async () => {
    const base = { id: 1, user_id: 10, order_type: 'month_card', amount: 198, pay_amount: 198, refund_amount: 198, refund_reason: 'Provider transport failure', status: 'COMPLETED', created_at: '2026-09-01T00:00:00Z', expires_at: '2026-09-01T00:30:00Z', fee_rate: 0, payment_type: 'wxpay', out_trade_no: 'payment-1' } as PaymentOrder
    const rows = [base, { ...base, id: 2, refund_amount: 0, refund_reason: undefined }, { ...base, id: 3, status: 'REFUND_PENDING', order_type: 'balance' }]
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
    } finally { wrapper.unmount() }
  })
})
