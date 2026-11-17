import type { PaymentOrder } from '@/types/payment'

function isStaleRefunding(order: PaymentOrder, now: number): boolean {
  const updatedAt = Date.parse(order.updated_at || '')
  return order.status === 'REFUNDING' && Number.isFinite(updatedAt) && updatedAt < now - 5 * 60_000
}

export function canConfirmMonthCardRefund(order: PaymentOrder | null, now = Date.now()): boolean {
  if (!order || order.order_type !== 'month_card' || order.refund_amount !== order.amount || !order.refund_reason?.trim()) return false
  if (order.status === 'REFUND_PENDING' || order.status === 'REFUND_FAILED') return true
  if (order.status === 'REFUNDING') return isStaleRefunding(order, now)
  return order.status === 'COMPLETED'
}

export function canConfirmBalanceRefund(order: PaymentOrder | null, now = Date.now()): boolean {
  return !!order && order.order_type === 'balance' && (order.status === 'REFUND_PENDING' || isStaleRefunding(order, now))
}

export function canConfirmRefund(order: PaymentOrder | null, now = Date.now()): boolean {
  return canConfirmBalanceRefund(order, now) || canConfirmMonthCardRefund(order, now)
}

export function canQueryRefund(order: PaymentOrder, now = Date.now()): boolean {
  return order.status === 'REFUND_PENDING' || (order.order_type === 'balance' && isStaleRefunding(order, now))
}

export function refundReferenceBytes(reference: string): number {
  return new TextEncoder().encode(reference.trim()).byteLength
}

export function validRefundReference(reference: string): boolean {
  const size = refundReferenceBytes(reference)
  return size >= 3 && size <= 500
}
