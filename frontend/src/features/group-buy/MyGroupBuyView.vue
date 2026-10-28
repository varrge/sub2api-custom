<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6 pb-8">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-bold">{{ t('groupBuy.myGroupBuy') }}</h1>
          <p class="mt-1 text-sm text-gray-500">{{ t('groupBuy.myGroupBuyDescription') }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <RouterLink v-if="paymentEnabled" to="/purchase?tab=group-buy" class="btn btn-primary">{{ t('groupBuy.purchase') }}</RouterLink>
          <RouterLink v-if="paymentEnabled" to="/group-buy" class="btn btn-secondary">{{ t('groupBuy.hall') }}</RouterLink>
          <button class="btn btn-secondary" :disabled="loading || saving || freezing || dragging" @click="refresh">{{ t('common.refresh') }}</button>
        </div>
      </div>
      <p v-if="(auth.user?.balance ?? 0) < 0" class="gb-notice gb-notice-error p-4" role="alert">
        {{ t('groupBuy.debt', { amount: usd(auth.user?.balance ?? 0) }) }}
        <RouterLink v-if="paymentEnabled" to="/purchase?tab=recharge" class="underline">{{ t('payment.tabTopUp') }}</RouterLink>
      </p>
      <p v-if="error" class="text-sm text-red-600" role="alert">{{ error }}</p>
      <div v-if="loading && !allCards.length" class="py-12 text-center text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="!cards.length && !error" class="gb-panel p-12 text-center text-gray-500" data-testid="wallet-empty">{{ t('groupBuy.wallet.empty') }}</div>
      <MonthCardWallet v-if="cards.length" :cards="cards" :selected="selected" :groups="groups" :holder="holder" :brand="app.siteName || 'Private'"
        :appearance="appearance" :now="now" :saving="saving" :freezing="freezing" :loading="loading" :order-error="orderError"
        :order-saved="orderSaved" :freeze-error="freezeError" @select="selectedId = $event" @reorder="reorder" @freeze="freeze" @dragging="dragging = $event" />
      <div v-if="allocations.length || allCards.length" class="space-y-3">
        <p class="text-sm text-gray-500">{{ t('groupBuy.sharedBillingHint') }}</p>
        <AllocationTable :allocations="allocations" :cards="allCards" />
      </div>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import './glass.css'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import AllocationTable from './AllocationTable.vue'
import MonthCardWallet from './MonthCardWallet.vue'
import { defaultMonthCardAppearance, type MonthCardAppearance } from './cardAppearance'
import { useMonthCardWallet } from './useMonthCardWallet'
import { usd } from './model'

withDefaults(defineProps<{ appearance?: MonthCardAppearance }>(), { appearance: () => defaultMonthCardAppearance })
const { t } = useI18n()
const auth = useAuthStore(), app = useAppStore()
const paymentEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.payment))
const holder = computed(() => auth.user?.username || auth.user?.email || '—')
const { allCards, cards, groups, allocations, selected, selectedId, now, loading, saving, freezing,
  error, orderError, orderSaved, freezeError, dragging, refresh, reorder, freeze } = useMonthCardWallet()
</script>
