import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import VotingPopover from '../VotingPopover.vue'
import { votingAPI, type Poll } from '@/api/voting'

vi.mock('@/api/voting', () => ({ votingAPI: { list: vi.fn(), get: vi.fn(), vote: vi.fn() } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: { voting_enabled: true }, showSuccess: vi.fn(), showWarning: vi.fn() }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 1 } }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ fullPath: '/dashboard' }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))

const fixture = (): Poll => ({ id: 1, title: '投票', description: '', image: '', status: 'open', revision: 5, voter_count: 0, has_voted: false, created_at: '', updated_at: '', my_option_ids: [], options: [{ id: 1, title: 'A', image: '', votes: 0 }, { id: 2, title: 'B', image: '', votes: 0 }] })
let view: VueWrapper
function board() { return view.findComponent({ name: 'VotingBoard' }) }
beforeEach(async () => {
  vi.clearAllMocks()
  localStorage.setItem('auth_user', JSON.stringify({ id: 1 }))
  localStorage.setItem('auth_token', 'test-session')
  vi.mocked(votingAPI.list).mockResolvedValue({ items: [fixture()], total: 1, page: 1, pages: 1, page_size: 12 })
  vi.mocked(votingAPI.get).mockImplementation(async () => fixture())
  view = mount(VotingPopover, { attachTo: document.body, global: { stubs: { teleport: true, VotingBoard: true } } })
  await view.get('[data-testid="header-voting"]').trigger('click')
  await flushPromises()
})
afterEach(() => view.unmount())

describe('voting submission flow', () => {
  it('keeps keyboard focus inside the panel while submitting and after showing results', async () => {
    view.unmount()
    view = mount(VotingPopover, { attachTo: document.body })
    await view.get('[data-testid="header-voting"]').trigger('click')
    await flushPromises()
    let finish!: () => void
    vi.mocked(votingAPI.vote).mockImplementation(() => new Promise<void>(resolve => { finish = resolve }))
    await board().get('[data-testid="vote-option-1"]').setValue(true)
    await board().get('[data-testid="vote-submit"]').trigger('click')
    board().get<HTMLButtonElement>('[data-testid="vote-confirm"]').element.focus()
    await board().get('[data-testid="vote-confirm"]').trigger('click')
    await flushPromises()
    const panel = document.querySelector('[data-testid="voting-popover"]')
    expect(document.activeElement).toBe(panel)
    vi.mocked(votingAPI.get).mockResolvedValue({ ...fixture(), has_voted: true, my_option_ids: [1] })
    finish()
    await flushPromises()
    expect(document.activeElement).toBe(panel)
    expect(board().get<HTMLInputElement>('[data-testid="vote-option-1"]').element.disabled).toBe(true)
  })

  it('closes and discards the ballot after another tab changes account', async () => {
    board().vm.$emit('update:selection', [1])
    localStorage.setItem('auth_user', JSON.stringify({ id: 2 }))
    window.dispatchEvent(new StorageEvent('storage', { key: 'auth_user' }))
    await flushPromises()
    expect(view.find('[role="dialog"]').exists()).toBe(false)
    await view.get('[data-testid="header-voting"]').trigger('click')
    expect(view.find('[role="dialog"]').exists()).toBe(false)
    expect(votingAPI.vote).not.toHaveBeenCalled()
  })

  it('checks session again before submitting even if its storage event has not arrived', async () => {
    board().vm.$emit('update:selection', [1])
    localStorage.setItem('auth_token', 'another-session')
    board().vm.$emit('submit')
    await flushPromises()
    expect(view.find('[role="dialog"]').exists()).toBe(false)
    expect(votingAPI.vote).not.toHaveBeenCalled()
  })

  it('loads only on open, closes on Escape and restores trigger focus', async () => {
    view.unmount()
    vi.mocked(votingAPI.list).mockClear()
    view = mount(VotingPopover, { attachTo: document.body, global: { stubs: { VotingBoard: true } } })
    expect(votingAPI.list).not.toHaveBeenCalled()
    expect(document.querySelector('[data-testid="voting-popover"]')).toBeNull()
    await view.get('[data-testid="header-voting"]').trigger('click')
    await flushPromises()
    expect(votingAPI.list).toHaveBeenCalledTimes(1)
    expect(document.activeElement).toBe(document.querySelector('[data-testid="voting-popover"]'))
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(document.querySelector('[data-testid="voting-popover"]')).toBeNull()
    expect(document.activeElement).toBe(view.get('[data-testid="header-voting"]').element)
  })

  it('ignores a late detail from a closed panel after reopening', async () => {
    let finish!: (value: Poll) => void
    let signal: AbortSignal | undefined
    vi.mocked(votingAPI.get).mockImplementationOnce((_id, currentSignal) => {
      signal = currentSignal
      return new Promise(resolve => { finish = resolve })
    })
    board().vm.$emit('select', 2)
    await flushPromises()
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    await flushPromises()
    expect(signal?.aborted).toBe(true)
    expect(view.find('[role="dialog"]').exists()).toBe(false)
    await view.get('[data-testid="header-voting"]').trigger('click')
    await flushPromises()
    finish({ ...fixture(), id: 2, title: 'Old detail' })
    await flushPromises()
    expect(board().props('poll').id).toBe(1)
  })

  it('retains the pending submission lock across close and reopen', async () => {
    let finish!: () => void
    vi.mocked(votingAPI.vote).mockImplementation(() => new Promise<void>(resolve => { finish = resolve }))
    board().vm.$emit('update:selection', [1])
    board().vm.$emit('submit')
    await flushPromises()
    await view.get('[data-testid="voting-close"]').trigger('click')
    await view.get('[data-testid="header-voting"]').trigger('click')
    expect(board().props('submitting')).toBe(true)
    board().vm.$emit('submit')
    expect(votingAPI.vote).toHaveBeenCalledTimes(1)
    vi.mocked(votingAPI.get).mockResolvedValue({ ...fixture(), has_voted: true, my_option_ids: [1] })
    finish()
    await flushPromises()
    expect(board().props('poll').has_voted).toBe(true)
  })

  it('returns to a valid page when the last poll on a later page is deleted', async () => {
    vi.mocked(votingAPI.list).mockResolvedValueOnce({ items: [fixture()], total: 13, page: 1, pages: 2, page_size: 12 })
    board().vm.$emit('refresh')
    await flushPromises()
    vi.mocked(votingAPI.list).mockResolvedValueOnce({ items: [], total: 1, page: 2, pages: 1, page_size: 12 })
    board().vm.$emit('page-change', 2)
    await flushPromises()
    expect(board().props('page')).toBe(1)
    expect(board().props('polls')).toHaveLength(1)
    expect(board().props('loading')).toBe(false)
  })

  it('sends selected options and displayed revision once even on repeated submission', async () => {
    let finish!: () => void
    vi.mocked(votingAPI.vote).mockImplementation(() => new Promise<void>(resolve => { finish = resolve }))
    board().vm.$emit('update:selection', [1, 2])
    board().vm.$emit('submit')
    board().vm.$emit('submit')
    await flushPromises()
    expect(votingAPI.vote).toHaveBeenCalledTimes(1)
    expect(votingAPI.vote).toHaveBeenCalledWith(1, 5, [1, 2])
    const voted = { ...fixture(), has_voted: true, my_option_ids: [1, 2], voter_count: 1 }
    vi.mocked(votingAPI.get).mockResolvedValue(voted)
    finish()
    await flushPromises()
    expect(board().props('poll')).toMatchObject({ has_voted: true })
    board().vm.$emit('submit')
    expect(votingAPI.vote).toHaveBeenCalledTimes(1)
  })

  it('recovers the recorded ballot after a lost response without letting the user modify it', async () => {
    vi.mocked(votingAPI.vote).mockRejectedValue({ message: 'network failed' })
    vi.mocked(votingAPI.get).mockResolvedValue({ ...fixture(), has_voted: true, my_option_ids: [2] })
    board().vm.$emit('update:selection', [2])
    board().vm.$emit('submit')
    await flushPromises()
    expect(board().props('poll')).toMatchObject({ has_voted: true, my_option_ids: [2] })
    expect(board().props('error')).toBe('network failed')
  })
})
