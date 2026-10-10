import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import AdminEntitlementsPanel from '../AdminEntitlementsPanel.vue'
import AdminEntitlementTeamsTable from '../AdminEntitlementTeamsTable.vue'
import AdminEntitlementMembersTable from '../AdminEntitlementMembersTable.vue'
import AllocationTable from '../AllocationTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import type { AdminMonthCard, AdminTeamEntitlement } from '@/types/groupBuy'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/groupBuy', () => ({ adminGroupBuyAPI: {
  entitlementTeams: vi.fn(), entitlementTeamCards: vi.fn(), soloEntitlements: vi.fn(), cards: vi.fn(), allocations: vi.fn()
} }))

const team: AdminTeamEntitlement = {
  id: 1, code: 'FULL-TEAM', product_id: 1,
  product: { id: 1, group_id: 3, name: 'Monthly', group_name: 'OpenAI', platform: 'openai', description: '', price_cny: 198, base_quota_usd: 700, tiers: [], max_members: 10, recruitment_hours: 48, for_sale: false, sort_order: 0 },
  member_count: 10, current_quota_usd: 800, next_quota_usd: 0, next_members: 0,
  starts_at: '2026-10-01T00:00:00Z', closes_at: '2026-10-03T00:00:00Z', status: 'full', joined: false,
  active_cards: 10, total_quota_usd: 8000, total_used_usd: 1200, weekly_quota_usd: 2000, weekly_used_usd: 600, expires_at: '2026-10-31T00:00:00Z'
}
const card: AdminMonthCard = { id: 11, user_id: 7, username: 'Customer', email: 'customer@example.test', group_id: 3, order_id: 91, code: 'CARD-11', group_name: 'OpenAI', platform: 'openai', product_name: 'Monthly', team_code: team.code, team_id: 1, status: 'active', total_quota_usd: 800, total_used_usd: 120, weekly_quota_usd: 200, weekly_used_usd: 60, starts_at: team.starts_at, expires_at: team.expires_at!, weekly_window_start: team.starts_at, weekly_window_end: team.expires_at!, priority: 0 }
const result = <T,>(items: T[], total = items.length) => ({ items, total, page: 1, page_size: 20, pages: Math.ceil(total / 20) })
const options = { props: { groups: [{ id: 3, name: 'OpenAI' }] }, global: { stubs: { BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><slot /></div>' } } } }
const mount = () => shallowMount(AdminEntitlementsPanel, options)

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(adminGroupBuyAPI.entitlementTeams).mockResolvedValue(result([team]))
  vi.mocked(adminGroupBuyAPI.entitlementTeamCards).mockResolvedValue(result([card]))
  vi.mocked(adminGroupBuyAPI.soloEntitlements).mockResolvedValue(result([{ ...card, team_id: null, team_code: '' }]))
  vi.mocked(adminGroupBuyAPI.cards).mockResolvedValue([card])
  vi.mocked(adminGroupBuyAPI.allocations).mockResolvedValue([])
})

describe('admin entitlement browsing', () => {
  it('loads active teams automatically, including full teams, without a user lookup', async () => {
    const wrapper = mount()
    await flushPromises()
    expect(adminGroupBuyAPI.entitlementTeams).toHaveBeenCalledWith({ page: 1, page_size: 20, validity: 'active', search: undefined, group_id: undefined }, expect.any(AbortSignal))
    expect(wrapper.getComponent(AdminEntitlementTeamsTable).props('items')).toEqual([team])
    expect(adminGroupBuyAPI.cards).not.toHaveBeenCalled()
    expect(adminGroupBuyAPI.allocations).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('opens members and their usage directly, then fetches a clicked customer’s charges', async () => {
    const wrapper = mount()
    await flushPromises()
    wrapper.getComponent(AdminEntitlementTeamsTable).vm.$emit('inspect', team)
    await flushPromises()
    expect(adminGroupBuyAPI.entitlementTeamCards).toHaveBeenCalledWith(team.code, { page: 1, page_size: 20, validity: 'all' }, expect.any(AbortSignal))
    const members = wrapper.getComponent(AdminEntitlementMembersTable)
    expect(members.props('items')).toEqual([card])
    members.vm.$emit('inspectUser', card.user_id)
    await flushPromises()
    expect(adminGroupBuyAPI.cards).toHaveBeenCalledWith(7)
    expect(adminGroupBuyAPI.allocations).toHaveBeenCalledWith(7)
    expect(wrapper.getComponent(AllocationTable).props('cards')).toEqual([card])
    wrapper.unmount()
  })

  it('filters on the server and resets pagination when a filter changes', async () => {
    vi.mocked(adminGroupBuyAPI.entitlementTeams).mockResolvedValue(result([team], 201))
    const wrapper = mount()
    await flushPromises()
    wrapper.getComponent(Pagination).vm.$emit('update:page', 3)
    await flushPromises()
    expect(adminGroupBuyAPI.entitlementTeams).toHaveBeenLastCalledWith(expect.objectContaining({ page: 3 }), expect.any(AbortSignal))
    await wrapper.get('input').setValue(' customer@example.test ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(adminGroupBuyAPI.entitlementTeams).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, search: 'customer@example.test' }), expect.any(AbortSignal))
    await wrapper.findAll('select')[1].setValue('all')
    await flushPromises()
    expect(adminGroupBuyAPI.entitlementTeams).toHaveBeenLastCalledWith(expect.objectContaining({ validity: 'all' }), expect.any(AbortSignal))
    await wrapper.findAll('select')[0].setValue(3)
    await flushPromises()
    expect(adminGroupBuyAPI.entitlementTeams).toHaveBeenLastCalledWith(expect.objectContaining({ group_id: 3 }), expect.any(AbortSignal))
    const sizeSelect = wrapper.findAll('select')[2]
    expect(sizeSelect.findAll('option').map(option => option.attributes('value'))).toEqual(['20', '50', '100'])
    await sizeSelect.setValue(100)
    await flushPromises()
    expect(adminGroupBuyAPI.entitlementTeams).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 100 }), expect.any(AbortSignal))
    expect(wrapper.getComponent(Pagination).props('showPageSizeSelector')).toBe(false)
    wrapper.unmount()
  })

  it('offers solo entitlements without a manual customer search', async () => {
    const wrapper = mount()
    await flushPromises()
    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    await flushPromises()
    expect(adminGroupBuyAPI.soloEntitlements).toHaveBeenCalledWith(expect.objectContaining({ validity: 'active', page: 1 }), expect.any(AbortSignal))
    expect(wrapper.findComponent(AdminEntitlementTeamsTable).exists()).toBe(false)
    expect(wrapper.getComponent(AdminEntitlementMembersTable).props('items')[0].team_id).toBeNull()
    wrapper.unmount()
  })

  it('ignores stale list responses and cancels the previous request', async () => {
    let finish: (value: ReturnType<typeof result<AdminTeamEntitlement>>) => void = () => {}
    vi.mocked(adminGroupBuyAPI.entitlementTeams).mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const wrapper = mount()
    const signal = vi.mocked(adminGroupBuyAPI.entitlementTeams).mock.calls[0][1]!
    await wrapper.findAll('select')[1].setValue('all')
    await flushPromises()
    expect(signal.aborted).toBe(true)
    finish(result([{ ...team, code: 'STALE' }]))
    await flushPromises()
    expect(wrapper.getComponent(AdminEntitlementTeamsTable).props('items')[0].code).toBe(team.code)
    wrapper.unmount()
  })

  it('does not replace a different team’s members with a late detail response', async () => {
    let finish: (value: ReturnType<typeof result<AdminMonthCard>>) => void = () => {}
    vi.mocked(adminGroupBuyAPI.entitlementTeamCards).mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const wrapper = mount()
    await flushPromises()
    wrapper.getComponent(AdminEntitlementTeamsTable).vm.$emit('inspect', team)
    await flushPromises()
    wrapper.findAllComponents(BaseDialog)[0].vm.$emit('close')
    wrapper.getComponent(AdminEntitlementTeamsTable).vm.$emit('inspect', { ...team, code: 'SECOND' })
    await flushPromises()
    finish(result([{ ...card, email: 'stale@example.test' }]))
    await flushPromises()
    expect(wrapper.getComponent(AdminEntitlementMembersTable).props('items')[0].email).toBe(card.email)
    wrapper.unmount()
  })

  it('shows a retryable error instead of treating failures as an empty list', async () => {
    vi.mocked(adminGroupBuyAPI.entitlementTeams).mockRejectedValueOnce(new Error('offline'))
    const wrapper = mount()
    await flushPromises()
    expect(wrapper.findComponent(AdminEntitlementTeamsTable).exists()).toBe(false)
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.getComponent(AdminEntitlementTeamsTable).props('items')).toEqual([team])
    wrapper.unmount()
  })
})
