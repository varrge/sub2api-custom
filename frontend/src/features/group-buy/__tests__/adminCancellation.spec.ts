import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminGroupBuyView from '../AdminGroupBuyView.vue'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import zh from '@/i18n/locales/zh'

vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/common/BaseDialog.vue', () => ({ default: { props: ['show', 'title'], template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/api/admin', () => ({ default: { groups: { getAll: vi.fn().mockResolvedValue([]) } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => {
  const message = key.split('.').reduce((obj: any, part) => obj?.[part], zh) || key
  return String(message).replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? ''))
} }) }))
vi.mock('@/api/groupBuy', () => ({ adminGroupBuyAPI: { products: vi.fn().mockResolvedValue([]), teams: vi.fn(), cancelTeam: vi.fn() }, groupBuyAPI: {} }))
const team = { id: 1, code: 'TEAM-TO-CANCEL', product_id: 1, product: { id: 1, name: '98月卡', max_members: 10 }, member_count: 2, current_quota_usd: 400, status: 'recruiting', closes_at: '2099-01-01T00:00:00Z' }
const button = (view: ReturnType<typeof mount>, text: string) => view.findAll('button').find(b => b.text() === text)!

describe('admin recruitment cancellation', () => {
  it('confirms the specific team and replaces its state without offering another cancellation', async () => {
    vi.mocked(adminGroupBuyAPI.teams).mockResolvedValue([team] as any)
    vi.mocked(adminGroupBuyAPI.cancelTeam).mockResolvedValue({ ...team, status: 'cancelled' } as any)
    const view = mount(AdminGroupBuyView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    await button(view, '拼团记录').trigger('click')
    await button(view, '取消招募').trigger('click')
    const dialog = view.get('[role="dialog"]')
    expect(dialog.text()).toContain('TEAM-TO-CANCEL')
    expect(dialog.text()).toContain('已购月卡和已获得额度保留，不自动退款')
    expect(adminGroupBuyAPI.cancelTeam).not.toHaveBeenCalled()
    await dialog.findAll('button').find(b => b.text() === '取消招募')!.trigger('click')
    await flushPromises()
    expect(adminGroupBuyAPI.cancelTeam).toHaveBeenCalledWith('TEAM-TO-CANCEL')
    expect(view.find('[role="dialog"]').exists()).toBe(false)
    expect(view.text()).toContain('已取消招募')
    expect(button(view, '取消招募')).toBeUndefined()
  })
})
