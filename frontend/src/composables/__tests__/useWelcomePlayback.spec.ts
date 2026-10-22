import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, type EffectScope } from 'vue'
import { useWelcomePlayback } from '../useWelcomePlayback'
import { welcomeEffects } from '@/utils/welcomeEffects'

let scope: EffectScope
const start = () => { scope = effectScope(); return scope.run(useWelcomePlayback)! }

describe('welcome effects playback', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => { scope?.stop(); vi.useRealTimers() })

  it.each(welcomeEffects)('plays %s once with the same lifecycle and cleans up', async effect => {
    const state = start()
    state.play('欢迎', effect)
    expect(state.effect.value).toBe(effect)
    expect(state.phase.value).toBe('waiting')
    await vi.advanceTimersByTimeAsync(350)
    expect(state.phase.value).toBe('entering')
    await vi.advanceTimersByTimeAsync(400)
    expect(state.phase.value).toBe('dwell')
    await vi.advanceTimersByTimeAsync(3500)
    expect(state.phase.value).toBe('exiting')
    await vi.advanceTimersByTimeAsync(250)
    expect(state.message.value).toBeNull()
    expect(state.effect.value).toBe('capsule')
    expect(vi.getTimerCount()).toBe(0)
  })

  it('replaces an in-flight effect, clears the old timer and permits early dismissal', async () => {
    const state = start()
    state.play('old', 'slide')
    await vi.advanceTimersByTimeAsync(800)
    state.play('new', 'halo')
    expect(state.effect.value).toBe('halo')
    expect(state.phase.value).toBe('waiting')
    expect(vi.getTimerCount()).toBe(1)
    state.dismiss()
    await vi.advanceTimersByTimeAsync(250)
    expect(state.message.value).toBeNull()
    await vi.advanceTimersByTimeAsync(5000)
    expect(state.message.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('falls back for old or unknown effects and clears effect state on disposal', () => {
    const state = start()
    state.play('old API')
    expect(state.effect.value).toBe('capsule')
    state.play('future API', 'future' as never)
    expect(state.effect.value).toBe('capsule')
    state.play('current', 'ribbon')
    scope.stop()
    expect(state.effect.value).toBe('capsule')
    expect(state.message.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })
})
