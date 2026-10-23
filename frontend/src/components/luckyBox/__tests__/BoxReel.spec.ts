import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import BoxReel from '../BoxReel.vue'
import type { BoxDraw, BoxPrize } from '@/api/luckyBox'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const prizes: BoxPrize[] = Array.from({ length: 7 }, (_, index) => ({
  id: `p${index}`, name: `Prize ${index}`, type: 'balance', amount: index + 1,
  rarity: 'rare', weight: 1, probability: 100 / 7,
  weights: { low: 1, medium: 1, high: 1 }, probabilities: { low: 100 / 7, medium: 100 / 7, high: 100 / 7 },
  displayText: '', icon: '', sortOrder: index,
}))
const draw = { id: 'fixed', prize: { ...prizes[6], name: 'Server-fixed result', amount: 99 }, status: 'delivered' } as BoxDraw
let view: VueWrapper<InstanceType<typeof BoxReel>> | undefined

beforeEach(() => {
  vi.useFakeTimers()
  vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
})
afterEach(() => {
  view?.unmount()
  view = undefined
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

function mountReel() {
  view = mount(BoxReel, { props: { prizes, tier: 'low', waiting: false }, global: { stubs: { Icon: true } } })
  return view
}

describe('BoxReel', () => {
  it('cycles all seven eligible fillers without changing the authoritative prize', async () => {
    const reel = mountReel()
    const finished = reel.vm.play(draw)
    await flushPromises()
    const fillers = reel.findAll('[data-testid="box-reel-item"]').filter(item => item.attributes('data-target-prize') === undefined)
    expect(new Set(fillers.map(item => item.get('.box-reel__item-name').text())).size).toBe(7)
    expect(reel.get('[data-target-prize]').text()).toContain('Server-fixed result')
    expect(reel.get('[data-target-prize]').text()).toContain('99.00')
    await vi.advanceTimersByTimeAsync(5700)
    await finished
    expect(reel.get('[data-target-prize]').classes()).toContain('is-winner')
  })

  it('settles the pending animation when unmounted rather than stranding its caller', async () => {
    const reel = mountReel()
    const finished = vi.fn()
    void reel.vm.play(draw).then(finished)
    await flushPromises()
    reel.unmount()
    view = undefined
    await flushPromises()
    expect(finished).toHaveBeenCalledOnce()
    await vi.advanceTimersByTimeAsync(5700)
    expect(finished).toHaveBeenCalledOnce()
  })

  it('finishes and recenters the immutable target if the viewport resizes mid-spin', async () => {
    let resize!: ResizeObserverCallback
    vi.stubGlobal('ResizeObserver', class {
      constructor(callback: ResizeObserverCallback) { resize = callback }
      observe() {}
      disconnect() {}
    })
    const reel = mountReel()
    const finished = vi.fn()
    void reel.vm.play(draw).then(finished)
    await flushPromises()
    const viewport = reel.get('[data-testid="box-reel-viewport"]').element
    const target = reel.get('[data-target-prize]').element
    Object.defineProperty(viewport, 'clientWidth', { configurable: true, value: 390 })
    Object.defineProperty(target, 'offsetLeft', { configurable: true, value: 4020 })
    Object.defineProperty(target, 'offsetWidth', { configurable: true, value: 124 })
    resize([], {} as ResizeObserver)
    await flushPromises()
    expect(reel.get('[data-testid="box-reel-track"]').attributes('style')).toContain('translate3d(-3887px')
    expect(finished).toHaveBeenCalledOnce()
    expect(reel.get('[data-target-prize]').text()).toContain('Server-fixed result')
  })

  it('still reveals the prize when audio initialization is blocked', async () => {
    vi.stubGlobal('AudioContext', class { constructor() { throw new Error('Audio unavailable') } })
    const reel = mountReel()
    await reel.get('[data-testid="box-reel-sound"]').setValue(true)
    expect(() => reel.vm.prepareSound()).not.toThrow()
    const finished = reel.vm.play(draw)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5700)
    await finished
    expect(reel.get('[data-target-prize]').classes()).toContain('is-winner')
  })
})
