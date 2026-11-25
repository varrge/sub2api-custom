import { apiClient } from '../client'
import type { BoxCardTier, BoxDraw, BoxProgressEntry, BoxRarity } from '../luckyBox'

export type ShakeRuleSource = 'order' | 'usage'
export type ShakeOrderType = 'subscription' | 'month_card' | 'balance'
export type ShakeBilling = 'balance' | 'subscription'
export type ShakeRuleMode = 'per_order' | 'threshold'
export type ShakePrizeType = 'balance' | 'extra_draw' | 'empty'

export interface ShakeRule {
  source: ShakeRuleSource
  order_type?: ShakeOrderType
  billing?: ShakeBilling
  mode: ShakeRuleMode
  group_id?: number | null
  threshold?: number
  quantity: number
  card_tier: BoxCardTier
}

export interface ShakePrize {
  id?: string
  name: string
  type: ShakePrizeType
  amount: number | null
  weights: Record<BoxCardTier, number>
  rarity: BoxRarity
  display_text: string
  icon: string
  sort_order: number
}

export interface ShakeConfig {
  name: string
  starts_at: string
  ends_at: string
  rules: ShakeRule[]
  prizes: ShakePrize[]
}

export type ShakeStatus = 'draft' | 'scheduled' | 'active' | 'ended'

export interface ShakeCampaign {
  id: string
  status: ShakeStatus
  stored_status: 'draft' | 'active' | 'ended'
  config_version: number
  config: ShakeConfig
  created_at: string
  ended_at: string | null
  end_reason: string
}

export interface ShakeStats {
  cards: { tier: BoxCardTier; available: number; consumed: number; expired: number }[]
  users: number
  draws: number
  balance_paid: number
}

export interface ShakeUserCards {
  user_id: number
  email: string
  by_tier: Record<BoxCardTier, number>
  total: number
  progress: BoxProgressEntry[]
}

export interface ShakeRewardJob {
  id: number
  campaign_id: string
  source_campaign_id: string
  source_starts_at: string
  source_ends_at: string
  period: 'daily' | 'event'
  delay_minutes: number
  policy_fingerprint: string
  enabled: boolean
  last_error: string
  last_checked_at: string | null
  created_at: string
  completed_days: number
  recipients: number
  cards_granted: number
}

const base = '/admin/shake'

export async function listCampaigns(): Promise<ShakeCampaign[]> {
  const { data } = await apiClient.get<{ items: ShakeCampaign[] }>(`${base}/campaigns`)
  return data.items
}

export async function getCampaign(id: string): Promise<{ campaign: ShakeCampaign; stats: ShakeStats }> {
  const { data } = await apiClient.get(`${base}/campaigns/${encodeURIComponent(id)}`)
  return data
}

export async function createCampaign(config: ShakeConfig): Promise<ShakeCampaign> {
  const { data } = await apiClient.post<ShakeCampaign>(`${base}/campaigns`, config)
  return data
}

export async function saveCampaign(id: string, configVersion: number, config: ShakeConfig): Promise<ShakeCampaign> {
  const { data } = await apiClient.put<ShakeCampaign>(`${base}/campaigns/${encodeURIComponent(id)}`, {
    config_version: configVersion,
    config
  })
  return data
}

export async function activateCampaign(id: string): Promise<ShakeCampaign> {
  const { data } = await apiClient.post<ShakeCampaign>(`${base}/campaigns/${encodeURIComponent(id)}/activate`, {})
  return data
}

export async function endCampaign(id: string, reason: string): Promise<{ campaign: ShakeCampaign; expired_cards: number }> {
  const { data } = await apiClient.post(`${base}/campaigns/${encodeURIComponent(id)}/end`, { reason })
  return data
}

export async function deleteDraft(id: string): Promise<void> {
  await apiClient.delete(`${base}/campaigns/${encodeURIComponent(id)}`)
}

export async function syncCampaign(id: string): Promise<{ cards_granted: number }> {
  const { data } = await apiClient.post(`${base}/campaigns/${encodeURIComponent(id)}/sync`, {})
  return data
}

export async function grantCards(id: string, payload: { user_id: number; card_tier: BoxCardTier; quantity: number; reason: string }): Promise<{ granted: number }> {
  const { data } = await apiClient.post(`${base}/campaigns/${encodeURIComponent(id)}/grants`, payload)
  return data
}

export async function getUserCards(id: string, userId: number): Promise<ShakeUserCards> {
  const { data } = await apiClient.get<ShakeUserCards>(`${base}/campaigns/${encodeURIComponent(id)}/users/${userId}`)
  return data
}

export async function listDraws(params: { campaign_id?: string; user_id?: number; status?: string }): Promise<BoxDraw[]> {
  const { data } = await apiClient.get<{ items: BoxDraw[] }>(`${base}/draws`, { params })
  return data.items
}

export async function voidDraw(drawId: string, reason: string): Promise<BoxDraw> {
  const { data } = await apiClient.post<BoxDraw>(`${base}/draws/${encodeURIComponent(drawId)}/void`, { reason })
  return data
}

export async function listRewardJobs(): Promise<ShakeRewardJob[]> {
  const { data } = await apiClient.get<{ items: ShakeRewardJob[] }>(`${base}/reward-jobs`)
  return data.items
}

export async function createRewardJob(campaignId: string): Promise<ShakeRewardJob> {
  const { data } = await apiClient.post<ShakeRewardJob>(`${base}/reward-jobs`, { campaign_id: campaignId })
  return data
}

export async function setRewardJobEnabled(id: number, enabled: boolean): Promise<ShakeRewardJob> {
  const { data } = await apiClient.post<ShakeRewardJob>(`${base}/reward-jobs/${id}/enabled`, { enabled })
  return data
}

export async function retryRewardJob(id: number): Promise<ShakeRewardJob> {
  const { data } = await apiClient.post<ShakeRewardJob>(`${base}/reward-jobs/${id}/retry`, {})
  return data
}

export async function refreshRewardPolicy(id: number): Promise<ShakeRewardJob> {
  const { data } = await apiClient.post<ShakeRewardJob>(`${base}/reward-jobs/${id}/refresh-policy`, {})
  return data
}
