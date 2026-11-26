import { apiClient } from './client'

export type BoxCardTier = 'low' | 'medium' | 'high'
export type BoxPrizeType = 'balance' | 'extra_draw' | 'empty'
export type BoxRarity = 'common' | 'rare' | 'epic' | 'legendary'
export type BoxDrawStatus = 'selected' | 'delivered' | 'delivery_failed' | 'voided'

export interface BoxPrize {
  id: string
  name: string
  type: BoxPrizeType
  amount: number | null
  weight: number
  probability: number
  weights: Record<BoxCardTier, number>
  probabilities: Record<BoxCardTier, number>
  rarity: BoxRarity
  displayText: string
  icon: string
  sortOrder: number
}

export interface BoxDraw {
  id: string
  requestId: string
  campaignId: string
  connectionId: string
  userId: string
  email: string
  cardTier: BoxCardTier
  status: BoxDrawStatus
  prize: {
    id: string
    name: string
    type: BoxPrizeType
    amount: number | null
    rarity: BoxRarity
  } | null
  errorMessage: string
  dispositionReason: string
  dispositionBy: string
  createdAt: string
  deliveredAt: string | null
}

export interface BoxCampaign {
  id: string
  connectionId: string
  name: string
  status: string
  startAt: string
  endAt: string
  activeConfigVersionId: string | null
}

/** One configured card rule with the user's progress toward its next card. */
export interface BoxProgressEntry {
  source: 'order' | 'usage'
  orderType?: 'subscription' | 'month_card' | 'balance'
  billing?: 'balance' | 'subscription'
  mode: 'per_order' | 'threshold'
  groupId?: number
  groupName?: string
  cardTier: BoxCardTier
  quantity: number
  threshold?: number
  amount: number
  remaining: number
  cardsEarned: number
}

export interface BoxBootstrap {
  campaign: BoxCampaign | null
  availableCards: number
  availableCardsByTier: Record<BoxCardTier, number>
  progress: BoxProgressEntry[]
  prizes: BoxPrize[]
  draws: BoxDraw[]
}

export interface BoxDrawResponse {
  draw: BoxDraw
  availableCards: number
  availableCardsByTier: Record<BoxCardTier, number>
}

export interface BoxDrawRequest {
  requestId: string
  cardTier: BoxCardTier
  campaignId: string
}

export type BoxDrawRecovery = (BoxDrawResponse & { found: true }) | ({
  found: false
  draw: null
  resolution?: 'closed_without_draw'
  userId?: string
  connectionId?: string
} & Partial<BoxDrawRequest>)

export async function recoverBoxDraw(payload: BoxDrawRequest, signal?: AbortSignal): Promise<BoxDrawRecovery> {
  const { data } = await apiClient.post('/activities/box/draws/recover', payload, { signal })
  return data
}

export async function getBoxBootstrap(signal?: AbortSignal): Promise<BoxBootstrap> {
  const { data } = await apiClient.get<BoxBootstrap>('/activities/box/bootstrap', { signal })
  return data
}

export async function createBoxDraw(
  payload: BoxDrawRequest,
  signal?: AbortSignal
): Promise<BoxDrawResponse> {
  const { data } = await apiClient.post<BoxDrawResponse>('/activities/box/draws', payload, { signal })
  return data
}
