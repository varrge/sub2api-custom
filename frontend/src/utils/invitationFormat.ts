import { formatCurrency } from '@/utils/format'

/** 10 → "10%", 12.5 → "12.5%"; trims float noise without rounding configured precision away. */
export function formatRate(rate: number): string {
  return `${Number(Number(rate || 0).toFixed(6))}%`
}

/** Amount in its own currency. Falls back to "CODE 1.00" for codes Intl does not accept. */
export function formatMoney(amount: number, currency = 'USD'): string {
  try {
    return formatCurrency(amount, currency)
  } catch {
    return `${currency} ${Number(amount || 0).toFixed(2)}`
  }
}

/** Splits a code into groups of four for reading aloud or checking by eye. */
export function codeGroups(code: string): string[] {
  return code.match(/.{1,4}/g) ?? []
}

/** 24 → { unit: 'days', count: 1 }, 36 → { unit: 'hours', count: 36 }; whole days read better than "72 hours". */
export function waitingPeriod(hours: number): { unit: 'days' | 'hours'; count: number } {
  return hours >= 24 && hours % 24 === 0 ? { unit: 'days', count: hours / 24 } : { unit: 'hours', count: hours }
}

export type SourceConcern = 'refund' | 'incomplete' | ''

/**
 * Whether the first payment behind a rebate deserves a second look before crediting.
 * Only a completed order is clean; refund states are the serious case.
 */
export function sourceConcern(sourceType: string, sourceStatus: string): SourceConcern {
  if (sourceType !== 'payment' || !sourceStatus || sourceStatus === 'COMPLETED') return ''
  return sourceStatus.includes('REFUND') ? 'refund' : 'incomplete'
}
