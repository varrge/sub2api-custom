import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminGroupBuyView from '../AdminGroupBuyView.vue'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import zh from '@/i18n/locales/zh'

vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/common/BaseDialog.vue', () => ({ default: { props: ['show', 'title'], template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/api/admin', () => ({ default: { groups: { getAll: vi.fn().mockResolvedValue([]) } } }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => {
  const message = key.split('.').reduce((obj: any, part) => obj?.[part], zh) || key
  return String(message).replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? ''))
} }) }))
vi.mock('@/api/groupBuy', () => ({ adminGroupBuyAPI: { products: vi.fn().mockResolvedValue([]), teams: vi.fn(), cancelTeam: vi.fn(), freezePolicy: vi.fn().mockResolvedValue({ enabled: false }), setFreezePolicy: vi.fn().mockResolvedValue(undefined) }, groupBuyAPI: {} }))
vi.mock('@/api/monthCardRules', () => ({ monthCardRulesAPI: { admin: vi.fn(), save: vi.fn(), publish: vi.fn(), get: vi.fn(), read: vi.fn() } }))
const team = { id: 1, code: 'TEAM-TO-CANCEL', product_id: 1, product: { id: 1, name: '98月卡', max_members: 10 }, member_count: 2, current_quota_usd: 400, status: 'recruiting', closes_at: '2099-01-01T00:00:00Z' }
const button = (view: ReturnType<typeof mount>, text: string) => view.findAll('button').find(b => b.text() === text)!

describe('admin freeze policy in Beijing time', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(adminGroupBuyAPI.teams).mockResolvedValue([])
    vi.mocked(adminGroupBuyAPI.freezePolicy).mockResolvedValue({
      enabled: true,
      starts_at: '2026-12-31T23:30:45Z',
      ends_at: '2027-01-02T07:30:45+08:00'
    })
  })

  async function openPolicy() {
    const view = mount(AdminGroupBuyView, { global: { stubs: { RouterLink: true } } })
    await flushPromises()
    return view
  }

  it('loads and saves Beijing values without shifting the instant or dropping seconds', async () => {
    const view = await openPolicy()
    const inputs = view.findAll<HTMLInputElement>('input[type="datetime-local"]')
    expect(inputs.map(input => input.element.value)).toEqual(['2027-01-01T07:30:45.000', '2027-01-02T07:30:45.000'])
    expect(inputs.every(input => input.element.validity.valid)).toBe(true)
    await button(view, zh.groupBuy.saveFreezePolicy).trigger('click')
    await flushPromises()
    expect(adminGroupBuyAPI.setFreezePolicy).toHaveBeenCalledWith({
      enabled: true,
      starts_at: '2027-01-01T07:30:45+08:00',
      ends_at: '2027-01-02T07:30:45+08:00'
    })
    view.unmount()
  })

  it('saves edited Beijing times through a host DST gap and allows empty boundaries', async () => {
    const view = await openPolicy()
    const inputs = view.findAll('input[type="datetime-local"]')
    await inputs[0].setValue('2027-03-14T02:30:15')
    await inputs[1].setValue('2027-03-15T02:30:45')
    await button(view, zh.groupBuy.saveFreezePolicy).trigger('click')
    await flushPromises()
    expect(adminGroupBuyAPI.setFreezePolicy).toHaveBeenLastCalledWith({
      enabled: true,
      starts_at: '2027-03-14T02:30:15+08:00',
      ends_at: '2027-03-15T02:30:45+08:00'
    })
    await inputs[0].setValue('')
    await inputs[1].setValue('')
    await button(view, zh.groupBuy.saveFreezePolicy).trigger('click')
    await flushPromises()
    expect(adminGroupBuyAPI.setFreezePolicy).toHaveBeenLastCalledWith({ enabled: true, starts_at: null, ends_at: null })
    view.unmount()
  })

  it.each([0, 1])('rejects malformed nonempty boundary %i and allows correction', async index => {
    const view = await openPolicy()
    const input = view.findAll<HTMLInputElement>('input[type="datetime-local"]')[index]
    input.element.type = 'text'
    await input.setValue('2027-02-30T12:00')
    await button(view, zh.groupBuy.saveFreezePolicy).trigger('click')
    await flushPromises()
    expect(adminGroupBuyAPI.setFreezePolicy).not.toHaveBeenCalled()
    expect(view.get('[role="alert"]').text()).toBe(zh.groupBuy.freezePolicyFailed)
    expect(button(view, zh.groupBuy.saveFreezePolicy).attributes('disabled')).toBeUndefined()
    await input.setValue(index === 0 ? '2027-01-01T07:30' : '2027-01-02T07:30')
    await button(view, zh.groupBuy.saveFreezePolicy).trigger('click')
    await flushPromises()
    expect(adminGroupBuyAPI.setFreezePolicy).toHaveBeenCalledTimes(1)
    expect(view.find('[role="alert"]').exists()).toBe(false)
    view.unmount()
  })
})

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
