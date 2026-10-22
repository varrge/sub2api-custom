import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import PrivilegeSettingsPanel from '../PrivilegeSettingsPanel.vue'

const mocks = vi.hoisted(() => ({ get: vi.fn(), update: vi.fn(), success: vi.fn(), error: vi.fn() }))
vi.mock('@/api/privileges', () => ({ getPrivilegeSettings: mocks.get, updatePrivilegeSettings: mocks.update }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.success, showError: mocks.error }) }))
let wrapper: VueWrapper

function start() {
  wrapper = mount(PrivilegeSettingsPanel, { global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] } })
  return wrapper
}

async function addRule(view: VueWrapper) {
  await view.get('[data-testid="personal-rule-add"]').trigger('click')
  const rules = view.findAll('[data-testid="personal-rule"]')
  return rules[rules.length - 1]
}

describe('PrivilegeSettingsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.get.mockReset().mockResolvedValue({ enterprise_welcome: { enabled: true, message: '欢迎尊贵的企业用户' } })
    mocks.update.mockReset().mockImplementation(async payload => payload)
  })
  afterEach(() => { wrapper?.unmount(); vi.useRealTimers() })

  it('edits the enterprise welcome toggle and text and saves only this configuration', async () => {
    const view = start()
    await flushPromises()
    expect(view.find('[data-testid="privilege-welcome"]').exists()).toBe(false)
    const toggle = view.get('[role="switch"]')
    expect(toggle.attributes('aria-label')).toBe('企业用户欢迎提示')
    await toggle.trigger('click')
    await view.get('[data-testid="enterprise-welcome-message"]').setValue(' 欢迎伙伴 ')
    await view.get('[data-testid="privilege-save"]').trigger('click')
    await flushPromises()
    expect(mocks.update).toHaveBeenCalledWith({
      enterprise_welcome: { enabled: false, message: '欢迎伙伴', effect: 'capsule' },
      personal_rules: []
    })
    expect(mocks.success).toHaveBeenCalledOnce()
  })

  it('previews without saving, closes completely, and connects validation to the input', async () => {
    vi.useFakeTimers()
    const view = start()
    await flushPromises()
    await view.get('[data-testid="privilege-preview"]').trigger('click')
    await vi.advanceTimersByTimeAsync(350)
    expect(view.get('[data-testid="privilege-welcome"]').classes()).toContain('pw-stage-entering')
    await view.get('[data-testid="privilege-welcome-close"]').trigger('click')
    await vi.advanceTimersByTimeAsync(250)
    expect(view.find('[data-testid="privilege-welcome"]').exists()).toBe(false)
    expect(mocks.update).not.toHaveBeenCalled()
    const input = view.get('[data-testid="enterprise-welcome-message"]')
    await input.setValue(' ')
    expect(input.attributes('aria-invalid')).toBe('true')
    expect(view.find('#' + input.attributes('aria-describedby')).exists()).toBe(true)
    expect((view.get('[data-testid="privilege-save"]').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('shows a recoverable loading error instead of editable defaults', async () => {
    mocks.get.mockRejectedValueOnce(new Error('offline'))
    const view = start()
    await flushPromises()
    expect(view.text()).toContain('offline')
    expect(view.find('[data-testid="privilege-save"]').exists()).toBe(false)
    await view.get('button').trigger('click')
    await flushPromises()
    expect(view.find('[data-testid="privilege-save"]').exists()).toBe(true)
    expect(mocks.update).not.toHaveBeenCalled()
  })

  it('counts emoji as code points and accepts 80 but rejects 81', async () => {
    const view = start()
    await flushPromises()
    const input = view.get('[data-testid="enterprise-welcome-message"]')
    expect(input.attributes('maxlength')).toBeUndefined()
    await input.setValue('👑'.repeat(80))
    expect(view.text()).toContain('80/80')
    expect(input.attributes('aria-invalid')).toBe('false')
    await view.get('[data-testid="privilege-save"]').trigger('click')
    await flushPromises()
    expect(mocks.update).toHaveBeenCalledWith({
      enterprise_welcome: { enabled: true, message: '👑'.repeat(80), effect: 'capsule' },
      personal_rules: []
    })
    await input.setValue('👑'.repeat(81))
    expect(view.text()).toContain('81/80')
    expect(input.attributes('aria-invalid')).toBe('true')
    expect((view.get('[data-testid="privilege-save"]').element as HTMLButtonElement).disabled).toBe(true)
    expect((view.get('[data-testid="privilege-preview"]').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('shows an empty personal rules state with a single add entry point', async () => {
    const view = start()
    await flushPromises()
    expect(view.findAll('[data-testid="personal-rule"]')).toHaveLength(0)
    expect(view.text()).toContain('暂无普通用户特权规则')
    expect(view.text()).toContain('同时满足多条仅显示最靠前一条')
    expect(view.text()).toContain('actual_cost')
    await view.get('[data-testid="personal-rule-add"]').trigger('click')
    expect(view.findAll('[data-testid="personal-rule"]')).toHaveLength(1)
  })

  it('adds all four rule kinds and switches manual to show the user ID input', async () => {
    const view = start()
    await flushPromises()
    const first = await addRule(view)
    const kindSelect = first.get('[data-testid="personal-rule-kind"]')
    expect(kindSelect.element).toBeInstanceOf(HTMLSelectElement)
    expect((kindSelect.element as HTMLSelectElement).value).toBe('total_spend')
    expect(first.find('[data-testid="personal-rule-threshold"]').exists()).toBe(true)
    expect(first.find('[data-testid="personal-rule-user-ids"]').exists()).toBe(false)

    await kindSelect.setValue('total_recharge')
    await addRule(view)
    await addRule(view)
    const fourth = await addRule(view)
    await fourth.get('[data-testid="personal-rule-kind"]').setValue('account_age_days')
    await fourth.get('[data-testid="personal-rule-kind"]').setValue('manual')
    expect(fourth.find('[data-testid="personal-rule-threshold"]').exists()).toBe(false)
    expect(fourth.find('[data-testid="personal-rule-user-ids"]').exists()).toBe(true)
    expect(fourth.get('[data-testid="personal-rule-user-ids"]').attributes('maxlength')).toBeUndefined()
    expect(view.text()).toContain('可在用户管理查看 ID')

    const rules = view.findAll('[data-testid="personal-rule"]')
    expect(rules.map(rule => rule.get('[data-testid="personal-rule-kind"]').element.value))
      .toEqual(['total_recharge', 'total_spend', 'total_spend', 'manual'])
    expect(rules[3].find('[data-testid="personal-rule-threshold"]').exists()).toBe(false)
  })

  it('reorders and deletes rules as drafts before saving', async () => {
    const view = start()
    await flushPromises()
    const first = await addRule(view)
    await first.get('[data-testid="personal-rule-title"]').setValue('头牌')
    const second = await addRule(view)
    await second.get('[data-testid="personal-rule-title"]').setValue('二牌')
    await second.get('[data-testid="personal-rule-threshold"]').setValue('100')

    await first.get('[data-testid="personal-rule-down"]').trigger('click')
    let titles = view.findAll('[data-testid="personal-rule"]')
      .map(rule => (rule.get('[data-testid="personal-rule-title"]').element as HTMLInputElement).value)
    expect(titles).toEqual(['二牌', '头牌'])

    await view.findAll('[data-testid="personal-rule"]')[1]
      .get('[data-testid="personal-rule-up"]').trigger('click')
    titles = view.findAll('[data-testid="personal-rule"]')
      .map(rule => (rule.get('[data-testid="personal-rule-title"]').element as HTMLInputElement).value)
    expect(titles).toEqual(['头牌', '二牌'])

    expect(mocks.update).not.toHaveBeenCalled()
    await view.findAll('[data-testid="personal-rule"]')[0]
      .get('[data-testid="personal-rule-delete"]').trigger('click')
    titles = view.findAll('[data-testid="personal-rule"]')
      .map(rule => (rule.get('[data-testid="personal-rule-title"]').element as HTMLInputElement).value)
    expect(titles).toEqual(['二牌'])
    expect(mocks.update).not.toHaveBeenCalled()

    await view.get('[data-testid="privilege-save"]').trigger('click')
    await flushPromises()
    expect(mocks.update).toHaveBeenCalledWith({
      enterprise_welcome: { enabled: true, message: '欢迎尊贵的企业用户', effect: 'capsule' },
      personal_rules: [{
        id: expect.any(String),
        enabled: true,
        title: '二牌',
        kind: 'total_spend',
        threshold: 100,
        user_ids: [],
        welcome_enabled: true,
        welcome_message: '欢迎回来',
        welcome_effect: 'capsule'
      }]
    })
  })

  it('saves a rule whose welcome animation is disabled but keeps its title', async () => {
    const view = start()
    await flushPromises()
    const rule = await addRule(view)
    await rule.get('[data-testid="personal-rule-title"]').setValue('沉默的守护者')
    await rule.get('[data-testid="personal-rule-welcome-message"]').setValue('')
    await rule.get('[data-testid="personal-rule-welcome-enabled"]').get('[role="switch"]').trigger('click')
    await view.get('[data-testid="privilege-save"]').trigger('click')
    await flushPromises()
    expect(mocks.update).toHaveBeenCalledWith({
      enterprise_welcome: { enabled: true, message: '欢迎尊贵的企业用户', effect: 'capsule' },
      personal_rules: [{
        id: expect.any(String),
        enabled: true,
        title: '沉默的守护者',
        kind: 'total_spend',
        threshold: 1,
        user_ids: [],
        welcome_enabled: false,
        welcome_message: '',
        welcome_effect: 'capsule'
      }]
    })
  })

  it('saves manual rules with parsed positive user IDs and zero threshold', async () => {
    const view = start()
    await flushPromises()
    const rule = await addRule(view)
    await rule.get('[data-testid="personal-rule-title"]').setValue('特邀用户')
    await rule.get('[data-testid="personal-rule-kind"]').setValue('manual')
    await rule.get('[data-testid="personal-rule-user-ids"]').setValue('7, 8 9')
    await view.get('[data-testid="privilege-save"]').trigger('click')
    await flushPromises()
    expect(mocks.update).toHaveBeenCalledWith({
      enterprise_welcome: { enabled: true, message: '欢迎尊贵的企业用户', effect: 'capsule' },
      personal_rules: [{
        id: expect.any(String),
        enabled: true,
        title: '特邀用户',
        kind: 'manual',
        threshold: 0,
        user_ids: [7, 8, 9],
        welcome_enabled: true,
        welcome_message: '欢迎回来',
        welcome_effect: 'capsule'
      }]
    })
  })

  it('disables saving while any rule is invalid and shows the rule error', async () => {
    const view = start()
    await flushPromises()
    const rule = await addRule(view)
    const title = rule.get('[data-testid="personal-rule-title"]')
    await title.setValue('')
    expect(title.attributes('aria-invalid')).toBe('true')
    expect(view.find('#' + title.attributes('aria-describedby')).exists()).toBe(true)
    expect(view.text()).toContain('头衔需为 1–24 个字符的单行文本')
    // 同一行的 threshold / welcome_message 也关联同一条行错误
    expect(rule.get('[data-testid="personal-rule-threshold"]').attributes('aria-invalid')).toBe('true')
    expect(rule.get('[data-testid="personal-rule-welcome-message"]').attributes('aria-invalid')).toBe('true')
    expect(rule.get('[data-testid="personal-rule-threshold"]').attributes('aria-describedby'))
      .toBe(title.attributes('aria-describedby'))
    // 普通规则草稿错误只禁用最终保存，不影响企业预览
    expect((view.get('[data-testid="privilege-save"]').element as HTMLButtonElement).disabled).toBe(true)
    expect((view.get('[data-testid="privilege-preview"]').element as HTMLButtonElement).disabled).toBe(false)
    expect(mocks.update).not.toHaveBeenCalled()

    // manual 的空 ID 同样错误且关联 textarea
    await title.setValue('体验官')
    await rule.get('[data-testid="personal-rule-kind"]').setValue('manual')
    const userIds = rule.get('[data-testid="personal-rule-user-ids"]')
    expect(userIds.attributes('aria-invalid')).toBe('true')
    expect(view.find('#' + userIds.attributes('aria-describedby')).exists()).toBe(true)
    expect((view.get('[data-testid="privilege-save"]').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('previews a valid rule welcome without saving', async () => {
    vi.useFakeTimers()
    const view = start()
    await flushPromises()
    const rule = await addRule(view)
    await rule.get('[data-testid="personal-rule-title"]').setValue('体验官')
    await rule.get('[data-testid="personal-rule-welcome-message"]').setValue('欢迎体验')
    await rule.get('[data-testid="personal-rule-preview"]').trigger('click')
    await vi.advanceTimersByTimeAsync(350)
    expect(view.get('[data-testid="privilege-welcome"]').text()).toContain('欢迎体验')
    expect(mocks.update).not.toHaveBeenCalled()
  })

  it('selects and previews independent enterprise and personal effects before explicitly saving', async () => {
    vi.useFakeTimers()
    const view = start()
    await flushPromises()
    const enterprise = view.get('[data-testid="enterprise-welcome-effect"]')
    expect((enterprise.get('input[value="capsule"]').element as HTMLInputElement).checked).toBe(true)
    await enterprise.get('input[value="slide"]').setValue(true)
    await view.get('[data-testid="privilege-preview"]').trigger('click')
    expect(view.get('[data-testid="privilege-welcome"]').attributes('data-effect')).toBe('slide')
    const rule = await addRule(view)
    await rule.get('[data-testid="personal-rule-title"]').setValue('伙伴')
    await rule.get('[data-testid="personal-rule-welcome-effect"] input[value="ribbon"]').setValue(true)
    await rule.get('[data-testid="personal-rule-preview"]').trigger('click')
    expect(view.get('[data-testid="privilege-welcome"]').attributes('data-effect')).toBe('ribbon')
    expect((enterprise.get('input[value="slide"]').element as HTMLInputElement).checked).toBe(true)
    expect(mocks.update).not.toHaveBeenCalled()
    await view.get('[data-testid="privilege-save"]').trigger('click')
    await flushPromises()
    expect(mocks.update.mock.calls[0][0]).toMatchObject({
      enterprise_welcome: { effect: 'slide' }, personal_rules: [{ welcome_effect: 'ribbon' }]
    })
    await rule.get('[data-testid="personal-rule-welcome-enabled"] [role="switch"]').trigger('click')
    expect(rule.findAll('[data-testid="personal-rule-welcome-effect"] input').every(input => (input.element as HTMLInputElement).disabled)).toBe(true)
  })
})
