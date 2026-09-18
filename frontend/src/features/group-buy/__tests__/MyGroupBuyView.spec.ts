import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia } from 'pinia'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import zh from '@/i18n/locales/zh'
import MyGroupBuyView from '../MyGroupBuyView.vue'
import SubscriptionsView from '@/views/user/SubscriptionsView.vue'
import MonthCardCard from '../MonthCardCard.vue'
import { groupBuyAPI } from '@/api/groupBuy'
import subscriptionsAPI from '@/api/subscriptions'
import type { MonthCard, ChargeAllocation } from '@/types/groupBuy'
import type { UserSubscription } from '@/types'

const mocks = vi.hoisted(() => ({ push: vi.fn(), showSuccess: vi.fn() }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => { const message = key.split('.').reduce((obj: any, part) => obj?.[part], zh) || key; return String(message).replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? '')) } }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: mocks.push }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: { subscription_enabled: false, payment_enabled: true }, showError: vi.fn(), showSuccess: mocks.showSuccess }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { balance: 0 }, refreshUser: vi.fn().mockResolvedValue(undefined) }) }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getUserGroupRates: vi.fn().mockResolvedValue({}) } }))
vi.mock('@/api/subscriptions', () => ({ default: { getMySubscriptions: vi.fn() } }))
vi.mock('@/api/groupBuy', () => ({ groupBuyAPI: { cards: vi.fn(), orders: vi.fn(), allocations: vi.fn(), freezeCard: vi.fn(), setOrder: vi.fn().mockResolvedValue(undefined) } }))

const card: MonthCard = {
  id: 1, user_id: 1, group_id: 3, order_id: 9, code: 'CARD-1', group_name: 'OpenAI', platform: 'openai', product_name: '月卡商品', team_code: 'TEAM-1', team_id: 7,
  status: 'active', total_quota_usd: 100, total_used_usd: 1, weekly_quota_usd: 25, weekly_used_usd: 1,
  starts_at: '2099-01-01T00:00:00Z', expires_at: '2099-01-31T00:00:00Z', weekly_window_start: '2099-01-01T00:00:00Z', weekly_window_end: '2099-01-08T00:00:00Z', priority: 0,
}

describe('separate entitlement pages', () => {
  let wrapper: VueWrapper
  const global = () => ({ plugins: [createPinia()], stubs: { AppLayout: { template: '<div><slot /></div>' }, RouterLink: { props: ['to'], template: '<a :href="typeof to === \'string\' ? to : to.path"><slot /></a>' } } })
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(groupBuyAPI.cards).mockResolvedValue([card, { ...card, id: 2, code: 'SOLO-2', team_id: null, team_code: '' }])
    vi.mocked(groupBuyAPI.orders).mockResolvedValue([{ group_id: 3, items: [{ kind: 'legacy', id: 1 }, { kind: 'card', id: 1 }, { kind: 'card', id: 2 }] }])
    vi.mocked(groupBuyAPI.allocations).mockResolvedValue([])
  })
  afterEach(() => wrapper?.unmount())

  it('shows group and solo month cards and freezes them while official subscriptions are disabled', async () => {
    wrapper = mount(MyGroupBuyView, { global: global() })
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('我的拼团')
    expect(wrapper.findAllComponents(MonthCardCard)).toHaveLength(2)
    expect(wrapper.text()).toContain('单独购买')
    expect(wrapper.find('[href="/purchase?tab=group-buy"]').exists()).toBe(true)
    const frozen = { ...card, status: 'frozen' as const, remaining_seconds: 86400 }
    vi.mocked(groupBuyAPI.freezeCard).mockResolvedValue(frozen)
    vi.mocked(groupBuyAPI.cards).mockResolvedValue([frozen])
    await wrapper.getComponent(MonthCardCard).get('button').trigger('click')
    await flushPromises()
    expect(groupBuyAPI.freezeCard).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('解冻套餐')
    expect(subscriptionsAPI.getMySubscriptions).not.toHaveBeenCalled()
  })

  it('preserves subscription references when saving a card group consumption order', async () => {
    wrapper = mount(MyGroupBuyView, { global: global() })
    await flushPromises()
    await wrapper.get('button[aria-expanded]').trigger('click')
    expect(wrapper.text()).toContain('订阅 #1')
    await wrapper.findAll('button[aria-label="下移"]')[0].trigger('click')
    const save = wrapper.findAll('button').find(button => button.text() === '保存')!
    await save.trigger('click')
    await flushPromises()
    expect(groupBuyAPI.setOrder).toHaveBeenCalledWith({ group_id: 3, items: [{ kind: 'card', id: 1 }, { kind: 'legacy', id: 1 }, { kind: 'card', id: 2 }] })
    expect(subscriptionsAPI.getMySubscriptions).not.toHaveBeenCalled()
  })

  it('preserves the shared billing history, including balance-only in-flight overage', async () => {
    const base = { api_key_id: 1, user_id: 1, group_id: 3, entitlement_id: 1, amount_usd: 1, weekly_window_start: null, started_at: card.starts_at, created_at: card.starts_at }
    const charges: ChargeAllocation[] = [
      { ...base, id: 1, request_id: 'mixed-call', kind: 'card' },
      { ...base, id: 2, request_id: 'mixed-call', kind: 'legacy' },
      { ...base, id: 3, request_id: 'mixed-call', kind: 'balance' },
      { ...base, id: 4, request_id: 'subscription-only-call', kind: 'legacy' },
      { ...base, id: 5, request_id: 'balance-only-overage', kind: 'balance' },
    ]
    vi.mocked(groupBuyAPI.allocations).mockResolvedValue(charges)
    wrapper = mount(MyGroupBuyView, { global: global() })
    await flushPromises()
    expect(wrapper.findAll('tbody tr')).toHaveLength(3)
    expect(wrapper.text()).toContain('mixed-call')
    expect(wrapper.text()).toContain('balance-only-overage')
    expect(wrapper.text()).toContain('已开始的调用也可能只产生余额补扣')
    expect(wrapper.findAll('tbody td')[3].text()).toBe('$3.00')
  })

  it('keeps the official subscription page and its renewal link free of month cards', async () => {
    vi.mocked(subscriptionsAPI.getMySubscriptions).mockResolvedValue([{ id: 7, group_id: 3, status: 'active', expires_at: '2099-01-01T00:00:00Z', group: { name: '官方套餐', platform: 'openai' } } as UserSubscription])
    wrapper = mount(SubscriptionsView, { global: global() })
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('我的订阅')
    expect(wrapper.text()).toContain('官方套餐')
    expect(wrapper.findComponent(MonthCardCard).exists()).toBe(false)
    expect(groupBuyAPI.cards).not.toHaveBeenCalled()
    const renew = wrapper.findAll('button').find(button => button.text() === '续费')!
    await renew.trigger('click')
    expect(mocks.push).toHaveBeenCalledWith({ path: '/purchase', query: { tab: 'subscription', group: '3' } })
  })
})
