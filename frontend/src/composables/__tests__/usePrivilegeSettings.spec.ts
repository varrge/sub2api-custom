import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { usePrivilegeSettings } from '../usePrivilegeSettings'

const mocks = vi.hoisted(() => ({
  get: vi.fn(), update: vi.fn(), showSuccess: vi.fn(), showError: vi.fn()
}))
vi.mock('@/api/privileges', () => ({ getPrivilegeSettings: mocks.get, updatePrivilegeSettings: mocks.update }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.showSuccess, showError: mocks.showError }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'zh' } }) }))
const defaults = () => ({ enterprise_welcome: { enabled: true, message: '欢迎尊贵的企业用户', effect: 'capsule' }, personal_rules: [] })
let wrapper: VueWrapper

function start() {
  let state!: ReturnType<typeof usePrivilegeSettings>
  wrapper = mount(defineComponent({
    setup() { state = usePrivilegeSettings(); return () => h('div') }
  }))
  return state
}

describe('privilege settings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.get.mockReset().mockResolvedValue(defaults())
    mocks.update.mockReset().mockImplementation(async payload => payload)
  })
  afterEach(() => { wrapper?.unmount(); vi.useRealTimers() })

  it('loads and saves the welcome toggle and trimmed message independently', async () => {
    const state = start()
    expect(state.loading.value).toBe(true)
    await flushPromises()
    state.settings.value!.enterprise_welcome = { enabled: false, message: ' 欢迎伙伴 ' }
    await state.save()
    expect(mocks.update).toHaveBeenCalledWith({ enterprise_welcome: { enabled: false, message: '欢迎伙伴', effect: 'capsule' }, personal_rules: [] })
    expect(state.settings.value!.enterprise_welcome.enabled).toBe(false)
    expect(mocks.showSuccess).toHaveBeenCalledOnce()
    expect(state.saving.value).toBe(false)
  })

  it('does not save defaults after a failed load and can retry', async () => {
    mocks.get.mockRejectedValueOnce(new Error('load failed'))
    const state = start()
    await flushPromises()
    expect(state.loadError.value).toBeTruthy()
    expect(state.settings.value).toBeNull()
    await state.save()
    expect(mocks.update).not.toHaveBeenCalled()
    await state.load()
    expect(state.loadError.value).toBe('')
    expect(state.settings.value).toEqual(defaults())
  })

  it('validates empty, long and multiline messages without requesting a save', async () => {
    const state = start()
    await flushPromises()
    for (const message of [' ', '企'.repeat(81), 'welcome\nback', 'hello\u0085world']) {
      state.settings.value!.enterprise_welcome.message = message
      expect(state.validationError.value).toBeTruthy()
      await state.save()
    }
    expect(mocks.update).not.toHaveBeenCalled()
    state.settings.value!.enterprise_welcome.message = '企'.repeat(80)
    expect(state.validationError.value).toBe('')
  })

  it('reports save errors without dropping edits or claiming success', async () => {
    mocks.update.mockRejectedValueOnce(new Error('save failed'))
    const state = start()
    await flushPromises()
    state.settings.value!.enterprise_welcome.message = '欢迎伙伴'
    await state.save()
    expect(mocks.showError).toHaveBeenCalledOnce()
    expect(mocks.showSuccess).not.toHaveBeenCalled()
    expect(state.settings.value!.enterprise_welcome.message).toBe('欢迎伙伴')
    expect(state.saving.value).toBe(false)
  })

  it('previews unsaved copy locally without persisting it', async () => {
    vi.useFakeTimers()
    const state = start()
    await flushPromises()
    state.settings.value!.enterprise_welcome.message = ' 欢迎伙伴 '
    state.showPreview()
    expect(state.preview.message.value).toBe('欢迎伙伴')
    await vi.advanceTimersByTimeAsync(350)
    expect(state.preview.phase.value).toBe('entering')
    expect(mocks.update).not.toHaveBeenCalled()
    wrapper.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('saves four condition types together, with manual assignments and ordered priority', async () => {
    const state = start()
    await flushPromises()
    for (const kind of ['total_spend', 'total_recharge', 'account_age_days', 'manual'] as const) {
      state.addRule()
      const index = state.settings.value!.personal_rules.length - 1
      state.changeRuleKind(index, kind)
      const rule = state.settings.value!.personal_rules[index]
      rule.title = ` ${kind} `
      rule.welcome_enabled = false
      rule.welcome_message = ''
      if (kind === 'manual') state.userIDsText.value[rule.id] = '42， 43\n44'
    }
    expect(state.validationError.value).toBe('')
    state.moveRule(3, -1)
    await state.save()
    const payload = mocks.update.mock.calls[0][0]
    expect(payload.personal_rules.map((r: { kind: string }) => r.kind)).toEqual(['total_spend', 'total_recharge', 'manual', 'account_age_days'])
    expect(payload.personal_rules[2]).toMatchObject({ title: 'manual', threshold: 0, user_ids: [42, 43, 44], welcome_enabled: false })
    state.removeRule(2)
    expect(state.settings.value!.personal_rules).toHaveLength(3)
    expect(mocks.update).toHaveBeenCalledTimes(1)
  })

  it('rejects invalid manual IDs without silently dropping them, and clears old conditions on kind change', async () => {
    const state = start()
    await flushPromises()
    state.addRule()
    const rule = state.settings.value!.personal_rules[0]
    rule.title = '伙伴'
    state.changeRuleKind(0, 'manual')
    for (const text of ['', '0', '-2', '42,42', '42,invalid', '9007199254740992', '1.2']) {
      state.userIDsText.value[rule.id] = text
      await state.save()
      expect(state.ruleErrors.value[rule.id]).toBeTruthy()
    }
    expect(mocks.update).not.toHaveBeenCalled()
    state.userIDsText.value[rule.id] = '42'
    state.changeRuleKind(0, 'account_age_days')
    expect(state.userIDsText.value[rule.id]).toBe('')
    for (const threshold of [0, -1, 1.1, 365001, Number.NaN, Number.POSITIVE_INFINITY]) {
      rule.threshold = threshold
      expect(state.ruleErrors.value[rule.id]).toBeTruthy()
    }
    rule.threshold = 1
    expect(state.ruleErrors.value[rule.id]).toBe('')
    state.showRulePreview(0)
    expect(state.preview.message.value).toBe('欢迎回来')
    expect(mocks.update).not.toHaveBeenCalled()
  })

  it('loads old enterprise-only responses without breaking or discarding newly added rules', async () => {
    mocks.get.mockResolvedValueOnce({ enterprise_welcome: { enabled: true, message: '欢迎尊贵的企业用户' } })
    const state = start()
    await flushPromises()
    expect(state.settings.value!.personal_rules).toEqual([])
    expect(state.settings.value!.enterprise_welcome.effect).toBe('capsule')
    state.addRule()
    const rule = state.settings.value!.personal_rules[0]
    expect(rule.welcome_effect).toBe('capsule')
    rule.title = '👑'.repeat(24)
    expect(state.ruleErrors.value[rule.id]).toBe('')
    rule.title += '👑'
    expect(state.ruleErrors.value[rule.id]).toBeTruthy()
    rule.title = 'test'
    rule.welcome_message = 'hello\u2028world'
    expect(state.ruleErrors.value[rule.id]).toBeTruthy()
  })

  it('previews and saves independent enterprise and personal effects without mutating the other choice', async () => {
    vi.useFakeTimers()
    const state = start()
    await flushPromises()
    state.settings.value!.enterprise_welcome.effect = 'ribbon'
    state.addRule()
    const rule = state.settings.value!.personal_rules[0]
    rule.title = '伙伴'
    rule.welcome_effect = 'halo'
    state.showPreview()
    expect(state.preview.effect.value).toBe('ribbon')
    await vi.advanceTimersByTimeAsync(750)
    state.showRulePreview(0)
    expect(state.preview.effect.value).toBe('halo')
    expect(state.preview.phase.value).toBe('waiting')
    expect(mocks.update).not.toHaveBeenCalled()
    expect(state.settings.value!.enterprise_welcome.effect).toBe('ribbon')
    await state.save()
    expect(mocks.update.mock.calls[0][0]).toMatchObject({
      enterprise_welcome: { effect: 'ribbon' }, personal_rules: [{ welcome_effect: 'halo' }]
    })
    state.settings.value!.personal_rules[0].welcome_effect = 'unknown' as never
    await state.save()
    expect(mocks.update).toHaveBeenCalledTimes(1)
    expect(state.ruleErrors.value[rule.id]).toBeTruthy()
    state.settings.value!.enterprise_welcome.effect = 'unknown' as never
    expect(state.enterpriseValidationError.value).toBeTruthy()
  })
})
