import type { PaymentOrder } from '@/types/payment'

export function canConfirmMonthCardRefund(order: PaymentOrder | null, now = Date.now()): boolean {
  if (!order || order.order_type !== 'month_card' || order.refund_amount !== order.amount || !order.refund_reason?.trim()) return false
  if (order.status === 'REFUND_PENDING' || order.status === 'REFUND_FAILED') return true
  if (order.status === 'REFUNDING') {
    const updatedAt = Date.parse(order.updated_at || '')
    return Number.isFinite(updatedAt) && updatedAt < now - 5 * 60_000
  }
  return order.status === 'COMPLETED'
}

export function refundReferenceBytes(reference: string): number {
  return new TextEncoder().encode(reference.trim()).byteLength
}

export function validRefundReference(reference: string): boolean {
  const size = refundReferenceBytes(reference)
  return size >= 3 && size <= 500
}
