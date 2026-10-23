import { describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { getBoxBootstrap, createBoxDraw, recoverBoxDraw } from '../luckyBox'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), post: vi.fn() } }))

describe('luckyBox transport', () => {
  it('fetches bootstrap with an abort signal and returns envelope data', async () => {
    const signal = new AbortController().signal
    const payload = {
      campaign: null,
      availableCards: 2,
      availableCardsByTier: { low: 0, medium: 0, high: 2 },
      progress: [],
      prizes: [],
      draws: [],
    }
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: payload })
    expect(await getBoxBootstrap(signal)).toEqual(payload)
    expect(apiClient.get).toHaveBeenLastCalledWith('/activities/box/bootstrap', { signal })
  })

  it('posts a draw with the idempotent requestId and cardTier passthrough', async () => {
    const signal = new AbortController().signal
    const payload = {
      requestId: 'box-123',
      cardTier: 'high' as const,
      campaignId: 'campaign-1',
    }
    const response = {
      draw: { id: 'd1', requestId: 'box-123', cardTier: 'high' },
      availableCards: 1,
      availableCardsByTier: { low: 0, medium: 0, high: 1 },
    }
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: response })
    expect(await createBoxDraw(payload, signal)).toEqual(response)
    expect(apiClient.post).toHaveBeenLastCalledWith('/activities/box/draws', payload, { signal })
  })

  it('uses the lookup-only recovery endpoint without creating a new draw', async () => {
    const payload = { requestId: 'box-unknown', cardTier: 'high' as const, campaignId: 'old-event' }
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: { found: false, draw: null } })
    expect(await recoverBoxDraw(payload)).toEqual({ found: false, draw: null })
    expect(apiClient.post).toHaveBeenLastCalledWith('/activities/box/draws/recover', payload, { signal: undefined })
  })
})
