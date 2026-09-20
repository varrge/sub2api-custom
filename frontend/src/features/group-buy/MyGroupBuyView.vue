<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-bold">{{ t('groupBuy.myGroupBuy') }}</h1>
          <p class="mt-1 text-sm text-gray-500">{{ t('groupBuy.myGroupBuyDescription') }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <RouterLink v-if="paymentEnabled" to="/purchase?tab=group-buy" class="btn btn-primary">{{ t('groupBuy.purchase') }}</RouterLink>
          <RouterLink v-if="paymentEnabled" to="/group-buy" class="btn btn-secondary">{{ t('groupBuy.hall') }}</RouterLink>
          <button class="btn btn-secondary" :disabled="loading" @click="refresh">{{ t('common.refresh') }}</button>
        </div>
      </div>
      <p v-if="(authStore.user?.balance ?? 0) < 0" class="gb-notice gb-notice-error p-4" role="alert">
        {{ t('groupBuy.debt', { amount: usd(authStore.user?.balance ?? 0) }) }}
        <RouterLink v-if="paymentEnabled" to="/purchase?tab=recharge" class="underline">{{ t('payment.tabTopUp') }}</RouterLink>
      </p>
      <p v-if="error" class="text-sm text-red-600" role="alert">{{ error }}</p>
      <div v-if="loading && !monthCards.length" class="py-12 text-center text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="!monthCards.length && !error" class="gb-panel p-12 text-center text-gray-500">{{ t('groupBuy.noCards') }}</div>
      <div v-if="monthCards.length" class="grid gap-6 lg:grid-cols-2">
        <MonthCardCard v-for="card in monthCards" :key="card.id" :card="card" manageable @changed="cardChanged" />
      </div>
      <ConsumptionOrder v-if="sortableEntitlements.length" :items="sortableEntitlements" @saved="refresh" />
      <div v-if="allocations.length || monthCards.length" class="space-y-3">
        <p class="text-sm text-gray-500">{{ t('groupBuy.sharedBillingHint') }}</p>
        <AllocationTable :allocations="allocations" :cards="monthCards" />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import './glass.css'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useGroupBuyStore } from '@/stores/groupBuy'
import { useAuthStore } from '@/stores/auth'
import { groupBuyAPI } from '@/api/groupBuy'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import type { ChargeAllocation, MonthCard } from '@/types/groupBuy'
import MonthCardCard from './MonthCardCard.vue'
import ConsumptionOrder from './ConsumptionOrder.vue'
import AllocationTable from './AllocationTable.vue'
import { orderedEntitlements, refKey, usd, type EntitlementItem } from './model'

const { t } = useI18n()
const store = useGroupBuyStore()
const authStore = useAuthStore()
const paymentEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.payment))
const loading = ref(false)
const error = ref('')
const allocations = ref<ChargeAllocation[]>([])
const cardItems = computed(() => orderedEntitlements(store.monthCards, [], store.entitlementOrders))
const monthCards = computed(() => cardItems.value.map(item => item.card!))
const sortableEntitlements = computed<EntitlementItem[]>(() => {
  const cards = new Map(cardItems.value.map(item => [`${item.group_id}:${refKey(item)}`, item]))
  // Keep every server reference in a card's billing group, including official subscriptions.
  // Sorting must not depend on the subscription feature or omit hidden entitlements.
  return store.entitlementOrders.filter(order => order.items.some(item => item.kind === 'card')).flatMap(order =>
    order.items.map(item => cards.get(`${order.group_id}:${refKey(item)}`) ?? {
      ...item,
      group_id: order.group_id,
      name: `${t(item.kind === 'legacy' ? 'payment.tabSubscribe' : 'groupBuy.cardCode')} #${item.id}`,
      expires_at: null,
    })
  )
})

async function refresh() {
  if (loading.value) return
  loading.value = true
  try {
    const [, charges] = await Promise.all([store.fetchMonthCards(true), groupBuyAPI.allocations()])
    allocations.value = charges
    error.value = ''
  } catch {
    error.value = t('groupBuy.loadFailed')
  } finally {
    loading.value = false
  }
}

function cardChanged(card: MonthCard) {
  store.updateMonthCard(card)
  void refresh()
}

let poller: ReturnType<typeof setInterval>
onMounted(() => {
  void refresh()
  poller = setInterval(() => {
    void refresh()
    authStore.refreshUser().catch(() => {})
  }, 30_000)
})
onUnmounted(() => clearInterval(poller))
</script>
