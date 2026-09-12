import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import zh from '@/i18n/locales/zh'
import TeamCard from '../TeamCard.vue'
import MonthCardCard from '../MonthCardCard.vue'
import ConsumptionOrder from '../ConsumptionOrder.vue'
import AllocationTable from '../AllocationTable.vue'
import { availableQuota, canJoin, orderedEntitlements, purchaseQuery, remainingMembers } from '../model'
import { groupBuyAPI } from '@/api/groupBuy'
import type { UserSubscription } from '@/types'
import type { GroupBuyProduct, GroupBuyTeam, MonthCard, ChargeAllocation } from '@/types/groupBuy'

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/api/groupBuy', () => ({ groupBuyAPI: { setOrder: vi.fn().mockResolvedValue(undefined), freezeCard: vi.fn(), thawCard: vi.fn() } }))
const product: GroupBuyProduct = { id: 1, group_id: 10, group_name: 'Actual group', platform: 'openai', name: 'Monthly', description: '', price_cny: 198, base_quota_usd: 940, tiers: [{ members: 3, quota_usd: 960 }, { members: 6, quota_usd: 1000 }, { members: 10, quota_usd: 1100 }], max_members: 10, recruitment_hours: 48, for_sale: true, sort_order: 0 }
const team: GroupBuyTeam = { id: 1, code: 'TEAM', product_id: 1, product, member_count: 4, current_quota_usd: 960, next_quota_usd: 1000, next_members: 6, starts_at: '2099-01-01T15:00:00Z', closes_at: '2099-01-03T15:00:00Z', status: 'recruiting', joined: false }
const card: MonthCard = { id: 1, user_id: 1, group_id: 10, order_id: 1, code: 'CARD', group_name: 'Actual group', platform: 'openai', product_name: 'Monthly', team_code: 'TEAM', team_id: 1, status: 'active', total_quota_usd: 1100, total_used_usd: 1000, weekly_quota_usd: 275, weekly_used_usd: 0, starts_at: '2099-01-01T15:00:00Z', expires_at: '2099-01-31T15:00:00Z', weekly_window_start: '2099-01-29T15:00:00Z', weekly_window_end: '2099-01-31T15:00:00Z', priority: 0 }
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => { const message = key.split('.').reduce((obj: any, part) => obj?.[part], zh) || key; return String(message).replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? '')) } }) }))
const global = () => ({ plugins: [createPinia()], stubs: { RouterLink: { template: '<a><slot /></a>' } } })

describe('group buy purchase and quota boundaries', () => {
  it('distinguishes two members to next tier from six to full team', () => {
    expect(remainingMembers(team)).toEqual({ next: 2, full: 6 })
    const wrapper = mount(TeamCard, { props: { team, now: Date.parse('2099-01-02T15:00:00Z') }, global: global() })
    expect(wrapper.text()).toContain('下一档每卡 $1,000.00，还差 2 人')
    expect(wrapper.text()).toContain('距离满团还差 6 人')
    expect(wrapper.text()).toContain('每张卡月总额度')
  })
  it.each([{ ...team, joined: true }, { ...team, status: 'closed' as const }, { ...team, status: 'cancelled' as const }, { ...team, member_count: 10 }, { ...team, closes_at: '2000-01-01T00:00:00Z' }])('disables duplicate or unavailable team checkout', current => {
    expect(canJoin(current)).toBe(false)
    const wrapper = mount(TeamCard, { props: { team: current, now: Date.now() }, global: global() })
    expect(wrapper.get('button.btn-primary').attributes('disabled')).toBeDefined()
  })
  it('caps fifth weekly window by the remaining whole-card quota and shows final exact window', () => {
    expect(availableQuota(card)).toBe(100)
    expect(availableQuota({ ...card, total_used_usd: 800 })).toBe(275)
    const wrapper = mount(MonthCardCard, { props: { card }, global: global() })
    expect(wrapper.text()).toContain('当前可用 $100.00')
    expect(wrapper.text()).toContain('最后周期，到期后不再重置')
    expect(wrapper.text()).toContain('$1,000.00 / $1,100.00')
  })
  it('keeps subscription deep links in the month-card catalog without an implicit renewal', () => {
    expect(purchaseQuery({ tab: 'subscription', group: '10' })).toEqual({ groupBuy: true, groupId: 10, productId: undefined, teamCode: '', mode: null })
    expect(purchaseQuery({ mode: 'join', team_code: 'OLD-SNAPSHOT' })).toMatchObject({ groupBuy: true, mode: 'join', teamCode: 'OLD-SNAPSHOT' })
  })
})

describe('mixed same-group order', () => {
  const legacy = { id: 1, group_id: 10, status: 'active', expires_at: '2099-01-15T00:00:00Z', group: { name: 'Legacy' } } as UserSubscription
  it('preserves saved card/legacy order, retains exhausted cards, and appends new cards within their group', () => {
    const items = orderedEntitlements([{ ...card, weekly_used_usd: 275 }, { ...card, id: 2 }, { ...card, id: 3, group_id: 20 }], [legacy], [{ group_id: 10, items: [{ kind: 'card', id: 1 }, { kind: 'legacy', id: 1 }] }])
    expect(items.map(item => [item.group_id, item.kind, item.id])).toEqual([[10, 'card', 1], [10, 'legacy', 1], [10, 'card', 2], [20, 'card', 3]])
  })
  it('saves only the complete selected group and supports keyboard-accessible arrow controls', async () => {
    const items = orderedEntitlements([card, { ...card, id: 3, group_id: 20 }], [legacy], [])
    const wrapper = mount(ConsumptionOrder, { props: { items }, global: global() })
    await wrapper.get('button').trigger('click')
    await wrapper.findAll('button[aria-label="下移"]')[0].trigger('click')
    await wrapper.get('button.btn-primary').trigger('click')
    expect(groupBuyAPI.setOrder).toHaveBeenCalledWith({ group_id: 10, items: [{ kind: 'card', id: 1 }, { kind: 'legacy', id: 1 }] })
  })
})

it('displays card and balance shares of the same real request as one conserved actual cost', () => {
  const base = { request_id: 'request-1', api_key_id: 9, user_id: 1, group_id: 10, weekly_window_start: card.weekly_window_start, started_at: card.starts_at, created_at: card.starts_at }
  const allocations: ChargeAllocation[] = [{ ...base, id: 1, kind: 'card', entitlement_id: 1, amount_usd: 1 }, { ...base, id: 2, kind: 'balance', entitlement_id: 0, amount_usd: 2 }]
  const wrapper = mount(AllocationTable, { props: { allocations, cards: [card] }, global: global() })
  const cells = wrapper.findAll('tbody td')
  expect(cells[1].text()).toContain('CARD: $1.00')
  expect(cells[2].text()).toBe('$2.00')
  expect(cells[3].text()).toBe('$3.00')
})

it('keeps legacy subscription and card identities distinct when their numeric IDs overlap', () => {
  const base = { request_id: 'request-overlap', api_key_id: 9, user_id: 1, group_id: 10, weekly_window_start: card.weekly_window_start, started_at: card.starts_at, created_at: card.starts_at }
  const allocations: ChargeAllocation[] = [{ ...base, id: 10, kind: 'legacy', entitlement_id: card.id, amount_usd: 1 }, { ...base, id: 11, kind: 'card', entitlement_id: card.id, amount_usd: 2 }]
  const wrapper = mount(AllocationTable, { props: { allocations, cards: [card] }, global: global() })
  const parts = wrapper.findAll('tbody td')[1].findAll('p')
  expect(parts[0].text()).toBe('旧订阅 #1: $1.00')
  expect(parts[1].text()).toBe('卡号 CARD: $2.00')
})


describe('month card freeze controls', () => {
  const frozenCard: MonthCard = { ...card, status: 'frozen', frozen_at: '2026-09-11T03:15:48Z', remaining_seconds: 23 * 86400 }
  it('shows paused time, retained quota and an unfreeze button only for manageable cards', async () => {
    const view = mount(MonthCardCard, { props: { card: frozenCard }, global: global() })
    expect(view.text()).toContain('剩余 23 天 · 已暂停计时')
    expect(view.text()).toContain('解冻后本周期可用 $100.00')
    expect(view.text()).toContain('解冻后会从冻结时所在周期继续')
    expect(view.text()).not.toContain('下次重置')
    expect(view.find('button').exists()).toBe(false)
    await view.setProps({ manageable: true })
    expect(view.get('button').text()).toBe('解冻套餐')
  })
  it('freezes and unfreezes the selected card, using the returned state', async () => {
    vi.mocked(groupBuyAPI.freezeCard).mockResolvedValue(frozenCard)
    vi.mocked(groupBuyAPI.thawCard).mockResolvedValue(card)
    const view = mount(MonthCardCard, { props: { card, manageable: true }, global: global() })
    await view.get('button').trigger('click')
    await flushPromises()
    expect(groupBuyAPI.freezeCard).toHaveBeenCalledWith(card.id)
    expect(view.emitted('changed')?.[0]).toEqual([frozenCard])
    await view.setProps({ card: frozenCard })
    await view.get('button').trigger('click')
    await flushPromises()
    expect(groupBuyAPI.thawCard).toHaveBeenCalledWith(card.id)
    expect(view.emitted('changed')?.[1]).toEqual([card])
  })
  it('keeps state unchanged on failure and allows a retry', async () => {
    vi.mocked(groupBuyAPI.thawCard).mockRejectedValueOnce(new Error('offline'))
    const view = mount(MonthCardCard, { props: { card: frozenCard, manageable: true }, global: global() })
    await view.get('button').trigger('click')
    await flushPromises()
    expect(view.get('[role="alert"]').text()).toBeTruthy()
    expect(view.emitted('changed')).toBeUndefined()
    expect(view.get('button').attributes('disabled')).toBeUndefined()
  })
  it('never offers freeze controls for expired or revoked cards', () => {
    for (const status of ['expired', 'revoked'] as const) {
      const view = mount(MonthCardCard, { props: { card: { ...card, status }, manageable: true }, global: global() })
      expect(view.find('button').exists()).toBe(false)
    }
  })
})
