import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ActivityLeaderboard from '../ActivityLeaderboard.vue'
import { getActivityLeaderboard, type ActivityLeaderboard as LeaderboardData } from '@/api/activityLeaderboard'
import zh from '@/i18n/locales/zh/activityLeaderboard'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: { value: 'zh' },
    t: (key: string, params: Record<string, unknown> = {}) => {
      const message = zh.activityLeaderboard[key.split('.')[1] as keyof typeof zh.activityLeaderboard] ?? key
      return message.replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? ''))
    },
  }),
}))

vi.mock('@/api/activityLeaderboard', () => ({ getActivityLeaderboard: vi.fn() }))
const fetchMock = vi.mocked(getActivityLeaderboard)
let wrapper: VueWrapper | undefined

const POPOVER = '[data-testid="activity-leaderboard-popover"]'
const TRIGGER = '[data-testid="header-activity-leaderboard"]'

function fixture(status: LeaderboardData['status'] = 'active'): LeaderboardData {
  return {
    campaign_id: 'double-festival-2026',
    starts_at: '2026-09-25T00:00:00+08:00',
    ends_at: '2026-10-08T00:00:00+08:00',
    status,
    updated_at: '2026-09-26T01:00:00Z',
    refresh_seconds: 60,
    participant_count: 24,
    entries: [{ rank: 1, alias: 'ab1234cd5678', amount: '19.12345678', is_me: false }],
    me: { rank: 24, alias: 'de1234fg5678', amount: '1.12345678', is_me: true },
  }
}

async function open() {
  wrapper = mount(ActivityLeaderboard, {
    attachTo: document.body,
    // Exercise component lifecycle without CSS animation timing; real browser
    // checks cover the transition and positioning.
    global: { stubs: { transition: true } },
  })
  expect(fetchMock).not.toHaveBeenCalled()
  await wrapper.get(TRIGGER).trigger('click')
  await flushPromises()
  return wrapper
}

function popover() {
  return document.querySelector(POPOVER) as HTMLElement | null
}

async function settle() {
  await flushPromises()
}

async function pressEscape() {
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
  await flushPromises()
  await settle()
}

describe('ActivityLeaderboard', () => {
  beforeEach(() => {
    fetchMock.mockReset()
    vi.useFakeTimers()
  })
  afterEach(() => {
    wrapper?.unmount()
    document.body.innerHTML = ''
    vi.useRealTimers()
  })

  it('fetches on open, shows self outside top 20 and formats Beijing time and billed credits', async () => {
    fetchMock.mockResolvedValue(fixture())
    await open()
    const body = document.body
    expect(body.querySelector('[role="dialog"]')).not.toBeNull()
    expect(body.querySelector('[data-testid="leaderboard-me"]')?.textContent).toContain('#24')
    expect(body.textContent).toContain('19.1235')
    expect(body.textContent).toContain('09:00')
    expect(body.textContent).toContain('余额、订阅及拼团月卡')
    expect(body.textContent).not.toContain('¥')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(60000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    await pressEscape()
    expect(popover()).toBeNull()
    await vi.advanceTimersByTimeAsync(60000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(document.body.classList.contains('modal-open')).toBe(false)
  })

  it.each(['upcoming', 'active', 'ended'] as const)('renders %s with no fictional participants', async (status) => {
    fetchMock.mockResolvedValue({ ...fixture(status), entries: [], me: null, participant_count: 0 })
    await open()
    expect(document.querySelectorAll('[data-testid="leaderboard-row"]')).toHaveLength(0)
    expect(document.querySelector('[data-testid="leaderboard-empty"]')).not.toBeNull()
    if (status === 'upcoming') {
      expect(document.body.textContent).toContain('即将开始')
      expect(document.querySelector('[data-testid="leaderboard-me"]')).toBeNull()
    } else if (status === 'ended') {
      expect(document.body.textContent).toContain('奖励以最终核验和公告为准')
    } else {
      expect(document.body.textContent).toContain('暂未上榜')
    }
  })

  it('labels temporary data and shows sample ranks before the event, then clears them on expiry', async () => {
    fetchMock.mockResolvedValueOnce({ ...fixture('upcoming'), demo: true, demo_expires_at: '2026-09-23T19:00:00+08:00' })
    fetchMock.mockResolvedValueOnce({ ...fixture('upcoming'), entries: [], me: null, participant_count: 0 })
    await open()
    expect(document.querySelector('[data-testid="leaderboard-status"]')?.textContent).toContain('演示数据')
    expect(document.querySelector('[data-testid="leaderboard-demo-notice"]')?.textContent).toContain('均为模拟数据')
    expect(document.querySelector('[data-testid="leaderboard-demo-notice"]')?.textContent).toContain('2026/09/23 19:00')
    expect(document.querySelectorAll('[data-testid="leaderboard-row"]')).toHaveLength(1)
    expect(document.querySelector('[data-testid="leaderboard-me"]')).not.toBeNull()
    await vi.advanceTimersByTimeAsync(60000)
    expect(document.querySelector('[data-testid="leaderboard-demo-notice"]')).toBeNull()
    expect(document.querySelectorAll('[data-testid="leaderboard-row"]')).toHaveLength(0)
    expect(document.querySelector('[data-testid="leaderboard-empty"]')).not.toBeNull()
  })

  it('supports retry after failure and retains data on failed refresh', async () => {
    fetchMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(fixture()).mockRejectedValueOnce(new Error('offline'))
    await open()
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('暂时无法加载')
    ;(document.querySelector('[role="alert"] button') as HTMLButtonElement).click()
    await flushPromises()
    expect(document.querySelector('[role="alert"]')).toBeNull()
    await vi.advanceTimersByTimeAsync(60000)
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('上次成功获取')
    expect(document.querySelectorAll('[data-testid="leaderboard-row"]')).toHaveLength(1)
  })

  it('aborts a closed request and ignores a late result after reopening', async () => {
    let resolveFirst!: (value: LeaderboardData) => void
    fetchMock.mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve }))
    fetchMock.mockResolvedValueOnce({ ...fixture(), me: null, entries: [] })
    const view = await open()
    const signal = fetchMock.mock.calls[0][0]!
    await pressEscape()
    expect(signal.aborted).toBe(true)
    await view.get(TRIGGER).trigger('click')
    await flushPromises()
    resolveFirst(fixture())
    await flushPromises()
    expect(document.querySelectorAll('[data-testid="leaderboard-row"]')).toHaveLength(0)
    view.unmount()
    wrapper = undefined
    await vi.advanceTimersByTimeAsync(60000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('opens a compact anchored popover without overlay, blur or scroll lock, and toggles closed', async () => {
    fetchMock.mockResolvedValue(fixture())
    const view = await open()
    const trigger = view.get(TRIGGER)
    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(trigger.attributes('aria-controls')).toBe('activity-leaderboard-popover')
    expect(trigger.attributes('aria-haspopup')).toBe('dialog')
    const panel = popover()
    expect(panel).not.toBeNull()
    expect(panel!.getAttribute('role')).toBe('dialog')
    expect(panel!.getAttribute('aria-modal')).toBe('false')
    expect(panel!.getAttribute('aria-label')).toBe('双节消费榜')
    expect(panel!.style.top).toMatch(/^\d+(\.\d+)?px$/)
    expect(panel!.style.left).toMatch(/^\d+(\.\d+)?px$/)
    expect(panel!.className).not.toContain('inset-0')
    expect(document.querySelector('.modal-overlay')).toBeNull()
    expect(document.body.classList.contains('modal-open')).toBe(false)
    expect(document.body.style.overflow).toBe('')
    expect(document.activeElement).toBe(panel)

    await trigger.trigger('click')
    await settle()
    expect(popover()).toBeNull()
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })

  it('closes on outside pointerdown and keeps the focus the user clicked to', async () => {
    fetchMock.mockResolvedValue(fixture())
    await open()
    expect(popover()).not.toBeNull()

    const outside = document.createElement('button')
    document.body.appendChild(outside)
    outside.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }))
    outside.focus()
    await settle()

    expect(popover()).toBeNull()
    expect(document.activeElement).toBe(outside)
    outside.remove()

    // Reopen: pointerdown inside the panel must not close it
    await wrapper!.get(TRIGGER).trigger('click')
    await flushPromises()
    const panel = popover()!
    panel.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }))
    await flushPromises()
    expect(popover()).not.toBeNull()
  })

  it('restores trigger focus when closed via Escape or the close button', async () => {
    fetchMock.mockResolvedValue(fixture())
    const view = await open()
    const triggerEl = view.get(TRIGGER).element as HTMLElement

    await pressEscape()
    expect(popover()).toBeNull()
    expect(document.activeElement).toBe(triggerEl)

    await view.get(TRIGGER).trigger('click')
    await flushPromises()
    ;(document.querySelector('[data-testid="leaderboard-close"]') as HTMLButtonElement).click()
    await settle()
    expect(popover()).toBeNull()
    expect(document.activeElement).toBe(triggerEl)
  })

  it('removes listeners and timers when closed or unmounted', async () => {
    fetchMock.mockResolvedValue(fixture())
    const view = await open()
    await pressEscape()

    // Listeners are gone: further Escape / pointerdown events are inert
    await pressEscape()
    document.body.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }))
    await settle()
    expect(popover()).toBeNull()
    expect(fetchMock).toHaveBeenCalledTimes(1)

    // Unmount while open: pending refresh is cancelled
    await view.get(TRIGGER).trigger('click')
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(2)
    view.unmount()
    wrapper = undefined
    await vi.advanceTimersByTimeAsync(120000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
