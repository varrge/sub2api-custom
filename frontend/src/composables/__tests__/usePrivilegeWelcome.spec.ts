import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, reactive, type EffectScope } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { usePrivilegeWelcome } from '../usePrivilegeWelcome'
import type { UserPrivileges } from '@/api/privileges'

const { getUserPrivileges } = vi.hoisted(() => ({ getUserPrivileges: vi.fn() }))
vi.mock('@/api/privileges', () => ({ getUserPrivileges }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
const auth = reactive({ isAuthenticated: true, user: { id: 42, balance: 100, username: 'enterprise' } as { id: number; balance: number; username: string; account_type?: string; role?: string } | null })
let scope: EffectScope

function start() {
  scope = effectScope()
  return scope.run(usePrivilegeWelcome)!
}

describe('privilege welcome playback', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    getUserPrivileges.mockReset().mockResolvedValue({ welcome: { message: '欢迎尊贵的企业用户' } })
    auth.isAuthenticated = true
    auth.user = { id: 42, balance: 100, username: 'enterprise' }
  })
  afterEach(() => {
    scope?.stop()
    vi.useRealTimers()
  })

  it('plays the chosen timings once and does not replay on profile refresh', async () => {
    const state = start()
    await flushPromises()
    expect(state.message.value).toBe('欢迎尊贵的企业用户')
    expect(state.phase.value).toBe('waiting')
    await vi.advanceTimersByTimeAsync(350)
    expect(state.phase.value).toBe('entering')
    await vi.advanceTimersByTimeAsync(400)
    expect(state.phase.value).toBe('dwell')
    auth.user = { id: 42, balance: 80, username: 'updated' }
    await nextTick()
    expect(getUserPrivileges).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(3500)
    expect(state.phase.value).toBe('exiting')
    await vi.advanceTimersByTimeAsync(250)
    expect(state.message.value).toBeNull()
    await vi.advanceTimersByTimeAsync(60000)
    expect(getUserPrivileges).toHaveBeenCalledTimes(1)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('makes repeated close clicks idempotent', async () => {
    const state = start()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(750)
    state.dismiss()
    state.dismiss()
    await vi.advanceTimersByTimeAsync(250)
    expect(state.message.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('does not request or show a greeting while signed out', async () => {
    auth.isAuthenticated = false
    auth.user = null
    const state = start()
    await flushPromises()
    expect(getUserPrivileges).not.toHaveBeenCalled()
    expect(state.message.value).toBeNull()
  })

  it('uses server eligibility and quietly skips disabled/ordinary users or failures', async () => {
    getUserPrivileges.mockResolvedValueOnce({ welcome: null }).mockRejectedValueOnce(new Error('unavailable'))
    const state = start()
    await flushPromises()
    expect(state.message.value).toBeNull()
    auth.user!.id = 43
    await flushPromises()
    expect(state.message.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('aborts the previous request and ignores stale responses after switching accounts', async () => {
    let resolveOld!: (result: UserPrivileges) => void
    getUserPrivileges.mockReturnValueOnce(new Promise<UserPrivileges>(resolve => { resolveOld = resolve }))
    const state = start()
    const previousSignal = getUserPrivileges.mock.calls[0][0] as AbortSignal
    expect(previousSignal.aborted).toBe(false)
    auth.user!.id = 43
    await flushPromises()
    expect(getUserPrivileges).toHaveBeenCalledTimes(2)
    expect(previousSignal.aborted).toBe(true)
    expect((getUserPrivileges.mock.calls[1][0] as AbortSignal).aborted).toBe(false)
    resolveOld({ welcome: { message: 'old account' } })
    await flushPromises()
    expect(state.message.value).toBe('欢迎尊贵的企业用户')
  })

  it('cleans up on logout, allows a fresh login, and ignores responses after disposal', async () => {
    const state = start()
    await flushPromises()
    const firstSignal = getUserPrivileges.mock.calls[0][0] as AbortSignal
    auth.isAuthenticated = false
    auth.user = null
    await nextTick()
    expect(firstSignal.aborted).toBe(true)
    expect(state.message.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
    auth.user = { id: 42, balance: 100, username: 'enterprise' }
    auth.isAuthenticated = true
    await flushPromises()
    expect(getUserPrivileges).toHaveBeenCalledTimes(2)
    expect(state.message.value).toBeTruthy()
    scope.stop()
    expect((getUserPrivileges.mock.calls[1][0] as AbortSignal).aborted).toBe(true)
    expect(state.message.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('ignores an in-flight response after logout or unmount', async () => {
    let resolve!: (result: UserPrivileges) => void
    getUserPrivileges.mockReturnValueOnce(new Promise<UserPrivileges>(done => { resolve = done }))
    const state = start()
    const signal = getUserPrivileges.mock.calls[0][0] as AbortSignal
    scope.stop()
    expect(signal.aborted).toBe(true)
    resolve({ welcome: { message: 'late' } })
    await flushPromises()
    expect(state.message.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('displays a title independently of welcome and clears it on identity changes or logout', async () => {
    getUserPrivileges.mockResolvedValueOnce({ title: '资深伙伴', welcome: null })
    const state = start()
    await flushPromises()
    expect(state.title.value).toBe('资深伙伴')
    expect(state.message.value).toBeNull()
    auth.user!.account_type = 'enterprise'
    await flushPromises()
    expect(state.title.value).toBeNull()
    expect(state.message.value).toBe('欢迎尊贵的企业用户')
    auth.isAuthenticated = false
    auth.user = null
    await nextTick()
    expect(state.title.value).toBeNull()
    expect(state.message.value).toBeNull()
  })

  it('never assigns a stale title after switching users', async () => {
    let resolve!: (result: UserPrivileges) => void
    getUserPrivileges.mockReturnValueOnce(new Promise<UserPrivileges>(done => { resolve = done }))
    getUserPrivileges.mockResolvedValueOnce({ title: '新伙伴', welcome: { message: '欢迎新伙伴', effect: 'halo' } })
    const state = start()
    auth.user!.id = 99
    await flushPromises()
    resolve({ title: '旧伙伴', welcome: { message: '旧欢迎', effect: 'slide' } })
    await flushPromises()
    expect(state.title.value).toBe('新伙伴')
    expect(state.message.value).toBe('欢迎新伙伴')
    expect(state.effect.value).toBe('halo')
    scope.stop()
    expect(state.title.value).toBeNull()
    expect(state.effect.value).toBe('capsule')
  })
})
