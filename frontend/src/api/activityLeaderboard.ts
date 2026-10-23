import { apiClient } from './client'

export interface ActivityLeaderboardEntry {
  rank: number
  alias: string
  amount: string
  is_me: boolean
}

export type ActivityLeaderboardStatus = 'disabled' | 'upcoming' | 'active' | 'ended'
export type LeaderboardView = 'cumulative' | 'daily'
export type LeaderboardDisplayMode = LeaderboardView | 'both'
export interface ActivityRankReward { rank: number; card_tier: 'low' | 'medium' | 'high'; quantity: number }
export interface ActivityRewardRules {
  ranked: ActivityRankReward[]
  participation: { enabled: boolean; card_tier: 'low' | 'medium' | 'high'; quantity: number; include_zero_cost: boolean }
}

export interface ActivityLeaderboardConfig {
  enabled: boolean
  title: string
  subtitle: string
  reward_description: string
  starts_at: string
  ends_at: string
  demo_expires_at: string | null
  period?: '' | 'daily' | 'event'
  auto_rewards?: boolean
  display_mode?: LeaderboardDisplayMode
  settlement_delay_minutes?: number
  reward_rules?: ActivityRewardRules
}

/** Lightweight public config: the stored config plus campaign metadata. */
export interface ActivityLeaderboardPublicConfig extends ActivityLeaderboardConfig {
  campaign_id: string
  status: ActivityLeaderboardStatus
}

export interface ActivityLeaderboard {
  display_mode?: LeaderboardDisplayMode
  view?: LeaderboardView
  enabled: boolean
  title: string
  subtitle: string
  reward_description: string
  demo?: boolean
  demo_expires_at?: string
  campaign_id: string
  starts_at: string
  ends_at: string
  status: ActivityLeaderboardStatus
  updated_at: string
  refresh_seconds: number
  participant_count: number
  entries: ActivityLeaderboardEntry[]
  me: ActivityLeaderboardEntry | null
}

export async function getActivityLeaderboard(signal?: AbortSignal, view?: LeaderboardView): Promise<ActivityLeaderboard> {
  const { data } = await apiClient.get<ActivityLeaderboard>('/activities/leaderboard', { signal, ...(view ? { params: { view } } : {}) })
  return data
}

export async function getActivityLeaderboardConfig(signal?: AbortSignal): Promise<ActivityLeaderboardPublicConfig> {
  const { data } = await apiClient.get<ActivityLeaderboardPublicConfig>('/activities/leaderboard/config', { signal })
  return data
}
