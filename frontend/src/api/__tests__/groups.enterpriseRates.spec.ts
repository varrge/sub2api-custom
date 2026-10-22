import { describe, expect, it, vi } from 'vitest'
import { getEnterpriseGroupRates, getUserGroupRates } from '@/api/groups'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))

describe('enterprise group-rate API separation', () => {
  it('loads explicit enterprise prices without changing the ordinary endpoint', async () => {
    get.mockResolvedValueOnce({ data: { 7: 0.2 } }).mockResolvedValueOnce({ data: { 7: 0.8 } })
    expect(await getEnterpriseGroupRates()).toEqual({ 7: 0.2 })
    expect(await getUserGroupRates()).toEqual({ 7: 0.8 })
    expect(get.mock.calls).toEqual([['/groups/enterprise-rates'], ['/groups/rates']])
  })
})
