import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import DoubleElevenView from '../DoubleElevenView.vue'
import { activityCampaignsAPI, type ActivityCampaignState } from '@/api/activityCampaigns'
import zh from '@/i18n/locales/zh/activityCampaigns'

vi.mock('@/api/activityCampaigns', () => ({
  activityCampaignsAPI: { get: vi.fn(), checkin: vi.fn(), quiz: vi.fn(), claim: vi.fn() },
}))
const authUser = ref<{ id: number } | null>({ id: 1 })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ get user() { return authUser.value } }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({
    locale: { value: 'zh' },
    t: (key: string, params: Record<string, unknown> = {}) => {
      const value = key.split('.').reduce<unknown>((obj, part) => (obj as Record<string, unknown>)?.[part], zh)
      return String(value ?? key).replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? ''))
    },
  }),
}))
enableAutoUnmount(afterEach)
function fixture(overrides: Partial<ActivityCampaignState> = {}): ActivityCampaignState {
  return {
    campaign: {
      id: 'double11-2026-wave1', revision: 2, enabled: true, title: '双十一开门礼',
      subtitle: '签到答题领好运', description: '每人最多三张普通卡',
      starts_at: '2026-11-01T00:00:00+08:00', ends_at: '2026-11-06T00:00:00+08:00',
      box_campaign_id: 'pool',
      questions: [1, 2, 3].map(n => ({ id: `q${n}`, prompt: `趣味题${n}`, options: ['甲', '乙', '丙'] })),
    },
    status: 'active', server_time: '2026-11-02T12:00:00+08:00', today: '2026-11-02',
    checkin_dates: [], quiz_completed: false, rewards: [], ...overrides,
  }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(r => { resolve = r })
  return { promise, resolve }
}
async function view(data = fixture()) {
  vi.mocked(activityCampaignsAPI.get).mockResolvedValue(data)
  const wrapper = mount(DoubleElevenView, {
    global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true, RouterLink: { template: '<a><slot /></a>' } } },
  })
  await flushPromises()
  return wrapper
}
beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-11-02T04:00:00Z'))
  authUser.value = { id: 1 }
})
afterEach(() => { vi.useRealTimers() })

describe('DoubleElevenView', () => {
  it('uses confirmed check-in state and prevents repeated submission', async () => {
    const wrapper = await view()
    const d = deferred<ActivityCampaignState>()
    vi.mocked(activityCampaignsAPI.checkin).mockReturnValue(d.promise)
    await wrapper.get('[data-testid="d11-checkin-button"]').trigger('click')
    await wrapper.get('[data-testid="d11-checkin-button"]').trigger('click')
    expect(activityCampaignsAPI.checkin).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).not.toContain('今日已签到')
    d.resolve(fixture({ checkin_dates: ['2026-11-02'], rewards: [{ task: 'first_checkin', earned_at: '2026-11-02T12:00:00+08:00', delivered_at: null }] }))
    await flushPromises()
    expect(wrapper.text()).toContain('今日已签到')
    expect(wrapper.get('[data-testid="d11-checkin-button"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="d11-claim-first_checkin"]').exists()).toBe(true)
  })
  it('preserves earned reward on failure and retries the same task without optimistic delivery', async () => {
    const reward = { task: 'first_checkin' as const, earned_at: '2026-11-02T12:00:00+08:00', delivered_at: null }
    const wrapper = await view(fixture({ rewards: [reward] }))
    vi.mocked(activityCampaignsAPI.claim).mockRejectedValueOnce({ message: '暂未确认' })
    await wrapper.get('[data-testid="d11-claim-first_checkin"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="d11-claim-first_checkin"]').exists()).toBe(true)
    vi.mocked(activityCampaignsAPI.claim).mockResolvedValue(fixture({ rewards: [{ ...reward, delivered_at: reward.earned_at }] }))
    await wrapper.get('[data-testid="d11-claim-first_checkin"]').trigger('click')
    await flushPromises()
    expect(activityCampaignsAPI.claim).toHaveBeenNthCalledWith(1, 'first_checkin')
    expect(activityCampaignsAPI.claim).toHaveBeenNthCalledWith(2, 'first_checkin')
    expect(wrapper.find('[data-testid="d11-claim-first_checkin"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="d11-reward-first_checkin"]').text()).toContain('已领取')
  })
  it('submits only selected answers and revision, allows retry and reloads on stale configuration', async () => {
    const wrapper = await view()
    await wrapper.get('[data-testid="d11-quiz-submit"]').trigger('click')
    expect(activityCampaignsAPI.quiz).not.toHaveBeenCalled()
    for (let n = 0; n < 3; n++) await wrapper.get(`[data-testid="d11-quiz-option-${n}-1"]`).trigger('click')
    vi.mocked(activityCampaignsAPI.quiz).mockRejectedValueOnce({ status: 422 })
    await wrapper.get('[data-testid="d11-quiz-submit"]').trigger('click')
    await flushPromises()
    expect(activityCampaignsAPI.quiz).toHaveBeenCalledWith(2, { q1: 1, q2: 1, q3: 1 })
    expect(wrapper.get('[data-testid="d11-quiz-error"]').text()).toContain('未全部答对')
    vi.mocked(activityCampaignsAPI.quiz).mockRejectedValueOnce({ status: 409 })
    await wrapper.get('[data-testid="d11-quiz-submit"]').trigger('click')
    await flushPromises()
    expect(activityCampaignsAPI.get).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="d11-quiz-option-0-1"]').attributes('aria-pressed')).toBe('false')
  })
  it.each(['disabled', 'upcoming', 'ended'] as const)('blocks participation when %s', async status => {
    const data = fixture({ status })
    if (status === 'upcoming') data.server_time = '2026-10-31T12:00:00+08:00'
    if (status === 'ended') data.server_time = '2026-11-06T00:00:00+08:00'
    const wrapper = await view(data)
    const button = wrapper.find('[data-testid="d11-checkin-button"]')
    expect(!button.exists() || button.attributes('disabled') !== undefined).toBe(true)
    expect(activityCampaignsAPI.checkin).not.toHaveBeenCalled()
  })
  it('discards responses from the previous signed-in user', async () => {
    const wrapper = await view()
    const d = deferred<ActivityCampaignState>()
    vi.mocked(activityCampaignsAPI.checkin).mockReturnValue(d.promise)
    await wrapper.get('[data-testid="d11-checkin-button"]').trigger('click')
    authUser.value = { id: 2 }
    await flushPromises()
    d.resolve(fixture({ rewards: [{ task: 'first_checkin', earned_at: '2026-11-02T12:00:00+08:00', delivered_at: null }] }))
    await flushPromises()
    expect(wrapper.find('[data-testid="d11-claim-first_checkin"]').exists()).toBe(false)
  })
  it('opens at the server-anchored start time and resets check-in at Beijing midnight', async () => {
    const upcoming = fixture({ status: 'upcoming', server_time: '2026-10-31T23:59:59+08:00' })
    const wrapper = await view(upcoming)
    expect(wrapper.get('[data-testid="d11-checkin-button"]').attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.get('[data-testid="d11-checkin-button"]').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
    const midnight = await view(fixture({ server_time: '2026-11-02T23:59:59+08:00', checkin_dates: ['2026-11-02'] }))
    expect(midnight.get('[data-testid="d11-checkin-button"]').attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(2000)
    expect(midnight.get('[data-testid="d11-checkin-button"]').attributes('disabled')).toBeUndefined()
  })
})
