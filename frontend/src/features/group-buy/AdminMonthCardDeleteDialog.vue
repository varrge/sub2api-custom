<template>
  <BaseDialog
    :show="show"
    :title="t('groupBuy.deleteCardTitle')"
    width="normal"
    :close-on-escape="!saving"
    :close-on-click-outside="!saving"
    :show-close-button="!saving"
    @close="emit('close')"
  >
    <form
      v-if="card"
      id="admin-month-card-delete"
      class="space-y-4"
      @submit.prevent="emit('confirm')"
    >
      <div class="gb-strip p-4 text-sm text-gray-700 dark:text-gray-300">
        <p class="font-medium text-gray-900 dark:text-white">
          {{ card.username || `#${card.user_id}` }}
          <span class="ml-2 text-xs font-normal text-gray-500 dark:text-gray-400">{{ card.email }} · #{{ card.user_id }}</span>
        </p>
        <p class="mt-2">
          <span class="gb-code">{{ card.code }}</span>
          <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">{{ card.product_name }} · {{ card.group_name }}</span>
        </p>
        <p class="mt-3 text-xs">
          <span class="gb-label">{{ t('groupBuy.totalUsage') }}:</span>
          {{ usd(card.total_used_usd) }} / {{ usd(card.total_quota_usd) }}
        </p>
      </div>

      <div class="gb-notice gb-notice-warn p-4 text-xs leading-5">
        <p>{{ t('groupBuy.deleteCardHint') }}</p>
      </div>

      <p v-if="error" class="gb-notice gb-notice-error p-3" role="alert">{{ error }}</p>
    </form>
    <template #footer>
      <button class="btn btn-secondary" :disabled="saving" @click="emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button
        class="btn btn-danger"
        type="submit"
        form="admin-month-card-delete"
        :disabled="saving || !card"
      >
        {{ saving ? t('common.processing') : t('groupBuy.confirmDeleteCard') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { AdminMonthCard } from '@/types/groupBuy'
import { usd } from './model'
import './glass.css'

defineProps<{
  show: boolean
  card: AdminMonthCard | null
  saving: boolean
  error: string
}>()
const emit = defineEmits<{
  close: []
  confirm: []
}>()
const { t } = useI18n()
</script>
