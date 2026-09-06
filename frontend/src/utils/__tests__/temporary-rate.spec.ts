import { describe, expect, it, vi } from 'vitest'
import {
  effectiveGroupRate,
  temporaryRateDateRange,
  temporaryRateEndDate,
  temporaryRateInputDate,
  temporaryRateStatus,
  useTemporaryRateNow
} from '../temporary-rate'

const activity = {
  rate_multiplier: 0.8,
  temporary_rate_enabled: true,
  temporary_rate_multiplier: 0.5,
  temporary_rate_starts_at: '2026-09-04T16:00:00Z',
  temporary_rate_ends_at: '2026-09-10T16:00:00Z'
}

describe('temporary group rate', () => {
  it('uses a half-open window and keeps user overrides first', () => {
    expect(temporaryRateStatus(activity, Date.parse('2026-09-04T15:59:59Z'))).toBe('upcoming')
    expect(effectiveGroupRate(activity, null, Date.parse('2026-09-04T16:00:00Z'))).toBe(0.5)
    expect(effectiveGroupRate(activity, 0.7, Date.parse('2026-09-05T00:00:00Z'))).toBe(0.7)
    expect(temporaryRateStatus(activity, Date.parse('2026-09-10T16:00:00Z'))).toBe('ended')
    expect(effectiveGroupRate(activity, null, Date.parse('2026-09-10T16:00:00Z'))).toBe(0.8)
  })

  it('preserves the exact legacy full-day interval in the server timezone', () => {
    expect(temporaryRateInputDate(activity.temporary_rate_starts_at, 'Asia/Shanghai')).toBe('2026-09-05T00:00:00')
    expect(temporaryRateInputDate(activity.temporary_rate_ends_at, 'Asia/Shanghai')).toBe('2026-09-11T00:00:00')
  })

  it('shows second-precision times without shifting the exclusive end', () => {
    const fields = {
      ...activity,
      temporary_rate_starts_at: '2026-09-05T04:34:56Z',
      temporary_rate_ends_at: '2026-09-05T04:34:57Z'
    }
    expect(temporaryRateInputDate(fields.temporary_rate_starts_at, 'Asia/Shanghai')).toBe('2026-09-05T12:34:56')
    expect(temporaryRateDateRange(fields, 'Asia/Shanghai')).toBe('2026-09-05 12:34:56 – 2026-09-05 12:34:57')
    expect(temporaryRateEndDate(fields, 'Asia/Shanghai')).toBe('2026-09-05 12:34:57')
    expect(temporaryRateInputDate(fields.temporary_rate_starts_at, 'UTC')).toBe('2026-09-05T04:34:56')
    expect(temporaryRateInputDate('invalid', 'Asia/Shanghai')).toBe('')
  })

  it('refreshes the shared rate clock across one-second activity boundaries', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-05T04:34:55Z'))
    try {
      const fields = {
        ...activity,
        temporary_rate_starts_at: '2026-09-05T04:34:56Z',
        temporary_rate_ends_at: '2026-09-05T04:34:57Z'
      }
      const now = useTemporaryRateNow()
      expect(temporaryRateStatus(fields, now.value)).toBe('upcoming')
      vi.advanceTimersByTime(1000)
      expect(effectiveGroupRate(fields, null, now.value)).toBe(0.5)
      vi.advanceTimersByTime(1000)
      expect(temporaryRateStatus(fields, now.value)).toBe('ended')
      expect(effectiveGroupRate(fields, null, now.value)).toBe(0.8)
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps disabled configured activities as canceled', () => {
    expect(temporaryRateStatus({ ...activity, temporary_rate_enabled: false })).toBe('canceled')
  })
})
