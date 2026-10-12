import { apiClient } from './client'
import type { ApiKeyModelAllowlist } from '@/types'

export interface ModelAccessGroup {
  id: number
  name: string
  platform: string
}

export interface ModelAccessKey {
  id: number
  name: string
  status: string
  expires_at: string | null
  groups: ModelAccessGroup[]
  group_ids: number[]
  model_allowlist: ApiKeyModelAllowlist
  model_group_routes?: Record<string, number[]>
  revision: string
}

export interface ModelAccessRow {
  id: number
  name: string
  status: string
  expires_at: string | null
  groups: ModelAccessGroup[]
  group_ids: number[]
  allowed: boolean
  originalAllowed: boolean
  selected_group_ids: number[]
  original_selected_group_ids: number[]
  groupsModified: boolean
  modified: boolean
  catalogState: 'listed' | 'unlisted' | 'unknown'
}

export interface ModelAccessEntry {
  id: number
  revision: string
  allowed: boolean
  group_ids?: number[]
}

export const modelKeyAccessAPI = {
  async list(signal?: AbortSignal): Promise<{ keys: ModelAccessKey[] }> {
    const { data } = await apiClient.get<{ keys: ModelAccessKey[] }>('/keys/model-access', { signal })
    return data
  },
  async update(model: string, entries: ModelAccessEntry[]): Promise<{ keys: ModelAccessKey[]; updated_count: number }> {
    const { data } = await apiClient.put<{ keys: ModelAccessKey[]; updated_count: number }>('/keys/model-access', { model, entries })
    return data
  }
}
