<template>
  <BaseDialog
    :show="show"
    :title="targetType === 'enterprise' ? t('enterpriseAdmin.conversion.titleToEnterprise') : t('enterpriseAdmin.conversion.titleToPersonal')"
    width="wide"
    @close="$emit('close')"
  >
    <div v-if="user" class="space-y-5">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('enterpriseAdmin.conversion.description') }}</p>

      <!-- 加载状态 -->
      <div v-if="loadingPreview" class="flex justify-center py-12">
        <svg class="h-10 w-10 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <template v-else>
        <!-- 预览加载失败 -->
        <div v-if="previewError" class="rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-800/50 dark:bg-red-900/10">
          <p class="text-sm text-red-600 dark:text-red-400">{{ t('enterpriseAdmin.conversion.previewFailed') }}</p>
          <button type="button" class="btn btn-secondary mt-3" data-test="retry-preview" @click="runPreview()">
            {{ t('enterpriseAdmin.conversion.repreview') }}
          </button>
        </div>

        <template v-else-if="preview">
          <!-- 阻止原因 -->
          <div
            v-if="preview.blockers.length > 0"
            class="rounded-xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-800/50 dark:bg-amber-900/10"
            data-test="conversion-blockers"
          >
            <p class="text-sm font-medium text-amber-700 dark:text-amber-300">{{ t('enterpriseAdmin.conversion.blockersTitle') }}</p>
            <ul class="mt-1 list-inside list-disc text-sm text-amber-600 dark:text-amber-400">
              <li v-for="(blocker, i) in preview.blockers" :key="i">{{ blocker }}</li>
            </ul>
          </div>

          <!-- 当前余额 -->
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-800/50">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('enterpriseAdmin.conversion.currentBalance') }}</p>
              <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">${{ preview.balance.toFixed(2) }}</p>
            </div>
            <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-800/50">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('enterpriseAdmin.conversion.credited') }}</p>
              <p class="mt-1 text-lg font-semibold text-emerald-600 dark:text-emerald-400">${{ preview.credited_usd.toFixed(2) }}</p>
            </div>
            <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-800/50">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('enterpriseAdmin.conversion.balanceAfter') }}</p>
              <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">${{ preview.balance_after.toFixed(2) }}</p>
            </div>
          </div>

          <!-- 月卡列表 -->
          <div v-if="preview.cards.length > 0">
            <div class="mb-2 flex items-center justify-between">
              <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">{{ t('enterpriseAdmin.conversion.cardsTitle') }}</h4>
              <span class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('enterpriseAdmin.conversion.remainingTotal') }}: <span class="font-medium text-gray-700 dark:text-gray-300">${{ preview.remaining_usd.toFixed(2) }}</span>
              </span>
            </div>
            <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-600">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-gray-200 bg-gray-50 text-left text-xs text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400">
                    <th class="px-3 py-2 font-medium">{{ t('enterpriseAdmin.conversion.col.code') }}</th>
                    <th class="px-3 py-2 font-medium">{{ t('enterpriseAdmin.conversion.col.group') }}</th>
                    <th class="px-3 py-2 font-medium">{{ t('enterpriseAdmin.conversion.col.totalQuota') }}</th>
                    <th class="px-3 py-2 font-medium">{{ t('enterpriseAdmin.conversion.col.used') }}</th>
                    <th class="px-3 py-2 font-medium">{{ t('enterpriseAdmin.conversion.col.remaining') }}</th>
                    <th class="px-3 py-2 font-medium">{{ t('enterpriseAdmin.conversion.col.expiresAt') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="card in preview.cards"
                    :key="card.id"
                    class="border-b border-gray-100 last:border-0 dark:border-dark-700"
                  >
                    <td class="px-3 py-2 font-mono text-xs text-gray-700 dark:text-gray-300">{{ card.code }}</td>
                    <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{{ card.group_name }}</td>
                    <td class="px-3 py-2 text-gray-700 dark:text-gray-300">${{ card.total_quota_usd.toFixed(2) }}</td>
                    <td class="px-3 py-2 text-gray-700 dark:text-gray-300">${{ card.total_used_usd.toFixed(2) }}</td>
                    <td class="px-3 py-2 font-medium text-gray-900 dark:text-white">${{ card.remaining_usd.toFixed(2) }}</td>
                    <td class="px-3 py-2 text-gray-500 dark:text-gray-400">{{ card.expires_at ? formatDateTime(card.expires_at) : '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- 统一折算倍率：仅当剩余合计 > 0 且目标为企业时才需要填写 -->
            <div v-if="requiresRate" class="mt-4">
              <label class="input-label">{{ t('enterpriseAdmin.conversion.conversionRate') }}</label>
              <input
                v-model="conversionRateInput"
                type="number"
                min="0"
                step="0.001"
                class="hide-spinner input w-40"
                data-test="conversion-rate-input"
              />
              <p class="input-hint">{{ t('enterpriseAdmin.conversion.rateHint') }}</p>
            </div>

            <!-- 有月卡但剩余合计为 0：零入账，不要求折算倍率 -->
            <p
              v-if="!requiresRate && targetType === 'enterprise' && preview.remaining_usd === 0"
              class="mt-4 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800/50 dark:text-gray-400"
              data-test="zero-credit-hint"
            >
              {{ t('enterpriseAdmin.conversion.zeroCredit') }}
            </p>
          </div>
          <p v-else class="rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800/50 dark:text-gray-400">
            {{ t('enterpriseAdmin.conversion.noCards') }}
          </p>

          <!-- 409 提示 -->
          <div
            v-if="conflict"
            class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-800/50 dark:bg-amber-900/10 dark:text-amber-300"
            data-test="conversion-conflict"
          >
            {{ t('enterpriseAdmin.conversion.conflict') }}
          </div>

          <!-- 显式确认 -->
          <label class="flex cursor-pointer items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input
              v-model="confirmed"
              type="checkbox"
              class="mt-0.5 h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500"
              data-test="conversion-confirm-checkbox"
            />
            {{ t('enterpriseAdmin.conversion.confirmCheckbox') }}
          </label>
        </template>
      </template>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="$emit('close')">{{ t('common.cancel') }}</button>
        <button
          type="button"
          :disabled="!canConfirm"
          class="btn btn-primary"
          data-test="conversion-confirm"
          @click="handleConvert"
        >
          {{ t('enterpriseAdmin.conversion.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import type { EnterpriseConversionPreview } from '@/api/admin/users'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const targetType = computed<'personal' | 'enterprise'>(() =>
  props.user?.account_type === 'enterprise' ? 'personal' : 'enterprise'
)

const preview = ref<EnterpriseConversionPreview | null>(null)
const previewToken = ref<string | null>(null)
const loadingPreview = ref(false)
const previewError = ref(false)
const conflict = ref(false)
const confirmed = ref(false)
const converting = ref(false)
const conversionRateInput = ref<string | number>('')
const operationId = ref('')

const generateOperationId = (): string => {
  const cryptoObj = globalThis.crypto as Crypto | undefined
  if (cryptoObj?.randomUUID) return cryptoObj.randomUUID()
  return `op-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

const parsedRate = computed<number | null>(() => {
  // v-model 在 type="number" 的输入上会自动转成 number，这里必须先转成字符串再解析。
  const raw = String(conversionRateInput.value ?? '').trim()
  if (raw === '') return null
  const num = Number(raw)
  return Number.isFinite(num) && num > 0 ? num : null
})

// 仅当目标为企业且有效月卡剩余合计 > 0 时才要求折算倍率；
// 剩余合计为 0 时是零入账，无月卡或企业→个人均不要求倍率、不创建折算单。
const requiresRate = computed(
  () => targetType.value === 'enterprise' && (preview.value?.remaining_usd ?? 0) > 0
)

const canConfirm = computed(() =>
  !converting.value &&
  !loadingPreview.value &&
  preview.value !== null &&
  previewToken.value !== null &&
  previewError.value === false &&
  preview.value.blockers.length === 0 &&
  confirmed.value &&
  (!requiresRate.value || parsedRate.value !== null)
)

let rateDebounce: ReturnType<typeof setTimeout> | undefined
// 预览请求序号：响应回来时若不是最新一次请求（或弹窗已关闭），直接丢弃，防止旧结果覆盖新结果。
let previewSeq = 0

const runPreview = async (rate: number | undefined = targetType.value === 'enterprise' ? parsedRate.value ?? undefined : undefined) => {
  if (!props.user || !props.show) return
  const seq = ++previewSeq
  confirmed.value = false
  previewToken.value = null
  loadingPreview.value = true
  previewError.value = false
  try {
    const data = await adminAPI.users.previewEnterpriseConversion(
      props.user.id,
      rate
    )
    if (seq !== previewSeq || !props.show) return
    preview.value = data
    previewToken.value = data.preview_token
  } catch (e) {
    if (seq !== previewSeq || !props.show) return
    console.error('Failed to preview enterprise conversion:', e)
    preview.value = null
    previewToken.value = null
    previewError.value = true
  } finally {
    if (seq === previewSeq) {
      loadingPreview.value = false
    }
  }
}

const invalidatePendingPreview = () => {
  clearTimeout(rateDebounce)
  previewSeq++
  loadingPreview.value = false
  previewToken.value = null
  confirmed.value = false
}

onBeforeUnmount(invalidatePendingPreview)

watch(
  () => [props.show, props.user?.id] as const,
  ([v]) => {
    invalidatePendingPreview()
    if (v && props.user) {
      // 每次打开对话框视为一次新的操作会话：生成新的 operation_id，
      // 清空倍率与确认状态，并自动加载一次不带倍率的预览。
      operationId.value = generateOperationId()
      preview.value = null
      previewToken.value = null
      previewError.value = false
      conflict.value = false
      confirmed.value = false
      conversionRateInput.value = ''
      void runPreview()
    }
  },
  { immediate: true }
)

watch(conversionRateInput, () => {
  if (!requiresRate.value) return
  // 倍率一旦变化，旧预览立即作废：清空确认勾选、清除 preview_token（提交按钮随之禁用），
  // 不等防抖结束，避免「旧 preview + 新 rate」被提交；防抖只用于重新拉取预览。
  invalidatePendingPreview()
  rateDebounce = setTimeout(() => {
    if (parsedRate.value === null) return
    void runPreview(parsedRate.value ?? undefined)
  }, 400)
})

const handleConvert = async () => {
  if (!props.user || !previewToken.value || !canConfirm.value) return
  if (requiresRate.value && parsedRate.value === null) return
  converting.value = true
  try {
    // 转换只提交身份、折算倍率、预览令牌与幂等标识；用户编辑/分组配置弹窗里
    // 未保存的草稿（如企业分组倍率）不在此隐式提交，它们是独立的保存动作。
    await adminAPI.users.convertEnterpriseAccount(props.user.id, {
      account_type: targetType.value,
      ...(requiresRate.value && parsedRate.value !== null
        ? { conversion_rate: parsedRate.value }
        : {}),
      preview_token: previewToken.value,
      operation_id: operationId.value
    })
    appStore.showSuccess(t('enterpriseAdmin.conversion.success'))
    emit('success')
    emit('close')
  } catch (e: any) {
    const status = e?.status ?? e?.response?.status
    if (status === 409) {
      // 预览已失效：作废 preview_token、撤销确认并自动重新预览，管理员必须重新勾选确认。
      // 复用同一 operation_id 保证重试幂等；绝不自动重试 convert。
      conflict.value = true
      confirmed.value = false
      previewToken.value = null
      appStore.showError(t('enterpriseAdmin.conversion.conflict'))
      void runPreview(requiresRate.value ? (parsedRate.value ?? undefined) : undefined)
    } else {
      appStore.showError(e?.message || t('enterpriseAdmin.conversion.failed'))
    }
  } finally {
    converting.value = false
  }
}
</script>

<style scoped>
.hide-spinner::-webkit-outer-spin-button,
.hide-spinner::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}
.hide-spinner {
  -moz-appearance: textfield;
}
</style>
