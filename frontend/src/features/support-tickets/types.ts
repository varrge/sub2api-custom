export const supportTicketCategories = ['account', 'billing', 'feature', 'other'] as const
export const supportTicketPriorities = ['low', 'normal', 'high', 'urgent'] as const
export const supportTicketStatuses = ['pending', 'in_progress', 'closed'] as const

export type SupportTicketCategory = (typeof supportTicketCategories)[number]
export type SupportTicketPriority = (typeof supportTicketPriorities)[number]
export type SupportTicketStatus = (typeof supportTicketStatuses)[number]
export type SupportTicketAuthorRole = 'user' | 'admin'

export interface SupportTicketIdentity {
  id: number
  username: string
  email?: string
}

export interface SupportTicketAttachment {
  id: number
  content_type: string
  size: number
  width: number | null
  height: number | null
}

export interface SupportTicketMessage {
  id: number
  author_role: SupportTicketAuthorRole
  author?: SupportTicketIdentity
  content: string
  created_at: string
  attachments: SupportTicketAttachment[]
}

export interface SupportTicket {
  id: number
  user_id: number
  user?: SupportTicketIdentity
  title: string
  category: SupportTicketCategory
  priority: SupportTicketPriority
  status: SupportTicketStatus
  unread: boolean
  created_at: string
  updated_at: string
  messages?: SupportTicketMessage[]
}

export interface SupportTicketPage {
  items: SupportTicket[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface SupportTicketListFilters {
  page: number
  page_size: number
  title?: string
  category?: SupportTicketCategory
  status?: SupportTicketStatus
  priority?: SupportTicketPriority
  user_search?: string
}

export interface SupportTicketCreateInput {
  title: string
  category: SupportTicketCategory
  priority: SupportTicketPriority
  content: string
  images: File[]
}

export interface SupportTicketReplyInput {
  content: string
  images: File[]
}

export const SUPPORT_TICKET_TITLE_MAX = 200
export const SUPPORT_TICKET_CONTENT_MAX = 10_000
export const SUPPORT_TICKET_IMAGE_MAX = 5 * 1024 * 1024
export const SUPPORT_TICKET_IMAGES_MAX = 3
export const SUPPORT_TICKET_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp'] as const

export function supportTicketCharacterCount(value: string): number {
  return [...value.trim()].length
}

export function supportTicketTextIsValid(value: string, max: number): boolean {
  const length = supportTicketCharacterCount(value)
  return length >= 1 && length <= max
}
