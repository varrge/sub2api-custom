<template>
  <BaseDialog :show="!!order" :title="t('groupBuy.confirmRefund')" @close="close">
    <form v-if="order" id="month-card-refund-confirm" class="space-y-4" @submit.prevent="submit">
      <div class="rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-800">
        <p>{{ t('payment.orders.orderId') }} #{{ order.id }} · {{ order.out_trade_no }}</p>
        <p class="mt-2">{{ t('payment.orders.payAmount') }}: {{ cny(order.pay_amount) }}</p>
        <p class="mt-2">{{ t('payment.admin.refundReason') }}: {{ order.refund_reason }}</p>
      </div>
      <p class="rounded-xl bg-amber-50 p-4 text-sm leading-6 text-amber-800 dark:bg-amber-950 dark:text-amber-200">{{ t('groupBuy.confirmRefundHint') }}</p>
      <label class="block text-sm" for="month-card-refund-reference">{{ t('groupBuy.refundReference') }}</label>
      <textarea id="month-card-refund-reference" v-model="reference" rows="3" class="input w-full" required :disabled="submitting" :aria-invalid="reference.length > 0 && !validRefundReference(reference)" aria-describedby="month-card-refund-reference-help" />
      <p id="month-card-refund-reference-help" class="text-xs text-gray-500">{{ t('groupBuy.refundReferenceHelp', { count: refundReferenceBytes(reference) }) }}</p>
      <label class="flex items-start gap-2 text-sm leading-6"><input v-model="verified" type="checkbox" class="mt-1" :disabled="submitting" required />{{ t('groupBuy.refundVerified') }}</label>
      <p v-if="!eligible" class="text-sm text-red-600" role="alert">{{ t('groupBuy.refundConfirmationUnavailable') }}</p>
      <p v-if="error" class="text-sm text-red-600" role="alert">{{ error }}</p>
    </form>
    <template #footer>
      <button class="btn btn-secondary" :disabled="submitting" @click="close">{{ t('common.cancel') }}</button>
      <button class="btn btn-danger" type="submit" form="month-card-refund-confirm" :disabled="!canSubmit">{{ submitting ? t('common.processing') : t('groupBuy.confirmRefund') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useTimestamp } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { PaymentOrder } from '@/types/payment'
import { cny } from './model'
import { canConfirmMonthCardRefund, refundReferenceBytes, validRefundReference } from './refundConfirmation'

const props = defineProps<{ order: PaymentOrder | null }>()
const emit = defineEmits<{ close: []; confirmed: [] }>()
const { t } = useI18n()
const now = useTimestamp({ interval: 1000 })
const reference = ref('')
const verified = ref(false)
const submitting = ref(false)
const error = ref('')
const eligible = computed(() => canConfirmMonthCardRefund(props.order, now.value))
const canSubmit = computed(() => eligible.value && verified.value && validRefundReference(reference.value) && !submitting.value)
watch(() => props.order?.id, () => { reference.value = ''; verified.value = false; error.value = '' })
function close() { if (!submitting.value) emit('close') }
async function submit() {
  if (!canSubmit.value || !props.order) return
  submitting.value = true
  error.value = ''
  try {
    const result = await adminPaymentAPI.confirmMonthCardRefund(props.order.id, { reference: reference.value.trim(), confirmed: true })
    if (result.data.success) emit('confirmed')
    else error.value = result.data.warning || t('groupBuy.refundConfirmationFailed')
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('groupBuy.refundConfirmationFailed'))
  } finally { submitting.value = false }
}
</script>
