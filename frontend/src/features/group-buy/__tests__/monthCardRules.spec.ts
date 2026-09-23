import { beforeEach, describe, expect, it, vi } from 'vitest'
import { monthCardRulesAPI } from '@/api/monthCardRules'
import { useAdminMonthCardRules, useMonthCardRules } from '../useMonthCardRules'
import type { MonthCardRules } from '@/types/monthCardRules'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/monthCardRules', () => ({ monthCardRulesAPI: { get: vi.fn(), read: vi.fn(), admin: vi.fn(), save: vi.fn(), publish: vi.fn() } }))
const rules = (read = false, publication = 1): MonthCardRules => ({ publication, documents: [
  { id: 'a', title: '拼团规则', content: '正文', version: `a${publication}`, read_at: read ? '2026-09-23T00:00:00Z' : null },
  { id: 'b', title: '购买须知', content: '须知', version: 'b1', read_at: read ? '2026-09-23T00:00:00Z' : null }
] })
beforeEach(() => vi.resetAllMocks())
describe('month-card account rule state', () => {
  it('requires all documents and an explicit checkbox even on repeat purchases', async () => {
    const state = useMonthCardRules()
    vi.mocked(monthCardRulesAPI.get).mockResolvedValue(rules())
    await state.load()
    state.setAccepted(true)
    expect(state.canPay.value).toBe(false)
    const partial = rules(); partial.documents[0].read_at = '2026-09-23'
    vi.mocked(monthCardRulesAPI.get).mockResolvedValue(partial)
    await state.read(state.documents.value[0])
    state.setAccepted(true)
    expect(state.canPay.value).toBe(false)
    vi.mocked(monthCardRulesAPI.get).mockResolvedValue(rules(true))
    await state.read(state.documents.value[1])
    expect(state.ready.value).toBe(true)
    expect(state.canPay.value).toBe(false)
    state.setAccepted(true)
    expect(state.canPay.value).toBe(true)
    await state.load()
    expect(state.ready.value).toBe(true)
    expect(state.canPay.value).toBe(false)
  })
  it('keeps rules unread when saving read state fails and fails closed on load failure', async () => {
    const state = useMonthCardRules()
    vi.mocked(monthCardRulesAPI.get).mockResolvedValue(rules())
    await state.load()
    vi.mocked(monthCardRulesAPI.read).mockRejectedValue(new Error('offline'))
    await state.read(state.documents.value[0])
    expect(state.documents.value[0].read_at).toBeNull()
    expect(state.error.value).toBeTruthy()
    state.setAccepted(true)
    expect(state.canPay.value).toBe(false)
    vi.mocked(monthCardRulesAPI.get).mockRejectedValue(new Error('offline'))
    await state.load()
    expect(state.documents.value).toEqual([])
    expect(state.canPay.value).toBe(false)
  })
  it('clears consent after a newer publication and ignores stale checkout responses', async () => {
    const state = useMonthCardRules()
    vi.mocked(monthCardRulesAPI.get).mockResolvedValue(rules(true))
    await state.load(); state.setAccepted(true)
    vi.mocked(monthCardRulesAPI.get).mockResolvedValue(rules(false, 2))
    await state.read(state.documents.value[0])
    expect(state.accepted.value).toBe(false)
    let resolve!: (data: MonthCardRules) => void
    vi.mocked(monthCardRulesAPI.get).mockReturnValueOnce(new Promise(r => { resolve = r }))
    const pending = state.load()
    state.reset()
    resolve(rules(true))
    await pending
    expect(state.state.value).toBeNull()
    expect(state.canPay.value).toBe(false)
  })
})
it('keeps administrator draft save separate from publishing', async () => {
  const current = { draft_revision: 1, publication: 1, documents: [{ id: 'a', title: 'a', content: 'a', active: true }], published_documents: rules().documents, published_at: '2026-09-23' }
  vi.mocked(monthCardRulesAPI.admin).mockResolvedValue(current)
  vi.mocked(monthCardRulesAPI.save).mockResolvedValue({ ...current, draft_revision: 2 })
  vi.mocked(monthCardRulesAPI.publish).mockResolvedValue({ ...current, draft_revision: 3, publication: 2 })
  const state = useAdminMonthCardRules()
  await state.load()
  await state.save(current.documents)
  expect(monthCardRulesAPI.publish).not.toHaveBeenCalled()
  await state.publish()
  expect(monthCardRulesAPI.publish).toHaveBeenCalledWith(2)
  expect(state.state.value?.publication).toBe(2)
})
