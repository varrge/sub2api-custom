import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ActivityLeaderboardSettings from '../ActivityLeaderboardSettings.vue'
import { activityLeaderboardConfigVersion } from '@/utils/activityLeaderboardEvents'

const mocks = vi.hoisted(() => ({
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    locale: { value: 'zh' },
    t: (key: string) => key,
  }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    settings: {
      getActivityLeaderboardSettings: (...args: unknown[]) => mocks.getConfig(...args),
      updateActivityLeaderboardSettings: (...args: unknown[]) => mocks.updateConfig(...args),
    },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showSuccess: mocks.showSuccess,
    showError: mocks.showError,
  }),
}))

let wrapper: VueWrapper | undefined

function configFixture(overrides: Record<string, unknown> = {}) {
  return {
    enabled: true,
    title: '中秋国庆双节消费榜',
    subtitle: '月满算力，双节开工！',
    reward_description: '前三名获得活动奖励，具体奖励另行公布。',
    starts_at: '2026-09-25T00:00:00+08:00',
    ends_at: '2026-10-08T00:00:00+08:00',
    demo_expires_at: null,
    ...overrides,
  }
}

async function mountSettings() {
  wrapper = mount(ActivityLeaderboardSettings, {
    global: { stubs: { Icon: { template: '<i />' } } },
  })
  await flushPromises()
  return wrapper
}

function inputValue(testid: string): string {
  return (wrapper!.get(`[data-testid="${testid}"]`).element as HTMLInputElement).value
}

async function setInput(testid: string, value: string) {
  await wrapper!.get(`[data-testid="${testid}"]`).setValue(value)
}

async function clickSave() {
  await wrapper!.get('[data-testid="activity-leaderboard-save"]').trigger('click')
  await flushPromises()
}

describe('ActivityLeaderboardSettings', () => {
  beforeEach(() => {
    mocks.getConfig.mockReset()
    mocks.updateConfig.mockReset()
    mocks.showSuccess.mockReset()
    mocks.showError.mockReset()
    mocks.getConfig.mockResolvedValue(configFixture())
    mocks.updateConfig.mockImplementation((payload: unknown) => Promise.resolve(payload))
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })

  it('loads the config and renders Beijing-time datetime-local values', async () => {
    await mountSettings()
    expect(mocks.getConfig).toHaveBeenCalledTimes(1)
    expect(inputValue('activity-leaderboard-title')).toBe('中秋国庆双节消费榜')
    expect(inputValue('activity-leaderboard-subtitle')).toBe('月满算力，双节开工！')
    expect(inputValue('activity-leaderboard-reward')).toBe('前三名获得活动奖励，具体奖励另行公布。')
    expect(inputValue('activity-leaderboard-starts-at')).toBe('2026-09-25T00:00')
    expect(inputValue('activity-leaderboard-ends-at')).toBe('2026-10-08T00:00')
    expect(inputValue('activity-leaderboard-demo-expires-at')).toBe('')
    expect(wrapper!.text()).toContain('admin.settings.activityLeaderboard.billingNote')
    expect(wrapper!.text()).toContain('admin.settings.activityLeaderboard.rulesNote')
  })

  it('shows a retry state on load failure instead of saveable defaults', async () => {
    mocks.getConfig.mockRejectedValueOnce(new Error('offline'))
    await mountSettings()
    expect(wrapper!.get('[data-testid="activity-leaderboard-load-error"]').text()).toContain(
      'admin.settings.activityLeaderboard.loadFailed',
    )
    expect(wrapper!.find('[data-testid="activity-leaderboard-save"]').exists()).toBe(false)

    mocks.getConfig.mockResolvedValueOnce(configFixture())
    await wrapper!.get('[data-testid="activity-leaderboard-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper!.find('[data-testid="activity-leaderboard-load-error"]').exists()).toBe(false)
    expect(inputValue('activity-leaderboard-title')).toBe('中秋国庆双节消费榜')
  })

  it('persists daily reward opt-in without changing cumulative campaign dates', async () => {
    mocks.getConfig.mockResolvedValue(configFixture({ period: 'daily', auto_rewards: true }))
    await mountSettings()
    const period = wrapper!.get('[data-testid="activity-leaderboard-settlement-period"]')
    const rewards = wrapper!.get('[data-testid="activity-leaderboard-auto-rewards"]')
    expect((period.element as HTMLSelectElement).value).toBe('daily')
    expect((rewards.element as HTMLInputElement).checked).toBe(true)
    await clickSave()
    expect(mocks.updateConfig).toHaveBeenLastCalledWith(expect.objectContaining({
      period: 'daily', auto_rewards: true,
      starts_at: '2026-09-25T00:00:00+08:00', ends_at: '2026-10-08T00:00:00+08:00',
    }))
    expect(wrapper!.text()).toContain('admin.settings.activityLeaderboard.dailyHint')
    await period.setValue('event')
    await clickSave()
    expect(mocks.updateConfig).toHaveBeenLastCalledWith(expect.objectContaining({ period: 'event', auto_rewards: true, display_mode: 'cumulative' }))
    expect((rewards.element as HTMLInputElement).disabled).toBe(false)
  })

  it('blocks saving and shows client-side validation errors', async () => {
    await mountSettings()
    await setInput('activity-leaderboard-title', '')
    await setInput('activity-leaderboard-starts-at', '2026-10-08T00:00')
    await setInput('activity-leaderboard-ends-at', '2026-09-25T00:00')
    await clickSave()

    expect(mocks.updateConfig).not.toHaveBeenCalled()
    const errors = wrapper!.get('[data-testid="activity-leaderboard-validation"]').text()
    expect(errors).toContain('admin.settings.activityLeaderboard.validation.title')
    expect(errors).toContain('admin.settings.activityLeaderboard.validation.periodOrder')
  })

  it('saves independent display and event settings with custom prizes', async () => {
    await mountSettings()
    await wrapper!.get('[data-testid="activity-leaderboard-display-mode"]').setValue('both')
    await wrapper!.get('[data-testid="activity-leaderboard-settlement-period"]').setValue('event')
    await wrapper!.get('[data-testid="activity-leaderboard-auto-rewards"]').setValue(true)
    await wrapper!.get('[data-testid="activity-leaderboard-delay"]').setValue('45')
    const first = wrapper!.findAll('[data-testid="activity-rank-reward"]')[0]
    await first.get('select').setValue('medium')
    await first.findAll('input')[1].setValue('5')
    await wrapper!.get('[data-testid="activity-participation-enabled"]').setValue(false)
    await setInput('activity-leaderboard-starts-at', '2026-09-25T10:30')
    await clickSave()
    expect(mocks.updateConfig).toHaveBeenLastCalledWith(expect.objectContaining({
      display_mode: 'both', period: 'event', auto_rewards: true, settlement_delay_minutes: 45,
      starts_at: '2026-09-25T10:30:00+08:00',
      reward_rules: expect.objectContaining({ ranked: expect.arrayContaining([{ rank: 1, card_tier: 'medium', quantity: 5 }]), participation: expect.objectContaining({ enabled: false }) }),
    }))
  })

  it('rejects duplicate ranks and waits shorter than the safety minimum', async () => {
    await mountSettings()
    await wrapper!.get('[data-testid="activity-leaderboard-delay"]').setValue('1')
    await wrapper!.findAll('[data-testid="activity-rank-reward"]')[1].findAll('input')[0].setValue('1')
    await clickSave()
    expect(mocks.updateConfig).not.toHaveBeenCalled()
    expect(wrapper!.text()).toContain('admin.settings.activityLeaderboard.validation.delay')
    expect(wrapper!.text()).toContain('admin.settings.activityLeaderboard.validation.rewardRules')
  })

  it('preserves legacy display-only events with non-midnight dates', async () => {
    mocks.getConfig.mockResolvedValue(configFixture({ starts_at: '2026-09-25T10:30:00+08:00', period: '', auto_rewards: false }))
    await mountSettings()
    await setInput('activity-leaderboard-title', 'Renamed event')
    await clickSave()
    expect(mocks.updateConfig).toHaveBeenLastCalledWith(expect.objectContaining({
      starts_at: '2026-09-25T10:30:00+08:00', title: 'Renamed event', period: 'event', auto_rewards: false,
    }))
  })

  it('rejects a period longer than 366 days', async () => {
    await mountSettings()
    await setInput('activity-leaderboard-ends-at', '2027-10-08T00:00')
    await clickSave()

    expect(mocks.updateConfig).not.toHaveBeenCalled()
    expect(wrapper!.get('[data-testid="activity-leaderboard-validation"]').text()).toContain(
      'admin.settings.activityLeaderboard.validation.periodLength',
    )
  })

  it('rejects a demo expiry later than the start time', async () => {
    await mountSettings()
    await setInput('activity-leaderboard-demo-expires-at', '2026-09-25T00:01')
    await clickSave()

    expect(mocks.updateConfig).not.toHaveBeenCalled()
    expect(wrapper!.get('[data-testid="activity-leaderboard-validation"]').text()).toContain(
      'admin.settings.activityLeaderboard.validation.demoAfterStart',
    )
  })

  it('saves the full config as Beijing-time RFC3339 and notifies the header entry', async () => {
    const versionBefore = activityLeaderboardConfigVersion.value
    await mountSettings()
    await setInput('activity-leaderboard-title', ' 双旦消费榜 ')
    await setInput('activity-leaderboard-demo-expires-at', '2026-09-20T12:00')
    await clickSave()

    expect(mocks.updateConfig).toHaveBeenCalledTimes(1)
    expect(mocks.updateConfig).toHaveBeenCalledWith({
      period: 'event',
      auto_rewards: false,
      display_mode: 'cumulative',
      settlement_delay_minutes: 10,
      reward_rules: {
        ranked: [{ rank: 1, card_tier: 'high', quantity: 1 }, { rank: 2, card_tier: 'medium', quantity: 2 }, { rank: 3, card_tier: 'medium', quantity: 1 }],
        participation: { enabled: true, card_tier: 'low', quantity: 1, include_zero_cost: true },
      },
      enabled: true,
      title: '双旦消费榜',
      subtitle: '月满算力，双节开工！',
      reward_description: '前三名获得活动奖励，具体奖励另行公布。',
      starts_at: '2026-09-25T00:00:00+08:00',
      ends_at: '2026-10-08T00:00:00+08:00',
      demo_expires_at: '2026-09-20T12:00:00+08:00',
    })
    expect(mocks.showSuccess).toHaveBeenCalledTimes(1)
    expect(mocks.showError).not.toHaveBeenCalled()
    expect(activityLeaderboardConfigVersion.value).toBe(versionBefore + 1)
  })

  it('sends null demo expiry after clearing and reports save failures', async () => {
    await mountSettings()
    await setInput('activity-leaderboard-demo-expires-at', '2026-09-20T12:00')
    await wrapper!.get('[data-testid="activity-leaderboard-demo-clear"]').trigger('click')
    expect(inputValue('activity-leaderboard-demo-expires-at')).toBe('')

    mocks.updateConfig.mockRejectedValueOnce(new Error('boom'))
    await clickSave()

    expect(mocks.updateConfig).toHaveBeenCalledWith(
      expect.objectContaining({ demo_expires_at: null }),
    )
    expect(mocks.showSuccess).not.toHaveBeenCalled()
    expect(mocks.showError).toHaveBeenCalledTimes(1)
  })
})
