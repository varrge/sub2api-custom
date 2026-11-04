import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { shallowMount, flushPromises } from '@vue/test-utils'
import AdminGiftUsageDialog from '../AdminGiftUsageDialog.vue'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import type { AdminMonthCard, ChargeAllocation, GiftCardBatch } from '@/types/groupBuy'
import type { PaginatedResponse } from '@/types'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/groupBuy', () => ({ adminGroupBuyAPI: { giftCards: vi.fn(), giftAllocations: vi.fn() } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
const auth = reactive({ user: { id: 30 } })
const batch: GiftCardBatch = { id: 9, name: '狂蹬卡', group_id: 2, group_name: 'OpenAI', source_group_id: 0, include_frozen: true, quota_usd: '15', valid_days: 7, actor_user_id: 30, recipient_count: 22, created_at: '2026-11-02T00:00:00Z', expires_at: '2026-11-09T00:00:00Z' }
const card: AdminMonthCard = { id: 42, user_id: 1, username: 'Gift recipient', email: 'gift@example.test', group_id: 2, group_name: 'OpenAI', platform: 'openai', order_id: 0, code: 'G-one', team_id: null, team_code: '', card_kind: 'gift', product_name: '狂蹬卡', status: 'active', total_quota_usd: 15, total_used_usd: 4, weekly_quota_usd: 15, weekly_used_usd: 4, starts_at: batch.created_at, expires_at: batch.expires_at, weekly_window_start: batch.created_at, weekly_window_end: batch.expires_at, priority: 0 }
const allocation: ChargeAllocation = { id: 5, request_id: 'gift-request-one', api_key_id: 7, user_id: 1, group_id: 2, kind: 'card', entitlement_id: card.id, amount_usd: 4, weekly_window_start: batch.created_at, started_at: batch.created_at, created_at: batch.created_at }
const result = <T>(items: T[], total = items.length, page = 1): PaginatedResponse<T> => ({ items, total, page, page_size: 20, pages: Math.ceil(total / 20) })
const usageResult = (items: ChargeAllocation[], total = items.length, gift = card) => ({ ...result(items, total), card: gift })
const wrappers: ReturnType<typeof shallowMount>[] = []
function mount() {
  const w = shallowMount(AdminGiftUsageDialog, { props: { batch }, global: { stubs: { BaseDialog: { template: '<div role="dialog"><slot /></div>' } } } })
  wrappers.push(w); return w
}
beforeEach(() => {
  vi.resetAllMocks(); auth.user = { id: 30 }
  vi.mocked(adminGroupBuyAPI.giftCards).mockResolvedValue(result([card], 22))
  vi.mocked(adminGroupBuyAPI.giftAllocations).mockResolvedValue(usageResult([allocation], 21))
})
afterEach(() => wrappers.splice(0).forEach(w => w.unmount()))

describe('gift usage inspection', () => {
  it('refreshes the card totals alongside deductions and keeps them when returning to the list', async () => {
    const w = mount(); await flushPromises()
    vi.mocked(adminGroupBuyAPI.giftAllocations).mockResolvedValueOnce(Object.assign(result([allocation]), { card: { ...card, total_used_usd: 8 } }))
    await w.get('[data-testid="gift-card-details-42"]').trigger('click'); await flushPromises()
    expect(w.text()).toContain('$8.00 / $15.00')
    vi.mocked(adminGroupBuyAPI.giftAllocations).mockResolvedValueOnce(Object.assign(result([allocation]), { card: { ...card, total_used_usd: 0 } }))
    await w.get('[data-testid="gift-detail-refresh"]').trigger('click'); await flushPromises()
    expect(w.text()).toContain('$0.00 / $15.00')
    await w.get('[data-testid="gift-usage-back"]').trigger('click')
    expect(w.text()).not.toContain('$4.00'); expect(w.text()).toContain('$15.00')
  })
  it('loads only the selected batch including expired cards, filters and paginates server-side', async () => {
    const w = mount(); await flushPromises()
    expect(adminGroupBuyAPI.giftCards).toHaveBeenCalledWith(9, { page: 1, page_size: 20, validity: 'all', search: '' }, expect.any(AbortSignal))
    expect(w.text()).toContain('gift@example.test'); expect(w.text()).toContain('$4.00'); expect(w.text()).toContain('$11.00')
    await w.get('[data-testid="gift-usage-next"]').trigger('click'); await flushPromises()
    expect(vi.mocked(adminGroupBuyAPI.giftCards).mock.lastCall?.[1].page).toBe(2)
    await w.get('[data-testid="gift-usage-search"]').setValue(' gift@example.test ')
    expect(adminGroupBuyAPI.giftCards).toHaveBeenCalledTimes(2)
    await w.get('[data-testid="gift-usage-filter"]').trigger('submit'); await flushPromises()
    expect(vi.mocked(adminGroupBuyAPI.giftCards).mock.lastCall?.[1]).toEqual({ page: 1, page_size: 20, validity: 'all', search: 'gift@example.test' })
    vi.mocked(adminGroupBuyAPI.giftCards).mockResolvedValueOnce(result([{ ...card, status: 'expired' }]))
    await w.get('[data-testid="gift-usage-validity"]').setValue('expired'); await flushPromises()
    expect(vi.mocked(adminGroupBuyAPI.giftCards).mock.lastCall?.[1].validity).toBe('expired')
    expect(w.text()).toContain('groupBuy.gifts.unavailable'); expect(w.text()).toContain('$11.00')
  })
  it('opens and paginates the exact gift card share without showing request-wide totals', async () => {
    const w = mount(); await flushPromises()
    await w.get('[data-testid="gift-card-details-42"]').trigger('click'); await flushPromises()
    expect(adminGroupBuyAPI.giftAllocations).toHaveBeenCalledWith(9, 42, 1, expect.any(AbortSignal))
    expect(w.text()).toContain('gift-request-one'); expect(w.text()).toContain('groupBuy.gifts.detailNotice')
    expect(w.text()).not.toContain('groupBuy.balanceCharge')
    await w.get('[data-testid="gift-detail-next"]').trigger('click'); await flushPromises()
    expect(adminGroupBuyAPI.giftAllocations).toHaveBeenLastCalledWith(9, 42, 2, expect.any(AbortSignal))
    await w.get('[data-testid="gift-usage-back"]').trigger('click')
    expect(w.find('[data-testid="gift-card-details-42"]').exists()).toBe(true)
    expect(w.text()).not.toContain('gift-request-one')
  })
  it('distinguishes failures from empty usage and supports retry', async () => {
    vi.mocked(adminGroupBuyAPI.giftCards).mockRejectedValueOnce(new Error('offline'))
    const w = mount(); await flushPromises()
    expect(w.find('[role="alert"]').exists()).toBe(true)
    expect(w.text()).not.toContain('groupBuy.gifts.noCardsFound')
    await w.get('[data-testid="gift-cards-retry"]').trigger('click'); await flushPromises()
    vi.mocked(adminGroupBuyAPI.giftAllocations).mockRejectedValueOnce(new Error('offline'))
    await w.get('[data-testid="gift-card-details-42"]').trigger('click'); await flushPromises()
    expect(w.find('[role="alert"]').exists()).toBe(true)
    expect(w.text()).not.toContain('groupBuy.gifts.noUsageRecords')
    vi.mocked(adminGroupBuyAPI.giftAllocations).mockResolvedValueOnce(usageResult([]))
    await w.get('[data-testid="gift-detail-retry"]').trigger('click'); await flushPromises()
    expect(w.text()).toContain('groupBuy.gifts.noUsageRecords')
    expect(w.find('[role="alert"]').exists()).toBe(false)
  })
  it('ignores late list responses after changing batch or account', async () => {
    let resolveOld!: (value: PaginatedResponse<AdminMonthCard>) => void
    vi.mocked(adminGroupBuyAPI.giftCards).mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    const w = mount(); await flushPromises()
    await w.setProps({ batch: { ...batch, id: 10 } }); await flushPromises()
    resolveOld(result([{ ...card, email: 'old@example.test' }])); await flushPromises()
    expect(w.text()).not.toContain('old@example.test')
    expect(vi.mocked(adminGroupBuyAPI.giftCards).mock.lastCall?.[0]).toBe(10)
    let resolveDetail!: (value: ReturnType<typeof usageResult>) => void
    vi.mocked(adminGroupBuyAPI.giftAllocations).mockReturnValueOnce(new Promise(resolve => { resolveDetail = resolve }))
    await w.get('[data-testid="gift-card-details-42"]').trigger('click')
    auth.user = { id: 31 }; await flushPromises()
    resolveDetail(usageResult([allocation])); await flushPromises()
    expect(w.emitted('close')).toHaveLength(1)
    expect(w.text()).not.toContain('gift@example.test'); expect(w.text()).not.toContain('gift-request-one')
  })
  it('ignores a late detail response after returning to the list and opening another gift', async () => {
    vi.mocked(adminGroupBuyAPI.giftCards).mockResolvedValueOnce(result([card, { ...card, id: 43, code: 'G-two' }]))
    let resolveOld!: (value: ReturnType<typeof usageResult>) => void
    vi.mocked(adminGroupBuyAPI.giftAllocations).mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    const w = mount(); await flushPromises()
    await w.get('[data-testid="gift-card-details-42"]').trigger('click')
    await w.get('[data-testid="gift-usage-back"]').trigger('click')
    vi.mocked(adminGroupBuyAPI.giftAllocations).mockResolvedValueOnce(usageResult([], 0, { ...card, id: 43, code: 'G-two' }))
    await w.get('[data-testid="gift-card-details-43"]').trigger('click'); await flushPromises()
    resolveOld(usageResult([allocation])); await flushPromises()
    expect(w.text()).toContain('G-two'); expect(w.text()).not.toContain('gift-request-one')
  })
})
