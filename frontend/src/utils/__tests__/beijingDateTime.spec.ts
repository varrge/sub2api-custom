import { describe, expect, it } from 'vitest'

import {
  beijingDateTimeLocalToRfc3339,
  parseBeijingDateTimeLocal,
  toBeijingDateTimeLocal,
  toBeijingRfc3339,
} from '../beijingDateTime'

describe('beijingDateTime', () => {
  it('converts RFC3339 to a datetime-local value in Beijing time regardless of browser timezone', () => {
    expect(toBeijingDateTimeLocal('2026-09-25T00:00:00+08:00')).toBe('2026-09-25T00:00')
    expect(toBeijingDateTimeLocal('2026-09-25T00:00:36+08:00')).toBe('2026-09-25T00:00:36')
    expect(toBeijingDateTimeLocal('2026-09-24T16:00:00Z')).toBe('2026-09-25T00:00')
    expect(toBeijingDateTimeLocal('2026-10-07T16:00:00Z')).toBe('2026-10-08T00:00')
    // UTC midnight boundary: Beijing date is already the next day.
    expect(toBeijingDateTimeLocal('2026-01-01T16:30:00Z')).toBe('2026-01-02T00:30')
  })

  it('returns empty string for missing or malformed input', () => {
    expect(toBeijingDateTimeLocal(null)).toBe('')
    expect(toBeijingDateTimeLocal(undefined)).toBe('')
    expect(toBeijingDateTimeLocal('')).toBe('')
    expect(toBeijingDateTimeLocal('not-a-date')).toBe('')
  })

  it('parses datetime-local input as Beijing wall-clock time', () => {
    const timestamp = parseBeijingDateTimeLocal('2026-09-25T00:00')
    expect(timestamp).toBe(Date.UTC(2026, 8, 24, 16, 0, 0))
    expect(parseBeijingDateTimeLocal('2026-09-25T00:00:00')).toBe(timestamp)
  })

  it('rejects malformed and impossible input', () => {
    expect(parseBeijingDateTimeLocal('')).toBeNull()
    expect(parseBeijingDateTimeLocal(null)).toBeNull()
    expect(parseBeijingDateTimeLocal('2026-09-25')).toBeNull()
    expect(parseBeijingDateTimeLocal('2026-13-01T00:00')).toBeNull()
    expect(parseBeijingDateTimeLocal('2026-02-31T00:00')).toBeNull()
    expect(parseBeijingDateTimeLocal('2026-09-25T24:00')).toBeNull()
    expect(parseBeijingDateTimeLocal('2026-09-25T00:60')).toBeNull()
    expect(parseBeijingDateTimeLocal('2026-09-25T00:00:00+08:00')).toBeNull()
  })

  it('round-trips through RFC3339 with an explicit +08:00 offset', () => {
    expect(toBeijingRfc3339(Date.UTC(2026, 8, 24, 16, 0, 0))).toBe('2026-09-25T00:00:00+08:00')
    expect(beijingDateTimeLocalToRfc3339('2026-09-25T00:00')).toBe('2026-09-25T00:00:00+08:00')
    expect(beijingDateTimeLocalToRfc3339('2026-10-08T00:00')).toBe('2026-10-08T00:00:00+08:00')
    expect(beijingDateTimeLocalToRfc3339('')).toBeNull()
    expect(beijingDateTimeLocalToRfc3339('bogus')).toBeNull()
  })
})
