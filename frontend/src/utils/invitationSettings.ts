import type { InvitationLevel, InvitationSettings } from '@/api/invitations'

export type LevelField = keyof InvitationLevel
/** i18n suffix under invitations.issues.* */
export type SettingsIssue =
  | 'required' | 'idFormat' | 'tooLong' | 'duplicate' | 'integer' | 'range' | 'precision' | 'zeroThreshold' | 'tooMany'

export interface SettingsIssues {
  validity_days?: SettingsIssue
  auto_approve_hours?: SettingsIssue
  general?: SettingsIssue
  levels: Partial<Record<LevelField, SettingsIssue>>[]
  count: number
}

const MAX_INT = 2147483647
/** One year, same bound as the server. */
export const MAX_AUTO_APPROVE_HOURS = 8760
const isNumber = (v: unknown): v is number => typeof v === 'number' && Number.isFinite(v)

function limitIssue(v: unknown): SettingsIssue | undefined {
  if (!isNumber(v)) return 'required'
  if (!Number.isInteger(v)) return 'integer'
  if (v < 0 || v > MAX_INT) return 'range'
}

/**
 * Mirrors the server rules so mistakes are shown next to the field that caused them.
 * `lockedIds` are ids that already exist on the server; they are exempt from the
 * format rule because they cannot be edited here.
 */
export function validateInvitationSettings(settings: InvitationSettings, lockedIds: readonly string[] = []): SettingsIssues {
  const issues: SettingsIssues = { levels: settings.levels.map(() => ({})), count: 0 }
  const days = settings.validity_days as unknown
  if (!isNumber(days)) issues.validity_days = 'required'
  else if (!Number.isInteger(days)) issues.validity_days = 'integer'
  else if (days < 1 || days > 3650) issues.validity_days = 'range'

  const hours = settings.auto_approve_hours as unknown
  if (!isNumber(hours)) issues.auto_approve_hours = 'required'
  else if (!Number.isInteger(hours)) issues.auto_approve_hours = 'integer'
  else if (hours < 0 || hours > MAX_AUTO_APPROVE_HOURS) issues.auto_approve_hours = 'range'

  if (settings.levels.length === 0) issues.general = 'required'
  else if (settings.levels.length > 100) issues.general = 'tooMany'

  const ids = new Map<string, number>()
  const thresholds = new Map<number, number>()
  settings.levels.forEach((level, i) => {
    const row = issues.levels[i]
    const id = String(level.id ?? '').trim()
    const name = String(level.name ?? '').trim()
    if (!id) row.id = 'required'
    else if (id.length > 64) row.id = 'tooLong'
    else if (!lockedIds.includes(id) && !/^[A-Za-z0-9_-]+$/.test(id)) row.id = 'idFormat'
    else if (ids.has(id)) row.id = 'duplicate'
    else ids.set(id, i)

    if (!name) row.name = 'required'
    else if ([...name].length > 100) row.name = 'tooLong'

    const threshold = level.spend_threshold as unknown
    if (!isNumber(threshold)) row.spend_threshold = 'required'
    else if (threshold < 0 || threshold >= 1e12) row.spend_threshold = 'range'
    else if (thresholds.has(threshold)) row.spend_threshold = 'duplicate'
    else thresholds.set(threshold, i)

    row.lifetime_invite_limit = limitIssue(level.lifetime_invite_limit)
    row.unused_invite_limit = limitIssue(level.unused_invite_limit)
    if (!row.lifetime_invite_limit) delete row.lifetime_invite_limit
    if (!row.unused_invite_limit) delete row.unused_invite_limit

    const rate = level.rebate_rate as unknown
    if (!isNumber(rate)) row.rebate_rate = 'required'
    else if (rate < 0 || rate > 100) row.rebate_rate = 'range'
    else if (Math.abs(rate * 1e6 - Math.round(rate * 1e6)) > 1e-6) row.rebate_rate = 'precision'
  })

  const thresholdsValid = issues.levels.every((row) => !row.spend_threshold)
  if (!issues.general && thresholdsValid && !thresholds.has(0)) issues.general = 'zeroThreshold'

  issues.count = (issues.validity_days ? 1 : 0) + (issues.auto_approve_hours ? 1 : 0) + (issues.general ? 1 : 0)
    + issues.levels.reduce((n, row) => n + Object.keys(row).length, 0)
  return issues
}

/** A free id such as "level-3" for a newly added level. */
export function suggestLevelId(existing: readonly string[]): string {
  for (let n = existing.length + 1; ; n++) {
    const id = `level-${n}`
    if (!existing.includes(id)) return id
  }
}

export function cloneSettings(settings: InvitationSettings): InvitationSettings {
  return { ...settings, levels: settings.levels.map((level) => ({ ...level })) }
}

export function sameSettings(a: InvitationSettings, b: InvitationSettings): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}
