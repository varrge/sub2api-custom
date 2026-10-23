import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { ref } from 'vue'
import BoxView from '../BoxView.vue'
import {
  getBoxBootstrap,
  createBoxDraw,
  recoverBoxDraw,
  type BoxBootstrap,
  type BoxCardTier,
  type BoxDraw,
} from '@/api/luckyBox'
import zh from '@/i18n/locales/zh/luckyBox'

vi.mock('@/api/luckyBox', () => ({
  getBoxBootstrap: vi.fn(),
  createBoxDraw: vi.fn(),
  recoverBoxDraw: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showWarning: vi.fn(), showSuccess: vi.fn() }),
}))

const authUser = ref<{ id: number; username: string } | null>({ id: 1, username: 'alice' })

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ get user() { return authUser.value } }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh' },
      t: (key: string, params: Record<string, unknown> = {}) => {
        const message = zh.luckyBox[key.split('.')[1] as keyof typeof zh.luckyBox] ?? key
        return message.replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? ''))
      },
    }),
  }
})

const bootstrapMock = vi.mocked(getBoxBootstrap)
const drawMock = vi.mocked(createBoxDraw)
const recoverMock = vi.mocked(recoverBoxDraw)
let wrapper: VueWrapper | undefined

const STORAGE_KEY = 'sub2api_lucky_box_pending_draw:1'

const CAMPAIGN = {
  id: 'c1',
  connectionId: 'conn1',
  name: '疯狂开箱',
  status: 'active',
  startAt: '2026-10-01T00:00:00+08:00',
  endAt: '2026-11-01T00:00:00+08:00',
  activeConfigVersionId: null,
}

const PRIZES = [
  {
    id: 'p1',
    name: '余额 5 元',
    type: 'balance' as const,
    amount: 5,
    weight: 10,
    probability: 50,
    weights: { low: 10, medium: 5, high: 1 },
    probabilities: { low: 50, medium: 30, high: 5 },
    rarity: 'rare' as const,
    displayText: '余额奖励',
    icon: '',
    sortOrder: 1,
  },
  {
    id: 'p2',
    name: '空',
    type: 'empty' as const,
    amount: null,
    weight: 10,
    probability: 50,
    weights: { low: 10, medium: 15, high: 19 },
    probabilities: { low: 50, medium: 70, high: 95 },
    rarity: 'common' as const,
    displayText: '未中奖',
    icon: '',
    sortOrder: 2,
  },
]

function drawFixture(overrides: Partial<BoxDraw> = {}): BoxDraw {
  return {
    id: 'd1',
    requestId: 'box-req-1',
    campaignId: CAMPAIGN.id,
    connectionId: 'conn1',
    userId: '1',
    email: 'a@b.c',
    cardTier: 'high',
    status: 'delivered',
    prize: { id: 'p1', name: '余额 5 元', type: 'balance', amount: 5, rarity: 'rare' },
    errorMessage: '',
    dispositionReason: '',
    dispositionBy: '',
    createdAt: '2026-10-10T10:00:00+08:00',
    deliveredAt: null,
    ...overrides,
  }
}

function bootstrapFixture(overrides: Partial<BoxBootstrap> = {}): BoxBootstrap {
  return {
    campaign: CAMPAIGN,
    availableCards: 2,
    availableCardsByTier: { low: 1, medium: 1, high: 2 },
    progress: [],
    prizes: PRIZES,
    draws: [],
    ...overrides,
  }
}

function drawResponse(draw: BoxDraw, count: number) {
  return {
    draw,
    availableCards: count,
    availableCardsByTier: { low: 0, medium: 0, high: count } as Record<BoxCardTier, number>,
  }
}

async function mountView() {
  wrapper = mount(BoxView, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true, transition: true } },
  })
  await flushPromises()
  return wrapper
}

describe('BoxView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-10T12:00:00+08:00'))
    bootstrapMock.mockReset()
    drawMock.mockReset()
    recoverMock.mockReset()
    recoverMock.mockResolvedValue({ found: false, draw: null })
    localStorage.clear()
    authUser.value = { id: 1, username: 'alice' }
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    document.body.innerHTML = ''
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('renders tier counts and selects the highest tier with cards by default', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    const view = await mountView()

    expect(view.get('[data-testid="box-tier-low"]').text()).toContain('1')
    expect(view.get('[data-testid="box-tier-medium"]').text()).toContain('1')
    expect(view.get('[data-testid="box-tier-high"]').text()).toContain('2')
    expect(view.get('[data-testid="box-tier-high"]').attributes('aria-pressed')).toBe('true')
    // High-tier probabilities shown by default: 5% / 95%
    expect(view.get('[data-testid="box-prize-p1"]').text()).toContain('5.00%')
    expect(view.get('[data-testid="box-prize-p2"]').text()).toContain('95.00%')
  })

  it('updates displayed probabilities when a different tier is selected', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    const view = await mountView()

    await view.get('[data-testid="box-tier-medium"]').trigger('click')

    expect(view.get('[data-testid="box-tier-medium"]').attributes('aria-pressed')).toBe('true')
    expect(view.get('[data-testid="box-prize-p1"]').text()).toContain('30.00%')
    expect(view.get('[data-testid="box-prize-p2"]').text()).toContain('70.00%')
  })

  it('renders a horizontal prize reel with a fixed center marker', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    const view = await mountView()
    expect(view.find('[data-testid="box-reel-viewport"]').exists()).toBe(true)
    expect(view.find('[data-testid="box-reel-marker"]').exists()).toBe(true)
    expect(view.findAll('[data-testid="box-reel-item"]').length).toBeGreaterThan(6)
  })

  it('disables the open button while a draw is pending', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    let resolveDraw!: (value: ReturnType<typeof drawResponse>) => void
    drawMock.mockImplementationOnce(
      () => new Promise((resolve) => { resolveDraw = resolve })
    )
    const view = await mountView()

    const open = view.get('[data-testid="box-open"]')
    expect(open.attributes('disabled')).toBeUndefined()
    await open.trigger('click')
    expect(drawMock).toHaveBeenCalledTimes(1)
    expect(view.get('[data-testid="box-open"]').attributes('disabled')).toBeDefined()

    resolveDraw(drawResponse(drawFixture({ requestId: drawMock.mock.calls[0][0].requestId }), 1))
    await flushPromises()
    expect(view.get('[data-testid="box-open"]').attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(100)
    await flushPromises()
    expect(view.get('[data-testid="box-open"]').attributes('disabled')).toBeUndefined()
  })

  it('keeps every draw control locked during the full reel animation and lands on the server prize', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockImplementation(async request => drawResponse(drawFixture({ requestId: request.requestId }), 1))
    const view = await mountView()
    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)
    expect(view.get('[data-testid="box-open"]').attributes('disabled')).toBeDefined()
    expect(view.get('[data-testid="box-tier-low"]').attributes('disabled')).toBeDefined()
    expect(view.get('[data-testid="box-refresh"]').attributes('disabled')).toBeDefined()
    expect(view.find('[data-testid="box-result"]').exists()).toBe(false)
    expect(view.find('[data-testid="box-draw-d1"]').exists()).toBe(false)
    expect(view.get('[data-target-prize]').text()).toContain('余额 5 元')
    await view.get('[data-testid="box-open"]').trigger('click')
    expect(drawMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(4700)
    await flushPromises()
    expect(view.get('[data-testid="box-result"]').text()).toContain('余额 5 元')
    expect(view.get('[data-testid="box-open"]').attributes('disabled')).toBeUndefined()
    vi.mocked(window.matchMedia).mockRestore()
  })

  it('reveals the server-returned prize and upserts it into history', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockImplementation(async request => drawResponse(drawFixture({ requestId: request.requestId }), 1))
    const view = await mountView()

    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()
    // Opening animation is shown first
    expect(view.find('[data-testid="box-opening"]').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(100)
    await flushPromises()

    const result = view.get('[data-testid="box-result"]')
    expect(result.text()).toContain('余额 5 元')
    expect(result.text()).toContain('稀有')
    expect(result.text()).toContain('已发放')
    expect(view.get('[data-testid="box-tier-high"]').text()).toContain('1')
    expect(view.get('[data-testid="box-draw-d1"]').text()).toContain('余额 5 元')
    // A confirmed draw leaves nothing pending, in memory or in storage.
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('does not expose the previous account prize when the account changes mid-animation', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockImplementation(async request => drawResponse(drawFixture({ requestId: request.requestId }), 1))
    const view = await mountView()
    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()
    expect(view.find('[data-testid="box-opening"]').exists()).toBe(true)
    authUser.value = { id: 2, username: 'bob' }
    await flushPromises()
    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()
    expect(view.find('[data-testid="box-draw-d1"]').exists()).toBe(false)
    expect(view.find('[data-testid="box-result"]').exists()).toBe(false)
    expect(document.querySelector('[data-testid="box-reveal"]')).toBeNull()
    expect(view.get('[data-testid="box-open"]').attributes('disabled')).toBeUndefined()
  })

  it('shows the fixed result if the campaign ends during the reel animation', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    vi.setSystemTime(new Date('2026-10-31T23:59:58+08:00'))
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockImplementation(async request => drawResponse(drawFixture({ requestId: request.requestId }), 1))
    const view = await mountView()
    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(view.text()).toContain('开箱活动已结束')
    expect(view.get('[data-testid="box-result"]').text()).toContain('余额 5 元')
    expect(drawMock).toHaveBeenCalledTimes(1)
  })

  it('keeps the exact same requestId when retrying a failed draw', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock
      .mockRejectedValueOnce({ status: 500, message: '服务器繁忙' })
      .mockImplementationOnce(async request => drawResponse(drawFixture({ requestId: request.requestId, status: 'delivered', deliveredAt: '2026-10-10T10:01:00+08:00' }), 1))
    const view = await mountView()

    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()

    expect(view.get('[data-testid="box-draw-error"]').text()).toContain('服务器繁忙')
    const firstCall = drawMock.mock.calls[0][0]
    expect(firstCall.requestId).toMatch(/^box-/)

    await view.get('[data-testid="box-retry-draw"]').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(100)
    await flushPromises()

    expect(drawMock).toHaveBeenCalledTimes(2)
    expect(drawMock.mock.calls[1][0]).toEqual(firstCall)
    expect(view.get('[data-testid="box-result"]').text()).toContain('已发放')
  })

  it('does not claim a failed delivery has been awarded in the result popup', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockImplementation(async request => drawResponse(drawFixture({ requestId: request.requestId, status: 'delivery_failed' }), 1))
    const view = await mountView()
    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(100)
    await flushPromises()
    const popup = document.querySelector('[data-testid="box-reveal"]')
    expect(popup?.textContent).toContain('发放失败')
    expect(popup?.textContent).not.toContain('已发放')
    expect(document.querySelector('[role="dialog"]')?.textContent).not.toContain('恭喜获得')
    expect(localStorage.getItem(STORAGE_KEY)).not.toBeNull()
  })

  it('issues no new draw id after a failed draw: open and tier switches stay blocked', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockRejectedValue({ status: 500, message: '服务器繁忙' })
    const view = await mountView()

    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()
    expect(drawMock).toHaveBeenCalledTimes(1)

    // Open button is disabled and even a forced click issues no new request.
    expect(view.get('[data-testid="box-open"]').attributes('disabled')).toBeDefined()
    await view.get('[data-testid="box-open"]').trigger('click')
    expect(drawMock).toHaveBeenCalledTimes(1)

    // Tier switches are blocked while the request is unresolved.
    expect(view.get('[data-testid="box-tier-low"]').attributes('disabled')).toBeDefined()
    await view.get('[data-testid="box-tier-low"]').trigger('click')
    expect(view.get('[data-testid="box-tier-high"]').attributes('aria-pressed')).toBe('true')

    // The persisted pending draw is scoped to the authenticated user.
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}')
    expect(stored.userId).toBe('1')
    expect(stored.requestId).toBe(drawMock.mock.calls[0][0].requestId)
  })

  it('recovers the same requestId after remount when the outcome is still unknown', async () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ userId: '1', campaignId: 'c1', requestId: 'box-pending-1', cardTier: 'high' })
    )
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockResolvedValue(drawResponse(drawFixture({ requestId: 'box-pending-1' }), 1))
    const view = await mountView()

    // No auto-retry on reload; the resume UI offers the identical requestId.
    expect(drawMock).not.toHaveBeenCalled()
    expect(view.find('[data-testid="box-retry-draw"]').exists()).toBe(true)

    await view.get('[data-testid="box-retry-draw"]').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(100)
    await flushPromises()

    expect(drawMock).toHaveBeenCalledTimes(1)
    expect(drawMock).toHaveBeenCalledWith({ requestId: 'box-pending-1', cardTier: 'high', campaignId: 'c1' }, expect.anything())
    expect(view.get('[data-testid="box-result"]').text()).toContain('余额 5 元')
  })

  it('recovers a recorded draw from bootstrap.draws by requestId after remount', async () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ userId: '1', campaignId: 'c1', requestId: 'box-req-1', cardTier: 'high' })
    )
    bootstrapMock.mockResolvedValue(
      bootstrapFixture({ draws: [drawFixture({ requestId: 'box-req-1' })] })
    )
    const view = await mountView()

    // Outcome already recorded: result shown, no retry offered, storage cleared.
    expect(view.find('[data-testid="box-retry-draw"]').exists()).toBe(false)
    expect(view.get('[data-testid="box-result"]').text()).toContain('余额 5 元')
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(drawMock).not.toHaveBeenCalled()
  })

  it('never resumes another user pending draw after an account switch', async () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ userId: '1', campaignId: 'c1', requestId: 'box-alice-1', cardTier: 'high' })
    )
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    const view = await mountView()
    expect(view.find('[data-testid="box-retry-draw"]').exists()).toBe(true)

    authUser.value = { id: 2, username: 'bob' }
    await flushPromises()

    // Alice's pending draw and results are gone; nothing is auto-retried.
    expect(view.find('[data-testid="box-retry-draw"]').exists()).toBe(false)
    expect(view.find('[data-testid="box-draw-error"]').exists()).toBe(false)
    expect(drawMock).not.toHaveBeenCalled()
    expect(view.get('[data-testid="box-tier-high"]').attributes('aria-pressed')).toBe('true')
  })

  it('flips to the ended state when the ticking clock reaches the campaign end', async () => {
    vi.setSystemTime(new Date('2026-10-31T23:59:30+08:00'))
    bootstrapMock.mockResolvedValue(
      bootstrapFixture({
        campaign: { ...CAMPAIGN, endAt: '2026-11-01T00:00:00+08:00' },
      })
    )
    const view = await mountView()
    expect(view.find('[data-testid="box-open"]').exists()).toBe(true)

    // Exact end boundary counts as ended (>= end).
    vi.setSystemTime(new Date('2026-11-01T00:00:00+08:00'))
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()

    expect(view.text()).toContain('开箱活动已结束')
    expect(view.find('[data-testid="box-open"]').exists()).toBe(false)
  })

  it('shows the ended state for an expired campaign', async () => {
    bootstrapMock.mockResolvedValue(
      bootstrapFixture({
        campaign: { ...CAMPAIGN, endAt: '2026-10-09T00:00:00+08:00' },
      })
    )
    const view = await mountView()

    expect(view.text()).toContain('开箱活动已结束')
    expect(view.text()).toContain(CAMPAIGN.name)
    expect(view.find('[data-testid="box-open"]').exists()).toBe(false)
  })

  it('retries a delivery_failed history row with that draw own requestId and cardTier', async () => {
    const failedDraw = drawFixture({
      id: 'd-fail',
      requestId: 'box-req-fail',
      cardTier: 'medium',
      status: 'delivery_failed',
    })
    bootstrapMock.mockResolvedValue(bootstrapFixture({ draws: [failedDraw] }))
    recoverMock.mockResolvedValue({ found: true,
      ...drawResponse({ ...failedDraw, status: 'delivered', deliveredAt: '2026-10-10T10:02:00+08:00' }, 2)
    })
    const view = await mountView()

    const retry = view.get('[data-testid="box-retry-delivery-d-fail"]')
    await retry.trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(100)
    await flushPromises()

    expect(drawMock).not.toHaveBeenCalled()
    expect(recoverMock).toHaveBeenCalledWith({ requestId: 'box-req-fail', cardTier: 'medium', campaignId: 'c1' }, expect.anything())
    expect(view.get('[data-testid="box-draw-d-fail"]').text()).toContain('已发放')
  })

  it('keeps recovery available after the campaign ends and never draws from a new campaign', async () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ userId: '1', campaignId: 'old-event', requestId: 'box-old-unknown', cardTier: 'high' }))
    bootstrapMock.mockResolvedValue(bootstrapFixture({ campaign: null, draws: [] }))
    const view = await mountView()
    await view.get('[data-testid="box-retry-draw"]').trigger('click')
    await flushPromises()
    expect(recoverMock).toHaveBeenCalledWith({ campaignId: 'old-event', requestId: 'box-old-unknown', cardTier: 'high' }, expect.anything())
    expect(drawMock).not.toHaveBeenCalled()
    expect(localStorage.getItem(STORAGE_KEY)).not.toBeNull()
    recoverMock.mockResolvedValue({ found: true, ...drawResponse(drawFixture({ campaignId: 'old-event', requestId: 'box-old-unknown', status: 'delivered' }), 0) })
    await view.get('[data-testid="box-retry-draw"]').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(100)
    expect(view.get('[data-testid="box-result"]').text()).toContain('已发放')
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('does not discard the saved request after a malformed success response', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockResolvedValue({ availableCards: 1, availableCardsByTier: { low: 1, medium: 0, high: 0 } } as never)
    const view = await mountView()
    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()
    expect(localStorage.getItem(STORAGE_KEY)).not.toBeNull()
    expect(view.get('[data-testid="box-open"]').attributes('disabled')).toBeDefined()
    expect(view.get('[data-testid="box-draw-error"]').text()).toContain('无法确认')
  })

  it('retains an ended-campaign failed delivery across reload until a terminal outcome', async () => {
    const pending = { userId: '1', campaignId: 'old-event', requestId: 'box-failed-old', cardTier: 'high' }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(pending))
    bootstrapMock.mockResolvedValue(bootstrapFixture({ campaign: null, draws: [] }))
    recoverMock.mockResolvedValue({ found: true, ...drawResponse(drawFixture({ campaignId: 'old-event', requestId: pending.requestId, status: 'delivery_failed' }), 0) })
    let view = await mountView()
    await view.get('[data-testid="box-retry-draw"]').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(100)
    expect(localStorage.getItem(STORAGE_KEY)).not.toBeNull()
    view.unmount()
    view = await mountView()
    expect(view.find('[data-testid="box-retry-draw"]').exists()).toBe(true)
    recoverMock.mockResolvedValue({ found: true, ...drawResponse(drawFixture({ campaignId: 'old-event', requestId: pending.requestId, status: 'delivered' }), 0) })
    await view.get('[data-testid="box-retry-draw"]').trigger('click')
    await flushPromises()
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('retains recovery when prize rarity is missing from a success response', async () => {
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockImplementation(async request => {
      const draw = drawFixture({ requestId: request.requestId })
      delete (draw.prize as Partial<NonNullable<BoxDraw['prize']>>).rarity
      return drawResponse(draw, 1)
    })
    const view = await mountView()
    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()
    expect(localStorage.getItem(STORAGE_KEY)).not.toBeNull()
    expect(view.get('[data-testid="box-draw-error"]').text()).toContain('无法确认')
  })

  it('keeps both users unresolved requests when accounts switch and returns to the first request', async () => {
    const original = { userId: '1', campaignId: 'c1', requestId: 'box-alice-pending', cardTier: 'high' }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(original))
    bootstrapMock.mockResolvedValue(bootstrapFixture())
    drawMock.mockRejectedValue(new Error('offline'))
    const view = await mountView()
    authUser.value = { id: 2, username: 'bob' }
    await flushPromises()
    await view.get('[data-testid="box-open"]').trigger('click')
    await flushPromises()
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY)!)).toEqual(original)
    expect(localStorage.getItem('sub2api_lucky_box_pending_draw:2')).not.toBeNull()
    authUser.value = { id: 1, username: 'alice' }
    await flushPromises()
    await view.get('[data-testid="box-retry-draw"]').trigger('click')
    await flushPromises()
    expect(recoverMock.mock.calls.at(-1)?.[0].requestId).toBe('box-alice-pending')
  })
})
