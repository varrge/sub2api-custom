import { expect, it, vi } from 'vitest'
import { apiClient } from '@/api/client'
import { getSharedCreditSummary } from '../sharedCredit'

vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn() } }))

it('reads the authenticated user summary without accessing admin settings', async () => {
  const data = { visible: false, server_time: '2026-11-03T10:00:00Z' }
  vi.mocked(apiClient.get).mockResolvedValue({ data })
  const signal = new AbortController().signal
  expect(await getSharedCreditSummary(signal)).toEqual(data)
  expect(apiClient.get).toHaveBeenCalledWith('/user/shared-credit', { signal })
})
