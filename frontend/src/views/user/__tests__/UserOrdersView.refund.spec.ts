import { describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import UserOrdersView from '../UserOrdersView.vue'
import { paymentAPI } from '@/api/payment'
import type { PaymentOrder } from '@/types/payment'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-router', async () => ({ ...await vi.importActual<typeof import('vue-router')>('vue-router'), useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/api/payment', () => ({ paymentAPI: { getMyOrders: vi.fn(), getRefundEligibleProviders: vi.fn(), requestRefund: vi.fn() } }))

describe('user order refund eligibility', () => {
  it('keeps month-card refunds admin-only even when the provider permits balance refund requests', async () => {
    const base = { id: 1, order_type: 'month_card', provider_instance_id: '2', status: 'COMPLETED', amount: 198 } as PaymentOrder
    const rows = [base, { ...base, id: 2, order_type: 'balance' }, { ...base, id: 3, order_type: 'subscription' }, { ...base, id: 4, order_type: 'balance', status: 'PENDING' }, { ...base, id: 5, order_type: 'balance', provider_instance_id: '3' }]
    vi.mocked(paymentAPI.getMyOrders).mockResolvedValue({ data: { items: rows, total: rows.length } } as never)
    vi.mocked(paymentAPI.getRefundEligibleProviders).mockResolvedValue({ data: { provider_instance_ids: ['2'] } } as never)
    const wrapper = shallowMount(UserOrdersView, { global: { stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      OrderTable: { props: ['orders'], template: '<div><div v-for="row in orders" :key="row.id" :data-order="row.id"><slot name="actions" :row="row" /></div></div>' },
      BaseDialog: true,
    } } })
    try {
      await flushPromises()
      for (const id of [1, 3, 4, 5]) {
        expect(wrapper.get(`[data-order="${id}"]`).text()).not.toContain('payment.orders.requestRefund')
      }
      expect(wrapper.get('[data-order="2"]').text()).toContain('payment.orders.requestRefund')
      expect(paymentAPI.requestRefund).not.toHaveBeenCalled()
    } finally { wrapper.unmount() }
  })
})
