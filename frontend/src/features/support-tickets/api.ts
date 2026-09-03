import { apiClient } from '@/api/client'
import type {
  SupportTicket,
  SupportTicketCreateInput,
  SupportTicketListFilters,
  SupportTicketMessage,
  SupportTicketPage,
  SupportTicketPriority,
  SupportTicketReplyInput,
  SupportTicketStatus,
} from './types'

function cleanListFilters(
  filters: SupportTicketListFilters,
  includeUserSearch: boolean,
): Record<string, string | number> {
  const params: Record<string, string | number> = {
    page: filters.page,
    page_size: filters.page_size,
  }
  const title = filters.title?.trim()
  const userSearch = filters.user_search?.trim()
  if (title) params.title = title
  if (filters.category) params.category = filters.category
  if (filters.status) params.status = filters.status
  if (filters.priority) params.priority = filters.priority
  if (includeUserSearch && userSearch) params.user_search = userSearch
  return params
}

function messageForm(input: SupportTicketReplyInput): FormData {
  const form = new FormData()
  form.append('content', input.content.trim())
  for (const image of input.images) form.append('images', image)
  return form
}

function createForm(input: SupportTicketCreateInput): FormData {
  const form = messageForm(input)
  form.append('title', input.title.trim())
  form.append('category', input.category)
  form.append('priority', input.priority)
  return form
}

function client(base: '/tickets' | '/admin/tickets') {
  return {
    async list(filters: SupportTicketListFilters): Promise<SupportTicketPage> {
      const { data } = await apiClient.get<SupportTicketPage>(base, {
        params: cleanListFilters(filters, base === '/admin/tickets'),
      })
      return data
    },
    async unreadCount(): Promise<number> {
      const { data } = await apiClient.get<{ count: number }>(`${base}/unread-count`)
      return data.count
    },
    async get(id: number): Promise<SupportTicket> {
      const { data } = await apiClient.get<SupportTicket>(`${base}/${id}`)
      return data
    },
    async reply(id: number, input: SupportTicketReplyInput): Promise<SupportTicketMessage> {
      const { data } = await apiClient.post<SupportTicketMessage>(
        `${base}/${id}/replies`,
        messageForm(input),
      )
      return data
    },
    async markRead(id: number, lastReadMessageID: number): Promise<void> {
      await apiClient.post(`${base}/${id}/read`, { last_read_message_id: lastReadMessageID })
    },
    async attachment(id: number, attachmentID: number): Promise<Blob> {
      const { data } = await apiClient.get<Blob>(`${base}/${id}/attachments/${attachmentID}`, {
        responseType: 'blob',
      })
      return data
    },
  }
}

const userClient = client('/tickets')
const adminClient = client('/admin/tickets')

export const supportTicketsUserAPI = {
  ...userClient,
  async create(input: SupportTicketCreateInput): Promise<SupportTicket> {
    const { data } = await apiClient.post<SupportTicket>('/tickets', createForm(input))
    return data
  },
}

export const supportTicketsAdminAPI = {
  ...adminClient,
  async updateStatus(id: number, status: SupportTicketStatus): Promise<SupportTicket> {
    const { data } = await apiClient.patch<SupportTicket>(`/admin/tickets/${id}/status`, { status })
    return data
  },
  async updatePriority(id: number, priority: SupportTicketPriority): Promise<SupportTicket> {
    const { data } = await apiClient.patch<SupportTicket>(`/admin/tickets/${id}/priority`, { priority })
    return data
  },
}

export function isSupportTicketFeatureDisabled(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const value = error as { code?: unknown; reason?: unknown }
  return value.code === 'FEATURE_DISABLED' || value.reason === 'FEATURE_DISABLED'
}
