<template>
  <AppLayout>
    <div class="mx-auto w-full min-w-0 max-w-5xl space-y-8">
      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center gap-3 py-16">
        <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('luckyBox.loading') }}</p>
      </div>

      <!-- Load error -->
      <div v-else-if="loadError" class="py-16 text-center">
        <Icon name="exclamationCircle" size="xl" class="mx-auto text-red-500" />
        <p class="mt-3 text-sm text-gray-700 dark:text-dark-300">{{ loadError }}</p>
        <button class="btn btn-secondary mt-4" data-testid="box-retry-bootstrap" @click="fetchBootstrap">
          {{ t('luckyBox.retry') }}
        </button>
      </div>

      <!-- No campaign -->
      <div v-else-if="!campaign" class="empty-state py-16">
        <div class="empty-state-icon"><Icon name="gift" size="xl" class="text-gray-400 dark:text-dark-500" /></div>
        <p class="empty-state-title">{{ t('luckyBox.noCampaignTitle') }}</p>
        <p class="empty-state-description">{{ t('luckyBox.noCampaignDescription') }}</p>
      </div>

      <!-- Upcoming / ended -->
      <div v-else-if="!isCampaignActive" class="empty-state py-16">
        <div class="empty-state-icon">
          <Icon name="clock" size="xl" class="text-gray-400 dark:text-dark-500" />
        </div>
        <p class="empty-state-title">
          {{ campaignPhase === 'upcoming' ? t('luckyBox.upcomingTitle') : t('luckyBox.endedTitle') }}
        </p>
        <p class="mt-1 text-sm font-medium text-gray-700 dark:text-dark-300">{{ campaign.name }}</p>
        <p class="empty-state-description">
          {{ t('luckyBox.campaignWindow', { start: formatDateTime(campaign.startAt), end: formatDateTime(campaign.endAt) }) }}
        </p>
      </div>

      <!-- Active campaign -->
      <template v-else>
        <!-- Compact header -->
        <header class="flex items-start gap-3">
          <div
            class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400"
          >
            <Icon name="gift" size="md" />
          </div>
          <div class="min-w-0 flex-1">
            <h1 class="text-lg font-bold text-gray-900 dark:text-white">{{ campaign.name }}</h1>
            <p class="mt-0.5 flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400">
              <Icon name="clock" size="xs" />
              {{ t('luckyBox.deadline', { time: formatDateTime(campaign.endAt) }) }}
            </p>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
              {{ t('luckyBox.myCards') }}:
              <span class="font-semibold text-amber-600 dark:text-amber-400">{{ availableCards }}</span>
            </p>
          </div>
        </header>

        <BoxReel
          :key="userId"
          ref="reel"
          :prizes="prizes"
          :tier="selectedTier"
          :waiting="drawPending && !opening"
        />

        <!-- Tier selector + open -->
        <section>
          <div class="grid grid-cols-3 gap-2">
            <button
              v-for="tier in tiers"
              :key="tier.value"
              type="button"
              :disabled="tier.count <= 0 || hasUnresolvedDraw || drawPending"
              :aria-pressed="selectedTier === tier.value"
              :data-testid="`box-tier-${tier.value}`"
              :class="['rounded-lg border px-2 py-3 text-center transition-colors', tierButtonClass(tier.value)]"
              @click="selectedTier = tier.value"
            >
              <span class="flex items-center justify-center gap-1 text-xs">
                <Icon :name="tier.icon" size="xs" />
                {{ tier.label }}
              </span>
              <span class="mt-1 block text-xl font-bold">{{ tier.count }}</span>
            </button>
          </div>

          <button
            class="btn btn-primary mt-3 w-full py-3"
            data-testid="box-open"
            :disabled="drawPending || hasUnresolvedDraw || selectedTierCount <= 0"
            @click="startDraw"
          >
            <svg
              v-if="drawPending"
              class="-ml-1 mr-2 h-5 w-5 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            <Icon v-else name="cube" size="md" class="mr-2" />
            {{ drawPending ? t('luckyBox.opening') : t('luckyBox.open') }}
          </button>
          <p v-if="selectedTierCount <= 0" class="mt-2 text-xs text-gray-500 dark:text-dark-400">
            {{ t('luckyBox.noCards') }}
          </p>
        </section>

        <p v-if="opening" class="sr-only" data-testid="box-opening" role="status">{{ t('luckyBox.opening') }}</p>

        <!-- Prize pool -->
        <section>
          <h2
            class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-500 dark:text-dark-400"
          >
            <Icon name="sparkles" size="sm" />
            {{ t('luckyBox.prizesTitle') }}
          </h2>
          <div v-if="prizes.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('luckyBox.emptyPrizes') }}
          </div>
          <ul v-else class="space-y-2">
            <li
              v-for="prize in prizes"
              :key="prize.id"
              class="flex items-center gap-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-800"
              :data-testid="`box-prize-${prize.id}`"
            >
              <div
                class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg"
                :style="{ backgroundColor: `${rarityColor(prize.rarity)}1f` }"
              >
                <Icon name="gift" size="md" :style="{ color: rarityColor(prize.rarity) }" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="break-words text-sm font-medium text-gray-900 dark:text-white">
                  {{ prize.name }}
                  <span
                    class="ml-2 inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold"
                    :style="{ color: rarityColor(prize.rarity), backgroundColor: `${rarityColor(prize.rarity)}1a` }"
                  >
                    {{ t(`luckyBox.rarity${capitalize(prize.rarity)}`) }}
                  </span>
                </p>
                <p class="text-xs text-gray-500 dark:text-dark-400">
                  {{ prize.displayText }}
                  <template v-if="prize.type === 'balance' && prize.amount !== null">
                    · ${{ prize.amount.toFixed(2) }}
                  </template>
                </p>
              </div>
              <div class="w-24 flex-shrink-0 text-right">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('luckyBox.probability') }}</p>
                <p class="text-sm font-semibold" :style="{ color: rarityColor(prize.rarity) }">
                  {{ tierProbability(prize).toFixed(2) }}%
                </p>
                <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    class="h-full rounded-full transition-all duration-300"
                    :style="{ width: `${Math.min(100, tierProbability(prize))}%`, backgroundColor: rarityColor(prize.rarity) }"
                  ></div>
                </div>
              </div>
            </li>
          </ul>
        </section>

        <!-- History -->
        <section>
          <div class="mb-3 flex items-center justify-between">
            <h2
              class="flex items-center gap-2 text-sm font-semibold text-gray-500 dark:text-dark-400"
            >
              <Icon name="clock" size="sm" />
              {{ t('luckyBox.historyTitle') }}
            </h2>
            <button
              class="btn btn-secondary"
              data-testid="box-refresh"
              :disabled="loading || drawPending"
              @click="fetchBootstrap"
            >
              <Icon name="refresh" size="sm" />
              {{ t('luckyBox.refresh') }}
            </button>
          </div>
          <p v-if="draws.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('luckyBox.historyEmpty') }}
          </p>
          <ul v-else class="space-y-2">
            <li
              v-for="draw in draws"
              :key="draw.id"
              class="flex flex-col gap-2 rounded-lg bg-gray-50 p-3 dark:bg-dark-800 sm:flex-row sm:items-center sm:justify-between"
              :data-testid="`box-draw-${draw.id}`"
            >
              <div class="min-w-0">
                <p class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ draw.prize?.name ?? '—' }}
                  <span class="ml-2 text-xs font-normal text-gray-500 dark:text-dark-400">
                    {{ tierLabel(draw.cardTier) }}
                  </span>
                </p>
                <p class="text-xs text-gray-500 dark:text-dark-400">
                  {{ formatDateTime(draw.createdAt) }}
                </p>
              </div>
              <div class="flex flex-shrink-0 items-center gap-2">
                <span
                  class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold"
                  :class="statusBadgeClass(draw.status)"
                >
                  {{ statusLabel(draw.status) }}
                </span>
                <button
                  v-if="draw.status === 'delivery_failed'"
                  class="btn btn-secondary px-3 py-1 text-xs"
                  :data-testid="`box-retry-delivery-${draw.id}`"
                  :disabled="drawPending || hasUnresolvedDraw"
                  @click="retryDelivery(draw)"
                >
                  {{ t('luckyBox.retryDelivery') }}
                </button>
              </div>
            </li>
          </ul>
        </section>
      </template>
      <!-- Recovery is independent of the currently active campaign. -->
      <section v-if="(lastDraw || pendingRequest || drawFailedMessage || drawNotice) && !opening" class="space-y-3 border-t border-gray-200 py-6 text-center dark:border-dark-600" data-testid="box-result" aria-live="polite">
        <template v-if="lastDraw?.prize">
          <p class="text-sm font-semibold" :style="{ color: rarityColor(lastDraw.prize.rarity) }">{{ t(`luckyBox.rarity${capitalize(lastDraw.prize.rarity)}`) }}</p>
          <p class="break-words text-xl font-semibold text-gray-900 dark:text-white">{{ lastDraw.prize.name }}</p>
          <p v-if="lastDraw.prize.type === 'balance' && lastDraw.prize.amount !== null" class="text-lg font-semibold text-cyan-600 dark:text-cyan-400">${{ lastDraw.prize.amount.toFixed(2) }}</p>
          <p class="text-sm text-gray-500">{{ statusLabel(lastDraw.status) }}</p>
          <button v-if="lastDraw.status === 'delivery_failed' || lastDraw.status === 'selected'" class="btn btn-secondary" :disabled="drawPending || hasUnresolvedDraw" @click="retryDelivery(lastDraw)">{{ t('luckyBox.retryDelivery') }}</button>
        </template>
        <p v-if="drawFailedMessage" class="text-sm text-red-600 dark:text-red-400" data-testid="box-draw-error">{{ drawFailedMessage }}</p>
        <p v-if="drawNotice" class="text-sm text-gray-600 dark:text-dark-300">{{ drawNotice }}</p>
        <template v-if="pendingRequest">
          <p class="text-sm text-amber-700 dark:text-amber-400">{{ t('luckyBox.unresolvedDraw') }}</p>
          <button class="btn btn-secondary" data-testid="box-retry-draw" :disabled="drawPending" @click="retryDraw">{{ t('luckyBox.retryDraw') }}</button>
        </template>
      </section>
    </div>
    <BaseDialog :show="showResultPopup && !!lastDraw?.prize" :title="revealTitle" width="narrow" @close="showResultPopup = false">
      <div v-if="lastDraw?.prize" class="box-reveal" :style="{ '--reward-color': rarityColor(lastDraw.prize.rarity) }" data-testid="box-reveal">
        <p class="box-reveal__rarity">{{ t(`luckyBox.rarity${capitalize(lastDraw.prize.rarity)}`) }}</p>
        <div class="box-reveal__reward">
          <span v-if="lastDraw.prize.type === 'balance' && lastDraw.prize.amount !== null" class="box-reveal__amount">${{ lastDraw.prize.amount.toFixed(2) }}</span>
          <Icon v-else :name="lastDraw.prize.type === 'extra_draw' ? 'refresh' : 'gift'" size="xl" class="h-20 w-20" />
        </div>
        <p class="break-words text-xl font-bold text-gray-900 dark:text-white">{{ lastDraw.prize.name }}</p>
        <p class="mt-3 text-sm" :class="lastDraw.status === 'delivery_failed' ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-dark-300'">{{ statusLabel(lastDraw.status) }}</p>
      </div>
      <template #footer>
        <button class="btn btn-primary w-full" data-testid="box-reveal-close" @click="showResultPopup = false">
          <Icon name="check" size="sm" class="mr-2" />
          {{ lastDraw?.status === 'delivered' ? t('luckyBox.resultClose') : t('luckyBox.resultDismiss') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getBoxBootstrap,
  createBoxDraw,
  recoverBoxDraw,
  type BoxBootstrap,
  type BoxCardTier,
  type BoxDraw,
  type BoxPrize,
  type BoxRarity,
  type BoxDrawRequest,
  type BoxDrawResponse,
} from '@/api/luckyBox'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BoxReel from '@/components/luckyBox/BoxReel.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const userId = computed(() => (authStore.user ? String(authStore.user.id) : ''))

const bootstrap = ref<BoxBootstrap | null>(null)
const loading = ref(false)
const loadError = ref('')
let bootstrapController: AbortController | null = null

const campaign = computed(() => bootstrap.value?.campaign ?? null)
const availableCards = ref(0)
const availableCardsByTier = ref<Record<BoxCardTier, number>>({ low: 0, medium: 0, high: 0 })
const prizes = computed<BoxPrize[]>(() => bootstrap.value?.prizes ?? [])
const draws = ref<BoxDraw[]>([])

const selectedTier = ref<BoxCardTier>('low')
const reel = ref<InstanceType<typeof BoxReel>>()

const TIER_ICONS: Record<BoxCardTier, 'sparkles' | 'fire' | 'trophy'> = {
  low: 'sparkles',
  medium: 'fire',
  high: 'trophy',
}

const tiers = computed(() => [
  { value: 'low' as BoxCardTier, label: t('luckyBox.tierLow'), count: availableCardsByTier.value.low, icon: TIER_ICONS.low },
  { value: 'medium' as BoxCardTier, label: t('luckyBox.tierMedium'), count: availableCardsByTier.value.medium, icon: TIER_ICONS.medium },
  { value: 'high' as BoxCardTier, label: t('luckyBox.tierHigh'), count: availableCardsByTier.value.high, icon: TIER_ICONS.high },
])

const selectedTierCount = computed(() => availableCardsByTier.value[selectedTier.value])

const TIER_ACTIVE_CLASS: Record<BoxCardTier, string> = {
  low: 'border-emerald-500 bg-emerald-50 font-semibold text-emerald-700 dark:border-emerald-500 dark:bg-emerald-900/30 dark:text-emerald-400',
  medium: 'border-cyan-500 bg-cyan-50 font-semibold text-cyan-700 dark:border-cyan-500 dark:bg-cyan-900/30 dark:text-cyan-400',
  high: 'border-amber-500 bg-amber-50 font-semibold text-amber-700 dark:border-amber-500 dark:bg-amber-900/30 dark:text-amber-400',
}
const TIER_IDLE_CLASS: Record<BoxCardTier, string> = {
  low: 'border-gray-200 text-gray-600 hover:border-emerald-300 hover:bg-emerald-50/60 dark:border-dark-600 dark:text-dark-300 dark:hover:border-emerald-700',
  medium: 'border-gray-200 text-gray-600 hover:border-cyan-300 hover:bg-cyan-50/60 dark:border-dark-600 dark:text-dark-300 dark:hover:border-cyan-700',
  high: 'border-gray-200 text-gray-600 hover:border-amber-300 hover:bg-amber-50/60 dark:border-dark-600 dark:text-dark-300 dark:hover:border-amber-700',
}

const tierButtonClass = (tier: BoxCardTier) =>
  selectedTier.value === tier ? TIER_ACTIVE_CLASS[tier] : TIER_IDLE_CLASS[tier]

// Campaign phase, driven by a ticking clock so an active page flips to
// upcoming/ended at the exact boundary.
const now = ref(Date.now())
const campaignPhase = computed<'upcoming' | 'active' | 'ended'>(() => {
  const c = campaign.value
  if (!c) return 'ended'
  const start = Date.parse(c.startAt)
  const end = Date.parse(c.endAt)
  if (c.status !== 'active') return ['upcoming', 'scheduled'].includes(c.status) ? 'upcoming' : 'ended'
  if (Number.isFinite(start) && now.value < start) return 'upcoming'
  if (Number.isFinite(end) && now.value >= end) return 'ended'
  return 'active'
})
const isCampaignActive = computed(() => campaignPhase.value === 'active')

// Draw state
const drawPending = ref(false)
const opening = ref(false)
const lastDraw = ref<BoxDraw | null>(null)
const showResultPopup = ref(false)
const revealTitle = computed(() => {
  const draw = lastDraw.value
  if (!draw) return ''
  if (draw.status !== 'delivered') return statusLabel(draw.status)
  return t(draw.prize?.type === 'empty' ? 'luckyBox.prizeEmpty' : 'luckyBox.resultTitle')
})
const drawFailedMessage = ref('')
const drawNotice = ref('')
const pendingRequest = ref<BoxDrawRequest | null>(null)
const hasUnresolvedDraw = computed(() => pendingRequest.value !== null)
let drawController: AbortController | null = null
let clockTimer: ReturnType<typeof setInterval> | null = null

// A draw whose outcome is unknown is persisted before the request is sent so a
// reload can resume the exact same idempotent requestId instead of starting a
// new draw. Scoped by authenticated user id; never resumed for another user.
const pendingStorageKey = (uid: string) => `sub2api_lucky_box_pending_draw:${uid}`

interface StoredPendingDraw {
  userId: string
  campaignId: string
  requestId: string
  cardTier: BoxCardTier
}

const readStoredPending = (uid: string): StoredPendingDraw | null => {
  try {
    const raw = localStorage.getItem(pendingStorageKey(uid))
    if (!raw) return null
    const parsed = JSON.parse(raw) as StoredPendingDraw
    if (parsed.userId !== uid) return null
    if (typeof parsed.requestId !== 'string' || typeof parsed.campaignId !== 'string') return null
    if (!['low', 'medium', 'high'].includes(parsed.cardTier)) return null
    return parsed
  } catch {
    return null
  }
}

const writeStoredPending = (entry: StoredPendingDraw): boolean => {
  try {
    localStorage.setItem(pendingStorageKey(entry.userId), JSON.stringify(entry))
    return true
  } catch {
    return false
  }
}

const clearStoredPending = (uid: string) => {
  try {
    localStorage.removeItem(pendingStorageKey(uid))
  } catch {
    // Ignore storage errors.
  }
}

const RARITY_COLORS: Record<BoxRarity, string> = {
  common: '#94a3b8',
  rare: '#38bdf8',
  epic: '#a855f7',
  legendary: '#f59e0b',
}

const rarityColor = (rarity: BoxRarity | undefined) =>
  rarity ? RARITY_COLORS[rarity] : RARITY_COLORS.common

const capitalize = (value: string) => value.charAt(0).toUpperCase() + value.slice(1)

const tierProbability = (prize: BoxPrize) =>
  prize.probabilities?.[selectedTier.value] ?? prize.probability

const tierLabel = (tier: BoxCardTier) =>
  tier === 'low' ? t('luckyBox.tierLow') : tier === 'medium' ? t('luckyBox.tierMedium') : t('luckyBox.tierHigh')

const statusLabel = (status: BoxDraw['status']) => {
  switch (status) {
    case 'selected':
      return t('luckyBox.statusSelected')
    case 'delivered':
      return t('luckyBox.statusDelivered')
    case 'delivery_failed':
      return t('luckyBox.statusDeliveryFailed')
    case 'voided':
      return t('luckyBox.statusVoided')
  }
}

const statusBadgeClass = (status: BoxDraw['status']) =>
  status === 'delivered'
    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
    : status === 'delivery_failed'
      ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
      : status === 'voided'
        ? 'bg-gray-200 text-gray-600 dark:bg-dark-700 dark:text-dark-400'
        : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'

const upsertDraw = (draw: BoxDraw) => {
  const index = draws.value.findIndex((d) => d.id === draw.id)
  if (index >= 0) {
    draws.value.splice(index, 1, draw)
  } else {
    draws.value.unshift(draw)
  }
}

const applyDrawResponse = (response: { draw: BoxDraw; availableCards: number; availableCardsByTier: Record<BoxCardTier, number> }) => {
  availableCards.value = response.availableCards
  availableCardsByTier.value = response.availableCardsByTier
  upsertDraw(response.draw)
}

const pickDefaultTier = (byTier: Record<BoxCardTier, number>): BoxCardTier => {
  if (byTier.high > 0) return 'high'
  if (byTier.medium > 0) return 'medium'
  return 'low'
}

// After a reload, a persisted pending draw either already has its recorded
// outcome in bootstrap.draws (recover it, nothing to retry) or is genuinely
// unresolved (surface a resume UI — never auto-retry).
const recoverPendingDraw = () => {
  const uid = userId.value
  if (!uid || !bootstrap.value) return
  const stored = readStoredPending(uid)
  if (!stored) return
  const recorded = draws.value.find((d) => d.requestId === stored.requestId && d.campaignId === stored.campaignId && d.userId === uid)
  if (recorded) {
    if (['delivered', 'voided'].includes(recorded.status)) {
      pendingRequest.value = null
      clearStoredPending(uid)
    } else {
      pendingRequest.value = { requestId: stored.requestId, cardTier: stored.cardTier, campaignId: stored.campaignId }
    }
    lastDraw.value = recorded
    drawFailedMessage.value = ''
    return
  }
  pendingRequest.value = { requestId: stored.requestId, cardTier: stored.cardTier, campaignId: stored.campaignId }
  drawFailedMessage.value = ''
}

const fetchBootstrap = async () => {
  if (drawPending.value) return
  bootstrapController?.abort()
  bootstrapController = new AbortController()
  const signal = bootstrapController.signal
  const uid = userId.value
  loading.value = true
  loadError.value = ''
  try {
    const data = await getBoxBootstrap(signal)
    if (signal.aborted || userId.value !== uid) return
    bootstrap.value = data
    availableCards.value = data.availableCards
    availableCardsByTier.value =
      data.availableCardsByTier ?? { low: data.availableCards, medium: 0, high: 0 }
    draws.value = data.draws
    selectedTier.value = pickDefaultTier(availableCardsByTier.value)
    recoverPendingDraw()
  } catch (error) {
    if (signal.aborted) return
    loadError.value =
      (error as { message?: string })?.message || t('luckyBox.loadFailed')
  } finally {
    if (!signal.aborted) loading.value = false
  }
}

const validDrawResponse = (value: BoxDrawResponse, payload: BoxDrawRequest, uid: string) => {
  const d = value?.draw
  return !!d && typeof d.id === 'string' && d.id.length > 0 && d.requestId === payload.requestId &&
    d.campaignId === payload.campaignId && d.cardTier === payload.cardTier && d.userId === uid &&
    ['selected', 'delivered', 'delivery_failed', 'voided'].includes(d.status) && !!d.prize &&
    typeof d.prize.name === 'string' && ['common', 'rare', 'epic', 'legendary'].includes(d.prize.rarity) &&
    ['balance', 'extra_draw', 'empty'].includes(d.prize.type) &&
    (d.prize.type !== 'balance' || (typeof d.prize.amount === 'number' && Number.isFinite(d.prize.amount) && d.prize.amount >= 0)) &&
    Number.isInteger(value.availableCards) && value.availableCards >= 0 &&
    (['low', 'medium', 'high'] as const).every(tier => Number.isInteger(value.availableCardsByTier?.[tier]) && value.availableCardsByTier[tier] >= 0)
}

const sendDraw = async (payload: BoxDrawRequest, recovery = false) => {
  now.value = Date.now()
  const uid = userId.value
  if (!uid || drawPending.value || (!recovery && (!isCampaignActive.value || payload.campaignId !== campaign.value?.id))) return
  drawController?.abort()
  drawController = new AbortController()
  const signal = drawController.signal
  drawPending.value = true
  drawFailedMessage.value = ''
  drawNotice.value = ''
  showResultPopup.value = false
  lastDraw.value = null
  opening.value = false
  try {
    let response: BoxDrawResponse
    if (recovery) {
      const recovered = await recoverBoxDraw(payload, signal)
      if (signal.aborted || userId.value !== uid) return
      if (recovered.found) response = recovered
      else {
        if (recovered.resolution) {
          if (recovered.resolution !== 'closed_without_draw' || recovered.userId !== uid ||
            recovered.requestId !== payload.requestId || recovered.campaignId !== payload.campaignId ||
            recovered.cardTier !== payload.cardTier) throw new Error(t('luckyBox.invalidDraw'))
          clearStoredPending(uid)
          pendingRequest.value = null
          drawNotice.value = t('luckyBox.closedWithoutDraw')
          return
        }
        now.value = Date.now()
        if (!isCampaignActive.value || campaign.value?.id !== payload.campaignId) throw new Error(t('luckyBox.noRecordedDraw'))
        response = await createBoxDraw(payload, signal)
      }
    } else response = await createBoxDraw(payload, signal)
    if (signal.aborted || userId.value !== uid) return
    if (!validDrawResponse(response, payload, uid)) throw new Error(t('luckyBox.invalidDraw'))
    // Keep recovery durable until the fixed reward reaches a terminal state.
    if (['delivered', 'voided'].includes(response.draw.status)) {
      pendingRequest.value = null
      clearStoredPending(uid)
    } else pendingRequest.value = payload
    if (!recovery && response.draw.status !== 'voided' && reel.value) {
      opening.value = true
      await reel.value.play(response.draw)
      if (signal.aborted || userId.value !== uid) return
    }
    if (response.draw.campaignId === campaign.value?.id) applyDrawResponse(response)
    lastDraw.value = response.draw
    opening.value = false
    showResultPopup.value = !recovery && response.draw.status !== 'voided'
  } catch (error) {
    if (signal.aborted || userId.value !== uid) return
    // Outcome unknown: keep the same requestId (in memory and in storage) and
    // only offer an idempotent retry — never a new draw.
    pendingRequest.value = payload
    lastDraw.value = null
    opening.value = false
    drawFailedMessage.value =
      (error as { message?: string })?.message || t('luckyBox.loadFailed')
  } finally {
    if (!signal.aborted) drawPending.value = false
  }
}

const startDraw = () => {
  now.value = Date.now()
  if (pendingRequest.value || drawPending.value) return
  if (selectedTierCount.value <= 0 || !isCampaignActive.value) return
  const uid = userId.value
  const campaignId = campaign.value?.id
  if (!uid || !campaignId) return
  const prior = readStoredPending(uid)
  if (prior) {
    pendingRequest.value = { requestId: prior.requestId, cardTier: prior.cardTier, campaignId: prior.campaignId }
    return
  }
  const payload = { requestId: `box-${crypto.randomUUID()}`, cardTier: selectedTier.value, campaignId }
  if (!writeStoredPending({ userId: uid, ...payload })) {
    drawFailedMessage.value = t('luckyBox.storageUnavailable')
    return
  }
  reel.value?.prepareSound()
  pendingRequest.value = payload
  void sendDraw(payload)
}

const retryDraw = () => {
  if (!pendingRequest.value || drawPending.value) return
  void sendDraw(pendingRequest.value, true)
}

const retryDelivery = (draw: BoxDraw) => {
  if (drawPending.value || pendingRequest.value) return
  if (draw.userId !== userId.value) return
  const payload = { requestId: draw.requestId, cardTier: draw.cardTier, campaignId: draw.campaignId }
  if (!writeStoredPending({ userId: userId.value, ...payload })) {
    drawFailedMessage.value = t('luckyBox.storageUnavailable')
    return
  }
  pendingRequest.value = payload
  void sendDraw(payload, true)
}

// Account switch: never leak another user's pending request, result or retry.
watch(userId, (next, previous) => {
  if (previous === undefined || next === previous) return
  drawController?.abort()
  drawPending.value = false
  opening.value = false
  lastDraw.value = null
  showResultPopup.value = false
  drawFailedMessage.value = ''
  drawNotice.value = ''
  pendingRequest.value = null
  bootstrap.value = null
  draws.value = []
  availableCards.value = 0
  availableCardsByTier.value = { low: 0, medium: 0, high: 0 }
  if (next) void fetchBootstrap()
}, { flush: 'sync' })

onMounted(() => {
  void fetchBootstrap()
  clockTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  bootstrapController?.abort()
  drawController?.abort()
  if (clockTimer !== null) {
    clearInterval(clockTimer)
    clockTimer = null
  }
})
</script>

<style scoped>
.box-reveal { padding: 12px 0 20px; text-align: center; }
.box-reveal__rarity { color: var(--reward-color); font-size: 13px; font-weight: 700; }
.box-reveal__reward {
  display: flex; align-items: center; justify-content: center; min-height: 152px; margin: 16px 0;
  border-top: 2px solid var(--reward-color); border-bottom: 2px solid var(--reward-color);
  background: color-mix(in srgb, var(--reward-color) 10%, transparent); color: var(--reward-color);
  animation: reward-arrive 300ms ease-out;
}
.box-reveal__amount { max-width: 100%; overflow-wrap: anywhere; font-size: 44px; line-height: 1.2; font-weight: 800; }
@keyframes reward-arrive { from { opacity: 0; transform: scale(0.92); } to { opacity: 1; transform: scale(1); } }
@media (prefers-reduced-motion: reduce) { .box-reveal__reward { animation: none; } }
</style>
