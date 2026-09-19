/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey, ApiKeyModelAllowlist } from '@/types'

export interface UpdateApiKeyGroupResult {
  api_key: ApiKey
  auto_granted_group_access: boolean
  granted_group_id?: number
  granted_group_name?: string
}

/**
 * Update an API key's group binding and optional allowed models
 * @param id - API Key ID
 * @param groupIds - Ordered group IDs (empty array unbinds all)
 * @param modelAllowlist - Explicit model restriction settings (omitted preserves them)
 * @returns Updated API key (binding does not grant group access)
 */
export async function updateApiKeyGroups(
  id: number,
  groupIds: number[],
  modelAllowlist?: Required<ApiKeyModelAllowlist>
): Promise<UpdateApiKeyGroupResult> {
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, {
    group_ids: [...groupIds],
    ...(modelAllowlist === undefined ? {} : { model_allowlist: modelAllowlist })
  })
  return data
}

export const apiKeysAPI = {
  updateApiKeyGroups
}

export default apiKeysAPI
