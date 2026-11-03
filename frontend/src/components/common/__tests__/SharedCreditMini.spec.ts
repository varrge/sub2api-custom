import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import SharedCreditMini from '../SharedCreditMini.vue'
import { getSharedCreditSummary, type UserSharedCreditSummary } from '@/api/sharedCredit'

vi.mock('@/api/sharedCredit', () => ({ getSharedCreditSummary: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }) }))

const api = vi.mocked(getSharedCreditSummary)
let wrapper: VueWrapper | undefined
const fixture = (overrides: Partial<UserSharedCreditSummary> = {}): UserSharedCreditSummary => ({
  visible: true, name: '双十一共享', status: 'active',
  server_time: '2026-11-03T10:00:00Z', starts_at: null, ends_at: '2026-11-03T12:00:00Z',
  total_usd: '1000', remaining_usd: '723.75000001', needs_recharge: false,
  groups: [{ id: 1, name: '已授权分组' }], ...overrides
})
async function render() {
  wrapper = mount(SharedCreditMini, { props: { userId: 1 }, attachTo: document.body, global: { stubs: { Teleport: true } } })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-11-03T10:00:00Z'))
  api.mockReset().mockResolvedValue(fixture())
})
afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  vi.useRealTimers()
  vi.restoreAllMocks()
  document.body.innerHTML = ''
})

describe('SharedCreditMini', () => {
  it('shows site credit and opens authorized groups, dates and the debt notice', async () => {
    api.mockResolvedValue(fixture({ needs_recharge: true }))
    const view = await render()
    expect(view.text()).toContain('$723.75')
    expect(view.find('[title="$723.75000001 USD"]').exists()).toBe(true)
    await view.get('[data-testid="shared-credit-trigger"]').trigger('click')
    await flushPromises()
    expect(view.text()).toContain('已授权分组')
    expect(view.text()).toContain('sharedCredit.activity.balanceNotice')
    expect(view.text()).toContain('sharedCredit.header.currencyNotice')
    expect(api).toHaveBeenCalledTimes(2)
    await view.get('[role="region"]').trigger('keydown', { key: 'Escape' })
    expect(view.find('[role="region"]').exists()).toBe(false)
    expect(document.activeElement).toBe(view.get('[data-testid="shared-credit-trigger"]').element)
  })

  it('does not render inactive activities or invent a zero on initial failure', async () => {
    api.mockRejectedValue(new Error('offline'))
    const view = await render()
    expect(view.find('button').exists()).toBe(false)
    api.mockResolvedValue({ visible: false, server_time: fixture().server_time })
    await vi.advanceTimersByTimeAsync(60_000)
    expect(view.find('button').exists()).toBe(false)
  })

  it('marks a failed refresh as unknown and recovers on retry', async () => {
    const view = await render()
    api.mockRejectedValue(new Error('offline'))
    await view.get('[data-testid="shared-credit-trigger"]').trigger('click')
    await flushPromises()
    expect(view.text()).toContain('$--')
    expect(view.text()).not.toContain('$723.75')
    expect(view.text()).toContain('sharedCredit.header.loadFailed')
    api.mockResolvedValue(fixture({ remaining_usd: '0.00000001' }))
    const retry = view.findAll('button').find(button => button.text() === 'sharedCredit.header.retry')!
    await retry.trigger('click')
    await flushPromises()
    expect(view.text()).toContain('<$0.01')
    expect(view.text()).not.toContain('sharedCredit.header.loadFailed')
  })

  it('hides at the server expiry despite the client clock being wrong', async () => {
    vi.setSystemTime(new Date('2020-01-01T00:00:00Z'))
    api.mockResolvedValue(fixture({ ends_at: '2026-11-03T10:00:02Z' }))
    const view = await render()
    expect(view.find('button').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(2000)
    expect(view.find('button').exists()).toBe(false)
  })

  it('changes scheduled status when the activity starts and preserves exhausted visibility', async () => {
    api.mockResolvedValue(fixture({ status: 'scheduled', starts_at: '2026-11-03T10:00:02Z' }))
    const view = await render()
    expect(view.text()).toContain('sharedCredit.header.scheduled')
    await vi.advanceTimersByTimeAsync(2000)
    expect(view.text()).not.toContain('sharedCredit.header.scheduled')
    api.mockResolvedValue(fixture({ status: 'exhausted', remaining_usd: '0' }))
    await vi.advanceTimersByTimeAsync(58_000)
    expect(view.text()).toContain('sharedCredit.header.exhausted')
    expect(view.text()).toContain('$0.00')
  })

  it('pauses background requests, refreshes on return, and does not overlap requests', async () => {
    await render()
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api).toHaveBeenCalledTimes(1)
    let resolve!: (value: UserSharedCreditSummary) => void
    api.mockImplementation(() => new Promise(done => { resolve = done }))
    visibility.mockReturnValue('visible')
    document.dispatchEvent(new Event('visibilitychange'))
    window.dispatchEvent(new Event('focus'))
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api).toHaveBeenCalledTimes(2)
    resolve(fixture())
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api).toHaveBeenCalledTimes(3)
  })

  it('aborts on logout or account replacement and ignores a late response', async () => {
    let resolve!: (value: UserSharedCreditSummary) => void
    api.mockImplementation(() => new Promise(done => { resolve = done }))
    await render()
    const signal = api.mock.calls[0][0]!
    wrapper!.unmount()
    wrapper = undefined
    expect(signal.aborted).toBe(true)
    api.mockResolvedValue({ visible: false, server_time: fixture().server_time })
    const newAccount = await render()
    resolve(fixture())
    await flushPromises()
    expect(newAccount.find('button').exists()).toBe(false)
  })

  it('clears the summary when another tab switches accounts', async () => {
    const view = await render()
    window.dispatchEvent(new StorageEvent('storage', {
      key: 'auth_user', oldValue: '{"id":1,"balance":1}', newValue: '{"id":1,"balance":2}'
    }))
    await flushPromises()
    expect(view.find('button').exists()).toBe(true)
    window.dispatchEvent(new StorageEvent('storage', {
      key: 'auth_user', oldValue: '{"id":1}', newValue: '{"id":2}'
    }))
    await flushPromises()
    expect(view.find('button').exists()).toBe(false)
    window.dispatchEvent(new Event('focus'))
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api).toHaveBeenCalledTimes(1)
  })

  it.each([null, '{"id":2}'])('resumes when another tab returns from %s to the same account', async (other) => {
    const view = await render()
    let resolveOld!: (value: UserSharedCreditSummary) => void
    api.mockImplementationOnce(() => new Promise(done => { resolveOld = done }))
    window.dispatchEvent(new Event('focus'))
    const oldSignal = api.mock.calls[1][0]!
    window.dispatchEvent(new StorageEvent('storage', { key: 'auth_user', oldValue: '{"id":1}', newValue: other }))
    await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    expect(view.find('button').exists()).toBe(false)
    api.mockResolvedValue(fixture({ remaining_usd: '55' }))
    window.dispatchEvent(new StorageEvent('storage', { key: 'auth_user', oldValue: other, newValue: '{"id":1}' }))
    await flushPromises()
    expect(view.text()).toContain('$55.00')
    resolveOld(fixture())
    await flushPromises()
    expect(view.text()).toContain('$55.00')
    expect(view.text()).not.toContain('$723.75')
  })
})
