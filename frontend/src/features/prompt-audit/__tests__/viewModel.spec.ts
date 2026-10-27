import { describe, expect, it } from 'vitest'
import type { PromptAuditConfig } from '../types'
import {
  buildUpdateRequest,
  configToDraft,
  draftFingerprint,
  emptyEventFilters,
  eventFilterPayload,
  eventQueryParams,
  hasExplicitDeleteRange,
  resolveDeleteRangeFilters,
  SCANNER_CATALOG,
} from '../viewModel'

const config = (): PromptAuditConfig => ({
  enabled: true,
  blocking_enabled: false,
  blocking_latest_turn_only: false,
  store_pass_events: false,
  effective_mode: 'async_audit',
  strategy: 'priority',
  worker_count: 4,
  queue_capacity: 100,
  scanners: SCANNER_CATALOG.map((item) => item.id),
  all_groups: true,
  group_ids: [],
  endpoints: [{
    id: 'guard-1', name: 'Guard One', protocol: 'openai_compatible', base_url: 'http://127.0.0.1:8000',
    model: 'sileader/qwen3guard:0.6b', timeout_ms: 3000, input_limit: 4000, enabled: true,
    has_token: true, token_status: 'configured',
  }],
  config_version: 7,
  updated_at: '2026-07-16T00:00:00Z',
  updated_by: 1,
  change_summary: '{}',
})

describe('Prompt Audit view model', () => {
  it('normalizes legacy null collections from the public config', () => {
    const legacy = { ...config(), group_ids: null, scanners: null, endpoints: null } as unknown as PromptAuditConfig
    expect(configToDraft(legacy)).toMatchObject({ group_ids: [], scanners: [], endpoints: [] })
  })

  it('models all nine official input scanners', () => {
    expect(SCANNER_CATALOG).toHaveLength(9)
    expect(SCANNER_CATALOG.map((item) => item.id)).toContain('suicide_and_self_harm')
  })

  it('keeps, replaces, or explicitly clears a saved token without copying plaintext from the server', () => {
    const draft = configToDraft(config())
    expect(draft.endpoints[0].token).toBe('')
    expect(buildUpdateRequest(draft).endpoints[0]).toMatchObject({ token: undefined, clear_token: false })

    draft.endpoints[0].token = 'temporary-canary-token'
    expect(buildUpdateRequest(draft).endpoints[0]).toMatchObject({ token: 'temporary-canary-token', clear_token: false })

    draft.endpoints[0].token = ''
    draft.endpoints[0].clear_token = true
    expect(buildUpdateRequest(draft).endpoints[0]).toMatchObject({ token: undefined, clear_token: true })
  })

  it('includes the optional narrow blocking scope in the update payload', () => {
    const draft = configToDraft(config())
    draft.blocking_latest_turn_only = true
    expect(buildUpdateRequest(draft)).toMatchObject({ blocking_latest_turn_only: true })
  })

  it('tracks dirty state from the full normalized save payload', () => {
    const original = configToDraft(config())
    const changed = configToDraft(config())
    expect(draftFingerprint(changed)).toBe(draftFingerprint(original))
    changed.queue_capacity += 1
    expect(draftFingerprint(changed)).not.toBe(draftFingerprint(original))
  })

  it('uses Beijing time for both list and delete filters, including the previous UTC day', () => {
    const filters = emptyEventFilters()
    expect(hasExplicitDeleteRange(filters)).toBe(false)
    filters.start_at = '2026-07-15T00:00'
    filters.end_at = '2026-07-16T00:00:15'
    filters.group_id = '9'
    expect(hasExplicitDeleteRange(filters)).toBe(true)
    const expected = {
      group_id: 9,
      start_at: '2026-07-14T16:00:00.000Z',
      end_at: '2026-07-15T16:00:15.000Z',
    }
    expect(eventQueryParams(filters)).toEqual(expected)
    expect(eventFilterPayload(filters)).toEqual(expected)
  })

  it.each([
    ['2026-01-15T00:00', '2026-01-14T16:00:00.000Z'],
    ['2026-03-08T02:30', '2026-03-07T18:30:00.000Z'],
    ['2026-11-01T01:30', '2026-10-31T17:30:00.000Z'],
  ])('interprets %s as Beijing time regardless of local daylight saving rules', (start_at, expected) => {
    expect(eventFilterPayload({ ...emptyEventFilters(), start_at })).toEqual({ start_at: expected })
  })

  it.each(['', 'invalid', '2026-02-30T12:00', '2026-07-15T24:00', 'invalidZ'])(
    'rejects the invalid start time %j for explicit deletion',
    (start_at) => {
      const filters = { ...emptyEventFilters(), start_at, end_at: '2026-07-16T00:00' }
      expect(hasExplicitDeleteRange(filters)).toBe(false)
      expect(eventFilterPayload(filters)).not.toHaveProperty('start_at')
    },
  )

  it.each(['2026-07-16T00:00', '2026-07-17T00:00'])('rejects empty or reversed delete ranges starting at %s', (start_at) => {
    expect(hasExplicitDeleteRange({ ...emptyEventFilters(), start_at, end_at: '2026-07-16T00:00' })).toBe(false)
  })

  it('preserves absolute timestamps with explicit offsets', () => {
    const filters = {
      ...emptyEventFilters(),
      start_at: '2026-07-15T00:00:00+08:00',
      end_at: '2026-07-15T02:00:00-07:00',
    }
    expect(hasExplicitDeleteRange(filters)).toBe(true)
    expect(eventFilterPayload(filters)).toEqual({
      start_at: '2026-07-14T16:00:00.000Z',
      end_at: '2026-07-15T09:00:00.000Z',
    })
  })

  it.each([
    ['1d', '2026-07-16T12:00:00.123Z'],
    ['7d', '2026-07-10T12:00:00.123Z'],
    ['30d', '2026-06-17T12:00:00.123Z'],
    ['90d', '2026-04-18T12:00:00.123Z'],
    ['all', '2026-07-17T12:00:00.123Z'],
  ] as const)('preserves the absolute epoch and cutoff in the %s delete payload', (preset, end_at) => {
    const filters = { ...emptyEventFilters(), decision: 'critical' }
    const resolved = resolveDeleteRangeFilters(filters, preset, Date.parse('2026-07-17T12:00:00.123Z'))
    const expected = { decision: 'critical', start_at: '1970-01-01T00:00:00.000Z', end_at }
    expect(resolved).toMatchObject(expected)
    expect(hasExplicitDeleteRange(resolved)).toBe(true)
    expect(eventFilterPayload(resolved)).toEqual(expected)
    expect(filters.start_at).toBe('')
    expect(filters.end_at).toBe('')
  })

  it('keeps a custom delete range in Beijing time until building the payload', () => {
    const filters = { ...emptyEventFilters(), start_at: '2026-07-01T00:00', end_at: '2026-07-02T00:00' }
    const resolved = resolveDeleteRangeFilters(filters, 'custom')
    expect(resolved).toEqual(filters)
    expect(resolved).not.toBe(filters)
    expect(eventFilterPayload(resolved)).toEqual({
      start_at: '2026-06-30T16:00:00.000Z',
      end_at: '2026-07-01T16:00:00.000Z',
    })
  })
})
