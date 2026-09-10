import { expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { adminPaymentAPI } from '../admin/payment'
vi.mock('../client', () => ({ apiClient: { post: vi.fn().mockResolvedValue({ data: { success: true } }) } }))
it('records a verified refund through its dedicated endpoint and explicit acknowledgement', async () => {
  await adminPaymentAPI.confirmMonthCardRefund(42, { reference: 'provider-ref-42', confirmed: true })
  expect(apiClient.post).toHaveBeenCalledTimes(1)
    expect(apiClient.post).toHaveBeenCalledWith('/admin/payment/orders/42/refund/confirm', { reference: 'provider-ref-42', confirmed: true })
})
