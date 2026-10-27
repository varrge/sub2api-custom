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
export type ActivityTask = string
export interface ActivityReward {
  task: ActivityTask
  earned_at: string
  delivered_at: string | null
  card_tier?: 'low' | 'medium'
  quantity?: number
}
export interface ActivityPurchase {
  order_id: number
  card_id: number
  card_code: string
  product_name: string
  price_cny: number
  paid_at: string
  team_code: string
  team_members: number
  team_required: number
  team_qualified: boolean
  purchase_task: string
  team_task: string | null
  eligible: boolean
}
export interface ActivityTreasure {
  id: string
  name: string
  clue: string
  revealed: boolean
  found: boolean
  line: string
}
export interface ActivityCampaignState {
  campaign: ActivityCampaignConfig
  status: 'disabled' | 'upcoming' | 'active' | 'ended'
  server_time: string
  today: string
  checkin_dates: string[]
  quiz_completed: boolean
  rewards: ActivityReward[]
  purchases?: ActivityPurchase[]
  treasures?: ActivityTreasure[]
}
export const waveOneID = 'double11-2026-wave1'
export const waveTwoID = 'double11-2026-wave2'
export const waveThreeID = 'double11-2026-wave3'

export const activityCampaignsAPI = {
  async get(id: string = waveOneID): Promise<ActivityCampaignState> {
    return (await apiClient.get(`/activities/campaigns/${id}`)).data
  },
  async checkin(id: string = waveOneID): Promise<ActivityCampaignState> {
    return (await apiClient.post(`/activities/campaigns/${id}/checkin`)).data
  },
  async quiz(revision: number, answers: Record<string, number>, id: string = waveOneID): Promise<ActivityCampaignState> {
    return (await apiClient.post(`/activities/campaigns/${id}/quiz`, { revision, answers })).data
  },
  async claim(task: string, id: string = waveOneID): Promise<ActivityCampaignState> {
    return (await apiClient.post(`/activities/campaigns/${id}/rewards/${encodeURIComponent(task)}/claim`)).data
  },
  async adminGet(id: string = waveOneID): Promise<ActivityCampaignConfig> {
    return (await apiClient.get(`/admin/activities/campaigns/${id}`)).data
  },
  async adminSave(config: ActivityCampaignConfig, id: string = config.id): Promise<ActivityCampaignConfig> {
    return (await apiClient.put(`/admin/activities/campaigns/${id}`, config)).data
  },
  async treasureVisit(path: string): Promise<{ treasure: null | { id: string; name: string; token: string } }> {
    return (await apiClient.post(`/activities/campaigns/${waveThreeID}/treasure/visit`, { path })).data
  },
  async treasureFind(id: string, token: string): Promise<ActivityCampaignState> {
    return (await apiClient.post(`/activities/campaigns/${waveThreeID}/treasure/find`, { id, token })).data
  }
}
