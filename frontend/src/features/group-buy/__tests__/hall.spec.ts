import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import GroupBuyHallView from '../GroupBuyHallView.vue'
import TeamCard from '../TeamCard.vue'
import { groupBuyAPI } from '@/api/groupBuy'
import type { GroupBuyTeam } from '@/types/groupBuy'

vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
const route = vi.hoisted(() => ({ query: {} as Record<string, string> }))
const push = vi.hoisted(() => vi.fn())
const replace = vi.hoisted(() => vi.fn())
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ push, replace }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/api/groupBuy', () => ({ groupBuyAPI: { teams: vi.fn(), team: vi.fn() } }))
const team = { id: 1, code: 'REAL-TEAM', status: 'recruiting', joined: false, closes_at: '2099-01-01T00:00:00Z', member_count: 4, product: { group_id: 10, group_name: 'Real group', max_members: 10 } } as GroupBuyTeam
const options = { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, RouterLink: true } } }
beforeEach(() => { route.query = {}; vi.clearAllMocks() })

describe('hall live data and share links', () => {
  it('removes full and expired teams from recruitment and forwards selected team to existing checkout', async () => {
    vi.mocked(groupBuyAPI.teams).mockResolvedValue([team, { ...team, id: 2, status: 'full' }, { ...team, id: 3, closes_at: '2000-01-01T00:00:00Z' }])
    const wrapper = shallowMount(GroupBuyHallView, options)
    await flushPromises()
    expect(wrapper.findAllComponents(TeamCard)).toHaveLength(1)
    wrapper.getComponent(TeamCard).vm.$emit('join', team)
    expect(push).toHaveBeenCalledWith({ path: '/purchase', query: { tab: 'subscription', mode: 'join', team_code: team.code } })
    wrapper.unmount()
  })
  it('keeps closed team details available from an existing share link', async () => {
    route.query = { team_code: 'CLOSED-TEAM' }
    vi.mocked(groupBuyAPI.teams).mockResolvedValue([])
    vi.mocked(groupBuyAPI.team).mockResolvedValue({ ...team, code: 'CLOSED-TEAM', status: 'closed' })
    const wrapper = shallowMount(GroupBuyHallView, options)
    await flushPromises()
    expect(groupBuyAPI.team).toHaveBeenCalledWith('CLOSED-TEAM')
    expect(wrapper.getComponent(TeamCard).props('team').status).toBe('closed')
    wrapper.unmount()
  })
  it('distinguishes an unavailable service from an empty hall', async () => {
    vi.mocked(groupBuyAPI.teams).mockRejectedValue(new Error('offline'))
    const wrapper = shallowMount(GroupBuyHallView, options)
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('groupBuy.loadFailed')
    expect(wrapper.text()).not.toContain('groupBuy.noTeams')
    wrapper.unmount()
  })
})
