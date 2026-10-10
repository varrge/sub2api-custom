import { beforeEach, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { adminGroupBuyAPI } from '../groupBuy'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), patch: vi.fn() } }))
beforeEach(() => { vi.clearAllMocks(); vi.mocked(apiClient.get).mockResolvedValue({ data: { items: [], total: 0, page: 1, page_size: 20, pages: 0 } }) })

it('requests paginated entitlements only through the admin namespace', async () => {
  const params = { page: 2, page_size: 20, validity: 'active' as const, search: 'customer@example.test', group_id: 3 }
  const signal = new AbortController().signal
  await adminGroupBuyAPI.entitlementTeams(params, signal)
  expect(apiClient.get).toHaveBeenCalledWith('/admin/group-buy/entitlements/teams', { params, signal })
  await adminGroupBuyAPI.entitlementTeamCards('TEAM/a b', { ...params, validity: 'all' }, signal)
  expect(apiClient.get).toHaveBeenCalledWith('/admin/group-buy/entitlements/teams/TEAM%2Fa%20b/cards', { params: { ...params, validity: 'all' }, signal })
  await adminGroupBuyAPI.soloEntitlements(params, signal)
  expect(apiClient.get).toHaveBeenCalledWith('/admin/group-buy/entitlements/cards', { params, signal })
})

it('saves exact decimal limits using the admin quota endpoint', async () => {
  vi.mocked(apiClient.patch).mockResolvedValue({ data: { updated_count: 2 } })
  const request = { card_ids: [11, 12], weekly_quota_usd: '100.12345678' }
  expect(await adminGroupBuyAPI.adjustQuotas(request)).toEqual({ updated_count: 2 })
  expect(apiClient.patch).toHaveBeenCalledWith('/admin/group-buy/entitlements/cards/quotas', request)
})
