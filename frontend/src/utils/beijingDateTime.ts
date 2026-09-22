/**
 * Beijing time (UTC+08:00, no daylight saving) helpers for `datetime-local`
 * inputs. The values are converted with explicit UTC arithmetic so the result
 * never depends on the browser's local timezone.
 */

const BEIJING_OFFSET_MS = 8 * 60 * 60 * 1000
const DATE_TIME_LOCAL = /^(\d{4,})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/

const pad = (value: number) => String(value).padStart(2, '0')

/**
 * RFC3339 timestamp → `datetime-local` value (YYYY-MM-DDTHH:mm) in Beijing time.
 * Returns an empty string for missing or malformed input.
 */
export function toBeijingDateTimeLocal(value?: string | null): string {
  if (!value) return ''
  const timestamp = Date.parse(value)
  if (!Number.isFinite(timestamp)) return ''
  const shifted = new Date(timestamp + BEIJING_OFFSET_MS)
  const date = `${shifted.getUTCFullYear()}-${pad(shifted.getUTCMonth() + 1)}-${pad(shifted.getUTCDate())}`
  const seconds = shifted.getUTCSeconds()
  return `${date}T${pad(shifted.getUTCHours())}:${pad(shifted.getUTCMinutes())}${seconds ? `:${pad(seconds)}` : ''}`
}

/**
 * `datetime-local` value interpreted as Beijing time → epoch milliseconds.
 * Returns null for malformed input or impossible calendar dates.
 */
export function parseBeijingDateTimeLocal(value?: string | null): number | null {
  if (!value) return null
  const match = DATE_TIME_LOCAL.exec(value)
  if (!match) return null

  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const hours = Number(match[4])
  const minutes = Number(match[5])
  const seconds = match[6] ? Number(match[6]) : 0
  if (month < 1 || month > 12 || day < 1 || day > 31 || hours > 23 || minutes > 59 || seconds > 59) {
    return null
  }

  const timestamp = Date.UTC(year, month - 1, day, hours, minutes, seconds) - BEIJING_OFFSET_MS
  // Reject values that rolled over (e.g. 2026-02-31 becoming March).
  const check = new Date(timestamp + BEIJING_OFFSET_MS)
  if (check.getUTCFullYear() !== year || check.getUTCMonth() !== month - 1 || check.getUTCDate() !== day) {
    return null
  }
  return timestamp
}

/**
 * Epoch milliseconds → RFC3339 string with an explicit +08:00 offset, the shape
 * the backend stores and returns.
 */
export function toBeijingRfc3339(timestamp: number): string {
  const shifted = new Date(timestamp + BEIJING_OFFSET_MS)
  const date = `${shifted.getUTCFullYear()}-${pad(shifted.getUTCMonth() + 1)}-${pad(shifted.getUTCDate())}`
  const time = `${pad(shifted.getUTCHours())}:${pad(shifted.getUTCMinutes())}:${pad(shifted.getUTCSeconds())}`
  return `${date}T${time}+08:00`
}

/** `datetime-local` value (Beijing time) → RFC3339 with +08:00, or null when invalid. */
export function beijingDateTimeLocalToRfc3339(value?: string | null): string | null {
  const timestamp = parseBeijingDateTimeLocal(value)
  return timestamp === null ? null : toBeijingRfc3339(timestamp)
}
