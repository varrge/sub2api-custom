import { describe, expect, it } from 'vitest'
import {
  effectiveGroupRate,
  temporaryRateInputDate,
  temporaryRateStatus
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

  it('shows the inclusive end date in the server timezone', () => {
    expect(temporaryRateInputDate(activity.temporary_rate_starts_at, 'Asia/Shanghai')).toBe('2026-09-05')
    expect(temporaryRateInputDate(activity.temporary_rate_ends_at, 'Asia/Shanghai', true)).toBe('2026-09-10')
  })

  it('keeps disabled configured activities as canceled', () => {
    expect(temporaryRateStatus({ ...activity, temporary_rate_enabled: false })).toBe('canceled')
  })
})
