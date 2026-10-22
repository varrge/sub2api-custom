/**
 * Usage tracking API endpoints
 * Handles usage logs and statistics retrieval
 */

import { apiClient } from './client'
import type {
  UsageLog,
  UsageQueryParams,
  UsageStatsResponse,
  PaginatedResponse,
  TrendDataPoint,
  ModelStat,
  GroupStat,
  UsageRequestType,
  UserErrorRequest,
  UserErrorRequestDetail,
  UserErrorListParams
} from '@/types'

// ==================== Dashboard Types ====================

export interface PlatformDashboardStats {
  platform: string
  total_requests: number
  total_tokens: number
  total_actual_cost: number
  today_requests: number
  today_tokens: number
  today_actual_cost: number
}

export interface UserDashboardStats {
  total_api_keys: number
  active_api_keys: number
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cache_creation_tokens: number
  total_cache_read_tokens: number
  total_tokens: number
  total_cost: number // 标准计费
  total_actual_cost: number // 实际扣除
  today_requests: number
  today_input_tokens: number
  today_output_tokens: number
  today_cache_creation_tokens: number
  today_cache_read_tokens: number
  today_tokens: number
  today_cost: number // 今日标准计费
  today_actual_cost: number // 今日实际扣除
  average_duration_ms: number
  rpm: number // 近5分钟平均每分钟请求数
  tpm: number // 近5分钟平均每分钟Token数
  by_platform?: PlatformDashboardStats[]
}

export interface TrendParams {
  start_date?: string
  end_date?: string
  granularity?: 'day' | 'hour'
  api_key_id?: number
  model?: string
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  native_compaction_v2?: boolean | null
  billing_type?: number | null
  billing_mode?: string | null
  timezone?: string
}

export interface TrendResponse {
  trend: TrendDataPoint[]
  start_date: string
  end_date: string
  granularity: string
}

export interface ModelStatsResponse {
  models: ModelStat[]
  start_date: string
  end_date: string
}

export interface ApiKeyDailyUsagePoint {
  date: string
  requests: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  total_tokens: number
  cost: number
  actual_cost: number
}

export interface ApiKeyDailyUsageResponse {
  items: ApiKeyDailyUsagePoint[]
  days: number
  start_date: string
  end_date: string
}

export interface UsageDashboardSnapshotV2Params extends TrendParams {
  include_trend?: boolean
  include_model_stats?: boolean
  include_group_stats?: boolean
}

export interface UsageDashboardSnapshotV2Response {
  generated_at: string
  start_date: string
  end_date: string
  granularity: string
  trend?: TrendDataPoint[]
  models?: ModelStat[]
  groups?: GroupStat[]
}

/**
 * List usage logs with optional filters
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 20)
 * @param apiKeyId - Filter by API key ID
 * @returns Paginated list of usage logs
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  apiKeyId?: number
): Promise<PaginatedResponse<UsageLog>> {
  const params: UsageQueryParams = {
    page,
    page_size: pageSize
  }

  if (apiKeyId !== undefined) {
    params.api_key_id = apiKeyId
  }

  const { data } = await apiClient.get<PaginatedResponse<UsageLog>>('/usage', {
    params
  })
  return data
}

/**
 * Get usage logs with advanced query parameters
 * @param params - Query parameters for filtering and pagination
 * @returns Paginated list of usage logs
 */
export async function query(
  params: UsageQueryParams & { sort_by?: string; sort_order?: 'asc' | 'desc' },
  config: { signal?: AbortSignal } = {}
): Promise<PaginatedResponse<UsageLog>> {
  const { data } = await apiClient.get<PaginatedResponse<UsageLog>>('/usage', {
    ...config,
    params
  })
  return data
}

/**
 * Get usage statistics for a specific period
 * @param period - Time period ('today', 'week', 'month', 'year')
 * @param apiKeyId - Optional API key ID filter
 * @returns Usage statistics
 */
export async function getStats(
  paramsOrPeriod: (UsageQueryParams & { period?: string; timezone?: string }) | string = 'today',
  apiKeyId?: number
): Promise<UsageStatsResponse> {
  const params: Record<string, unknown> = typeof paramsOrPeriod === 'string'
    ? { period: paramsOrPeriod }
    : { ...paramsOrPeriod }

  if (apiKeyId !== undefined) {
    params.api_key_id = apiKeyId
  }

  const { data } = await apiClient.get<UsageStatsResponse>('/usage/stats', {
    params
  })
  return data
}

/**
 * Get usage statistics for a date range
 * @param startDate - Start date (YYYY-MM-DD format)
 * @param endDate - End date (YYYY-MM-DD format)
 * @param apiKeyId - Optional API key ID filter
 * @returns Usage statistics
 */
export async function getStatsByDateRange(
  startDate: string,
  endDate: string,
  apiKeyId?: number
): Promise<UsageStatsResponse> {
  const params: Record<string, unknown> = {
    start_date: startDate,
    end_date: endDate
  }

  if (apiKeyId !== undefined) {
    params.api_key_id = apiKeyId
  }

  const { data } = await apiClient.get<UsageStatsResponse>('/usage/stats', {
    params
  })
  return data
}

/**
 * Get usage by date range
 * @param startDate - Start date (YYYY-MM-DD format)
 * @param endDate - End date (YYYY-MM-DD format)
 * @param apiKeyId - Optional API key ID filter
 * @returns Usage logs within date range
 */
export async function getByDateRange(
  startDate: string,
  endDate: string,
  apiKeyId?: number
): Promise<PaginatedResponse<UsageLog>> {
  const params: UsageQueryParams = {
    start_date: startDate,
    end_date: endDate,
    page: 1,
    page_size: 100
  }

  if (apiKeyId !== undefined) {
    params.api_key_id = apiKeyId
  }

  const { data } = await apiClient.get<PaginatedResponse<UsageLog>>('/usage', {
    params
  })
  return data
}

/**
 * Get detailed usage log by ID
 * @param id - Usage log ID
 * @returns Usage log details
 */
export async function getById(id: number): Promise<UsageLog> {
  const { data } = await apiClient.get<UsageLog>(`/usage/${id}`)
  return data
}

// ==================== Dashboard API ====================

/**
 * Get user dashboard statistics
 * @returns Dashboard statistics for current user
 */
export async function getDashboardStats(): Promise<UserDashboardStats> {
  const { data } = await apiClient.get<UserDashboardStats>('/usage/dashboard/stats')
  return data
}

/**
 * Get user usage trend data
 * @param params - Query parameters for filtering
 * @returns Usage trend data for current user
 */
export async function getDashboardTrend(params?: TrendParams): Promise<TrendResponse> {
  const { data } = await apiClient.get<TrendResponse>('/usage/dashboard/trend', { params })
  return data
}

/**
 * Get user model usage statistics
 * @param params - Query parameters for filtering
 * @returns Model usage statistics for current user
 */
export async function getDashboardModels(params?: {
  start_date?: string
  end_date?: string
  api_key_id?: number
  model?: string
  model_source?: 'requested'
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  native_compaction_v2?: boolean | null
  billing_type?: number | null
  billing_mode?: string | null
  timezone?: string
}): Promise<ModelStatsResponse> {
  const { data } = await apiClient.get<ModelStatsResponse>('/usage/dashboard/models', { params })
  return data
}

/**
 * Get daily usage details for one API key owned by the current user.
 * @param apiKeyId - API key ID
 * @param days - Number of days to include (1-90)
 * @returns Daily usage detail rows
 */
export async function getMyApiKeyDailyUsage(
  apiKeyId: number,
  days: number = 30
): Promise<ApiKeyDailyUsageResponse> {
  const { data } = await apiClient.get<ApiKeyDailyUsageResponse>(
    `/user/api-keys/${apiKeyId}/usage/daily`,
    { params: { days } }
  )
  return data
}

export async function getDashboardSnapshotV2(
  params?: UsageDashboardSnapshotV2Params
): Promise<UsageDashboardSnapshotV2Response> {
  const { data } = await apiClient.get<UsageDashboardSnapshotV2Response>(
    '/usage/dashboard/snapshot-v2',
    { params }
  )
  return data
}

export interface BatchApiKeyUsageStats {
  api_key_id: number
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchApiKeysUsageResponse {
  stats: Record<string, BatchApiKeyUsageStats>
}

/**
 * Get batch usage stats for user's own API keys
 * @param apiKeyIds - Array of API key IDs
 * @param options - Optional request options
 * @returns Usage stats map keyed by API key ID
 */
export async function getDashboardApiKeysUsage(
  apiKeyIds: number[],
  options?: {
    signal?: AbortSignal
  }
): Promise<BatchApiKeysUsageResponse> {
  const { data } = await apiClient.post<BatchApiKeysUsageResponse>(
    '/usage/dashboard/api-keys-usage',
    {
      api_key_ids: apiKeyIds
    },
    {
      signal: options?.signal
    }
  )
  return data
}

export async function listMyErrorRequests(
  params: UserErrorListParams
): Promise<PaginatedResponse<UserErrorRequest>> {
  const { data } = await apiClient.get<PaginatedResponse<UserErrorRequest>>('/usage/errors', {
    params
  })
  return data
}

export async function getMyErrorDetail(id: number): Promise<UserErrorRequestDetail> {
  const { data } = await apiClient.get<UserErrorRequestDetail>(`/usage/errors/${id}`)
  return data
}

// ==================== Monthly export ====================

export type UsageMonthlyExportKind = 'summary' | 'details' | 'conversions' | 'all'

export interface UsageMonthlyExportParams {
  /** 月份，格式 YYYY-MM */
  month: string
  /** summary/details/conversions 返回 CSV；all 返回含三份 CSV 的 ZIP（同一数据库快照） */
  kind?: UsageMonthlyExportKind
  group_id?: number
  /** 仅 summary/details 生效；all 与 conversions 会被忽略（不发送） */
  api_key_id?: number
}

export interface UsageMonthlyExportResult {
  blob: Blob
  filename: string
}

/** 月度用量导出：CSV（单类）或 ZIP（kind=all 完整对账包）。 */
export async function exportMonthly(
  params: UsageMonthlyExportParams
): Promise<UsageMonthlyExportResult> {
  const kind = params.kind ?? 'all'
  // 契约：kind=all（完整 ZIP 对账包）与 kind=conversions（折算入账）不支持按 API Key
  // 筛选，这里防御性地丢弃 api_key_id；group_id 对全部 kind 仍然有效。
  const apiKeyId = kind === 'summary' || kind === 'details' ? params.api_key_id : undefined
  const response = await apiClient.get('/usage/monthly-export', {
    params: {
      month: params.month,
      kind,
      group_id: params.group_id,
      api_key_id: apiKeyId
    },
    responseType: 'blob'
  })
  const blob = response.data as Blob
  const disposition = String(response.headers?.['content-disposition'] ?? '')
  const match = /filename\*?=(?:UTF-8'')?"?([^";]+)/i.exec(disposition)
  const fallback = `usage-${params.month}-${kind}.${kind === 'all' ? 'zip' : 'csv'}`
  const filename = match ? decodeURIComponent(match[1].trim()) : fallback
  return { blob, filename }
}

export const usageAPI = {
  list,
  query,
  getStats,
  getStatsByDateRange,
  getByDateRange,
  getById,
  // Dashboard
  getDashboardStats,
  getDashboardTrend,
  getDashboardModels,
  getMyApiKeyDailyUsage,
  getDashboardSnapshotV2,
  getDashboardApiKeysUsage,
  // Error requests
  listMyErrorRequests,
  getMyErrorDetail,
  // Monthly export
  exportMonthly
}

export default usageAPI
