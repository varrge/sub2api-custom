import { describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { getAdminActivityLeaderboard, exportActivityLeaderboard } from '../admin/activityLeaderboard'
vi.mock('../client', () => ({ apiClient: { get: vi.fn() } }))
describe('admin leaderboard transport', () => {
  it('uses only admin endpoints and passes cancellation, campaign and export scope', async () => {
    const signal = new AbortController().signal
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: { entries: [] } })
    expect(await getAdminActivityLeaderboard(signal)).toEqual({ entries: [] })
    expect(apiClient.get).toHaveBeenLastCalledWith('/admin/activities/leaderboard', { signal })
    const csv = new Blob(['csv'])
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: csv })
    expect(await exportActivityLeaderboard('campaign', 'all', signal)).toBe(csv)
    expect(apiClient.get).toHaveBeenLastCalledWith('/admin/activities/leaderboard/export', { params: { campaign_id: 'campaign', scope: 'all' }, responseType: 'blob', signal })
  })
})
