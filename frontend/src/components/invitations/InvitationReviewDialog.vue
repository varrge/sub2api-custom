<template>
  <BaseDialog :show="!!rebate" :title="t(`invitations.review.${action}Title`)" :close-on-escape="!submitting" :close-on-click-outside="!submitting" :show-close-button="!submitting" @close="close">
    <form v-if="rebate" class="space-y-4" novalidate data-testid="invitation-review-form" @submit.prevent="submit">
      <p class="text-sm text-gray-800 dark:text-gray-200" :class="action === 'revoke' ? 'rounded-xl border border-amber-300 bg-amber-50 p-3 text-amber-900 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-200' : ''" data-testid="invitation-review-consequence">
        {{ t(`invitations.review.${action}Body`, { email: rebate.inviter_email, amount }) }}
      </p>

      <p v-if="warning" role="alert" class="rounded-xl border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-200" data-testid="invitation-review-warning">
        {{ warning }}
      </p>

      <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-900">
        <dt class="text-gray-500 dark:text-dark-400">{{ t('invitations.review.inviter') }}</dt>
        <dd class="break-all text-gray-900 dark:text-gray-100">{{ rebate.inviter_email }} <span class="text-gray-500 dark:text-dark-400">#{{ rebate.inviter_id }}</span></dd>
        <dt class="text-gray-500 dark:text-dark-400">{{ t('invitations.review.invitee') }}</dt>
        <dd class="break-all text-gray-900 dark:text-gray-100">{{ rebate.invitee_email }} <span v-if="rebate.invitee_id" class="text-gray-500 dark:text-dark-400">#{{ rebate.invitee_id }}</span></dd>
        <dt class="text-gray-500 dark:text-dark-400">{{ t('invitations.review.source') }}</dt>
        <dd class="text-gray-900 dark:text-gray-100">{{ t(`invitations.sources.${rebate.source_type}`) }} #{{ rebate.source_id }}, {{ formatDateTime(rebate.source_occurred_at) }}</dd>
        <dt class="text-gray-500 dark:text-dark-400">{{ t('invitations.review.calculation') }}</dt>
        <dd class="tabular-nums text-gray-900 dark:text-gray-100">
          {{ t('invitations.review.calculationValue', { first: formatMoney(rebate.first_amount, rebate.source_currency), rate: formatRate(rebate.rebate_rate), reward: amount }) }}
          <span class="block text-xs text-gray-500 dark:text-dark-400">{{ rebate.level_name }}</span>
        </dd>
      </dl>

      <div>
        <label for="invitation-review-reason" class="input-label">{{ t(reasonRequired ? 'invitations.review.reason' : 'invitations.review.reasonOptional') }}</label>
        <textarea
          id="invitation-review-reason"
          v-model="reason"
          class="input"
          :class="reasonMissing ? 'input-error' : ''"
          rows="3"
          maxlength="1000"
          :disabled="submitting"
          :aria-invalid="reasonMissing"
          aria-describedby="invitation-review-reason-hint"
          data-testid="invitation-review-reason"
        ></textarea>
        <p id="invitation-review-reason-hint" class="input-hint">
          <span v-if="reasonMissing" class="text-red-600 dark:text-red-400">{{ t('invitations.review.reasonRequired') }} </span>{{ t('invitations.review.reasonHint') }}
        </p>
      </div>

      <p v-if="error" role="alert" class="text-sm text-red-700 dark:text-red-300" data-testid="invitation-review-error">
        <span class="font-medium">{{ t('invitations.review.failed') }}</span>{{ t('invitations.sep') }}{{ error }}
      </p>

      <div class="flex flex-wrap justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="close">{{ t('common.cancel') }}</button>
        <button type="submit" class="btn" :class="action === 'approve' ? 'btn-primary' : 'btn-danger'" :disabled="submitting" data-testid="invitation-review-submit">
          {{ submitting ? t('common.processing') : t(`invitations.review.${action}Submit`, { amount }) }}
        </button>
      </div>
    </form>
  </BaseDialog>
  <TotpStepUpDialog :controller="stepUp" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { isStepUpCancelled, useStepUp } from '@/composables/useStepUp'
import { invitationsAPI, type InvitationRebate, type ReviewAction } from '@/api/invitations'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { invitationErrorMessage, invitationErrorReason } from '@/utils/invitationErrors'
import { formatMoney, formatRate, sourceConcern } from '@/utils/invitationFormat'

const props = defineProps<{ rebate: InvitationRebate | null; action: ReviewAction }>()
const emit = defineEmits<{
  close: []
  /** The record changed on the server, either by this action or by someone else's. */
  changed: [rebate: InvitationRebate | null]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const stepUp = useStepUp()
const reason = ref('')
const submitting = ref(false)
const error = ref('')
const attempted = ref(false)

const amount = computed(() => formatMoney(props.rebate?.rebate_amount ?? 0))
// Crediting a reward for a first payment that was refunded or never completed is the mistake to prevent.
const warning = computed(() => {
  if (!props.rebate || props.action !== 'approve') return ''
  const concern = sourceConcern(props.rebate.source_type, props.rebate.source_status)
  if (!concern) return ''
  return t(concern === 'refund' ? 'invitations.review.refundWarning' : 'invitations.review.incompleteWarning', { status: props.rebate.source_status })
})
const reasonRequired = computed(() => props.action !== 'approve')
const reasonMissing = computed(() => attempted.value && reasonRequired.value && !reason.value.trim())

function close() {
  if (!submitting.value) emit('close')
}
async function submit() {
  if (!props.rebate || submitting.value) return
  attempted.value = true
  error.value = ''
  if (reasonRequired.value && !reason.value.trim()) return
  submitting.value = true
  const rebateId = props.rebate.id
  const action = props.action
  const reviewReason = reason.value.trim()
  try {
    const updated = await stepUp.run(() => invitationsAPI.review(rebateId, action, reviewReason))
    appStore.showSuccess(t(`invitations.review.${props.action}Done`))
    emit('changed', updated)
    emit('close')
  } catch (err) {
    if (isStepUpCancelled(err)) return
    if (invitationErrorReason(err) === 'INVITATION_CONFLICT') {
      // Another admin got there first. Keep the dialog open so the reason is visible, and refresh behind it.
      error.value = t('invitations.review.stale')
      emit('changed', null)
    } else {
      error.value = invitationErrorMessage(err, t, t('invitations.review.failed'))
    }
  } finally {
    submitting.value = false
  }
}

watch(() => [props.rebate?.id, props.action], () => {
  reason.value = ''
  error.value = ''
  attempted.value = false
})
</script>
