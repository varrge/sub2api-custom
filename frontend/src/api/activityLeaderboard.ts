import { apiClient } from './client'

export interface ActivityLeaderboardEntry {
  rank: number
  alias: string
  amount: string
  is_me: boolean
}

export type ActivityLeaderboardStatus = 'disabled' | 'upcoming' | 'active' | 'ended'

export interface ActivityLeaderboardConfig {
  enabled: boolean
  title: string
  subtitle: string
  reward_description: string
  starts_at: string
  ends_at: string
  demo_expires_at: string | null
}

/** Lightweight public config: the stored config plus campaign metadata. */
export interface ActivityLeaderboardPublicConfig extends ActivityLeaderboardConfig {
  campaign_id: string
  status: ActivityLeaderboardStatus
}

export interface ActivityLeaderboard {
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

export async function getActivityLeaderboard(signal?: AbortSignal): Promise<ActivityLeaderboard> {
  const { data } = await apiClient.get<ActivityLeaderboard>('/activities/leaderboard', { signal })
  return data
}

export async function getActivityLeaderboardConfig(signal?: AbortSignal): Promise<ActivityLeaderboardPublicConfig> {
  const { data } = await apiClient.get<ActivityLeaderboardPublicConfig>('/activities/leaderboard/config', { signal })
  return data
}
