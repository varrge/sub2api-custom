import { apiClient } from '@/api/client'

export interface SharedCreditState {
	name: string
	starts_at: string | null
	ends_at: string | null
	status: string
  enabled: boolean
  total_usd: string
  used_usd: string
  reserved_usd: string
  remaining_usd: string
  group_ids: number[]
  revision: number
  updated_at: string
}

export interface SharedCreditAllocation {
  id: number
  request_id: string
  user_id: number
  api_key_id: number
  group_id: number
  model: string
  amount_usd: string
  covered_cost_usd: string
  created_at: string
}

export interface SharedCreditUsageResponse {
  items: SharedCreditAllocation[]
  total: number
  page: number
}

export interface UpdateSharedCreditPayload {
	name: string
	starts_at: string | null
	ends_at: string | null
  enabled: boolean
  total_usd: string
  group_ids: number[]
  revision: number
}

export interface UpdateSharedCreditGroupPayload {
  enabled: boolean
  revision: number
}

export const getState = async (): Promise<SharedCreditState> => {
  const { data } = await apiClient.get<SharedCreditState>('/admin/settings/shared-credit')
  return data
}

export const updateState = async (payload: UpdateSharedCreditPayload): Promise<SharedCreditState> => {
  const { data } = await apiClient.put<SharedCreditState>('/admin/settings/shared-credit', payload)
  return data
}

export const getUsage = async (page: number = 1): Promise<SharedCreditUsageResponse> => {
  const { data } = await apiClient.get<SharedCreditUsageResponse>('/admin/settings/shared-credit/usage', {
    params: { page }
  })
  return data
}

export const setGroup = async (
  id: number,
  payload: UpdateSharedCreditGroupPayload
): Promise<SharedCreditState> => {
  const { data } = await apiClient.put<SharedCreditState>(`/admin/settings/shared-credit/groups/${id}`, payload)
  return data
}
