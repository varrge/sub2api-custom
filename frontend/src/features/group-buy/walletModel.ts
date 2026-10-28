import type { EntitlementOrder, EntitlementRef, MonthCard } from '@/types/groupBuy'
import { availableQuota, orderedEntitlements, refKey, type EntitlementItem } from './model'

export interface WalletGroup { id: number; name: string; items: EntitlementItem[] }

export function visibleWalletCards(cards: MonthCard[], now: number): MonthCard[] {
  return cards.filter(card => card.status === 'frozen'
    ? card.remaining_seconds === undefined || card.remaining_seconds > 0
    : card.status === 'active' && Date.parse(card.expires_at) > now)
}

export function spendableQuota(card: MonthCard, now: number): number {
  return card.status === 'active' && Date.parse(card.starts_at) <= now && Date.parse(card.expires_at) > now
    ? availableQuota(card) : 0
}

export function cardDaysRemaining(card: MonthCard, now: number): number {
  return Math.max(0, Math.ceil((card.status === 'frozen'
    ? card.remaining_seconds ?? 0
    : (Date.parse(card.expires_at) - now) / 1000) / 86400))
}

export function walletGroups(cards: MonthCard[], orders: EntitlementOrder[], now: number): WalletGroup[] {
  const visible = visibleWalletCards(cards, now)
  const items = orderedEntitlements(visible, [], orders)
  return [...new Set(items.map(item => item.group_id))].map(id => {
    const own = items.filter(item => item.group_id === id)
    const ordered = orders.find(order => order.group_id === id)?.items ?? []
    const keyed = new Map(own.map(item => [refKey(item), item]))
    const groupItems: EntitlementItem[] = ordered.flatMap(ref => {
      const card = keyed.get(refKey(ref))
      if (card) { keyed.delete(refKey(ref)); return [card] }
      return ref.kind === 'legacy' ? [{ ...ref, group_id: id, name: `#${ref.id}`, expires_at: null }] : []
    })
    return { id, name: own[0].card!.group_name, items: [...groupItems, ...keyed.values()] }
  })
}

/** Replace visible slots only; hidden server refs retain their original positions. */
export function completeWalletOrder(current: EntitlementRef[], shown: EntitlementRef[], reordered: EntitlementRef[]): EntitlementRef[] | null {
  const currentKeys = new Set(current.map(refKey))
  const shownKeys = new Set(shown.map(refKey))
  const nextKeys = new Set(reordered.map(refKey))
  if (shownKeys.size !== shown.length || reordered.length !== shown.length || nextKeys.size !== shownKeys.size ||
      [...nextKeys].some(key => !shownKeys.has(key) || !currentKeys.has(key))) return null
  let index = 0
  return current.map(ref => {
    if (!shownKeys.has(refKey(ref))) return { ...ref }
    const next = reordered[index++]
    return { kind: next.kind, id: next.id }
  })
}
