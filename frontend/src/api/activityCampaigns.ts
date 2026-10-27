import { apiClient } from './client'

export interface ActivityQuestion {
  id: string
  prompt: string
  options: string[]
  answer?: number // Admin only; never returned by the user API.
}
export interface ActivityCampaignConfig {
  id: string
  revision: number
  enabled: boolean
  title: string
  subtitle: string
  description: string
  starts_at: string
  ends_at: string // Exclusive, RFC3339; UI edits in Asia/Shanghai.
  box_campaign_id: string
  questions: ActivityQuestion[]
}
export type ActivityTask = 'first_checkin' | 'three_checkins' | 'quiz'
export interface ActivityReward {
  task: ActivityTask
  earned_at: string
  delivered_at: string | null
}
export interface ActivityCampaignState {
  campaign: ActivityCampaignConfig
  status: 'disabled' | 'upcoming' | 'active' | 'ended'
  server_time: string
  today: string
  checkin_dates: string[]
  quiz_completed: boolean
  rewards: ActivityReward[]
}
export const waveOneID = 'double11-2026-wave1'
export const activityCampaignsAPI = {
  async get(): Promise<ActivityCampaignState> {
    return (await apiClient.get(`/activities/campaigns/${waveOneID}`)).data
  },
  async checkin(): Promise<ActivityCampaignState> {
    return (await apiClient.post(`/activities/campaigns/${waveOneID}/checkin`)).data
  },
  async quiz(revision: number, answers: Record<string, number>): Promise<ActivityCampaignState> {
    return (await apiClient.post(`/activities/campaigns/${waveOneID}/quiz`, { revision, answers })).data
  },
  async claim(task: ActivityTask): Promise<ActivityCampaignState> {
    return (await apiClient.post(`/activities/campaigns/${waveOneID}/rewards/${task}/claim`)).data
  },
  async adminGet(): Promise<ActivityCampaignConfig> {
    return (await apiClient.get(`/admin/activities/campaigns/${waveOneID}`)).data
  },
  async adminSave(config: ActivityCampaignConfig): Promise<ActivityCampaignConfig> {
    return (await apiClient.put(`/admin/activities/campaigns/${waveOneID}`, config)).data
  }
}
