import { beforeEach, describe, expect, it, vi } from 'vitest'
import { emptyEventFilters, resolveDeleteRangeFilters } from '../viewModel'

const client = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn(), post: vi.fn(), delete: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))

import promptAuditAPI from '../api'

describe('Prompt Audit API', () => {
  beforeEach(() => Object.values(client).forEach((mock) => mock.mockReset()))

  it('uses the independent admin route namespace', async () => {
    client.get.mockResolvedValue({ data: { config_version: 1 } })
    await promptAuditAPI.getConfig()
    expect(client.get).toHaveBeenCalledWith('/admin/prompt-audit/config')

    client.get.mockResolvedValue({ data: { process_status: 'running' } })
    await promptAuditAPI.getRuntime()
    expect(client.get).toHaveBeenCalledWith('/admin/prompt-audit/runtime')
  })

  it('sends a temporary probe token only in the request and never invents response credentials', async () => {
    client.post.mockResolvedValue({ data: { ok: true, token_applied: true } })
    const result = await promptAuditAPI.probeEndpoint({
      id: 'guard-1', name: 'Guard', protocol: 'openai_compatible', base_url: 'http://127.0.0.1:8000', model: 'guard',
      token: 'api-canary-secret', clear_token: false, timeout_ms: 1000, input_limit: 1000, enabled: true, has_token: false, token_status: 'missing',
    })
    expect(client.post).toHaveBeenCalledWith('/admin/prompt-audit/endpoints/probe', expect.objectContaining({ endpoint: expect.objectContaining({ token: 'api-canary-secret' }) }))
    expect(JSON.stringify(result)).not.toContain('api-canary-secret')
  })

  it.each([
    ['custom', '2026-07-14T16:00:00.000Z', '2026-07-15T16:00:00.000Z'],
    ['7d', '1970-01-01T00:00:00.000Z', '2026-07-10T12:00:00.123Z'],
    ['all', '1970-01-01T00:00:00.000Z', '2026-07-17T12:00:00.123Z'],
  ] as const)('uses identical %s boundaries for listing, previewing, and confirmed deletion', async (preset, start_at, end_at) => {
    const preview = {
      matched_count: 2, filter_summary: {}, snapshot_max_id: 10, filter_hash: 'a'.repeat(64), confirmation_token: 'opaque-token', expires_at: '2026-07-16T00:05:00Z',
    }
    client.get.mockResolvedValue({ data: { items: [], total: 0 } })
    client.post.mockResolvedValueOnce({ data: preview })
      .mockResolvedValueOnce({ data: { deleted_events: 2, deleted_jobs: 2 } })
    const filters = resolveDeleteRangeFilters({
      ...emptyEventFilters(), start_at: '2026-07-15T00:00', end_at: '2026-07-16T00:00',
    }, preset, Date.parse('2026-07-17T12:00:00.123Z'))

    await promptAuditAPI.listEvents(filters, 1, 20)
    const result = await promptAuditAPI.previewDelete(filters)
    await promptAuditAPI.deleteEventsByFilter(filters, result)

    expect(client.get).toHaveBeenCalledWith('/admin/prompt-audit/events', {
      params: { page: 1, page_size: 20, start_at, end_at },
    })
    expect(client.post).toHaveBeenCalledWith('/admin/prompt-audit/events/delete-preview', { start_at, end_at })
    expect(client.post).toHaveBeenCalledWith('/admin/prompt-audit/events/delete-by-filter', expect.objectContaining({
      filter: { start_at, end_at },
      snapshot_max_id: 10, filter_hash: 'a'.repeat(64), confirmation_token: 'opaque-token', confirm: true,
    }))
  })
})
