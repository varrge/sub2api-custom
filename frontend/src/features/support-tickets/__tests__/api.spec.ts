import { beforeEach, describe, expect, it, vi } from 'vitest'

const http = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
}))

vi.mock('@/api/client', () => ({ apiClient: http }))

import {
  isSupportTicketFeatureDisabled,
  supportTicketsAdminAPI,
  supportTicketsUserAPI,
} from '../api'

describe('support ticket API contract', () => {
  beforeEach(() => vi.clearAllMocks())

  it('uses the user list endpoint and sends only trimmed, populated filters', async () => {
    http.get.mockResolvedValue({
      data: { items: [], total: 0, page: 2, page_size: 20, pages: 1 },
    })

    await supportTicketsUserAPI.list({
      page: 2,
      page_size: 20,
      title: '  invoice  ',
      user_search: ' ignored for this client ',
      category: 'billing',
      status: 'pending',
      priority: 'high',
    })

    expect(http.get).toHaveBeenCalledWith('/tickets', {
      params: {
        page: 2,
        page_size: 20,
        title: 'invoice',
        category: 'billing',
        status: 'pending',
        priority: 'high',
      },
    })

    await supportTicketsAdminAPI.list({
      page: 1,
      page_size: 10,
      user_search: '  alice@example.com  ',
    })
    expect(http.get).toHaveBeenLastCalledWith('/admin/tickets', {
      params: { page: 1, page_size: 10, user_search: 'alice@example.com' },
    })
  })

  it('creates and replies with trimmed multipart text and repeated images', async () => {
    const first = new File(['one'], 'one.png', { type: 'image/png' })
    const second = new File(['two'], 'two.webp', { type: 'image/webp' })
    http.post
      .mockResolvedValueOnce({ data: { id: 7 } })
      .mockResolvedValueOnce({ data: { id: 9 } })

    await supportTicketsUserAPI.create({
      title: '  Payment issue ',
      category: 'billing',
      priority: 'normal',
      content: '  Please help.\n ',
      images: [first, second],
    })
    await supportTicketsAdminAPI.reply(7, { content: '  Fixed. ', images: [first] })

    const createForm = http.post.mock.calls[0][1] as FormData
    expect(http.post.mock.calls[0][0]).toBe('/tickets')
    expect(createForm.get('title')).toBe('Payment issue')
    expect(createForm.get('content')).toBe('Please help.')
    expect(createForm.get('category')).toBe('billing')
    expect(createForm.get('priority')).toBe('normal')
    expect(createForm.getAll('images')).toEqual([first, second])

    const replyForm = http.post.mock.calls[1][1] as FormData
    expect(http.post.mock.calls[1][0]).toBe('/admin/tickets/7/replies')
    expect(replyForm.get('content')).toBe('Fixed.')
    expect(replyForm.getAll('images')).toEqual([first])
  })

  it('uses separate user/admin unread, detail, read, attachment, and admin update routes', async () => {
    http.get.mockResolvedValue({ data: { count: 4 } })
    http.post.mockResolvedValue({ data: {} })
    http.patch.mockResolvedValue({ data: { id: 3 } })

    await expect(supportTicketsUserAPI.unreadCount()).resolves.toBe(4)
    await supportTicketsAdminAPI.get(3)
    await supportTicketsUserAPI.markRead(3, 11)
    await supportTicketsAdminAPI.attachment(3, 8)
    await supportTicketsAdminAPI.updateStatus(3, 'closed')
    await supportTicketsAdminAPI.updatePriority(3, 'urgent')

    expect(http.get).toHaveBeenCalledWith('/tickets/unread-count')
    expect(http.get).toHaveBeenCalledWith('/admin/tickets/3')
    expect(http.post).toHaveBeenCalledWith('/tickets/3/read', { last_read_message_id: 11 })
    expect(http.get).toHaveBeenCalledWith('/admin/tickets/3/attachments/8', {
      responseType: 'blob',
    })
    expect(http.patch).toHaveBeenCalledWith('/admin/tickets/3/status', { status: 'closed' })
    expect(http.patch).toHaveBeenCalledWith('/admin/tickets/3/priority', { priority: 'urgent' })
  })

  it('recognizes the backend feature-disabled reason', () => {
    expect(isSupportTicketFeatureDisabled({ reason: 'FEATURE_DISABLED' })).toBe(true)
    expect(isSupportTicketFeatureDisabled({ code: 'FEATURE_DISABLED' })).toBe(true)
    expect(isSupportTicketFeatureDisabled({ status: 404 })).toBe(false)
  })
})
