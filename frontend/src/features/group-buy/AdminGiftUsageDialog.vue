<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import { useAuthStore } from '@/stores/auth'
import type { AdminMonthCard, ChargeAllocation, EntitlementValidity, GiftCardBatch } from '@/types/groupBuy'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { exactDate, progress, usd } from './model'

const props = defineProps<{ batch: GiftCardBatch }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const auth = useAuthStore()
const pageSize = 20
const cards = ref<AdminMonthCard[]>([]), page = ref(1), total = ref(0)
const search = ref(''), validity = ref<EntitlementValidity>('all')
const loading = ref(false), error = ref('')
const selectedCard = ref<AdminMonthCard | null>(null)
const allocations = ref<ChargeAllocation[]>([]), detailPage = ref(1), detailTotal = ref(0)
const detailLoading = ref(false), detailError = ref('')
let cardsRequest: AbortController | undefined, detailRequest: AbortController | undefined
let appliedSearch = '', appliedValidity: EntitlementValidity = 'all'
const remaining = (card: AdminMonthCard) => Math.max(0, card.total_quota_usd - card.total_used_usd)

async function loadCards(nextPage = 1) {
  cardsRequest?.abort()
  const request = cardsRequest = new AbortController()
  const query = search.value.trim()
  if (query !== appliedSearch || validity.value !== appliedValidity) nextPage = 1
  appliedSearch = query; appliedValidity = validity.value
  page.value = nextPage; loading.value = true; error.value = ''; cards.value = []; total.value = 0
  try {
    const result = await adminGroupBuyAPI.giftCards(props.batch.id, { page: nextPage, page_size: pageSize, validity: appliedValidity, search: query }, request.signal)
    if (request.signal.aborted) return
    cards.value = result.items; total.value = result.total
  } catch (err) {
    if (!request.signal.aborted) error.value = extractApiErrorMessage(err, t('groupBuy.gifts.usageLoadFailed'))
  } finally { if (!request.signal.aborted) loading.value = false }
}
function inspect(card: AdminMonthCard) {
  selectedCard.value = card
  void loadAllocations(1)
}
function back() {
  detailRequest?.abort()
  selectedCard.value = null; allocations.value = []; detailTotal.value = 0; detailError.value = ''; detailLoading.value = false
}
async function loadAllocations(nextPage = 1) {
  if (!selectedCard.value) return
  detailRequest?.abort()
  const request = detailRequest = new AbortController()
  detailPage.value = nextPage; detailLoading.value = true; detailError.value = ''; allocations.value = []; detailTotal.value = 0
  try {
    const result = await adminGroupBuyAPI.giftAllocations(props.batch.id, selectedCard.value.id, nextPage, request.signal)
    if (request.signal.aborted) return
    allocations.value = result.items; detailTotal.value = result.total
    selectedCard.value = { ...selectedCard.value, ...result.card }
    cards.value = cards.value.map(card => card.id === result.card.id ? { ...card, ...result.card } : card)
  } catch (err) {
    if (!request.signal.aborted) detailError.value = extractApiErrorMessage(err, t('groupBuy.gifts.usageLoadFailed'))
  } finally { if (!request.signal.aborted) detailLoading.value = false }
}
watch(() => props.batch.id, () => {
  back(); search.value = ''; validity.value = 'all'
  void loadCards(1)
}, { immediate: true })
watch(() => auth.user?.id, () => {
  cardsRequest?.abort(); back(); cards.value = []; total.value = 0; error.value = ''; loading.value = false
  emit('close')
}, { flush: 'sync' })
onBeforeUnmount(() => { cardsRequest?.abort(); detailRequest?.abort() })
</script>
<template>
  <BaseDialog
    :show="true"
    :title="t('groupBuy.gifts.usageTitle')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="flex flex-col gap-4 text-sm text-gray-700 dark:text-gray-200">
      <header class="rounded-lg border border-gray-200 bg-gray-50/75 p-3 dark:border-gray-700 dark:bg-gray-800/50 sm:p-4">
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4 text-xs sm:text-sm">
          <div>
            <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.batchName') }}</span>
            <span class="font-medium break-all">{{ batch.name }} <span class="text-xs text-gray-400">#{{ batch.id }}</span></span>
          </div>
          <div>
            <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.groupName') }}</span>
            <span class="font-medium break-all">{{ batch.group_name || '-' }}</span>
          </div>
          <div>
            <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.recipientsAndQuota') }}</span>
            <span class="font-medium">{{ batch.recipient_count }} {{ t('groupBuy.gifts.people') }} / {{ usd(Number(batch.quota_usd)) }}</span>
          </div>
          <div>
            <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.batchExpiry') }}</span>
            <span class="font-medium">{{ batch.expires_at ? exactDate(batch.expires_at) : t('groupBuy.gifts.neverExpires') }}</span>
          </div>
        </div>
      </header>
      <section v-if="selectedCard" class="flex flex-col gap-4">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 pb-3 dark:border-gray-700">
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="btn btn-secondary btn-sm inline-flex items-center gap-1"
              data-testid="gift-usage-back"
              @click="back"
            >
              <Icon name="arrowLeft" class="h-4 w-4" />
              <span>{{ t('groupBuy.gifts.backToList') }}</span>
            </button>
            <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">{{ t('groupBuy.gifts.cardDetailTitle') }}</h3>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm inline-flex items-center gap-1"
            data-testid="gift-detail-refresh"
            :disabled="detailLoading"
            @click="loadAllocations(detailPage)"
          >
            <Icon name="refresh" class="h-4 w-4" :class="{ 'animate-spin': detailLoading }" />
            <span>{{ t('groupBuy.gifts.refresh') }}</span>
          </button>
        </div>
        <div class="grid grid-cols-1 gap-3 rounded-lg border border-gray-200 bg-gray-50/50 p-3 sm:grid-cols-3 dark:border-gray-700 dark:bg-gray-800/30">
          <div>
            <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.customer') }}</span>
            <span class="font-medium break-all">{{ selectedCard.username || selectedCard.email }}</span>
            <span class="block text-xs text-gray-400">#{{ selectedCard.user_id }}</span>
          </div>
          <div>
            <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.cardCode') }}</span>
            <span class="font-mono font-medium break-all">{{ selectedCard.code }}</span>
          </div>
          <div>
            <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.usedAndTotal') }}</span>
            <span class="font-medium">{{ usd(selectedCard.total_used_usd) }} / {{ usd(selectedCard.total_quota_usd) }}</span>
            <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">
              ({{ t('groupBuy.gifts.remaining') }}: {{ usd(remaining(selectedCard)) }})
            </span>
          </div>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('groupBuy.gifts.detailNotice') }}
        </p>
        <div v-if="detailLoading" role="status" class="py-8 text-center text-sm text-gray-500">
          <Icon name="refresh" class="mx-auto mb-2 h-5 w-5 animate-spin" />
          <span>{{ t('groupBuy.gifts.loading') }}</span>
        </div>
        <div v-else-if="detailError" role="alert" class="rounded-lg border border-red-200 bg-red-50 p-4 text-center text-sm text-red-600 dark:border-red-800 dark:bg-red-950/40 dark:text-red-400">
          <p>{{ detailError }}</p>
          <button
            type="button"
            class="btn btn-secondary btn-sm mt-2"
            data-testid="gift-detail-retry"
            @click="loadAllocations(detailPage)"
          >
            {{ t('groupBuy.gifts.retry') }}
          </button>
        </div>
        <div v-else-if="allocations.length > 0" class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
          <table class="gb-table w-full text-left text-xs sm:text-sm">
            <thead>
              <tr class="border-b border-gray-200 bg-gray-50 text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400">
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap">{{ t('groupBuy.gifts.callTime') }}</th>
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap">{{ t('groupBuy.gifts.requestId') }}</th>
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap">{{ t('groupBuy.gifts.apiKey') }}</th>
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap text-right">{{ t('groupBuy.gifts.allocationAmount') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
              <tr v-for="item in allocations" :key="item.id" class="hover:bg-gray-50/50 dark:hover:bg-gray-800/50">
                <td class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap text-gray-600 dark:text-gray-300">
                  {{ exactDate(item.started_at || item.created_at) }}
                </td>
                <td class="px-3 py-2 sm:px-4 sm:py-3 font-mono text-xs break-all max-w-[160px] sm:max-w-xs">
                  {{ item.request_id || '-' }}
                </td>
                <td class="px-3 py-2 sm:px-4 sm:py-3 font-mono text-xs whitespace-nowrap">
                  {{ item.api_key_id ? `#${item.api_key_id}` : '-' }}
                </td>
                <td class="px-3 py-2 sm:px-4 sm:py-3 font-medium text-right whitespace-nowrap text-gray-900 dark:text-gray-100">
                  {{ usd(item.amount_usd) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="py-8 text-center text-sm text-gray-400">
          {{ t('groupBuy.gifts.noUsageRecords') }}
        </div>
        <div v-if="!detailLoading && !detailError && (detailTotal > 0 || detailPage > 1)" class="flex flex-wrap items-center justify-between gap-2 pt-2">
          <span class="text-xs text-gray-500">
            {{ t('groupBuy.gifts.paginationRecords', { total: detailTotal, page: detailPage }) }}
          </span>
          <div class="flex items-center gap-1.5">
            <button
              type="button"
              class="btn btn-secondary btn-sm p-1.5"
              data-testid="gift-detail-prev"
              :aria-label="t('groupBuy.gifts.prevPage')"
              :disabled="detailPage <= 1"
              @click="loadAllocations(detailPage - 1)"
            >
              <Icon name="chevronLeft" class="h-4 w-4" />
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm p-1.5"
              data-testid="gift-detail-next"
              :aria-label="t('groupBuy.gifts.nextPage')"
              :disabled="detailPage * pageSize >= detailTotal"
              @click="loadAllocations(detailPage + 1)"
            >
              <Icon name="chevronRight" class="h-4 w-4" />
            </button>
          </div>
        </div>
      </section>
      <section v-else class="flex flex-col gap-4">
        <form
          data-testid="gift-usage-filter"
          class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between"
          @submit.prevent="loadCards(1)"
        >
          <div class="flex flex-1 flex-col gap-2 sm:flex-row sm:items-center">
            <div class="w-full sm:max-w-xs">
              <label for="gift-card-search" class="sr-only">{{ t('groupBuy.gifts.searchPlaceholder') }}</label>
              <input
                id="gift-card-search"
                v-model="search"
                type="text"
                maxlength="200"
                data-testid="gift-usage-search"
                class="input w-full text-xs sm:text-sm"
                :placeholder="t('groupBuy.gifts.searchPlaceholder')"
              />
            </div>
            <div class="w-full sm:w-auto">
              <label for="gift-card-validity" class="sr-only">{{ t('groupBuy.gifts.validityFilter') }}</label>
              <select
                id="gift-card-validity"
                v-model="validity"
                data-testid="gift-usage-validity"
                class="input w-full text-xs sm:text-sm sm:w-auto"
                @change="loadCards(1)"
              >
                <option value="all">{{ t('groupBuy.gifts.validityAll') }}</option>
                <option value="active">{{ t('groupBuy.gifts.validityActive') }}</option>
                <option value="expired">{{ t('groupBuy.gifts.validityExpired') }}</option>
              </select>
            </div>
            <button type="submit" class="btn btn-secondary btn-sm self-start sm:self-auto">
              {{ t('groupBuy.gifts.search') }}
            </button>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm self-start sm:self-auto inline-flex items-center gap-1"
            data-testid="gift-usage-refresh"
            :disabled="loading"
            @click="loadCards(page)"
          >
            <Icon name="refresh" class="h-4 w-4" :class="{ 'animate-spin': loading }" />
            <span>{{ t('groupBuy.gifts.refresh') }}</span>
          </button>
        </form>
        <div v-if="loading" role="status" class="py-8 text-center text-sm text-gray-500">
          <Icon name="refresh" class="mx-auto mb-2 h-5 w-5 animate-spin" />
          <span>{{ t('groupBuy.gifts.loading') }}</span>
        </div>
        <div v-else-if="error" role="alert" class="rounded-lg border border-red-200 bg-red-50 p-4 text-center text-sm text-red-600 dark:border-red-800 dark:bg-red-950/40 dark:text-red-400">
          <p>{{ error }}</p>
          <button
            type="button"
            class="btn btn-secondary btn-sm mt-2"
            data-testid="gift-cards-retry"
            @click="loadCards(page)"
          >
            {{ t('groupBuy.gifts.retry') }}
          </button>
        </div>
        <div v-else-if="cards.length > 0" class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
          <table class="gb-table w-full text-left text-xs sm:text-sm">
            <thead>
              <tr class="border-b border-gray-200 bg-gray-50 text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400">
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap">{{ t('groupBuy.gifts.customer') }}</th>
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap">{{ t('groupBuy.gifts.cardCode') }}</th>
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap min-w-[140px]">{{ t('groupBuy.gifts.usageProgress') }}</th>
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap">{{ t('groupBuy.gifts.remaining') }}</th>
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap">{{ t('groupBuy.gifts.statusAndExpiry') }}</th>
                <th scope="col" class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap text-right">{{ t('groupBuy.gifts.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
              <tr v-for="card in cards" :key="card.id" class="hover:bg-gray-50/50 dark:hover:bg-gray-800/50">
                <td class="px-3 py-2 sm:px-4 sm:py-3">
                  <div class="font-medium break-all">{{ card.username || card.email }}</div>
                  <div class="text-xs text-gray-400 break-all">
                    <span v-if="card.username && card.email">{{ card.email }} &middot; </span>
                    <span>#{{ card.user_id }}</span>
                  </div>
                </td>
                <td class="px-3 py-2 sm:px-4 sm:py-3 font-mono text-xs break-all max-w-[150px]">
                  {{ card.code }}
                </td>
                <td class="px-3 py-2 sm:px-4 sm:py-3">
                  <div class="flex items-center justify-between text-xs mb-1 font-medium">
                    <span>{{ usd(card.total_used_usd) }}</span>
                    <span class="text-gray-400">/ {{ usd(card.total_quota_usd) }}</span>
                  </div>
                  <div class="gb-progress w-full h-1.5 rounded-full bg-gray-200 dark:bg-gray-700 overflow-hidden">
                    <div
                      class="gb-progress-fill h-full bg-primary-600 transition-all"
                      :style="{ width: Math.min(100, Math.max(0, progress(card.total_used_usd, card.total_quota_usd))) + '%' }"
                    />
                  </div>
                </td>
                <td class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap font-medium">
                  <span>{{ usd(remaining(card)) }}</span>
                  <span v-if="card.status !== 'active'" class="block text-xs text-gray-400">{{ t('groupBuy.gifts.unavailable') }}</span>
                </td>
                <td class="px-3 py-2 sm:px-4 sm:py-3 whitespace-nowrap">
                  <div>
                    <span
                      class="gb-pill text-xs px-2 py-0.5 rounded-full inline-block"
                      :class="{
                        'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300': card.status === 'active' && card.total_used_usd < card.total_quota_usd,
                        'bg-amber-50 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300': card.status === 'active' && card.total_used_usd >= card.total_quota_usd,
                        'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400': card.status === 'expired',
                        'bg-rose-50 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300': card.status === 'revoked'
                      }"
                    >
                      {{ card.status === 'active' && card.total_used_usd >= card.total_quota_usd ? t('groupBuy.gifts.exhausted') : t('groupBuy.' + card.status) }}
                    </span>
                  </div>
                  <div class="text-xs text-gray-400 mt-0.5">
                    {{ card.expires_at ? exactDate(card.expires_at) : t('groupBuy.gifts.neverExpires') }}
                  </div>
                </td>
                <td class="px-3 py-2 sm:px-4 sm:py-3 text-right whitespace-nowrap">
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :data-testid="`gift-card-details-${card.id}`"
                    @click="inspect(card)"
                  >
                    {{ t('groupBuy.gifts.viewDetails') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="py-8 text-center text-sm text-gray-400">
          {{ t('groupBuy.gifts.noCardsFound') }}
        </div>
        <div v-if="!loading && !error && (total > 0 || page > 1)" class="flex flex-wrap items-center justify-between gap-2 pt-2">
          <span class="text-xs text-gray-500">
            {{ t('groupBuy.gifts.paginationCards', { total, page }) }}
          </span>
          <div class="flex items-center gap-1.5">
            <button
              type="button"
              class="btn btn-secondary btn-sm p-1.5"
              data-testid="gift-usage-prev"
              :aria-label="t('groupBuy.gifts.prevPage')"
              :disabled="page <= 1"
              @click="loadCards(page - 1)"
            >
              <Icon name="chevronLeft" class="h-4 w-4" />
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm p-1.5"
              data-testid="gift-usage-next"
              :aria-label="t('groupBuy.gifts.nextPage')"
              :disabled="page * pageSize >= total"
              @click="loadCards(page + 1)"
            >
              <Icon name="chevronRight" class="h-4 w-4" />
            </button>
          </div>
        </div>
      </section>
    </div>
  </BaseDialog>
</template>
