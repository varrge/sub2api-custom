import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import Center from '../DoubleElevenView.vue'
import Hidden from '../HiddenWhaleChallenge.vue'
import Admin from '@/views/admin/settings/ActivityCampaignSettings.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { activityCampaignsAPI as api, type ActivityCampaignState, waveOneID, waveTwoID, waveThreeID } from '@/api/activityCampaigns'

const shared = vi.hoisted(() => ({
  auth: { user: { id: 1 }, isAuthenticated: true, isEnterprise: false },
  route: { path: '/dashboard' },
  flag: { enabled: true }
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => shared.auth }))
vi.mock('vue-router', () => ({ useRoute: () => shared.route }))
vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: { activityCampaigns: {} }, isFeatureFlagEnabled: () => shared.flag.enabled
}))
vi.mock('@/api/activityCampaigns', () => ({
  waveOneID: 'double11-2026-wave1', waveTwoID: 'double11-2026-wave2', waveThreeID: 'double11-2026-wave3',
  activityCampaignsAPI: {
    get: vi.fn(), checkin: vi.fn(), quiz: vi.fn(), claim: vi.fn(),
    treasureVisit: vi.fn(), treasureFind: vi.fn(), adminGet: vi.fn(), adminSave: vi.fn()
  }
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ locale: { value: 'zh' }, t: (key: string) => key })
}))
const global = { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true, RouterLink: { template: '<a><slot /></a>' } } }
function state(id = waveOneID, title = id): ActivityCampaignState {
  return {
    campaign: {
      id, revision: 1, enabled: true, title, subtitle: '', description: '',
      starts_at: '2026-11-01T00:00:00+08:00', ends_at: '2026-11-16T00:00:00+08:00', box_campaign_id: 'pool',
      questions: id === waveOneID ? [1, 2, 3].map(n => ({ id: `q${n}`, prompt: `Q${n}`, options: ['A', 'B'], answer: 0 })) : []
    },
    status: 'active', server_time: '2026-11-13T12:00:00+08:00', today: '2026-11-13',
    checkin_dates: [], quiz_completed: false, rewards: [], purchases: [], treasures: []
  }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(r => { resolve = r })
  return { promise, resolve }
}
const challenge = { treasure: { id: 'welcome', name: 'Whale', token: 'a'.repeat(64) } }
function foundState() {
  return {
    ...state(waveThreeID),
    treasures: [{ id: 'welcome', name: 'Whale', clue: 'Hint', revealed: true, found: true, line: 'Hello!' }]
  }
}
enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-11-13T04:00:00Z'))
  localStorage.clear()
  shared.auth = reactive({ user: { id: 1 }, isAuthenticated: true, isEnterprise: false })
  shared.route = reactive({ path: '/dashboard' })
  shared.flag = reactive({ enabled: true })
  vi.mocked(api.get).mockImplementation(async id => state(id))
  vi.mocked(api.adminGet).mockImplementation(async id => state(id).campaign)
  vi.mocked(api.treasureVisit).mockResolvedValue(challenge)
})
afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('activity state isolation', () => {
  it('ignores initial GET responses after the signed-in user changes', async () => {
    const old = deferred<ActivityCampaignState>()
    vi.mocked(api.get).mockImplementation(id =>
      shared.auth.user.id === 1 && id === waveOneID ? old.promise : Promise.resolve(state(id, 'Current account')))
    const w = mount(Center, { global })
    await flushPromises()
    shared.auth.user = { id: 2 }
    await flushPromises()
    old.resolve(state(waveOneID, 'Previous account private progress'))
    await flushPromises()
    expect(w.text()).toContain('Current account')
    expect(w.text()).not.toContain('Previous account')
  })

  it('keeps a pending claim bound to its wave and prevents switching during submission', async () => {
    const one = state()
    one.rewards = [{ task: 'first_checkin', earned_at: one.server_time, delivered_at: null }]
    vi.mocked(api.get).mockImplementation(async id => id === waveOneID ? one : state(id))
    const w = mount(Center, { global })
    await flushPromises()
    const pending = deferred<ActivityCampaignState>()
    vi.mocked(api.claim).mockReturnValue(pending.promise)
    await w.get('[data-testid="d11-claim-first_checkin"]').trigger('click')
    expect(w.get(`[data-testid="wave-tab-${waveTwoID}"]`).attributes('disabled')).toBeDefined()
    await w.get(`[data-testid="wave-tab-${waveTwoID}"]`).trigger('click')
    pending.resolve({ ...one, rewards: [{ ...one.rewards[0], delivered_at: one.server_time }] })
    await flushPromises()
    expect(w.find('[data-testid="d11-wave2-products"]').exists()).toBe(false)
    await w.get(`[data-testid="wave-tab-${waveTwoID}"]`).trigger('click')
    await flushPromises()
    expect(w.get('[data-testid="d11-hero"]').text()).toContain(waveTwoID)
  })

  it('closes wave-three claims at its deadline and uses its own ended notice', async () => {
    const ended = { ...state(waveThreeID), status: 'ended' as const, server_time: '2026-11-16T00:00:00+08:00',
      rewards: [{ task: 'treasure_three', earned_at: '2026-11-13T12:00:00+08:00', delivered_at: null }] }
    vi.mocked(api.get).mockImplementation(async id => id === waveThreeID ? ended : state(id))
    const w = mount(Center, { global })
    await flushPromises()
    await w.get(`[data-testid="wave-tab-${waveThreeID}"]`).trigger('click')
    await flushPromises()
    expect(w.get('[data-testid="d11-ended-notice"]').text()).toBe('activityCampaigns.wave3.endedNotice')
    expect(w.get('[data-testid="d11-claim-treasure-three"]').attributes('disabled')).toBeDefined()
    await w.get('[data-testid="d11-claim-treasure-three"]').trigger('click')
    expect(api.claim).not.toHaveBeenCalled()
  })
})

describe('hidden whale lifecycle', () => {
  it('waits for the opt-in flag and immediately hides on disable or enterprise conversion', async () => {
    shared.flag.enabled = false
    const w = mount(Hidden, { global })
    await flushPromises()
    expect(api.treasureVisit).not.toHaveBeenCalled()
    shared.flag.enabled = true
    await flushPromises()
    expect(w.find('[data-testid="hidden-whale-button"]').exists()).toBe(true)
    shared.auth.isEnterprise = true
    await flushPromises()
    expect(w.find('[data-testid="hidden-whale-widget"]').exists()).toBe(false)
    shared.auth.isEnterprise = false
    await flushPromises()
    shared.flag.enabled = false
    await flushPromises()
    expect(w.find('[data-testid="hidden-whale-widget"]').exists()).toBe(false)
  })

  it('never visits unsupported routes and discards an older route challenge', async () => {
    const old = deferred<typeof challenge>()
    vi.mocked(api.treasureVisit).mockReturnValue(old.promise)
    const w = mount(Hidden, { global })
    shared.route.path = '/admin/settings'
    old.resolve(challenge)
    await flushPromises()
    expect(api.treasureVisit).toHaveBeenCalledTimes(1)
    expect(w.find('[data-testid="hidden-whale-widget"]').exists()).toBe(false)
  })

  it('discards a find result after account or route changes', async () => {
    const w = mount(Hidden, { global })
    await flushPromises()
    const pending = deferred<ActivityCampaignState>()
    vi.mocked(api.treasureFind).mockReturnValue(pending.promise)
    await w.get('[data-testid="hidden-whale-button"]').trigger('click')
    shared.auth.user = { id: 2 }
    await flushPromises()
    pending.resolve(foundState())
    await flushPromises()
    expect(document.querySelector('[data-testid="whale-success-modal"]')).toBeNull()
    expect(w.find('[data-testid="hidden-whale-button"]').exists()).toBe(true)
  })

  it('recovers a committed find after a lost response without inventing progress', async () => {
    vi.mocked(api.treasureFind).mockRejectedValue(new Error('Lost reply'))
    vi.mocked(api.get).mockResolvedValue(foundState())
    const w = mount(Hidden, { attachTo: document.body, global })
    await flushPromises()
    await w.get('[data-testid="hidden-whale-button"]').trigger('click')
    await flushPromises()
    expect(document.querySelector('[data-testid="whale-success-modal"]')).not.toBeNull()
    expect(w.find('[data-testid="hidden-whale-error"]').exists()).toBe(false)
    expect(api.treasureVisit).toHaveBeenCalledTimes(1)
  })

  it('moves focus into a named dialog and supports Escape', async () => {
    vi.mocked(api.treasureFind).mockResolvedValue(foundState())
    const w = mount(Hidden, { attachTo: document.body, global })
    await flushPromises()
    const button = w.get('[data-testid="hidden-whale-button"]')
    ;(button.element as HTMLElement).focus()
    await button.trigger('click')
    await flushPromises()
    const dialog = document.querySelector('[role="dialog"]')!
    expect(dialog.getAttribute('aria-labelledby')).toBeTruthy()
    expect(dialog.contains(document.activeElement)).toBe(true)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(w.getComponent(BaseDialog).props('show')).toBe(false)
  })
})

describe('admin wave switching', () => {
  it('requires saving or refreshing a draft before changing waves', async () => {
    const w = mount(Admin, { global })
    await flushPromises()
    await w.get('[data-testid="campaign-title"]').setValue('Unsaved change')
    expect(w.get('[data-testid="admin-wave-selector"]').attributes('disabled')).toBeDefined()
    await w.get('[data-testid="campaign-refresh"]').trigger('click')
    await flushPromises()
    expect(w.get('[data-testid="admin-wave-selector"]').attributes('disabled')).toBeUndefined()
  })

  it('does not expose the old wave save action while a new wave loads', async () => {
    const w = mount(Admin, { global })
    await flushPromises()
    const pending = deferred<ActivityCampaignState['campaign']>()
    vi.mocked(api.adminGet).mockReturnValue(pending.promise)
    await w.get('[data-testid="admin-wave-selector"]').setValue(waveTwoID)
    expect(w.find('[data-testid="campaign-save"]').exists()).toBe(false)
    expect(api.adminSave).not.toHaveBeenCalled()
    pending.resolve(state(waveTwoID).campaign)
    await flushPromises()
    expect(w.get('[data-testid="campaign-title"]').element).toHaveProperty('value', waveTwoID)
  })
})
