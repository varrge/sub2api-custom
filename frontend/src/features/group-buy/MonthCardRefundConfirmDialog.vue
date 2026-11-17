<template>
  <BaseDialog :show="!!order" :title="confirmationTitle" @close="close">
    <form v-if="order" id="month-card-refund-confirm" class="space-y-4" @submit.prevent="submit">
      <div class="gb-strip p-4 text-sm text-gray-700 dark:text-gray-300">
        <p>{{ t('payment.orders.orderId') }} #{{ order.id }} · {{ order.out_trade_no }}</p>
        <p class="mt-2">{{ t('payment.orders.payAmount') }}: <strong class="gb-accent">{{ isBalance ? formatPaymentAmount(order.pay_amount, order.currency, locale) : cny(order.pay_amount) }}</strong></p>
        <p v-if="isBalance" class="mt-2">{{ t('payment.admin.refundPaymentAmount') }}: <strong class="gb-accent">{{ refundPayAmount === null ? t('payment.admin.refundAmountUnknown') : formatPaymentAmount(refundPayAmount, order.currency, locale) }}</strong></p>
        <p class="mt-2">{{ t('payment.admin.refundReason') }}: {{ order.refund_reason }}</p>
      </div>
      <p class="gb-notice gb-notice-warn p-4">{{ isBalance ? t('payment.admin.verifyBalanceRefundHint') : t('groupBuy.confirmRefundHint') }}</p>
      <fieldset v-if="isBalance" class="space-y-2" :disabled="submitting">
        <legend class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('payment.admin.verifiedRefundOutcome') }}</legend>
        <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300"><input v-model="outcome" type="radio" name="refund-outcome" value="success" required />{{ t('payment.admin.verifiedRefundSuccess') }}</label>
        <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300"><input v-model="outcome" type="radio" name="refund-outcome" value="failed" required />{{ t('payment.admin.verifiedRefundFailed') }}</label>
      </fieldset>
      <div v-if="isBalance && reconciliationRequired" class="space-y-2">
        <p class="gb-notice gb-notice-warn p-3">{{ t('payment.admin.refundReconciliationHint') }}</p>
        <label for="refund-reconciliation-amount" class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('payment.admin.refundBalanceAmount') }}</label>
        <input id="refund-reconciliation-amount" v-model.number="refundAmount" type="number" min="0" :max="order.amount" step="any" class="input w-full" required :disabled="submitting" />
        <label for="refund-reconciliation-deducted" class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('payment.admin.refundBalanceDeducted') }}</label>
        <input id="refund-reconciliation-deducted" v-model.number="balanceDeducted" type="number" min="0" :max="refundAmount || order.amount" step="any" class="input w-full" required :disabled="submitting" />
      </div>
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="month-card-refund-reference">{{ t('groupBuy.refundReference') }}</label>
      <textarea id="month-card-refund-reference" v-model="reference" rows="3" class="input w-full" required :disabled="submitting" :aria-invalid="reference.length > 0 && !validRefundReference(reference)" aria-describedby="month-card-refund-reference-help" />
      <p id="month-card-refund-reference-help" class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.refundReferenceHelp', { count: refundReferenceBytes(reference) }) }}</p>
      <label class="flex items-start gap-2 text-sm leading-6 text-gray-700 dark:text-gray-300"><input v-model="verified" type="checkbox" class="mt-1" :disabled="submitting" required />{{ isBalance ? t('payment.admin.balanceRefundVerified') : t('groupBuy.refundVerified') }}</label>
      <p v-if="!eligible" class="gb-notice gb-notice-error p-3" role="alert">{{ t('groupBuy.refundConfirmationUnavailable') }}</p>
      <p v-if="error" class="gb-notice gb-notice-error p-3" role="alert">{{ error }}</p>
    </form>
    <template #footer>
      <button class="btn btn-secondary" :disabled="submitting" @click="close">{{ t('common.cancel') }}</button>
      <button class="btn btn-danger" type="submit" form="month-card-refund-confirm" :disabled="!canSubmit">{{ submitting ? t('common.processing') : confirmationTitle }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useTimestamp } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import { formatPaymentAmount } from '@/components/payment/currency'
import type { PaymentOrder } from '@/types/payment'
import { cny } from './model'
import { canConfirmRefund, refundReferenceBytes, validRefundReference } from './refundConfirmation'
import './glass.css'

const props = defineProps<{ order: PaymentOrder | null }>()
const emit = defineEmits<{ close: []; confirmed: [] }>()
const { t, locale } = useI18n()
const now = useTimestamp({ interval: 1000 })
const reference = ref('')
const verified = ref(false)
const submitting = ref(false)
const error = ref('')
const outcome = ref<'success' | 'failed' | ''>('')
const reconciliationRequired = ref(false)
const refundAmount = ref<number | ''>('')
const balanceDeducted = ref<number | ''>('')
const isBalance = computed(() => props.order?.order_type === 'balance')
const confirmationTitle = computed(() => isBalance.value ? t('payment.admin.verifyRefund') : t('groupBuy.confirmRefund'))
const eligible = computed(() => canConfirmRefund(props.order, now.value))
const validReconciliation = computed(() => !reconciliationRequired.value || (
  typeof refundAmount.value === 'number' && Number.isFinite(refundAmount.value) && refundAmount.value > 0 && refundAmount.value <= (props.order?.amount || 0) &&
  typeof balanceDeducted.value === 'number' && Number.isFinite(balanceDeducted.value) && balanceDeducted.value >= 0 && balanceDeducted.value <= refundAmount.value
))
const refundPayAmount = computed(() => {
  const order = props.order
  const amount = reconciliationRequired.value ? refundAmount.value : order?.refund_amount
  return order && order.amount > 0 && typeof amount === 'number' && Number.isFinite(amount) && amount > 0 && amount <= order.amount
    ? order.pay_amount * amount / order.amount : null
})
const canSubmit = computed(() => eligible.value && verified.value && validRefundReference(reference.value) && !submitting.value &&
  (!isBalance.value || (!!outcome.value && Number.isFinite(Date.parse(props.order?.updated_at || '')) && validReconciliation.value)))
watch(() => props.order?.id, () => {
  reference.value = ''; verified.value = false; error.value = ''; outcome.value = ''
  reconciliationRequired.value = false; refundAmount.value = ''; balanceDeducted.value = ''
})
function close() { if (!submitting.value) emit('close') }
async function submit() {
  if (!canSubmit.value || !props.order) return
  submitting.value = true
  error.value = ''
  try {
    const acknowledgement = { reference: reference.value.trim(), confirmed: true as const }
    const result = isBalance.value && outcome.value && props.order.updated_at
      ? await adminPaymentAPI.confirmBalanceRefund(props.order.id, {
        ...acknowledgement, outcome: outcome.value, updated_at: props.order.updated_at,
        ...(reconciliationRequired.value ? { refund_amount: Number(refundAmount.value), balance_deducted: Number(balanceDeducted.value) } : {}),
      })
      : await adminPaymentAPI.confirmMonthCardRefund(props.order.id, acknowledgement)
    if (result.data.success || (isBalance.value && result.data.resolved)) emit('confirmed')
    else error.value = result.data.warning || t('groupBuy.refundConfirmationFailed')
  } catch (err) {
    if (isBalance.value && extractApiErrorCode(err) === 'REFUND_RECONCILIATION_REQUIRED') {
      if (!reconciliationRequired.value) refundAmount.value = props.order.refund_amount > 0 ? props.order.refund_amount : ''
      reconciliationRequired.value = true
      verified.value = false
    }
    error.value = extractApiErrorMessage(err, t('groupBuy.refundConfirmationFailed'))
  } finally { submitting.value = false }
}
</script>
