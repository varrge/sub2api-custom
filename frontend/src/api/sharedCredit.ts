import { apiClient } from '@/api/client'

export interface UserSharedCreditSummary {
  visible: boolean
  server_time: string
  name?: string
  status?: 'scheduled' | 'active' | 'exhausted'
  total_usd?: string
  remaining_usd?: string
  starts_at?: string | null
  ends_at?: string | null
  needs_recharge?: boolean
  groups?: { id: number; name: string }[]
}

export async function getSharedCreditSummary(signal?: AbortSignal): Promise<UserSharedCreditSummary> {
  const { data } = await apiClient.get<UserSharedCreditSummary>('/user/shared-credit', { signal })
  return data
}
