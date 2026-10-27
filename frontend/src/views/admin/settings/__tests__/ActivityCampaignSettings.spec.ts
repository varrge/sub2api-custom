import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ActivityCampaignSettings from '../ActivityCampaignSettings.vue'
import { activityCampaignsAPI, type ActivityCampaignConfig } from '@/api/activityCampaigns'
import zh from '@/i18n/locales/zh/activityCampaigns'
vi.mock('@/api/activityCampaigns', () => ({ activityCampaignsAPI: { adminGet: vi.fn(), adminSave: vi.fn() } }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({
    t: (key: string, params: Record<string, unknown> = {}) => {
      const value = key.split('.').reduce<unknown>((o, part) => (o as Record<string, unknown>)?.[part], zh)
      return String(value ?? key).replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? ''))
    },
  }),
}))
enableAutoUnmount(afterEach)
function config(): ActivityCampaignConfig {
  return { id: 'double11-2026-wave1', revision: 3, enabled: false, title: '双十一开门礼',
    subtitle: '趣味答题', description: '活动说明', starts_at: '2026-10-31T16:00:00Z', ends_at: '2026-11-05T16:00:00Z',
    box_campaign_id: '', questions: [1, 2, 3].map(n => ({ id: `q${n}`, prompt: `问题${n}`, options: ['一', '二'], answer: 0 })) }
}
async function view() {
  const wrapper = mount(ActivityCampaignSettings, { global: { stubs: { Icon: true, RouterLink: { template: '<a><slot /></a>' } } } })
  await flushPromises()
  return wrapper
}
beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(activityCampaignsAPI.adminGet).mockResolvedValue(config())
  vi.mocked(activityCampaignsAPI.adminSave).mockImplementation(async payload => ({ ...payload, revision: payload.revision + 1 }))
})
describe('ActivityCampaignSettings', () => {
  it('converts Beijing input independently from browser timezone and uses an independent save button', async () => {
    const wrapper = await view()
    expect((wrapper.get('[data-testid="campaign-start"]').element as HTMLInputElement).value).toBe('2026-11-01T00:00')
    expect(wrapper.find('form').exists()).toBe(false)
    for (const button of wrapper.findAll('button')) expect(button.attributes('type')).toBe('button')
    await wrapper.get('[data-testid="campaign-title"]').setValue('新标题')
    await wrapper.get('[data-testid="campaign-start"]').setValue('2026-11-01T09:30')
    await wrapper.get('[data-testid="campaign-save"]').trigger('click')
    await flushPromises()
    expect(activityCampaignsAPI.adminSave).toHaveBeenCalledWith(expect.objectContaining({
      revision: 3, title: '新标题', starts_at: '2026-11-01T01:30:00.000Z', ends_at: '2026-11-05T16:00:00Z',
    }))
    expect(wrapper.text()).toContain('活动配置已保存')
  })
  it('preserves unsaved edits on conflict and requires a refresh before saving again', async () => {
    const wrapper = await view()
    vi.mocked(activityCampaignsAPI.adminSave).mockRejectedValueOnce({ status: 409 })
    await wrapper.get('[data-testid="campaign-title"]').setValue('不要覆盖')
    await wrapper.get('[data-testid="campaign-save"]').trigger('click')
    await flushPromises()
    expect((wrapper.get('[data-testid="campaign-title"]').element as HTMLInputElement).value).toBe('不要覆盖')
    expect(wrapper.get('[data-testid="campaign-save"]').attributes('disabled')).toBeDefined()
    vi.mocked(activityCampaignsAPI.adminGet).mockResolvedValue({ ...config(), revision: 4, title: '服务器新标题' })
    await wrapper.get('[data-testid="campaign-refresh"]').trigger('click')
    await flushPromises()
    expect((wrapper.get('[data-testid="campaign-title"]').element as HTMLInputElement).value).toBe('服务器新标题')
    expect(wrapper.get('[data-testid="campaign-save"]').attributes('disabled')).toBeUndefined()
  })
  it('preserves original seconds when only changing presentation or enable state', async () => {
    const c = { ...config(), starts_at: '2026-11-01T00:00:30.123+08:00', ends_at: '2026-11-06T00:00:40.456+08:00' }
    vi.mocked(activityCampaignsAPI.adminGet).mockResolvedValue(c)
    const wrapper = await view()
    await wrapper.get('[data-testid="campaign-title"]').setValue('仅改标题')
    await wrapper.get('[data-testid="campaign-save"]').trigger('click')
    await flushPromises()
    expect(activityCampaignsAPI.adminSave).toHaveBeenCalledWith(expect.objectContaining({
      starts_at: c.starts_at, ends_at: c.ends_at,
    }))
  })
  it('rejects invalid date order and retains the answer index when an earlier option is removed', async () => {
    const c = config()
    c.questions[0].options = ['甲', '乙', '丙']
    c.questions[0].answer = 2
    vi.mocked(activityCampaignsAPI.adminGet).mockResolvedValue(c)
    const wrapper = await view()
    await wrapper.get('[data-testid="campaign-end"]').setValue('2026-10-30T00:00')
    await wrapper.get('[data-testid="campaign-save"]').trigger('click')
    expect(activityCampaignsAPI.adminSave).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="campaign-end"]').setValue('2026-11-06T00:00')
    const remove = wrapper.get('[data-testid="campaign-question-0"]').findAll('button').find(b => b.attributes('aria-label') === '删除 选项 1')
    expect(remove).toBeDefined()
    await remove!.trigger('click')
    await wrapper.get('[data-testid="campaign-save"]').trigger('click')
    await flushPromises()
    expect(vi.mocked(activityCampaignsAPI.adminSave).mock.calls[0][0].questions[0]).toMatchObject({ options: ['乙', '丙'], answer: 1 })
  })
})
