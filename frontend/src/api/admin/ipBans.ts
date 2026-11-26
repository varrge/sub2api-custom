/**
 * Permanent IP bans (admin only).
 *
 * Automatic rules (decoy accounts, sensitive probes, repeated failures against
 * the administrator) and manual bans never expire; they are lifted here.
 */
import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type IPBanReason =
  | 'honeypot_email'
  | 'sensitive_path_probe'
  | 'admin_login_failures'
  | 'admin_api_unauthorized'
  | 'manual'

export interface IPBan {
  id: number
  ip: string
  reason: IPBanReason | string
  source: 'auto' | 'manual' | 'backfill' | string
  detail: Record<string, unknown>
  note: string
  created_by: string
  created_at: string
  hits: number
  last_hit_at: string | null
  revoked_at: string | null
  revoked_by: string
  revoke_note: string
}

export interface IPBanQuery {
  page?: number
  page_size?: number
  status?: 'active' | 'revoked' | 'all'
  reason?: string
  q?: string
}

export interface IPBanSummary {
  active_total: number
  by_reason: Record<string, number>
  trusted_proxy_configured: boolean
  enabled: boolean
}

export interface IPBanSettings {
  enabled: boolean
  honeypot_emails: string[]
  ban_sensitive_probes: boolean
  admin_login_failures: number
  admin_login_window_minutes: number
  admin_unauthorized: number
  admin_unauthorized_window_minutes: number
  allowlist: string[]
  honeypot_conflicts?: string[]
}

const base = '/admin/ip-bans'

export async function list(params: IPBanQuery): Promise<PaginatedResponse<IPBan>> {
  const { data } = await apiClient.get<PaginatedResponse<IPBan>>(base, { params })
  return data
}

export async function summary(): Promise<IPBanSummary> {
  const { data } = await apiClient.get<IPBanSummary>(`${base}/summary`)
  return data
}

export async function create(ip: string, note: string): Promise<IPBan> {
  const { data } = await apiClient.post<IPBan>(base, { ip, note })
  return data
}

export async function revoke(id: number, note: string): Promise<void> {
  await apiClient.post(`${base}/${id}/revoke`, { note })
}

export async function getSettings(): Promise<IPBanSettings> {
  const { data } = await apiClient.get<IPBanSettings>(`${base}/settings`)
  return data
}

export async function saveSettings(settings: IPBanSettings): Promise<IPBanSettings> {
  const { honeypot_conflicts: _ignored, ...payload } = settings
  const { data } = await apiClient.put<IPBanSettings>(`${base}/settings`, payload)
  return data
}

export const ipBansAPI = { list, summary, create, revoke, getSettings, saveSettings }
export default ipBansAPI
