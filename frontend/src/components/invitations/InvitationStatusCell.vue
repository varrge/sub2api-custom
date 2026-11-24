<template>
  <span class="badge whitespace-nowrap" :class="tone">{{ t(`invitations.statuses.${status}`) }}</span>
  <span v-if="reviewedAt" class="mt-1 block whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">{{ formatDateTimeToMinute(reviewedAt) }}</span>
  <span v-if="autoApproved" class="mt-1 block max-w-[10rem] whitespace-normal text-xs text-gray-500 dark:text-dark-400" data-testid="invitation-auto-approved">{{ t('invitations.records.autoApproved') }}</span>
  <p v-if="reason" class="mt-1 max-w-[10rem] whitespace-pre-line break-words text-xs text-gray-600 dark:text-dark-300">{{ reason }}</p>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTimeToMinute } from '@/utils/format'

const props = defineProps<{
  status: string
  reviewedAt?: string | null
  autoApproved?: boolean
  reason?: string
}>()
const { t } = useI18n()

const tone = computed(() => {
  if (props.status === 'approved' || props.status === 'used') return 'badge-success'
  if (props.status === 'pending') return 'badge-warning'
  if (props.status === 'unused') return 'badge-primary'
  if (props.status === 'rejected') return 'badge-danger'
  return 'badge-gray'
})
</script>
