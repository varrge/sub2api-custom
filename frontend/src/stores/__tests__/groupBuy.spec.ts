import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useGroupBuyStore } from '../groupBuy'
import { useSubscriptionStore } from '../subscriptions'
import { groupBuyAPI } from '@/api/groupBuy'
import type { MonthCard } from '@/types/groupBuy'

vi.mock('@/api/groupBuy', () => ({ groupBuyAPI: { cards: vi.fn(), orders: vi.fn() } }))
vi.mock('@/api/subscriptions', () => ({ default: { getActiveSubscriptions: vi.fn().mockResolvedValue([]) } }))

const card = { id: 1, status: 'active' } as MonthCard
const order = { group_id: 3, items: [{ kind: 'card' as const, id: 1 }] }

describe('independent month-card cache', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.mocked(groupBuyAPI.cards).mockReset().mockResolvedValue([card])
    vi.mocked(groupBuyAPI.orders).mockReset().mockResolvedValue([order])
  })
  afterEach(() => vi.useRealTimers())

  it('deduplicates reads, caches results and refreshes after expiry', async () => {
    const store = useGroupBuyStore()
    await Promise.all([store.fetchMonthCards(), store.fetchMonthCards()])
    await store.fetchMonthCards()
    expect(groupBuyAPI.cards).toHaveBeenCalledOnce()
    expect(store.entitlementOrders).toEqual([order])
    await vi.advanceTimersByTimeAsync(61_000)
    await store.fetchMonthCards()
    expect(groupBuyAPI.cards).toHaveBeenCalledTimes(2)
  })

  it('keeps month cards and their polling when subscriptions are cleared', async () => {
    const store = useGroupBuyStore()
    await store.fetchMonthCards()
    store.startPolling()
    useSubscriptionStore().clear()
    expect(store.monthCards).toEqual([card])
    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(groupBuyAPI.cards).toHaveBeenCalledTimes(2)
    store.clear()
    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(groupBuyAPI.cards).toHaveBeenCalledTimes(2)
    expect(store.monthCards).toEqual([])
    expect(store.entitlementOrders).toEqual([])
  })

  it.each(['logout', 'freeze'] as const)('does not let an earlier read undo %s', async action => {
    const store = useGroupBuyStore()
    await store.fetchMonthCards()
    let resolve!: (cards: MonthCard[]) => void
    vi.mocked(groupBuyAPI.cards).mockReturnValueOnce(new Promise(done => { resolve = done }))
    const pending = store.fetchMonthCards(true)
    if (action === 'logout') store.clear()
    else store.updateMonthCard({ ...card, status: 'frozen' })
    resolve([card])
    await pending
    expect(store.monthCards).toEqual(action === 'logout' ? [] : [{ ...card, status: 'frozen' }])
  })
})
