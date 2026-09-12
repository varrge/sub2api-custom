export interface GroupBuyTier {
  members: number
  quota_usd: number
}
export interface GroupBuyProduct {
  id: number
  group_id: number
  group_name: string
  platform: string
  name: string
  description: string
  price_cny: number
  base_quota_usd: number
  tiers: GroupBuyTier[]
  max_members: number
  recruitment_hours: number
  for_sale: boolean
  sort_order: number
}
export interface GroupBuyTeam {
  id: number
  code: string
  product_id: number
  product: GroupBuyProduct
  member_count: number
  current_quota_usd: number
  next_quota_usd: number
  next_members: number
  starts_at: string
  closes_at: string
  status: 'recruiting' | 'full' | 'closed' | 'cancelled'
  joined: boolean
  cards?: MonthCard[]
}
export interface MonthCard {
  id: number
  user_id: number
  group_id: number
  order_id: number
  code: string
  group_name: string
  platform: string
  product_name: string
  team_code: string
  team_id: number | null
  status: 'active' | 'frozen' | 'expired' | 'revoked'
  total_quota_usd: number
  total_used_usd: number
  weekly_quota_usd: number
  weekly_used_usd: number
  starts_at: string
  expires_at: string
  weekly_window_start: string
  weekly_window_end: string
  priority: number
  freeze_allowed?: boolean
  frozen_at?: string | null
  remaining_seconds?: number
}
export interface EntitlementRef {
  kind: 'card' | 'legacy'
  id: number
}
export interface EntitlementOrder {
  group_id: number
  items: EntitlementRef[]
}
export type MonthCardMode = 'solo' | 'create' | 'join'
export interface MonthCardSelection {
  product: GroupBuyProduct
  mode: MonthCardMode
  team?: GroupBuyTeam
}
export interface ChargeAllocation {
  id: number
  request_id: string
  api_key_id: number
  user_id: number
  group_id: number
  kind: 'card' | 'legacy' | 'balance'
  entitlement_id: number
  amount_usd: number
  weekly_window_start: string | null
  started_at: string
  created_at: string
}
