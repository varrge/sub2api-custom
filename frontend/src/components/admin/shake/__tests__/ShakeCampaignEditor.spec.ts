import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ShakeCampaignEditor from '../ShakeCampaignEditor.vue'
import type { ShakeConfig } from '@/api/admin/shake'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => `${key}${params.tier ? `:${params.tier}` : ''}` })
}))

const mountEditor = (initial?: ShakeConfig) =>
  mount(ShakeCampaignEditor, { props: { initial }, global: { stubs: { Icon: true } } })

describe('ShakeCampaignEditor', () => {
  it('emits a normalized config in Beijing time without stray rule fields', async () => {
    const wrapper = mountEditor()
    await wrapper.get('[data-testid="shake-name"]').setValue('双十一摇摇乐')
    await wrapper.get('[data-testid="shake-start"]').setValue('2030-11-01T00:00')
    await wrapper.get('[data-testid="shake-end"]').setValue('2030-11-12T00:00')
    await wrapper.get('[data-testid="shake-rule-0"] input[type="number"]').setValue('10')
    await wrapper.get('form').trigger('submit')
    const [config] = wrapper.emitted('save')![0] as [ShakeConfig]
    expect(config.name).toBe('双十一摇摇乐')
    expect(Date.parse(config.starts_at)).toBe(Date.parse('2030-11-01T00:00:00+08:00'))
    expect(Date.parse(config.ends_at)).toBe(Date.parse('2030-11-12T00:00:00+08:00'))
    expect(config.rules[0]).toEqual({ source: 'order', order_type: 'subscription', mode: 'per_order', group_id: 10, quantity: 1, card_tier: 'medium' })
    expect(config.rules[1]).toEqual({ source: 'usage', billing: 'balance', mode: 'threshold', threshold: 10, quantity: 1, card_tier: 'low' })
    expect(config.prizes.map((p) => p.sort_order)).toEqual([0, 1, 2])
    expect(config.prizes.filter((p) => p.type !== 'balance').every((p) => p.amount === null)).toBe(true)
  })

  it('blocks saving when a card tier cannot win anything', async () => {
    const wrapper = mountEditor()
    for (const row of wrapper.findAll('[data-testid^="shake-prize-"]')) {
      // Third weight column is the high-tier weight.
      await row.findAll('td')[6].get('input').setValue('0')
    }
    expect(wrapper.text()).toContain('shakeAdmin.tierEmpty:luckyBox.tierHigh')
    expect(wrapper.get('[data-testid="shake-save"]').attributes('disabled')).toBeDefined()
  })

  it('locks the start time of an enabled campaign', () => {
    const wrapper = mount(ShakeCampaignEditor, {
      props: { startLocked: true, initial: { name: 'x', starts_at: '2030-11-01T00:00:00+08:00', ends_at: '2030-11-12T00:00:00+08:00', rules: [], prizes: [] } },
      global: { stubs: { Icon: true } }
    })
    expect(wrapper.get('[data-testid="shake-start"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="shake-end"]').attributes('disabled')).toBeUndefined()
  })
  it('freezes earning scopes and modes but leaves rewards editable after activation', () => {
    const wrapper = mount(ShakeCampaignEditor, { props: { startLocked: true }, global: { stubs: { Icon: true } } })
    expect(wrapper.get('[data-testid="shake-add-rule"]').attributes('disabled')).toBeDefined()
    const rule = wrapper.get('[data-testid="shake-rule-0"]')
    for (const label of ['ruleSource', 'orderType', 'mode', 'groupId', 'remove']) {
      expect(rule.get(`[aria-label="shakeAdmin.${label}"]`).attributes('disabled')).toBeDefined()
    }
    for (const label of ['quantity', 'cardTier']) {
      expect(rule.get(`[aria-label="shakeAdmin.${label}"]`).attributes('disabled')).toBeUndefined()
    }
  })

  it('labels month-card thresholds in CNY', async () => {
    const wrapper = mountEditor()
    const rule = wrapper.get('[data-testid="shake-rule-0"]')
    await rule.get('[aria-label="shakeAdmin.orderType"]').setValue('month_card')
    await rule.get('[aria-label="shakeAdmin.mode"]').setValue('threshold')
    expect(rule.find('[aria-label="shakeAdmin.thresholdCny"]').exists()).toBe(true)
  })

})
