<!-- frontend/src/features/group-buy/MonthCardWallet.vue -->
<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonthCard, EntitlementRef } from '@/types/groupBuy'
import type { WalletGroup } from './walletModel'
import type { MonthCardAppearance } from './cardAppearance'
import MonthCardFace from './MonthCardFace.vue'
import MonthCardWalletOrder from './MonthCardWalletOrder.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { cny, exactDate } from './model'
import './month-card-wallet.css'

const props = defineProps<{
  selected: MonthCard | null
  cards: MonthCard[]
  groups: WalletGroup[]
  holder: string
  brand: string
  appearance: MonthCardAppearance
  now: number
  loading: boolean
  saving: boolean
  freezing: boolean
  orderError: string
  orderSaved: boolean
  freezeError: string
}>()

const emit = defineEmits<{
  (e: 'select', id: number): void
  (e: 'reorder', groupId: number, items: EntitlementRef[]): void
  (e: 'freeze', card: MonthCard): void
  (e: 'dragging', value: boolean): void
}>()

const { t } = useI18n()

const detailsOpen = ref(false)
watch(() => props.selected?.id, () => { detailsOpen.value = false })

const isFrozen = computed(() => props.selected?.status === 'frozen')

</script>

<template>
  <div class="mcw-container">
    <section v-if="selected" class="mcw-hero" :aria-label="t('groupBuy.wallet.cardDetails')">
      <MonthCardFace :card="selected" :holder="holder" :brand="brand" :appearance="appearance" :now="now" />
      <div class="mcw-card-toolbar">
        <p v-if="selected.freeze_allowed !== false" class="mcw-freeze-help">{{ t('groupBuy.freezeHelp') }}</p>
        <div class="mcw-actions">
          <button v-if="selected.freeze_allowed !== false" class="mcw-btn mcw-btn-outline" :disabled="saving || freezing || loading"
            data-testid="wallet-freeze" @click="emit('freeze', selected)">{{ isFrozen ? t('groupBuy.thaw') : t('groupBuy.freeze') }}</button>
          <button class="mcw-btn mcw-btn-primary" data-testid="wallet-details" @click="detailsOpen = true">{{ t('groupBuy.wallet.cardDetails') }}</button>
        </div>
      </div>
      <div v-if="freezeError" class="mcw-error-text" role="alert">{{ freezeError }}</div>
    </section>

    <!-- Wallet Strip Section -->
    <div class="mcw-strip">
      <div class="mcw-strip-header">
        <h3 class="mcw-strip-title">{{ t('groupBuy.wallet.cardPack', { count: cards.length }) }}</h3>
        <span class="mcw-strip-hint">{{ t('groupBuy.wallet.dragHint') }}</span>

        <div class="mcw-strip-status" role="status" aria-live="polite">
          <span v-if="saving" class="mcw-status-saving">{{ t('groupBuy.wallet.saving') }}</span>
          <span v-else-if="orderError" class="mcw-status-error" role="alert">{{ orderError }}</span>
          <span v-else-if="orderSaved" class="mcw-status-saved">{{ t('groupBuy.wallet.saved') }}</span>
        </div>
      </div>

      <MonthCardWalletOrder
        v-if="cards.length > 0"
        :groups="groups"
        :selected-id="selected?.id ?? null"
        :saving="saving || freezing || loading"
        @select="emit('select', $event)"
        @reorder="(groupId, items) => emit('reorder', groupId, items)"
        @dragging="emit('dragging', $event)"
      />
    </div>

    <!-- Details Dialog -->
    <BaseDialog
      :show="detailsOpen" @close="detailsOpen = false"
      :title="t('groupBuy.wallet.cardDetails')"

    >
      <div v-if="selected" class="mcw-details-list" data-testid="wallet-details-panel">
        <p class="mcw-detail-hint">{{ t('groupBuy.wallet.identityHint') }}</p>
        <div class="mcw-detail-item">
          <span class="mcw-detail-label">{{ t('groupBuy.cardCode') }}</span>
          <span class="mcw-detail-value mcw-mono">{{ selected.code }}</span>
        </div>
        <div class="mcw-detail-item">
          <span class="mcw-detail-label">{{ t('groupBuy.teamCode') }}</span>
          <RouterLink v-if="selected.team_code" class="mcw-detail-value mcw-mono underline" :to="{ path: '/group-buy', query: { team_code: selected.team_code } }">{{ selected.team_code }}</RouterLink>
          <span v-else class="mcw-detail-value">{{ t('groupBuy.solo') }}</span>
        </div>
        <div class="mcw-detail-item">
          <span class="mcw-detail-label">{{ t('groupBuy.obtained') }}</span>
          <span class="mcw-detail-value">{{ exactDate(selected.starts_at) }}</span>
        </div>
        <div class="mcw-detail-item">
          <span class="mcw-detail-label">{{ t('groupBuy.expires') }}</span>
          <span class="mcw-detail-value">{{ isFrozen ? t('groupBuy.frozenExpiry') : exactDate(selected.expires_at) }}</span>
        </div>
        <div v-if="!isFrozen" class="mcw-detail-item">
          <span class="mcw-detail-label">{{ t('groupBuy.wallet.cycle') }}</span>
          <span class="mcw-detail-value">{{ exactDate(selected.weekly_window_start) }} — {{ exactDate(selected.weekly_window_end) }}</span>
        </div>

        <div class="mcw-divider"></div>

        <div class="mcw-detail-item mcw-price-item">
          <span class="mcw-detail-label">{{ t('groupBuy.wallet.originalPrice') }}</span>
          <span class="mcw-detail-value">
            {{ selected.original_price_cny != null ? cny(selected.original_price_cny) : t('groupBuy.wallet.noPrice') }}
          </span>
        </div>
        <div class="mcw-detail-item mcw-price-item">
          <span class="mcw-detail-label">{{ t('groupBuy.wallet.purchasePrice') }}</span>
          <span class="mcw-detail-value">
            {{ selected.purchase_price_cny != null ? cny(selected.purchase_price_cny) : t('groupBuy.wallet.noPrice') }}
          </span>
        </div>
        <div class="mcw-detail-hint">{{ t('groupBuy.wallet.feeHint') }}</div>
      </div>
    </BaseDialog>
  </div>
</template>
