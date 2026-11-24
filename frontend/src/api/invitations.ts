import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export interface InvitationLevel {
  id: string
  name: string
  spend_threshold: number
  lifetime_invite_limit: number
  unused_invite_limit: number
  rebate_rate: number
}

export interface InvitationSettings {
  enabled: boolean
  validity_days: number
  /** Hours a pending rebate waits before it is credited automatically; 0 = manual review only. */
  auto_approve_hours: number
  levels: InvitationLevel[]
}

export interface InvitationOverview {
  enabled: boolean
  validity_days: number
  auto_approve_hours: number
  level: InvitationLevel
  /** The whole ladder, lowest threshold first. */
  levels: InvitationLevel[]
  recharge_total: number
  next_level: InvitationLevel | null
  next_level_remaining: number
  override_level_id: string
  lifetime_used: number
  unused_count: number
  summary: { pending: number; paid: number; rejected_or_revoked: number }
}

export type InviteStatus = 'unused' | 'used' | 'expired' | 'disabled'

export interface InvitationCode {
  id: number
  code: string
  inviter_id: number
  inviter_email: string
  invitee_id: number | null
  invitee_email: string
  status: InviteStatus
  created_at: string
  expires_at: string
  used_at: string | null
}

export type RebateStatus = 'pending' | 'approved' | 'rejected' | 'revoked'
export type ReviewAction = 'approve' | 'reject' | 'revoke'

export interface InvitationRebate {
  id: number
  invite_id: number
  invite_code: string
  inviter_id: number
  inviter_email: string
  invitee_id: number | null
  invitee_email: string
  source_type: 'payment' | 'redeem_code'
  source_id: number
  source_currency: string
  /** Admin view only: current status of the first payment order, '' for a redeemed card code. */
  source_status: string
  source_occurred_at: string
  first_amount: number
  rebate_rate: number
  rebate_amount: number
  level_name: string
  status: RebateStatus
  review_reason: string
  /** Credited by the waiting period instead of an administrator. */
  auto_approved: boolean
  created_at: string
  reviewed_at: string | null
}

export interface InvitationOverride {
  user_id: number
  email: string
  level_id: string
  updated_at: string
}

export interface InvitationListParams {
  page: number
  page_size: number
  /** Admin only: restrict to one inviter. */
  user_id?: number
  status?: InviteStatus | RebateStatus | ''
  /** Matches the code; the admin view also matches inviter/invitee email. */
  q?: string
}

export interface InvitationStats {
  total_invites: number
  used_invites: number
  active_invites: number
  pending_count: number
  pending_amount: number
  approved_count: number
  approved_amount: number
  declined_count: number
  declined_amount: number
  overridden_users: number
}

export const invitationsAPI = {
  async overview(): Promise<InvitationOverview> {
    return (await apiClient.get<InvitationOverview>('/user/invitations')).data
  },
  async apply(requestId: string): Promise<InvitationCode> {
    return (await apiClient.post<InvitationCode>('/user/invitations/codes', { request_id: requestId })).data
  },
  async codes(params: InvitationListParams, admin = false): Promise<PaginatedResponse<InvitationCode>> {
    return (await apiClient.get<PaginatedResponse<InvitationCode>>(`/${admin ? 'admin' : 'user'}/invitations/codes`, { params })).data
  },
  async rebates(params: InvitationListParams, admin = false): Promise<PaginatedResponse<InvitationRebate>> {
    return (await apiClient.get<PaginatedResponse<InvitationRebate>>(`/${admin ? 'admin' : 'user'}/invitations/rebates`, { params })).data
  },
  /** Admin: void an unused code. The slot returns to the inviter's quota. */
  async disableCode(id: number): Promise<InvitationCode> {
    return (await apiClient.post<InvitationCode>(`/admin/invitations/codes/${id}/disable`)).data
  },
  /** Admin: users whose level was set by hand, most recent change first. */
  async overrides(params: { page: number; page_size: number }): Promise<PaginatedResponse<InvitationOverride>> {
    return (await apiClient.get<PaginatedResponse<InvitationOverride>>('/admin/invitations/overrides', { params })).data
  },
  async stats(): Promise<InvitationStats> {
    return (await apiClient.get<InvitationStats>('/admin/invitations/stats')).data
  },
  async settings(): Promise<InvitationSettings> {
    return (await apiClient.get<InvitationSettings>('/admin/invitations/settings')).data
  },
  async saveSettings(settings: InvitationSettings): Promise<InvitationSettings> {
    return (await apiClient.put<InvitationSettings>('/admin/invitations/settings', settings)).data
  },
  async userOverview(userId: number): Promise<InvitationOverview> {
    return (await apiClient.get<InvitationOverview>(`/admin/invitations/users/${userId}`)).data
  },
  async setLevel(userId: number, levelId: string): Promise<InvitationOverview | null> {
    return (await apiClient.put<InvitationOverview | null>(`/admin/invitations/users/${userId}/level`, { level_id: levelId })).data
  },
  async review(id: number, action: ReviewAction, reason: string): Promise<InvitationRebate> {
    return (await apiClient.post<InvitationRebate>(`/admin/invitations/rebates/${id}/${action}`, { reason })).data
  }
}
