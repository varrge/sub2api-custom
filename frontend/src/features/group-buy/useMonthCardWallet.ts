import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useGroupBuyStore } from '@/stores/groupBuy'
import { groupBuyAPI } from '@/api/groupBuy'
import type { ChargeAllocation, EntitlementOrder, EntitlementRef, MonthCard } from '@/types/groupBuy'
import { extractApiErrorMessage } from '@/utils/apiError'
import { completeWalletOrder, visibleWalletCards, walletGroups } from './walletModel'
import { orderedEntitlements } from './model'

export function useMonthCardWallet() {
  const { t } = useI18n()
  const auth = useAuthStore()
  const shared = useGroupBuyStore()
  const identity = computed(() => `${auth.user?.id ?? ''}:${auth.user?.account_type ?? 'personal'}`)
  const allCards = ref<MonthCard[]>([])
  const orders = ref<EntitlementOrder[]>([])
  const allocations = ref<ChargeAllocation[]>([])
  const selectedId = ref<number | null>(null)
  const now = ref(Date.now())
  const loading = ref(false), saving = ref(false), freezing = ref(false)
  const error = ref(''), orderError = ref(''), freezeError = ref('')
  const orderSaved = ref(false)
  const dragging = ref(false)
  let generation = 0, disposed = false
  let timer: ReturnType<typeof setInterval> | undefined
  const cards = computed(() => orderedEntitlements(visibleWalletCards(allCards.value, now.value), [], orders.value).map(item => item.card!))
  const groups = computed(() => walletGroups(allCards.value, orders.value, now.value))
  const selected = computed(() => cards.value.find(card => card.id === selectedId.value) ?? null)
  watch(cards, value => {
    if (!value.some(card => card.id === selectedId.value)) selectedId.value = value[0]?.id ?? null
  }, { flush: 'sync' })
  const current = (at: number) => !disposed && generation === at

  async function refresh() {
    if (disposed || !auth.user?.id || auth.isEnterprise || loading.value || saving.value || freezing.value || dragging.value) return
    const at = ++generation
    loading.value = true
    try {
      const [nextCards, nextOrders, nextCharges] = await Promise.all([groupBuyAPI.cards(), groupBuyAPI.orders(), groupBuyAPI.allocations()])
      if (!current(at)) return
      orders.value = nextOrders
      allCards.value = nextCards
      allocations.value = nextCharges
      now.value = Date.now()
      error.value = ''
    } catch {
      if (current(at)) error.value = t('groupBuy.loadFailed')
    } finally {
      if (current(at)) loading.value = false
    }
  }

  async function reorder(groupId: number, items: EntitlementRef[]) {
    if (saving.value || freezing.value || loading.value) return
    orderSaved.value = false
    orderError.value = ''
    const group = groups.value.find(item => item.id === groupId)
    const original = orders.value.find(order => order.group_id === groupId)
    if (!group || !original) { orderError.value = t('groupBuy.orderChanged'); return }
    const complete = completeWalletOrder(original.items, group.items, items)
    if (!complete) { orderError.value = t('groupBuy.orderChanged'); return }
    if (JSON.stringify(complete) === JSON.stringify(original.items)) return
    const at = ++generation
    saving.value = true
    orderError.value = ''
    orderSaved.value = false
    dragging.value = false
    orders.value = orders.value.map(order => order.group_id === groupId ? { group_id: groupId, items: complete } : order)
    try {
      await groupBuyAPI.setOrder({ group_id: groupId, items: complete })
      if (!current(at)) return
      orderSaved.value = true
    } catch {
      if (!current(at)) return
      orders.value = orders.value.map(order => order.group_id === groupId ? original : order)
      orderError.value = t('groupBuy.orderChanged')
    } finally {
      if (current(at)) {
        saving.value = false
        void refresh()
      }
    }
  }

  async function freeze(card: MonthCard) {
    const own = cards.value.find(item => item.id === card.id)
    if (!own || own.freeze_allowed === false || saving.value || freezing.value || loading.value) return
    const at = ++generation
    freezing.value = true
    freezeError.value = ''
    try {
      const next = own.status === 'frozen' ? await groupBuyAPI.thawCard(own.id) : await groupBuyAPI.freezeCard(own.id)
      if (!current(at)) return
      if (next.id !== own.id || next.user_id !== own.user_id) throw new Error('Invalid card response')
      allCards.value = allCards.value.map(item => item.id === next.id ? { ...item, ...next } : item)
      shared.updateMonthCard(next)
    } catch (e) {
      if (current(at)) freezeError.value = extractApiErrorMessage(e, t('groupBuy.freezeFailed'))
    } finally {
      if (current(at)) {
        freezing.value = false
        void refresh()
      }
    }
  }

  watch(identity, () => {
    generation++
    dragging.value = false
    loading.value = saving.value = freezing.value = false
    allCards.value = []; orders.value = []; allocations.value = []
    selectedId.value = null
    error.value = orderError.value = freezeError.value = ''
    orderSaved.value = false
    void refresh()
  }, { flush: 'sync' })
  onMounted(() => {
    void refresh()
    let ticks = 0
    timer = setInterval(() => {
      now.value = Date.now()
      if (++ticks % 30 === 0) void refresh()
    }, 1000)
  })
  onUnmounted(() => { disposed = true; generation++; clearInterval(timer) })
  return { allCards, cards, groups, allocations, selected, selectedId, now, loading, saving, freezing,
    error, orderError, orderSaved, freezeError, dragging, refresh, reorder, freeze }
}
