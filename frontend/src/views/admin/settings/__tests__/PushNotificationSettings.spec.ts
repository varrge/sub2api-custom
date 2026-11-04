import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import PushNotificationSettings from '../PushNotificationSettings.vue'
import { pushNotificationsApi, type PushConfig } from '@/api/admin/pushNotifications'

vi.mock('@/api/admin/pushNotifications', () => ({ pushNotificationsApi: { get: vi.fn(), save: vi.fn(), history: vi.fn(), preview: vi.fn(), test: vi.fn() } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'zh' } }) }))
enableAutoUnmount(afterEach)

function config(): PushConfig {
  return { revision: 2, enabled: true, channels: [{
    id: 'first', name: '运营提醒', type: 'serverchan', enabled: true, credential: '', configured: true, endpoint_hint: 'sctapi.ftqq.com',
    events: ['payment.recharged'], title_template: '{{event_name}}', body_template: '## {{event_name}}\n\n{{details}}', tags_template: '运营', quota_threshold: 20, serverchan_channel: '', webhook_format: 'json',
  }] }
}
async function view() { const w = mount(PushNotificationSettings); await flushPromises(); return w }
function button(w: ReturnType<typeof mount>, label: string) { return w.findAll('button').find(b => b.text().includes(label))! }

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(pushNotificationsApi.get).mockResolvedValue(config())
  vi.mocked(pushNotificationsApi.history).mockResolvedValue([])
  vi.mocked(pushNotificationsApi.save).mockImplementation(async c => ({ ...c, revision: c.revision + 1, channels: c.channels.map(ch => ({ ...ch, credential: '', configured: true })) }))
  vi.mocked(pushNotificationsApi.test).mockResolvedValue({ queued: true })
})

describe('Push notification settings', () => {
  it('preserves masked secrets, supports multiple events and keeps saving independent from the parent form', async () => {
    const w = await view()
    expect(w.find('form').exists()).toBe(false)
    for (const b of w.findAll('button')) expect(b.attributes('type')).toBe('button')
    expect(w.get('#channel-credential').attributes('type')).toBe('password')
    expect((w.get('#channel-credential').element as HTMLInputElement).value).toBe('')
    await w.get('[id="evt-team.full"]').setValue(true)
    await w.get('[id="evt-shared_credit.low"]').setValue(true)
    await w.get('#quota-threshold').setValue(15)
    expect(button(w, '真实测试发送').attributes('disabled')).toBeDefined()
    await button(w, '保存修改').trigger('click'); await flushPromises()
    expect(pushNotificationsApi.save).toHaveBeenCalledWith(expect.objectContaining({ revision: 2, channels: [expect.objectContaining({ credential: '', events: ['payment.recharged', 'team.full', 'shared_credit.low'], quota_threshold: 15 })] }))
    await button(w, '真实测试发送').trigger('click'); await flushPromises()
    expect(pushNotificationsApi.test).toHaveBeenCalledWith('first', 3)
  })

  it('retains drafts on conflict, and locks edits while save is pending', async () => {
    const w = await view()
    let reject!: (error: unknown) => void
    vi.mocked(pushNotificationsApi.save).mockImplementationOnce(() => new Promise((_resolve, fail) => { reject = fail }))
    await w.get('#channel-name').setValue('不要丢失')
    await button(w, '保存修改').trigger('click')
    expect(w.get('fieldset').attributes('disabled')).toBeDefined()
    reject({ status: 409, message: '配置已更新，请刷新' }); await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('配置已更新')
    expect((w.get('#channel-name').element as HTMLInputElement).value).toBe('不要丢失')
    expect(w.get('fieldset').attributes('disabled')).toBeUndefined()
  })

  it('uses safe Markdown preview and does not send credentials for preview', async () => {
    vi.mocked(pushNotificationsApi.preview).mockResolvedValue({ title: '满团通知', body: '## 满团成功\n\n**已发卡** <script>alert(1)</script><img src="https://example.com/track"><a href="javascript:alert(1)">bad</a>', tags: '月卡' })
    const w = await view()
    await w.get('#channel-credential').setValue('SCTnewsecret')
    await button(w, '生成预览').trigger('click'); await flushPromises()
    expect(pushNotificationsApi.preview).toHaveBeenCalledWith(expect.objectContaining({ credential: '' }), 'payment.recharged')
    expect(w.get('[data-testid="push-preview"] h2').text()).toBe('满团成功')
    expect(w.find('[data-testid="push-preview"] script').exists()).toBe(false)
    expect(w.find('[data-testid="push-preview"] img').exists()).toBe(false)
    expect(w.get('[data-testid="push-preview"] a').attributes('href')).toBeUndefined()
    await w.get('#title-template').setValue('新标题')
    expect(w.find('[data-testid="push-preview"]').exists()).toBe(false)
  })

  it('discards a stale preview after changing events', async () => {
    let finish!: (v: { title: string; body: string; tags: string }) => void
    vi.mocked(pushNotificationsApi.preview).mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const w = await view()
    await button(w, '生成预览').trigger('click')
    await w.get('#preview-event-select').setValue('team.full')
    finish({ title: 'stale', body: 'stale', tags: '' }); await flushPromises()
    expect(w.find('[data-testid="push-preview"]').exists()).toBe(false)
  })

  it('blocks editing after initial load failure and supports retry', async () => {
    vi.mocked(pushNotificationsApi.get).mockRejectedValueOnce(new Error('network'))
    const w = await view()
    expect(w.find('fieldset').exists()).toBe(false)
    await button(w, '重新加载').trigger('click'); await flushPromises()
    expect(w.find('#channel-name').exists()).toBe(true)
  })

  it('adds a separate custom webhook without carrying the existing secret', async () => {
    const w = await view()
    await button(w, 'Webhook').trigger('click')
    expect((w.get('#channel-name').element as HTMLInputElement).value).toBe('Webhook')
    expect((w.get('#channel-credential').element as HTMLInputElement).value).toBe('')
    expect(button(w, '真实测试发送').attributes('disabled')).toBeDefined()
    await w.get('#channel-credential').setValue('https://example.com/hook')
    await button(w, '保存修改').trigger('click'); await flushPromises()
    expect(vi.mocked(pushNotificationsApi.save).mock.calls[0][0].channels).toHaveLength(2)
    expect((w.get('#channel-credential').element as HTMLInputElement).value).toBe('')
  })
})
