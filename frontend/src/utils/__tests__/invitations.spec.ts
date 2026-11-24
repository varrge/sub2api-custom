import { beforeEach, describe, expect, it } from 'vitest'
import { beginApply, clearPendingApply, isUncertainApplyError, loadPendingApply, newRequestId } from '@/utils/invitationApply'
import { codeGroups, formatMoney, formatRate, sourceConcern, waitingPeriod } from '@/utils/invitationFormat'
import { suggestLevelId, validateInvitationSettings } from '@/utils/invitationSettings'
import {
  clearRegistrationInvitation,
  loadRegistrationInvitation,
  registrationInvitationLink,
  storeRegistrationInvitation
} from '@/utils/registrationInvitation'
import type { InvitationLevel, InvitationSettings } from '@/api/invitations'

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

function level(patch: Partial<InvitationLevel> = {}): InvitationLevel {
  return { id: 'default', name: 'Default', spend_threshold: 0, lifetime_invite_limit: 3, unused_invite_limit: 3, rebate_rate: 10, ...patch }
}
function settings(levels: InvitationLevel[], validity_days = 30, auto_approve_hours = 0): InvitationSettings {
  return { enabled: true, validity_days, auto_approve_hours, levels }
}

describe('invitation apply idempotency', () => {
  beforeEach(() => localStorage.clear())

  it('reuses one request id per user until it is cleared', () => {
    const first = beginApply(7)
    expect(first).toMatch(UUID)
    expect(beginApply(7)).toBe(first)
    expect(loadPendingApply(7)).toBe(first)
    // Another account on the same browser must not replay it.
    expect(loadPendingApply(8)).toBe('')
    expect(beginApply(8)).not.toBe(first)

    clearPendingApply(7)
    expect(loadPendingApply(7)).toBe('')
    expect(beginApply(7)).not.toBe(first)
  })

  it('ignores a stored value that is not a UUID', () => {
    localStorage.setItem('sub2api_invitation_apply:7', 'not-a-uuid')
    expect(loadPendingApply(7)).toBe('')
    expect(beginApply(7)).toMatch(UUID)
  })

  it('creates a valid v4 id without crypto.randomUUID (plain-HTTP deployments)', () => {
    const original = crypto.randomUUID
    Object.defineProperty(crypto, 'randomUUID', { value: undefined, configurable: true })
    try {
      const a = newRequestId()
      expect(a).toMatch(UUID)
      expect(newRequestId()).not.toBe(a)
    } finally {
      Object.defineProperty(crypto, 'randomUUID', { value: original, configurable: true })
    }
  })

  it('treats only definite business rejections as final', () => {
    for (const status of [400, 403, 404, 409, 422]) expect(isUncertainApplyError({ status })).toBe(false)
    for (const err of [{ status: 0 }, { status: 401 }, { status: 408 }, { status: 429 }, { status: 500 }, { status: 502 }, {}, null, new Error('timeout')]) {
      expect(isUncertainApplyError(err)).toBe(true)
    }
  })
})

describe('invitation settings validation', () => {
  it('accepts a valid configuration', () => {
    const result = validateInvitationSettings(settings([level(), level({ id: 'vip', name: 'VIP', spend_threshold: 100, rebate_rate: 12.5 })]))
    expect(result.count).toBe(0)
  })

  it('reports each problem on the field that caused it', () => {
    const result = validateInvitationSettings(settings([
      level({ name: ' ', rebate_rate: 101 }),
      level({ id: 'default', spend_threshold: 0, lifetime_invite_limit: 1.5, unused_invite_limit: -1 }),
      level({ id: 'has space', spend_threshold: Number.NaN, rebate_rate: 0.0000001 })
    ], 0))
    expect(result.validity_days).toBe('range')
    expect(result.levels[0]).toEqual({ name: 'required', rebate_rate: 'range' })
    expect(result.levels[1]).toEqual({ id: 'duplicate', spend_threshold: 'duplicate', lifetime_invite_limit: 'integer', unused_invite_limit: 'range' })
    expect(result.levels[2]).toEqual({ id: 'idFormat', spend_threshold: 'required', rebate_rate: 'precision' })
    expect(result.count).toBe(10)
  })

  it('requires a starting level at threshold 0 and at least one level', () => {
    expect(validateInvitationSettings(settings([level({ spend_threshold: 5 })])).general).toBe('zeroThreshold')
    expect(validateInvitationSettings(settings([])).general).toBe('required')
    // A missing threshold is reported on the field, not doubled as a general error.
    expect(validateInvitationSettings(settings([level({ spend_threshold: Number.NaN })])).general).toBeUndefined()
  })

  it('does not reject identifiers that already exist on the server', () => {
    const legacy = settings([level({ id: '默认 等级' })])
    expect(validateInvitationSettings(legacy).levels[0].id).toBe('idFormat')
    expect(validateInvitationSettings(legacy, ['默认 等级']).count).toBe(0)
  })

  it('accepts rates with up to six decimals and rejects fractional days', () => {
    expect(validateInvitationSettings(settings([level({ rebate_rate: 0.000001 })])).count).toBe(0)
    expect(validateInvitationSettings(settings([level({ rebate_rate: 33.333333 })])).count).toBe(0)
    expect(validateInvitationSettings(settings([level()], 1.5)).validity_days).toBe('integer')
  })

  it('accepts a waiting period of 0 (manual review) up to one year, in whole hours', () => {
    for (const hours of [0, 1, 24, 8760]) {
      expect(validateInvitationSettings(settings([level()], 30, hours)).count, String(hours)).toBe(0)
    }
    expect(validateInvitationSettings(settings([level()], 30, -1)).auto_approve_hours).toBe('range')
    expect(validateInvitationSettings(settings([level()], 30, 8761)).auto_approve_hours).toBe('range')
    expect(validateInvitationSettings(settings([level()], 30, 1.5)).auto_approve_hours).toBe('integer')
    // An emptied number box arrives as '' and must not be saved as 0, which would silently turn the feature off.
    const emptied = validateInvitationSettings({ ...settings([level()]), auto_approve_hours: '' as unknown as number })
    expect(emptied.auto_approve_hours).toBe('required')
    expect(emptied.count).toBe(1)
  })

  it('suggests an unused level id', () => {
    expect(suggestLevelId(['default'])).toBe('level-2')
    expect(suggestLevelId(['default', 'level-3'])).toBe('level-4')
    expect(suggestLevelId(['level-2'])).toBe('level-3')
  })
})

describe('invitation formatting', () => {
  it('formats rates without float noise', () => {
    expect(formatRate(10)).toBe('10%')
    expect(formatRate(12.5)).toBe('12.5%')
    expect(formatRate(0.1 + 0.2)).toBe('0.3%')
    expect(formatRate(0)).toBe('0%')
  })

  it('formats amounts in their own currency and survives unknown codes', () => {
    expect(formatMoney(2, 'USD')).toContain('2.00')
    expect(formatMoney(20, 'CNY')).toContain('20.00')
    expect(formatMoney(20, 'CNY')).not.toBe(formatMoney(20, 'USD'))
    expect(formatMoney(5, '??')).toBe('?? 5.00')
  })

  it('states a waiting period in whole days when it is one', () => {
    expect(waitingPeriod(24)).toEqual({ unit: 'days', count: 1 })
    expect(waitingPeriod(72)).toEqual({ unit: 'days', count: 3 })
    expect(waitingPeriod(36)).toEqual({ unit: 'hours', count: 36 })
    expect(waitingPeriod(1)).toEqual({ unit: 'hours', count: 1 })
  })

  it('flags a first payment that is refunded or not completed, and nothing else', () => {
    expect(sourceConcern('payment', 'COMPLETED')).toBe('')
    for (const status of ['REFUNDED', 'PARTIALLY_REFUNDED', 'REFUND_REQUESTED', 'REFUNDING', 'REFUND_PENDING', 'REFUND_FAILED']) {
      expect(sourceConcern('payment', status), status).toBe('refund')
    }
    for (const status of ['PAID', 'RECHARGING', 'FAILED']) {
      expect(sourceConcern('payment', status), status).toBe('incomplete')
    }
    // Card codes have no order; the inviter's own view never receives a status.
    expect(sourceConcern('redeem_code', '')).toBe('')
    expect(sourceConcern('payment', '')).toBe('')
  })

  it('groups a code in fours', () => {
    expect(codeGroups('NI0123456789ABCDEF')).toEqual(['NI01', '2345', '6789', 'ABCD', 'EF'])
    expect(codeGroups('')).toEqual([])
  })
})

describe('registration invitation transport', () => {
  beforeEach(() => sessionStorage.clear())

  it('keeps the code for the registration flow and clears it afterwards', () => {
    expect(storeRegistrationInvitation(' NI00AA ')).toBe('NI00AA')
    expect(loadRegistrationInvitation()).toBe('NI00AA')
    // Repeated query parameters arrive as an array from vue-router.
    expect(storeRegistrationInvitation(['NI11BB', 'ignored'])).toBe('NI11BB')
    expect(loadRegistrationInvitation()).toBe('NI11BB')
    clearRegistrationInvitation()
    expect(loadRegistrationInvitation()).toBe('')
  })

  it('treats non-string values as no code', () => {
    storeRegistrationInvitation('NI00AA')
    expect(storeRegistrationInvitation(null)).toBe('')
    expect(loadRegistrationInvitation()).toBe('')
  })

  it('builds a same-origin registration link with an encoded code', () => {
    const link = new URL(registrationInvitationLink('NI 00&AA'))
    expect(link.origin).toBe(window.location.origin)
    expect(link.pathname).toBe('/register')
    expect(link.searchParams.get('invitation_code')).toBe('NI 00&AA')
    expect([...link.searchParams.keys()]).toEqual(['invitation_code'])
  })
})
