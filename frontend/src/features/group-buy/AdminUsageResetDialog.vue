<template>
  <BaseDialog
    :show="show"
    :title="
      isBulk
        ? t('groupBuy.batchUsageResetTitle', { count: cards.length })
        : t('groupBuy.usageResetTitle')
    "
    width="wide"
    :close-on-escape="!saving"
    :close-on-click-outside="!saving"
    :show-close-button="!saving"
    @close="emit('close')"
  >
    <form
      v-if="cards.length"
      id="admin-usage-reset"
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
            {{ usd(single.total_used_usd) }}
            <span v-if="draft.resetTotal" class="gb-accent-sky font-medium">
              → {{ usd(0) }}
            </span>
            / {{ usd(single.total_quota_usd) }}
          </p>
          <p class="text-xs">
            <span class="gb-label">{{ t('groupBuy.weeklyUsage') }}:</span>
            {{ usd(single.weekly_used_usd) }}
            <span v-if="draft.resetWeekly" class="gb-accent-sky font-medium">
              → {{ usd(0) }}
            </span>
            / {{ usd(single.weekly_quota_usd) }}
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
            <span>
              {{ t('groupBuy.totalUsage') }}: {{ usd(card.total_used_usd) }}
              <span v-if="draft.resetTotal" class="gb-accent-sky font-medium">→ {{ usd(0) }}</span>
              / {{ usd(card.total_quota_usd) }}
            </span>
            <span>
              {{ t('groupBuy.weeklyUsage') }}: {{ usd(card.weekly_used_usd) }}
              <span v-if="draft.resetWeekly" class="gb-accent-sky font-medium">→ {{ usd(0) }}</span>
              / {{ usd(card.weekly_quota_usd) }}
            </span>
          </li>
        </ul>
      </div>

      <div class="space-y-3">
        <label class="flex items-start gap-2 text-sm leading-6 text-gray-700 dark:text-gray-300">
          <input
            type="checkbox"
            class="mt-1"
            :checked="draft.resetWeekly"
            :disabled="saving"
            @change="patch({ resetWeekly: ($event.target as HTMLInputElement).checked })"
          />
          {{ t('groupBuy.resetWeeklyUsage') }}
        </label>
        <label class="flex items-start gap-2 text-sm leading-6 text-gray-700 dark:text-gray-300">
          <input
            type="checkbox"
            class="mt-1"
            :checked="draft.resetTotal"
            :disabled="saving"
            @change="patch({ resetTotal: ($event.target as HTMLInputElement).checked })"
          />
          {{ t('groupBuy.resetTotalUsage') }}
        </label>
      </div>

      <div class="gb-notice gb-notice-warn space-y-1 p-4 text-xs leading-5">
        <p>{{ t('groupBuy.usageResetScopeHint') }}</p>
        <p>{{ t('groupBuy.usageResetPartialHint') }}</p>
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
        form="admin-usage-reset"
        :disabled="saving || (!draft.resetTotal && !draft.resetWeekly)"
      >
        {{ saving ? t('common.processing') : t('groupBuy.confirmUsageReset') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { AdminMonthCard, UsageResetDraft } from '@/types/groupBuy'
import { usd } from './model'
import './glass.css'

const props = defineProps<{
  show: boolean
  cards: AdminMonthCard[]
  saving: boolean
  error: string
  draft: UsageResetDraft
}>()
const emit = defineEmits<{
  close: []
  save: []
  'update:draft': [draft: UsageResetDraft]
}>()
const { t } = useI18n()

const isBulk = computed(() => props.cards.length > 1)
const single = computed(() => (props.cards.length === 1 ? props.cards[0] : null))

function patch(partial: Partial<UsageResetDraft>) {
  emit('update:draft', { ...props.draft, ...partial })
}
</script>
