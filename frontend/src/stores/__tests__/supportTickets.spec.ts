import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  userUnread: vi.fn(),
  adminUnread: vi.fn(),
}))

vi.mock('@/features/support-tickets/api', () => ({
  supportTicketsUserAPI: { unreadCount: api.userUnread },
  supportTicketsAdminAPI: { unreadCount: api.adminUnread },
}))

import { useSupportTicketStore } from '../supportTickets'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

describe('support ticket unread state', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads and keeps user/admin unread ticket counts separately', async () => {
    api.userUnread.mockResolvedValue(2)
    api.adminUnread.mockResolvedValue(7)
    const store = useSupportTicketStore()

    await store.refreshUserUnread()
    expect(store.userUnreadCount).toBe(2)
    expect(store.userUnreadLoaded).toBe(true)
    expect(store.adminUnreadCount).toBe(0)
    expect(store.adminUnreadLoaded).toBe(false)

    await store.refreshAdminUnread()
    expect(store.adminUnreadCount).toBe(7)
    expect(store.adminUnreadLoaded).toBe(true)
  })

  it('resets both identities and does not mark failed requests as loaded', async () => {
    api.userUnread.mockRejectedValue(new Error('offline'))
    const store = useSupportTicketStore()

    await expect(store.refreshUserUnread()).rejects.toThrow('offline')
    expect(store.userUnreadLoaded).toBe(false)
    store.adminUnreadCount = 4
    store.adminUnreadLoaded = true

    store.reset()
    expect(store.userUnreadCount).toBe(0)
    expect(store.adminUnreadCount).toBe(0)
    expect(store.userUnreadLoaded).toBe(false)
    expect(store.adminUnreadLoaded).toBe(false)
  })

  it('ignores a pre-reset response and initializes the next session independently', async () => {
    const oldSession = deferred<number>()
    const newSession = deferred<number>()
    api.userUnread
      .mockReturnValueOnce(oldSession.promise)
      .mockReturnValueOnce(newSession.promise)
    const store = useSupportTicketStore()

    const staleRequest = store.initializeUserUnread()
    store.reset()
    const currentRequest = store.initializeUserUnread()
    expect(api.userUnread).toHaveBeenCalledTimes(2)

    oldSession.resolve(8)
    await staleRequest
    expect(store.userUnreadCount).toBe(0)
    expect(store.userUnreadLoaded).toBe(false)

    newSession.resolve(3)
    await currentRequest
    expect(store.userUnreadCount).toBe(3)
    expect(store.userUnreadLoaded).toBe(true)
  })

  it('coalesces concurrent initialization requests', async () => {
    const response = deferred<number>()
    api.adminUnread.mockReturnValueOnce(response.promise)
    const store = useSupportTicketStore()

    const first = store.initializeAdminUnread()
    const second = store.initializeAdminUnread()
    expect(api.adminUnread).toHaveBeenCalledOnce()

    response.resolve(6)
    await expect(Promise.all([first, second])).resolves.toEqual([6, 6])
    expect(store.adminUnreadCount).toBe(6)
    expect(store.adminUnreadLoaded).toBe(true)
  })

  it('does not let an older initialization overwrite a newer explicit refresh', async () => {
    const older = deferred<number>()
    const newer = deferred<number>()
    api.userUnread
      .mockReturnValueOnce(older.promise)
      .mockReturnValueOnce(newer.promise)
    const store = useSupportTicketStore()

    const initialization = store.initializeUserUnread()
    const refresh = store.refreshUserUnread()
    newer.resolve(9)
    await refresh
    expect(store.userUnreadCount).toBe(9)

    older.resolve(2)
    await initialization
    expect(store.userUnreadCount).toBe(9)
    expect(store.userUnreadLoaded).toBe(true)
  })
})
