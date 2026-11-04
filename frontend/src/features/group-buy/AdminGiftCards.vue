<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminGroup } from '@/types'
import type { GiftCardBatch, GiftCardConfig, GiftCardIssue, GiftCardPreview } from '@/types/groupBuy'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import AdminGiftUsageDialog from './AdminGiftUsageDialog.vue'
import { exactDate, usd } from './model'

const props = defineProps<{ groups: AdminGroup[] }>()
const { t } = useI18n()
const auth = useAuthStore()
const form = reactive<GiftCardConfig>({ name: '狂蹬卡', group_id: 0, source_group_id: 0, include_frozen: true, quota_usd: '', valid_days: 7 })
const targets = computed(() => props.groups.filter(group => group.subscription_type === 'subscription' && group.status === 'active'))
const preview = ref<GiftCardPreview | null>(null)
const previewing = ref(false), issuing = ref(false), loading = ref(false), confirmOpen = ref(false)
const error = ref(''), historyError = ref(''), success = ref('')
const batches = ref<GiftCardBatch[]>([]), page = ref(1), total = ref(0)
const usageBatch = ref<GiftCardBatch | null>(null)
const uncertain = ref(false)
let pending: GiftCardIssue | null = null
let alive = true, generation = 0, historyGeneration = 0
const pendingKey = () => `month-card-gift-pending:${identity()}`
const valid = computed(() => form.name.trim().length > 0 && !!form.group_id && /^\d+(\.\d{1,8})?$/.test(form.quota_usd) && Number(form.quota_usd) > 0 && Number(form.quota_usd) <= 1e9 && Number.isInteger(form.valid_days) && form.valid_days >= 1 && form.valid_days <= 365)
const busy = computed(() => previewing.value || issuing.value || uncertain.value)
const identity = () => auth.user?.id
const current = (epoch: number, user: number | undefined) => alive && epoch === generation && identity() === user
watch(form, () => { if (!uncertain.value) { generation++; preview.value = null; pending = null; confirmOpen.value = false; success.value = ''; error.value = '' } }, { flush: 'sync' })
watch(identity, () => {
  usageBatch.value = null
  generation++; historyGeneration++; preview.value = null; pending = null; batches.value = []; total.value = 0
  confirmOpen.value = false; uncertain.value = false; issuing.value = false; previewing.value = false; loading.value = false
  error.value = ''; historyError.value = ''; success.value = ''
  if (identity() != null) { restorePending(); void loadHistory(1) }
})
onBeforeUnmount(() => { alive = false; generation++; historyGeneration++ })

async function loadHistory(nextPage = page.value) {
  const epoch = ++historyGeneration, user = identity()
  loading.value = true; historyError.value = ''
  try {
    const result = await adminGroupBuyAPI.giftBatches(nextPage)
    if (!alive || epoch !== historyGeneration || user !== identity()) return
    batches.value = result.items; total.value = result.total; page.value = nextPage
  } catch (err) {
    if (alive && epoch === historyGeneration && user === identity()) historyError.value = extractApiErrorMessage(err, t('groupBuy.gifts.loadFailed'))
  } finally { if (alive && epoch === historyGeneration && user === identity()) loading.value = false }
}
async function prepare() {
  if (!valid.value || busy.value) return
  const epoch = ++generation, user = identity(), config = { ...form }
  previewing.value = true; error.value = ''; success.value = ''
  try {
    const result = await adminGroupBuyAPI.previewGift(config)
    if (!current(epoch, user)) return
    preview.value = result
    pending = { ...config, request_id: crypto.randomUUID(), preview_token: result.preview_token }
  } catch (err) { if (current(epoch, user)) error.value = extractApiErrorMessage(err, t('groupBuy.gifts.previewFailed')) }
  finally { if (current(epoch, user)) previewing.value = false }
}
async function issue() {
  if (issuing.value || !pending || !preview.value?.recipient_count) return
  const epoch = generation, user = identity()
  issuing.value = true; error.value = ''
  // Save before sending so refresh or a lost HTTP response cannot duplicate a batch.
  try {
    sessionStorage.setItem(pendingKey(), JSON.stringify({ request: pending, preview: preview.value }))
  } catch {
    issuing.value = false; error.value = t('groupBuy.gifts.receiptFailed'); return
  }
  try {
    const result = await adminGroupBuyAPI.issueGift(pending)
    if (!current(epoch, user)) return
    uncertain.value = false; confirmOpen.value = false; pending = null; preview.value = null
    clearReceipt()
    success.value = t('groupBuy.gifts.issued', { count: result.recipient_count, id: result.id })
    void loadHistory(1)
  } catch (err) {
    if (!current(epoch, user)) return
    const apiError = err as { status?: number; response?: { status?: number } }
    const status = apiError?.status ?? apiError?.response?.status
    if (status != null && status >= 400 && status < 500) {
      uncertain.value = false; preview.value = null; pending = null; confirmOpen.value = false
      clearReceipt()
      error.value = extractApiErrorMessage(err, t('groupBuy.gifts.previewAgain'))
    } else {
      uncertain.value = true
      error.value = t('groupBuy.gifts.retryHint')
    }
  } finally { if (current(epoch, user)) issuing.value = false }
}
function closeConfirmation() { if (!issuing.value && !uncertain.value) confirmOpen.value = false }
function clearReceipt() {
  try { sessionStorage.removeItem(pendingKey()) } catch { /* Retrying this receipt remains idempotent. */ }
}
defineExpose({ refresh: loadHistory })
function restorePending() {
  try {
    const saved = JSON.parse(sessionStorage.getItem(pendingKey()) ?? 'null') as { request: GiftCardIssue; preview: GiftCardPreview } | null
    if (saved?.request?.request_id && saved.request.preview_token && saved.preview?.recipient_count > 0) {
      for (const key of Object.keys(form) as (keyof GiftCardConfig)[]) Object.assign(form, { [key]: saved.request[key] })
      pending = saved.request; preview.value = saved.preview; uncertain.value = true; confirmOpen.value = true
    }
  } catch { /* A malformed local receipt is ignored; the server retains batch history. */ }
}
onMounted(() => {
  restorePending()
  void loadHistory()
})
</script>
<template>
<div class="space-y-8 bg-transparent">
  <section aria-labelledby="gift-form-heading" class="space-y-4">
    <div class="flex items-center justify-between border-b border-gray-200 pb-3 dark:border-gray-700">
      <div>
        <h2 id="gift-form-heading" class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('groupBuy.gifts.issueTitle') }}
        </h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('groupBuy.gifts.issueDesc') }}
        </p>
      </div>
    </div>

    <div class="rounded border border-gray-200 bg-gray-50/50 p-3 text-xs leading-5 text-gray-600 dark:border-gray-800 dark:bg-gray-800/30 dark:text-gray-400">
      <ul class="list-inside list-disc space-y-0.5">
        <li>{{ t('groupBuy.gifts.rulePerAccount') }}</li>
        <li>{{ t('groupBuy.gifts.ruleDeduplicate') }}</li>
        <li>{{ t('groupBuy.gifts.rulePaidOnly') }}</li>
        <li>{{ t('groupBuy.gifts.ruleNoRefill') }}</li>
        <li>{{ t('groupBuy.gifts.ruleImmediateActive') }}</li>
      </ul>
    </div>

    <form class="grid grid-cols-1 gap-4 lg:grid-cols-3" @submit.prevent="prepare">
      <div class="space-y-1.5">
        <label for="gift-name-input" class="block text-xs font-medium text-gray-700 dark:text-gray-300">
          {{ t('groupBuy.gifts.cardName') }}
        </label>
        <input
          id="gift-name-input"
          v-model="form.name"
          type="text"
          class="input w-full"
          :disabled="busy"
          data-testid="gift-name"
          maxlength="100"
          required
        />
      </div>

      <div class="space-y-1.5">
        <label for="gift-target-select" class="block text-xs font-medium text-gray-700 dark:text-gray-300">
          {{ t('groupBuy.gifts.targetGroup') }}
        </label>
        <select
          id="gift-target-select"
          v-model.number="form.group_id"
          class="input w-full"
          :disabled="busy"
          data-testid="gift-target"
          required
        >
          <option :value="0" disabled>{{ t('groupBuy.gifts.selectTargetGroup') }}</option>
          <option v-for="g in targets" :key="g.id" :value="g.id">
            {{ g.name }} (#{{ g.id }})
          </option>
        </select>
      </div>

      <div class="space-y-1.5">
        <label for="gift-quota-input" class="block text-xs font-medium text-gray-700 dark:text-gray-300">
          {{ t('groupBuy.gifts.quotaUsd') }}
        </label>
        <input
          id="gift-quota-input"
          v-model="form.quota_usd"
          type="text"
          inputmode="decimal"
          class="input w-full"
          placeholder="0.00"
          :disabled="busy"
          data-testid="gift-quota"
          required
        />
      </div>

      <div class="space-y-1.5">
        <label for="gift-days-input" class="block text-xs font-medium text-gray-700 dark:text-gray-300">
          {{ t('groupBuy.gifts.validDays') }}
        </label>
        <input
          id="gift-days-input"
          v-model.number="form.valid_days"
          type="number"
          min="1"
          max="365"
          class="input w-full"
          :disabled="busy"
          data-testid="gift-days"
          required
        />
      </div>

      <div class="space-y-1.5">
        <label for="gift-source-select" class="block text-xs font-medium text-gray-700 dark:text-gray-300">
          {{ t('groupBuy.gifts.sourceFilterGroup') }}
        </label>
        <select
          id="gift-source-select"
          v-model.number="form.source_group_id"
          class="input w-full"
          :disabled="busy"
          data-testid="gift-source"
        >
          <option :value="0">{{ t('groupBuy.gifts.allActiveMonthCards') }}</option>
          <option v-for="g in props.groups" :key="g.id" :value="g.id">
            {{ g.name }} (#{{ g.id }})
          </option>
        </select>
      </div>

      <div class="flex flex-col justify-end pb-1.5">
        <label class="inline-flex items-center gap-2 text-xs font-medium text-gray-700 dark:text-gray-300">
          <input
            v-model="form.include_frozen"
            type="checkbox"
            class="rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700"
            :disabled="busy"
            data-testid="gift-frozen"
          />
          {{ t('groupBuy.gifts.includeFrozen') }}
        </label>
      </div>

      <div class="flex flex-wrap items-center gap-3 pt-2 lg:col-span-3">
        <button
          type="button"
          class="btn btn-secondary inline-flex items-center gap-1.5"
          :disabled="!valid || busy"
          data-testid="gift-preview"
          @click="prepare"
        >
          <Icon name="eye" class="h-4 w-4" />
          {{ previewing ? t('groupBuy.gifts.calculating') : t('groupBuy.gifts.previewAction') }}
        </button>

        <button
          type="button"
          class="btn btn-primary inline-flex items-center gap-1.5"
          :disabled="!preview || !preview.recipient_count || busy"
          data-testid="gift-open-confirm"
          @click="confirmOpen = true"
        >
          <Icon name="gift" class="h-4 w-4" />
          {{ t('groupBuy.gifts.confirmIssueAction') }}
        </button>
      </div>
    </form>

    <div v-if="preview" class="rounded border border-gray-200 p-4 dark:border-gray-700">
      <div v-if="preview.recipient_count > 0" class="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.previewRecipients') }}</div>
          <div class="mt-1 text-lg font-semibold break-all text-gray-900 dark:text-white">{{ preview.recipient_count }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.previewPerCapita') }}</div>
          <div class="mt-1 text-lg font-semibold break-all text-gray-900 dark:text-white">{{ usd(Number(form.quota_usd)) }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.previewTotalQuota') }}</div>
          <div class="mt-1 text-lg font-semibold break-all text-gray-900 dark:text-white">{{ usd(Number(preview.total_quota_usd)) }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.previewValidDays') }}</div>
          <div class="mt-1 text-lg font-semibold break-all text-gray-900 dark:text-white">{{ form.valid_days }} {{ t('groupBuy.gifts.daysUnit') }}</div>
        </div>
      </div>
      <div v-else class="text-center text-xs text-gray-500 dark:text-gray-400">
        {{ t('groupBuy.gifts.noMatchingRecipients') }}
      </div>
    </div>

    <div v-if="error" role="alert" class="rounded border border-red-200 bg-red-50 p-3 text-xs text-red-800 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-300">
      {{ error }}
    </div>
    <div v-if="success" role="status" class="rounded border border-green-200 bg-green-50 p-3 text-xs text-green-800 dark:border-green-900/50 dark:bg-green-950/20 dark:text-green-300">
      {{ success }}
    </div>
  </section>

  <section aria-labelledby="gift-history-heading" class="space-y-4 pt-4">
    <div class="flex items-center justify-between border-b border-gray-200 pb-3 dark:border-gray-700">
      <div>
        <h2 id="gift-history-heading" class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('groupBuy.gifts.historyTitle') }}
        </h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('groupBuy.gifts.historyDesc') }}
        </p>
      </div>
      <button
        type="button"
        class="btn btn-secondary inline-flex items-center gap-1 text-xs"
        :disabled="loading"
        @click="loadHistory()"
      >
        <Icon name="refresh" class="h-3.5 w-3.5" />
        {{ loading ? t('groupBuy.gifts.refreshing') : t('groupBuy.gifts.refresh') }}
      </button>
    </div>

    <div v-if="historyError" role="alert" class="rounded border border-red-200 bg-red-50 p-3 text-xs text-red-800 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-300">
      {{ historyError }}
    </div>

    <div class="overflow-x-auto">
      <table class="gb-table w-full text-left text-xs">
        <thead>
          <tr class="border-b border-gray-200 text-gray-500 dark:border-gray-700 dark:text-gray-400">
            <th class="py-2.5 px-3 font-medium">{{ t('groupBuy.gifts.thBatchId') }}</th>
            <th class="py-2.5 px-3 font-medium">{{ t('groupBuy.gifts.thName') }}</th>
            <th class="py-2.5 px-3 font-medium">{{ t('groupBuy.gifts.thGroup') }}</th>
            <th class="py-2.5 px-3 font-medium">{{ t('groupBuy.gifts.thQuota') }}</th>
            <th class="py-2.5 px-3 font-medium">{{ t('groupBuy.gifts.thCount') }}</th>
            <th class="py-2.5 px-3 font-medium">{{ t('groupBuy.gifts.thCreatedAt') }}</th>
            <th class="py-2.5 px-3 font-medium">{{ t('groupBuy.gifts.thExpiresAt') }}</th>
            <th class="py-2.5 px-3 font-medium">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-gray-800">
          <tr v-if="batches.length === 0 && !loading">
            <td colspan="8" class="py-8 text-center text-gray-500 dark:text-gray-400">
              {{ t('groupBuy.gifts.noHistory') }}
            </td>
          </tr>
          <tr v-for="item in batches" :key="item.id" class="hover:bg-gray-50/50 dark:hover:bg-gray-800/30">
            <td class="py-2.5 px-3 font-mono text-gray-600 dark:text-gray-300">#{{ item.id }}</td>
            <td class="py-2.5 px-3 font-medium text-gray-900 dark:text-white">{{ item.name }}</td>
            <td class="py-2.5 px-3 text-gray-600 dark:text-gray-300">{{ item.group_name }}</td>
            <td class="py-2.5 px-3 font-mono text-gray-900 dark:text-white">{{ usd(Number(item.quota_usd)) }}</td>
            <td class="py-2.5 px-3 text-gray-600 dark:text-gray-300">{{ item.recipient_count }}</td>
            <td class="py-2.5 px-3 text-gray-500 dark:text-gray-400">{{ exactDate(item.created_at) }}</td>
            <td class="py-2.5 px-3 text-gray-500 dark:text-gray-400">{{ exactDate(item.expires_at) }}</td>
            <td class="py-2.5 px-3">
              <button type="button" class="btn btn-secondary btn-sm whitespace-nowrap" :data-testid="`gift-usage-${item.id}`" @click="usageBatch = item">
                {{ t('groupBuy.gifts.viewUsage') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="total > 20" class="flex items-center justify-between border-t border-gray-200 pt-3 dark:border-gray-700">
      <span class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('groupBuy.gifts.paginationTotal', { total, page, maxPage: Math.ceil(total / 20) || 1 }) }}
      </span>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="btn btn-secondary inline-flex items-center p-1.5 text-xs"
          :disabled="page <= 1 || loading"
          :aria-label="t('groupBuy.gifts.prevPage')"
          @click="loadHistory(page - 1)"
        >
          <Icon name="chevronLeft" class="h-4 w-4" />
        </button>
        <button
          type="button"
          class="btn btn-secondary inline-flex items-center p-1.5 text-xs"
          :disabled="page * 20 >= total || loading"
          :aria-label="t('groupBuy.gifts.nextPage')"
          @click="loadHistory(page + 1)"
        >
          <Icon name="chevronRight" class="h-4 w-4" />
        </button>
      </div>
    </div>
  </section>

  <AdminGiftUsageDialog v-if="usageBatch" :key="usageBatch.id" :batch="usageBatch" @close="usageBatch = null" />

  <BaseDialog
    :show="confirmOpen"
    :title="t('groupBuy.gifts.dialogTitle')"
    width="wide"
    :close-on-escape="!issuing && !uncertain"
    :close-on-click-outside="!issuing && !uncertain"
    :show-close-button="!issuing && !uncertain"
    @close="closeConfirmation"
  >
    <div class="space-y-4 text-xs">
      <p class="text-gray-600 dark:text-gray-300">
        {{ t('groupBuy.gifts.dialogDesc') }}
      </p>

      <dl class="grid grid-cols-2 gap-3 rounded border border-gray-200 p-3 dark:border-gray-700">
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.cardName') }}</dt>
          <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ form.name }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.targetGroup') }}</dt>
          <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">
            {{ targets.find(g => g.id === form.group_id)?.name || form.group_id }}
          </dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.dialogRecipients') }}</dt>
          <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ preview?.recipient_count }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.dialogPerQuota') }}</dt>
          <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ usd(Number(form.quota_usd)) }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.dialogTotalQuota') }}</dt>
          <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ preview ? usd(Number(preview.total_quota_usd)) : '-' }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.gifts.dialogValidDays') }}</dt>
          <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ form.valid_days }} {{ t('groupBuy.gifts.daysUnit') }}</dd>
        </div>
      </dl>

      <div v-if="uncertain" role="alert" class="rounded border border-amber-200 bg-amber-50 p-3 text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/20 dark:text-amber-300">
        {{ t('groupBuy.gifts.retryHint') }}
      </div>

      <div v-if="error" role="alert" class="rounded border border-red-200 bg-red-50 p-3 text-red-800 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-300">
        {{ error }}
      </div>
    </div>

    <template #footer>
      <button
        type="button"
        class="btn btn-secondary"
        :disabled="issuing || uncertain"
        @click="closeConfirmation"
      >
        {{ t('common.cancel') }}
      </button>
      <button
        v-if="uncertain"
        type="button"
        class="btn btn-primary inline-flex items-center gap-1.5"
        :disabled="issuing"
        data-testid="gift-issue"
        @click="issue"
      >
        <Icon name="refresh" class="h-4 w-4" />
        {{ issuing ? t('groupBuy.gifts.issuing') : t('groupBuy.gifts.retryAction') }}
      </button>
      <button
        v-else
        type="button"
        class="btn btn-primary inline-flex items-center gap-1.5"
        :disabled="issuing"
        data-testid="gift-issue"
        @click="issue"
      >
        <Icon name="check" class="h-4 w-4" />
        {{ issuing ? t('groupBuy.gifts.issuing') : t('groupBuy.gifts.confirmIssue') }}
      </button>
    </template>
  </BaseDialog>
</div>
</template>
