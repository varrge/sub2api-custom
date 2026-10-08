import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive } from 'vue'
import { saveAs } from 'file-saver'
import ActivityLeaderboard from '../ActivityLeaderboard.vue'
import { getActivityLeaderboard, getActivityLeaderboardConfig } from '@/api/activityLeaderboard'
import { getAdminActivityLeaderboard, exportActivityLeaderboard, type AdminActivityLeaderboard } from '@/api/admin/activityLeaderboard'
import { notifyActivityLeaderboardConfigSaved } from '@/utils/activityLeaderboardEvents'
import zh from '@/i18n/locales/zh/activityLeaderboard'

const auth = reactive({ isAdmin: true, user: { id: 1 } })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('file-saver', () => ({ saveAs: vi.fn() }))
vi.mock('@/api/activityLeaderboard', () => ({ getActivityLeaderboard: vi.fn(), getActivityLeaderboardConfig: vi.fn() }))
vi.mock('@/api/admin/activityLeaderboard', () => ({ getAdminActivityLeaderboard: vi.fn(), exportActivityLeaderboard: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'zh' }, t: (key: string, params: Record<string, unknown> = {}) => {
  const [section, leaf] = key.split('.')
  const messages = zh[section as keyof typeof zh] as Record<string, string>
  return (messages?.[leaf] || key).replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? ''))
} }) }))
const board: AdminActivityLeaderboard = {
  enabled: true, title: '双节消费榜', subtitle: '', reward_description: '', campaign_id: 'activity-first',
  starts_at: '2026-09-25T00:00:00+08:00', ends_at: '2026-10-08T00:00:00+08:00', status: 'ended',
  updated_at: '2026-10-08T01:00:00+08:00', refresh_seconds: 60, participant_count: 25,
  entries: [{ rank: 1, user_id: 151, email: 'winner@example.test', alias: '012abc345def', amount: '725.1234567890', is_me: false }],
}
const config = { ...board, demo_expires_at: null }
const adminAPI = vi.mocked(getAdminActivityLeaderboard)
const exportAPI = vi.mocked(exportActivityLeaderboard)
let wrapper: VueWrapper | undefined
beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers()
  auth.isAdmin = true
  auth.user.id = 1
  vi.mocked(getActivityLeaderboardConfig).mockResolvedValue(config)
  adminAPI.mockResolvedValue(board)
  exportAPI.mockResolvedValue(new Blob(['csv'], { type: 'text/csv' }))
  vi.mocked(getActivityLeaderboard).mockResolvedValue({ ...board, entries: [{ rank: 1, alias: '012abc345def', amount: '725.1234567890', is_me: false }], me: null })
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; document.body.innerHTML = ''; vi.useRealTimers() })
async function open() {
  wrapper = mount(ActivityLeaderboard, { attachTo: document.body, global: { stubs: { transition: true } } })
  await flushPromises()
  await wrapper.get('[data-testid="header-activity-leaderboard"]').trigger('click')
  await flushPromises()
}
async function click(testId: string) {
  const button = document.querySelector(`[data-testid="${testId}"]`) as HTMLButtonElement
  button.click()
  await flushPromises()
}

describe('admin activity identities and award exports', () => {
  it('shows real ID, email and alias only after loading the admin endpoint', async () => {
    await open()
    expect(adminAPI).toHaveBeenCalledTimes(1)
    expect(getActivityLeaderboard).not.toHaveBeenCalled()
    expect(document.querySelector('[data-testid="leaderboard-user-id"]')?.textContent).toContain('151')
    expect(document.querySelector('[data-testid="leaderboard-email"]')?.textContent).toBe('winner@example.test')
    expect(document.body.textContent).toContain('012abc345def')
    expect(document.querySelector('[data-testid="leaderboard-me"]')).toBeNull()
    expect(document.body.textContent).toContain('25 人上榜')
  })
  it.each(['top3', 'all'] as const)('downloads %s with the displayed campaign ID', async scope => {
    await open()
    await click(`leaderboard-export-${scope}`)
    expect(exportAPI).toHaveBeenCalledWith('activity-first', scope, expect.any(AbortSignal))
    expect(saveAs).toHaveBeenCalledWith(expect.any(Blob), `leaderboard-activity-first-${scope}.csv`)
  })
  it('ordinary users load the public endpoint and have no identity or export controls', async () => {
    auth.isAdmin = false
    await open()
    expect(adminAPI).not.toHaveBeenCalled()
    expect(getActivityLeaderboard).toHaveBeenCalledTimes(1)
    expect(document.querySelector('[data-testid="leaderboard-admin-tools"]')).toBeNull()
    expect(document.querySelector('[data-testid="leaderboard-user-id"]')).toBeNull()
    expect(document.body.textContent).not.toContain('winner@example.test')
  })
  it('aborts downloads and clears private rows immediately when the account role changes', async () => {
    let resolve!: (blob: Blob) => void
    exportAPI.mockImplementationOnce(() => new Promise(done => { resolve = done }))
    await open()
    await click('leaderboard-export-all')
    expect((document.querySelector('[data-testid="leaderboard-export-all"]') as HTMLButtonElement).disabled).toBe(true)
    const signal = exportAPI.mock.calls[0][2]!
    auth.isAdmin = false
    await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(document.body.textContent).not.toContain('winner@example.test')
    resolve(new Blob(['private']))
    await flushPromises()
    expect(saveAs).not.toHaveBeenCalled()
  })
  it('keeps the pending export when routine auth refresh replaces the same user', async () => {
    let resolve!: (blob: Blob) => void
    exportAPI.mockImplementationOnce(() => new Promise(done => { resolve = done }))
    await open()
    await click('leaderboard-export-all')
    const signal = exportAPI.mock.calls[0][2]!
    auth.user = { id: 1 }
    await flushPromises()
    expect(signal.aborted).toBe(false)
    expect(adminAPI).toHaveBeenCalledTimes(1)
    expect(document.body.textContent).toContain('winner@example.test')
    resolve(new Blob(['private']))
    await flushPromises()
    expect(saveAs).toHaveBeenCalledTimes(1)
  })
  it('ignores a late admin response after switching accounts', async () => {
    let resolve!: (value: AdminActivityLeaderboard) => void
    adminAPI.mockImplementationOnce(() => new Promise(done => { resolve = done }))
    await open()
    const signal = adminAPI.mock.calls[0][0]!
    auth.isAdmin = false
    await flushPromises()
    expect(signal.aborted).toBe(true)
    resolve(board)
    await flushPromises()
    expect(document.body.textContent).not.toContain('winner@example.test')
  })
  it('reports an export failure and permits retry without hiding the rows', async () => {
    exportAPI.mockRejectedValueOnce({ status: 500 })
    await open()
    await click('leaderboard-export-top3')
    expect(document.body.textContent).toContain('名单导出失败')
    expect(document.body.textContent).toContain('winner@example.test')
    await click('leaderboard-export-top3')
    expect(saveAs).toHaveBeenCalledTimes(1)
    expect(document.body.textContent).not.toContain('名单导出失败')
  })
  it('reloads a changed campaign and requires another export click', async () => {
    exportAPI.mockRejectedValueOnce({ status: 409 })
    await open()
    adminAPI.mockResolvedValue({ ...board, campaign_id: 'activity-next' })
    await click('leaderboard-export-top3')
    expect(adminAPI).toHaveBeenCalledTimes(2)
    expect(document.body.textContent).toContain('活动已变更')
    expect(saveAs).not.toHaveBeenCalled()
    await click('leaderboard-export-top3')
    expect(exportAPI).toHaveBeenLastCalledWith('activity-next', 'top3', expect.any(AbortSignal))
  })
  it('cancels export when configured activity changes, even if the old response arrives', async () => {
    let resolve!: (blob: Blob) => void
    exportAPI.mockImplementationOnce(() => new Promise(done => { resolve = done }))
    await open()
    await click('leaderboard-export-all')
    vi.mocked(getActivityLeaderboardConfig).mockResolvedValue({ ...config, campaign_id: 'new' })
    notifyActivityLeaderboardConfigSaved()
    await flushPromises()
    expect(exportAPI.mock.calls[0][2]!.aborted).toBe(true)
    resolve(new Blob(['old']))
    await flushPromises()
    expect(saveAs).not.toHaveBeenCalled()
  })
  it('keeps retry available after an initial failure and clears rows on later permission failure', async () => {
    adminAPI.mockRejectedValueOnce({ status: 503 })
    await open()
    expect(document.body.textContent).toContain('榜单暂时无法加载')
    const retry = [...document.querySelectorAll('button')].find(button => button.textContent === '重试')!
    retry.click()
    await flushPromises()
    expect(document.body.textContent).toContain('winner@example.test')
    adminAPI.mockRejectedValueOnce({ status: 403 })
    await vi.advanceTimersByTimeAsync(60000)
    expect(document.body.textContent).not.toContain('winner@example.test')
  })
  it('disables exports before the activity starts or with no participants', async () => {
    adminAPI.mockResolvedValue({ ...board, status: 'upcoming', entries: [], participant_count: 0 })
    await open()
    expect((document.querySelector('[data-testid="leaderboard-export-all"]') as HTMLButtonElement).disabled).toBe(true)
    await click('leaderboard-export-all')
    expect(exportAPI).not.toHaveBeenCalled()
  })
})
