import { beforeEach, describe, expect, it, vi } from 'vitest'
import { exportMonthly } from '@/api/usage'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))

beforeEach(() => {
  vi.clearAllMocks()
  get.mockResolvedValue({
    data: new Blob(['x']),
    headers: { 'content-disposition': 'attachment; filename="usage-2026-10-summary.csv"' },
  })
})

describe('usageAPI.exportMonthly api_key_id contract', () => {
  it.each(['all', 'conversions'] as const)(
    'does not send api_key_id for kind=%s but keeps group_id',
    async (kind) => {
      await exportMonthly({ month: '2026-10', kind, group_id: 7, api_key_id: 42 })
      const [, config] = get.mock.calls[0]
      expect(config.params.month).toBe('2026-10')
      expect(config.params.kind).toBe(kind)
      expect(config.params.group_id).toBe(7)
      expect(config.params.api_key_id).toBeUndefined()
    },
  )

  it.each(['summary', 'details'] as const)('sends api_key_id for kind=%s', async (kind) => {
    await exportMonthly({ month: '2026-10', kind, api_key_id: 42 })
    const [, config] = get.mock.calls[0]
    expect(config.params.api_key_id).toBe(42)
  })

  it('defaults kind to all and therefore drops api_key_id', async () => {
    await exportMonthly({ month: '2026-10', api_key_id: 42 })
    const [, config] = get.mock.calls[0]
    expect(config.params.kind).toBe('all')
    expect(config.params.api_key_id).toBeUndefined()
  })
})
