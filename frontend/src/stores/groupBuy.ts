import { defineStore } from 'pinia'
import { ref } from 'vue'
import { groupBuyAPI } from '@/api/groupBuy'
import type { MonthCard, EntitlementOrder } from '@/types/groupBuy'

const CACHE_TTL_MS = 60_000

// Month-card ownership is independent of the official subscription switch.
export const useGroupBuyStore = defineStore('groupBuy', () => {
  const monthCards = ref<MonthCard[]>([])
  const entitlementOrders = ref<EntitlementOrder[]>([])
  let monthCardsPromise: Promise<void> | null = null
  let monthCardsFetchedAt = 0
  let monthCardsGeneration = 0
  function updateMonthCard(card: MonthCard) {
    // Discard reads started before this mutation returned.
    monthCardsGeneration++
    monthCardsFetchedAt = 0
    monthCardsPromise = null
    monthCards.value = monthCards.value.map(item => item.id === card.id ? card : item)
  }
  async function fetchMonthCards(force = false): Promise<void> {
    if (!force && monthCardsFetchedAt && Date.now() - monthCardsFetchedAt < CACHE_TTL_MS) return
    if (monthCardsPromise && !force) return monthCardsPromise
    const generation = ++monthCardsGeneration
    const request = Promise.all([groupBuyAPI.cards(), groupBuyAPI.orders()]).then(([cards, orders]) => {
      if (generation !== monthCardsGeneration) return
      monthCards.value = cards
      entitlementOrders.value = orders
      monthCardsFetchedAt = Date.now()
    }).finally(() => { if (monthCardsPromise === request) monthCardsPromise = null })
    monthCardsPromise = request
    return request
  }

  let poller: ReturnType<typeof setInterval> | null = null

  function startPolling() {
    if (poller) return
    poller = setInterval(() => {
      fetchMonthCards(true).catch(() => {})
    }, 5 * 60 * 1000)
  }

  function stopPolling() {
    if (poller) clearInterval(poller)
    poller = null
  }

  function clear() {
    stopPolling()
    monthCardsGeneration++
    monthCardsPromise = null
    monthCardsFetchedAt = 0
    monthCards.value = []
    entitlementOrders.value = []
  }

  return { monthCards, entitlementOrders, updateMonthCard, fetchMonthCards, startPolling, stopPolling, clear }
})
