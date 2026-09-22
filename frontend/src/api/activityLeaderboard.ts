import { apiClient } from './client'

export interface ActivityLeaderboardEntry {
  rank: number
  alias: string
  amount: string
  is_me: boolean
}

export interface ActivityLeaderboard {
  demo?: boolean
  demo_expires_at?: string
  campaign_id: string
  starts_at: string
  ends_at: string
  status: 'upcoming' | 'active' | 'ended'
  updated_at: string
  refresh_seconds: number
  participant_count: number
  entries: ActivityLeaderboardEntry[]
  me: ActivityLeaderboardEntry | null
}

export async function getActivityLeaderboard(signal?: AbortSignal): Promise<ActivityLeaderboard> {
  const { data } = await apiClient.get<ActivityLeaderboard>('/activities/double-festival/leaderboard', { signal })
  return data
}
