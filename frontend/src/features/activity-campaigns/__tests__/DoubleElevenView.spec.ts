import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import DoubleElevenView from '../DoubleElevenView.vue'
import {
  activityCampaignsAPI,
  type ActivityCampaignState,
  waveOneID,
  waveTwoID,
  waveThreeID
} from '@/api/activityCampaigns'
import zh from '@/i18n/locales/zh/activityCampaigns'

vi.mock('@/api/activityCampaigns', () => ({
  activityCampaignsAPI: {
    get: vi.fn(),
    checkin: vi.fn(),
    quiz: vi.fn(),
    claim: vi.fn(),
    treasureVisit: vi.fn(),
    treasureFind: vi.fn()
  },
  waveOneID: 'double11-2026-wave1',
  waveTwoID: 'double11-2026-wave2',
  waveThreeID: 'double11-2026-wave3',
}))

const authUser = ref<{ id: number } | null>({ id: 1 })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ get user() { return authUser.value }, get isAuthenticated() { return !!authUser.value }, isEnterprise: false }) }))
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
  vi.mocked(activityCampaignsAPI.get).mockImplementation(async (id) => {
    if (id === waveOneID) return data
    if (id === waveTwoID) {
      return fixture({
        campaign: {
          id: waveTwoID, revision: 1, enabled: true, title: '双十一月卡拼团',
          subtitle: '多买多送', description: '拼团享好礼',
          starts_at: data.campaign.starts_at, ends_at: data.campaign.ends_at,
          box_campaign_id: 'pool2', questions: []
        },
        server_time: data.server_time, status: data.status,
        purchases: [
          {
            order_id: 1001, card_id: 10, card_code: 'CODE-10', product_name: 'Premium Card',
            price_cny: 58, paid_at: '2026-11-07T12:00:00+08:00', team_code: 'TEAM-100',
            team_members: 2, team_required: 3, team_qualified: false,
            purchase_task: 'purchase_1001', team_task: 'team_1001', eligible: true
          }
        ],
        rewards: [
          { task: 'purchase_1001', earned_at: '2026-11-07T12:01:00+08:00', delivered_at: null }
        ]
      })
    }
    if (id === waveThreeID) {
      return fixture({
        campaign: {
          id: waveThreeID, revision: 1, enabled: true, title: '双十一寻鲸记',
          subtitle: '全站寻宝', description: '捕获小巨鲸',
          starts_at: data.campaign.starts_at, ends_at: data.campaign.ends_at,
          box_campaign_id: 'pool3', questions: []
        },
        server_time: data.server_time, status: data.status,
        treasures: [
          { id: 'dashboard', name: 'Dashboard Whale', clue: 'Look closely at dashboard', revealed: true, found: false, line: '' },
          { id: 'keys', name: 'API Keys Whale', clue: 'Look closely at keys', revealed: true, found: true, line: 'Hello keys!' },
          { id: 'usage', name: 'Usage Whale', clue: 'Look closely at usage', revealed: false, found: false, line: '' },
          { id: 'group-buy', name: 'Group Buy Whale', clue: 'Look closely at group-buy', revealed: false, found: false, line: '' },
          { id: 'profile', name: 'Profile Whale', clue: 'Look closely at profile', revealed: false, found: false, line: '' }
        ],
        rewards: [
          { task: 'treasure_three', earned_at: '2026-11-13T12:00:00+08:00', delivered_at: null }
        ]
      })
    }
    return data
  })
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
  localStorage.clear()
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
    expect(activityCampaignsAPI.claim).toHaveBeenNthCalledWith(1, 'first_checkin', waveOneID)
    expect(activityCampaignsAPI.claim).toHaveBeenNthCalledWith(2, 'first_checkin', waveOneID)
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
    expect(activityCampaignsAPI.quiz).toHaveBeenCalledWith(2, { q1: 1, q2: 1, q3: 1 }, waveOneID)
    expect(wrapper.get('[data-testid="d11-quiz-error"]').text()).toContain('未全部答对')
    vi.mocked(activityCampaignsAPI.quiz).mockRejectedValueOnce({ status: 409 })
    await wrapper.get('[data-testid="d11-quiz-submit"]').trigger('click')
    await flushPromises()
    expect(activityCampaignsAPI.get).toHaveBeenCalledTimes(4) // 3 during initial settle + 1 on 409 reload
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

  // Wave 2 & Wave 3 interaction tests
  describe('Wave 2 and Wave 3 Navigation and Actions', () => {
    it('supports switching waves and executing specific actions', async () => {
      const wrapper = await view()

      // Select Wave 2
      await wrapper.get(`[data-testid="wave-tab-${waveTwoID}"]`).trigger('click')
      await flushPromises()

      // Verify Wave 2 products block is shown
      expect(wrapper.find('[data-testid="d11-wave2-products"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="d11-wave2-progress"]').exists()).toBe(true)

      // Verify purchases lists
      expect(wrapper.find('[data-testid="d11-purchase-10"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="d11-claim-purchase-1001"]').exists()).toBe(true)

      // Trigger Wave 2 Purchase Claim
      const second = await activityCampaignsAPI.get(waveTwoID)
      vi.mocked(activityCampaignsAPI.claim).mockResolvedValueOnce({ ...second,
        rewards: [
          { task: 'purchase_1001', earned_at: '2026-11-07T12:01:00+08:00', delivered_at: '2026-11-07T12:02:00+08:00' }
        ]
      })
      await wrapper.get('[data-testid="d11-claim-purchase-1001"]').trigger('click')
      await flushPromises()
      expect(activityCampaignsAPI.claim).toHaveBeenCalledWith('purchase_1001', waveTwoID)

      // Select Wave 3
      await wrapper.get(`[data-testid="wave-tab-${waveThreeID}"]`).trigger('click')
      await flushPromises()

      // Verify Wave 3 clues list and progress block are shown
      expect(wrapper.find('[data-testid="d11-wave3-progress"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="d11-wave3-clues"]').exists()).toBe(true)

      // Clue cards assertions
      const clues = wrapper.findAll('[data-testid^="d11-clue-"]')
      expect(clues).toHaveLength(5)

      // First clue is revealed
      expect(clues[0].text()).toContain('Look closely at dashboard')
      // Third clue is not revealed yet
      expect(clues[2].text()).toContain('???')

      // Verify Wave 3 milestone claim
      expect(wrapper.find('[data-testid="d11-claim-treasure-three"]').exists()).toBe(true)
      const third = await activityCampaignsAPI.get(waveThreeID)
      vi.mocked(activityCampaignsAPI.claim).mockResolvedValueOnce({ ...third,
        rewards: [
          { task: 'treasure_three', earned_at: '2026-11-13T12:00:00+08:00', delivered_at: '2026-11-13T12:05:00+08:00' }
        ]
      })
      await wrapper.get('[data-testid="d11-claim-treasure-three"]').trigger('click')
      await flushPromises()
      expect(activityCampaignsAPI.claim).toHaveBeenCalledWith('treasure_three', waveThreeID)
    })
  })
})
