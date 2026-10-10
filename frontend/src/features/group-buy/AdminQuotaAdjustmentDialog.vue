<template>
  <BaseDialog
    :show="show"
    :title="
      isBulk
        ? t('groupBuy.batchAdjustQuotaTitle', { count: cards.length })
        : t('groupBuy.adjustQuota')
    "
    width="wide"
    :close-on-escape="!saving"
    :close-on-click-outside="!saving"
    :show-close-button="!saving"
    @close="emit('close')"
  >
    <form
      v-if="cards.length"
      id="admin-quota-adjustment"
      class="space-y-4"
      @submit.prevent="emit('save')"
    >
      <div v-if="!isBulk && single" class="gb-strip p-4 text-sm text-gray-700 dark:text-gray-300">
        <p class="font-medium text-gray-900 dark:text-white">
          {{ single.username || `#${single.user_id}` }}
          <span class="ml-2 text-xs font-normal text-gray-500 dark:text-gray-400">{{ single.email }} · #{{ single.user_id }}</span>
        </p>
        <p class="mt-2">
          <span class="gb-code">{{ single.code }}</span>
          <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">{{ single.product_name }} · {{ single.group_name }}</span>
        </p>
        <div class="mt-3 grid gap-2 sm:grid-cols-2">
          <p class="text-xs">
            <span class="gb-label">{{ t('groupBuy.totalUsage') }}:</span>
            {{ usd(single.total_used_usd) }} / {{ usd(single.total_quota_usd) }}
          </p>
          <p class="text-xs">
            <span class="gb-label">{{ t('groupBuy.weeklyUsage') }}:</span>
            {{ usd(single.weekly_used_usd) }} / {{ usd(single.weekly_quota_usd) }}
          </p>
        </div>
      </div>
      <div v-else class="gb-strip p-4 text-sm text-gray-700 dark:text-gray-300">
        <p class="font-medium text-gray-900 dark:text-white">
          {{ t('groupBuy.quotaSelectedCount', { count: cards.length }) }}
        </p>
        <ul class="mt-2 max-h-40 space-y-1 overflow-y-auto text-xs text-gray-500 dark:text-gray-400">
          <li v-for="card in cards" :key="card.id" class="flex flex-wrap items-center gap-2">
            <span class="font-medium text-gray-700 dark:text-gray-300">{{ card.username || `#${card.user_id}` }}</span>
            <span class="gb-code">{{ card.code }}</span>
            <span>{{ card.product_name }} · {{ card.group_name }}</span>
          </li>
        </ul>
      </div>

      <div class="space-y-3">
        <label class="flex items-start gap-2 text-sm leading-6 text-gray-700 dark:text-gray-300">
          <input
            type="checkbox"
            class="mt-1"
            :checked="draft.setTotal"
            :disabled="saving"
            @change="patch({ setTotal: ($event.target as HTMLInputElement).checked })"
          />
          {{ t('groupBuy.quotaSetTotal') }}
        </label>
        <input
          v-if="draft.setTotal"
          type="text"
          inputmode="decimal"
          class="input w-full"
          :value="draft.total"
          :placeholder="t('groupBuy.quotaNewTotal')"
          :aria-label="t('groupBuy.quotaNewTotal')"
          :disabled="saving"
          @input="patch({ total: ($event.target as HTMLInputElement).value })"
        />
        <label class="flex items-start gap-2 text-sm leading-6 text-gray-700 dark:text-gray-300">
          <input
            type="checkbox"
            class="mt-1"
            :checked="draft.setWeekly"
            :disabled="saving"
            @change="patch({ setWeekly: ($event.target as HTMLInputElement).checked })"
          />
          {{ t('groupBuy.quotaSetWeekly') }}
        </label>
        <input
          v-if="draft.setWeekly"
          type="text"
          inputmode="decimal"
          class="input w-full"
          :value="draft.weekly"
          :placeholder="t('groupBuy.quotaNewWeekly')"
          :aria-label="t('groupBuy.quotaNewWeekly')"
          :disabled="saving"
          @input="patch({ weekly: ($event.target as HTMLInputElement).value })"
        />
      </div>

      <div class="gb-notice gb-notice-warn space-y-1 p-4 text-xs leading-5">
        <p>{{ t('groupBuy.quotaAdjustReplaceHint') }}</p>
        <p>{{ t('groupBuy.quotaAdjustAmountHint') }}</p>
        <p>{{ t('groupBuy.quotaAdjustManualHint') }}</p>
      </div>

      <p v-if="error" class="gb-notice gb-notice-error p-3" role="alert">{{ error }}</p>
    </form>
    <template #footer>
      <button class="btn btn-secondary" :disabled="saving" @click="emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button
        class="btn btn-primary"
        type="submit"
        form="admin-quota-adjustment"
        :disabled="saving"
      >
        {{ saving ? t('common.processing') : t('common.save') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { AdminMonthCard, QuotaAdjustmentDraft } from '@/types/groupBuy'
import { usd } from './model'
import './glass.css'

const props = defineProps<{
  show: boolean
  cards: AdminMonthCard[]
  saving: boolean
  error: string
  draft: QuotaAdjustmentDraft
}>()
const emit = defineEmits<{
  close: []
  save: []
  'update:draft': [draft: QuotaAdjustmentDraft]
}>()
const { t } = useI18n()

const isBulk = computed(() => props.cards.length > 1)
const single = computed(() => (props.cards.length === 1 ? props.cards[0] : null))

function patch(partial: Partial<QuotaAdjustmentDraft>) {
  emit('update:draft', { ...props.draft, ...partial })
}
</script>
