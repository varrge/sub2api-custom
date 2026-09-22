import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ActivityLeaderboard from '../ActivityLeaderboard.vue'
import { getActivityLeaderboard, getActivityLeaderboardConfig, type ActivityLeaderboard as Board, type ActivityLeaderboardPublicConfig } from '@/api/activityLeaderboard'
import { notifyActivityLeaderboardConfigSaved } from '@/utils/activityLeaderboardEvents'

vi.mock('@/api/activityLeaderboard', () => ({ getActivityLeaderboard: vi.fn(), getActivityLeaderboardConfig: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'zh' }, t: (key: string, params: Record<string, unknown> = {}) => key + JSON.stringify(params) }) }))
const boardAPI = vi.mocked(getActivityLeaderboard)
const configAPI = vi.mocked(getActivityLeaderboardConfig)
const config: ActivityLeaderboardPublicConfig = { enabled: true, title: 'First campaign', subtitle: '', reward_description: '', starts_at: '2026-09-25T00:00:00+08:00', ends_at: '2026-10-08T00:00:00+08:00', demo_expires_at: null, campaign_id: 'first', status: 'active' }
const board: Board = { ...config, demo_expires_at: undefined, entries: [{ rank: 1, alias: 'old-user', amount: '8.0', is_me: false }], me: null, participant_count: 1, refresh_seconds: 60, updated_at: '2026-09-26T00:00:00+08:00' }
let wrapper: VueWrapper | undefined
beforeEach(() => { vi.useFakeTimers(); vi.resetAllMocks(); configAPI.mockResolvedValue({ ...config }); boardAPI.mockResolvedValue({ ...board }) })
afterEach(() => { wrapper?.unmount(); wrapper = undefined; document.body.innerHTML = ''; vi.useRealTimers() })
async function open() {
  wrapper = mount(ActivityLeaderboard, { attachTo: document.body, global: { stubs: { transition: true } } })
  await flushPromises()
  await wrapper.get('[data-testid="header-activity-leaderboard"]').trigger('click')
  await flushPromises()
}
describe('leaderboard configuration synchronization', () => {
  it('clears old rows immediately when a new campaign is configured even if reload fails', async () => {
    await open()
    expect(document.body.textContent).toContain('old-user')
    configAPI.mockResolvedValue({ ...config, campaign_id: 'second', title: 'Second campaign' })
    boardAPI.mockRejectedValueOnce(new Error('unavailable'))
    notifyActivityLeaderboardConfigSaved()
    await flushPromises()
    expect(document.body.textContent).not.toContain('old-user')
    expect(document.body.textContent).toContain('Second campaign')
  })
  it('aborts the old aggregate and ignores its late response after a settings change', async () => {
    let resolveOld!: (value: Board) => void
    boardAPI.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    await open()
    const oldSignal = boardAPI.mock.calls[0][0]!
    configAPI.mockResolvedValue({ ...config, campaign_id: 'second', title: 'Second campaign' })
    boardAPI.mockResolvedValue({ ...board, campaign_id: 'second', title: 'Second campaign', entries: [] })
    notifyActivityLeaderboardConfigSaved()
    await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    resolveOld(board)
    await flushPromises()
    expect(document.body.textContent).not.toContain('old-user')
    expect(document.body.textContent).toContain('Second campaign')
  })
  it('hides and closes when aggregate confirms disabled while metadata is stale', async () => {
    boardAPI.mockResolvedValue({ ...board, enabled: false, status: 'disabled', entries: [] })
    await open()
    expect(wrapper!.find('[data-testid="header-activity-leaderboard"]').exists()).toBe(false)
    expect(document.querySelector('[role="dialog"]')).toBeNull()
  })
})
