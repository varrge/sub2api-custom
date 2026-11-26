import { apiClient } from './client'

export type EvaluationKind = 'candy' | 'pelican'
export type EvaluationEffort = 'low' | 'medium' | 'high'

export interface EvaluationGroup {
  group_id: number
  group_name: string
  group_platform: string
  enabled: boolean
  candy_enabled: boolean
  pelican_enabled: boolean
  automatic: boolean
  interval_seconds: number
  model: string
  effort: EvaluationEffort
  last_run_at: string | null
  next_run_at: string | null
  running: boolean
}

export interface EvaluationConfig extends EvaluationGroup {
  revision: number
  runs: number
  daily_limit: number
}

export type EvaluationConfigInput = Pick<EvaluationConfig,
  'revision' | 'enabled' | 'candy_enabled' | 'pelican_enabled' | 'automatic' |
  'interval_seconds' | 'model' | 'effort' | 'runs' | 'daily_limit'
>

export interface EvaluationRecord {
  id: number
  kind: EvaluationKind
  status: 'passed' | 'mismatch' | 'generated' | 'error'
  model: string
  effort: EvaluationEffort
  created_at: string
  answer: string
  expected_answer: string
  prompt_version: string
  html: string
  error: string
  duration_ms: number
}

export interface EvaluationDetail {
  group: EvaluationGroup
  records: Record<EvaluationKind, EvaluationRecord[]>
}

export async function listEvaluationGroups(signal?: AbortSignal) {
  const { data } = await apiClient.get<{ items: EvaluationGroup[] }>('/group-evaluations', { signal })
  return data
}

export async function getEvaluationDetail(groupId: number, admin = false, signal?: AbortSignal) {
  const { data } = await apiClient.get<EvaluationDetail>(
    `${admin ? '/admin' : ''}/group-evaluations/${groupId}`, { signal },
  )
  return data
}

export async function listEvaluationConfigs(signal?: AbortSignal) {
  const { data } = await apiClient.get<{ items: EvaluationConfig[] }>('/admin/group-evaluations', { signal })
  return data
}

export async function saveEvaluationConfig(groupId: number, config: EvaluationConfigInput) {
  const { data } = await apiClient.put<EvaluationConfig>(`/admin/group-evaluations/${groupId}`, config)
  return data
}

export async function runEvaluation(groupId: number) {
  const { data } = await apiClient.post<{ accepted: boolean }>(`/admin/group-evaluations/${groupId}/run`)
  return data
}
