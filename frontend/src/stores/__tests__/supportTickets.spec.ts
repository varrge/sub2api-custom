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
})
