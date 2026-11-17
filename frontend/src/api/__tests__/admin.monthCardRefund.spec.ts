import { beforeEach, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { adminPaymentAPI } from '../admin/payment'
vi.mock('../client', () => ({ apiClient: { post: vi.fn().mockResolvedValue({ data: { success: true } }) } }))
beforeEach(() => vi.clearAllMocks())
it('records a verified refund through its dedicated endpoint and explicit acknowledgement', async () => {
  await adminPaymentAPI.confirmMonthCardRefund(42, { reference: 'provider-ref-42', confirmed: true })
  expect(apiClient.post).toHaveBeenCalledTimes(1)
  expect(apiClient.post).toHaveBeenCalledWith('/admin/payment/orders/42/refund/confirm', { reference: 'provider-ref-42', confirmed: true })
})

it('submits the verified balance outcome with the order version and explicit legacy reconciliation amounts', async () => {
  const request = { reference: 'provider-ref-42', confirmed: true as const, outcome: 'failed' as const, updated_at: '2026-11-17T09:00:00Z', refund_amount: 10, balance_deducted: 0 }
  await adminPaymentAPI.confirmBalanceRefund(42, request)
  expect(apiClient.post).toHaveBeenCalledTimes(1)
  expect(apiClient.post).toHaveBeenCalledWith('/admin/payment/orders/42/refund/confirm', request)
})
