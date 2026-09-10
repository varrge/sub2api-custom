import type { LocationQuery } from 'vue-router'
import type { UserSubscription } from '@/types'
import type {
  EntitlementOrder,
  EntitlementRef,
  GroupBuyTeam,
  MonthCard,
  MonthCardMode
} from '@/types/groupBuy'

export const usd = (amount: number) =>
  new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    maximumFractionDigits: 8
  }).format(amount)
export const cny = (amount: number) =>
  new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' }).format(
    amount
  )
export const exactDate = (value: string) =>
  new Date(value).toLocaleString(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZoneName: 'short'
  })
export const progress = (used: number, total: number) =>
  total > 0 ? Math.min(100, Math.max(0, (used / total) * 100)) : 0
export const availableQuota = (card: MonthCard) =>
  Math.max(
    0,
    Math.min(
      card.total_quota_usd - card.total_used_usd,
      card.weekly_quota_usd - card.weekly_used_usd
    )
  )
export const canJoin = (team: GroupBuyTeam, now = Date.now()) =>
  !team.joined &&
  team.status === 'recruiting' &&
  team.member_count < team.product.max_members &&
  Date.parse(team.closes_at) > now
export const remainingMembers = (team: GroupBuyTeam) => ({
  next: Math.max(0, team.next_members - team.member_count),
  full: Math.max(0, team.product.max_members - team.member_count)
})
export const refKey = (item: EntitlementRef) => `${item.kind}:${item.id}`
export function purchaseQuery(query: LocationQuery) {
  const mode: MonthCardMode | null = ['solo', 'create', 'join'].includes(
    String(query.mode)
  )
    ? (query.mode as MonthCardMode)
    : null
  return {
    groupId: Number(query.group) || undefined,
    productId: Number(query.product_id) || undefined,
    teamCode: typeof query.team_code === 'string' ? query.team_code : '',
    mode,
    groupBuy:
      query.tab === 'subscription' || query.tab === 'group-buy' || !!mode
  }
}
export interface EntitlementItem extends EntitlementRef {
  group_id: number
  name: string
  expires_at: string | null
  card?: MonthCard
  legacy?: UserSubscription
}
export function orderedEntitlements(
  cards: MonthCard[],
  legacy: UserSubscription[],
  orders: EntitlementOrder[]
): EntitlementItem[] {
  const items: EntitlementItem[] = [
    ...cards.map((card) => ({
      kind: 'card' as const,
      id: card.id,
      group_id: card.group_id,
      name: `${card.product_name} · ${card.code}`,
      expires_at: card.expires_at,
      card
    })),
    ...legacy.map((sub) => ({
      kind: 'legacy' as const,
      id: sub.id,
      group_id: sub.group_id,
      name: sub.group?.name || `#${sub.group_id}`,
      expires_at: sub.expires_at,
      legacy: sub
    }))
  ]
  const ranks = new Map(
    orders.flatMap((order) =>
      order.items.map(
        (item, rank) => [`${order.group_id}:${refKey(item)}`, rank] as const
      )
    )
  )
  return items.sort(
    (a, b) =>
      a.group_id - b.group_id ||
      (ranks.get(`${a.group_id}:${refKey(a)}`) ?? Number.MAX_SAFE_INTEGER) -
        (ranks.get(`${b.group_id}:${refKey(b)}`) ?? Number.MAX_SAFE_INTEGER) ||
      (Date.parse(a.expires_at || '') || Infinity) -
        (Date.parse(b.expires_at || '') || Infinity) ||
      a.id - b.id
  )
}
