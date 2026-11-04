import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import VotingView from '../VotingView.vue'
import { adminVotingAPI, type Poll } from '@/api/voting'

vi.mock('@/api/voting', () => ({ adminVotingAPI: { list: vi.fn(), get: vi.fn(), create: vi.fn(), update: vi.fn(), delete: vi.fn() } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))
const poll: Poll = { id: 1, title: '投票', description: '', image: '', status: 'open', revision: 3, voter_count: 0, has_voted: false, created_at: '', updated_at: '', my_option_ids: [], options: [{ id: 1, title: 'A', image: '', votes: 0 }, { id: 2, title: 'B', image: '', votes: 0 }] }
let view: VueWrapper
const manager = () => view.findComponent({ name: 'VotingManager' })
beforeEach(async () => {
  vi.clearAllMocks()
  vi.mocked(adminVotingAPI.list).mockResolvedValue({ items: [poll], total: 1, page: 1, pages: 1, page_size: 12 })
  vi.mocked(adminVotingAPI.get).mockResolvedValue(poll)
  vi.mocked(adminVotingAPI.delete).mockResolvedValue(undefined)
  view = mount(VotingView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, VotingManager: true, ConfirmDialog: true } } })
  await flushPromises()
})
afterEach(() => view.unmount())

describe('voting management flow', () => {
  it('does not reopen the editor after deleting a poll with a pending detail request', async () => {
    let finish!: (poll: Poll) => void
    vi.mocked(adminVotingAPI.get).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    manager().vm.$emit('edit', 1)
    manager().vm.$emit('delete', 1)
    view.findComponent({ name: 'ConfirmDialog' }).vm.$emit('confirm')
    vi.mocked(adminVotingAPI.list).mockResolvedValue({ items: [], total: 0, page: 1, pages: 0, page_size: 12 })
    await flushPromises()
    finish(poll)
    await flushPromises()
    expect(adminVotingAPI.delete).toHaveBeenCalledWith(1, 3)
    expect(manager().props('formOpen')).toBe(false)
    expect(manager().props('detailLoading')).toBe(false)
    expect(manager().props('poll')).toBeNull()
  })

  it('keeps edited images and option titles in the save payload', async () => {
    manager().vm.$emit('edit', 1)
    await flushPromises()
    expect(manager().props('form')).toMatchObject({ revision: 3, options: [{ title: 'A', image: '' }, { title: 'B', image: '' }] })
    const updated = { ...manager().props('form'), image: 'data:image/png;base64,test', status: 'closed' }
    manager().vm.$emit('update:form', updated)
    vi.mocked(adminVotingAPI.update).mockResolvedValue({ id: 1 })
    manager().vm.$emit('save')
    await flushPromises()
    expect(adminVotingAPI.update).toHaveBeenCalledWith(1, updated)
    expect(manager().props('formOpen')).toBe(false)
  })
})
