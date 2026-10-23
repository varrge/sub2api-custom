import { apiClient } from '../client'
import type { ActivityLeaderboard, ActivityLeaderboardEntry, LeaderboardView } from '../activityLeaderboard'

export interface AdminActivityLeaderboardEntry extends ActivityLeaderboardEntry {
  user_id: number
  email: string
}

export interface AdminActivityLeaderboard extends Omit<ActivityLeaderboard, 'entries' | 'me' | 'demo' | 'demo_expires_at'> {
  entries: AdminActivityLeaderboardEntry[]
}

export type ActivityLeaderboardExportScope = 'top3' | 'all'

export async function getAdminActivityLeaderboard(signal?: AbortSignal, view?: LeaderboardView): Promise<AdminActivityLeaderboard> {
  const { data } = await apiClient.get<AdminActivityLeaderboard>('/admin/activities/leaderboard', { signal, ...(view ? { params: { view } } : {}) })
  return data
}

export async function exportActivityLeaderboard(campaignId: string, scope: ActivityLeaderboardExportScope, signal?: AbortSignal, view?: LeaderboardView): Promise<Blob> {
  const { data } = await apiClient.get<Blob>('/admin/activities/leaderboard/export', {
    params: { campaign_id: campaignId, scope, ...(view ? { view } : {}) },
    responseType: 'blob',
    signal,
  })
  return data
}
