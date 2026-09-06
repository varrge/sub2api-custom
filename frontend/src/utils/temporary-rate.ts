import { createGlobalState, useNow } from '@vueuse/core'

export interface TemporaryRateFields {
  temporary_rate_enabled?: boolean
  temporary_rate_multiplier?: number
  temporary_rate_starts_at?: string | null
  temporary_rate_ends_at?: string | null
}

export type TemporaryRateStatus = 'none' | 'upcoming' | 'active' | 'ended' | 'canceled'

const useTemporaryRateClock = createGlobalState(() => useNow({ interval: 1000 }))

export const useTemporaryRateNow = () => useTemporaryRateClock()

export function temporaryRateStatus(
  fields?: TemporaryRateFields | null,
  now: number | Date = Date.now()
): TemporaryRateStatus {
  if (!fields?.temporary_rate_starts_at || !fields.temporary_rate_ends_at) return 'none'
  if (!fields.temporary_rate_enabled) return 'canceled'

  const startsAt = Date.parse(fields.temporary_rate_starts_at)
  const endsAt = Date.parse(fields.temporary_rate_ends_at)
  if (!Number.isFinite(startsAt) || !Number.isFinite(endsAt) || startsAt >= endsAt) return 'none'

  const timestamp = now instanceof Date ? now.getTime() : now
  if (timestamp < startsAt) return 'upcoming'
  if (timestamp < endsAt) return 'active'
  return 'ended'
}

export function effectiveGroupRate(
  fields: TemporaryRateFields & { rate_multiplier?: number },
  userRate?: number | null,
  now: number | Date = Date.now()
): number {
  if (userRate !== null && userRate !== undefined) return userRate
  if (temporaryRateStatus(fields, now) === 'active' && (fields.temporary_rate_multiplier ?? 0) > 0) {
    return fields.temporary_rate_multiplier!
  }
  return fields.rate_multiplier ?? 1
}

function calendarDateTime(timestamp: number, timeZone?: string): string {
  const options: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
    ...(timeZone ? { timeZone } : {})
  }
  try {
    const parts = new Intl.DateTimeFormat('en-CA', options).formatToParts(timestamp)
    const part = (type: Intl.DateTimeFormatPartTypes) => parts.find((item) => item.type === type)?.value
    return `${part('year')}-${part('month')}-${part('day')}T${part('hour')}:${part('minute')}:${part('second')}`
  } catch {
    return new Date(timestamp).toISOString().slice(0, 19)
  }
}

export function temporaryRateInputDate(
  value?: string | null,
  timeZone?: string
): string {
  if (!value) return ''
  const timestamp = Date.parse(value)
  if (!Number.isFinite(timestamp)) return ''
  return calendarDateTime(timestamp, timeZone)
}

export function temporaryRateDateRange(fields: TemporaryRateFields, timeZone?: string): string {
  const start = temporaryRateInputDate(fields.temporary_rate_starts_at, timeZone).replace('T', ' ')
  const end = temporaryRateEndDate(fields, timeZone)
  return start && end ? `${start} – ${end}` : ''
}

export function temporaryRateEndDate(fields: TemporaryRateFields, timeZone?: string): string {
  return temporaryRateInputDate(fields.temporary_rate_ends_at, timeZone).replace('T', ' ')
}
