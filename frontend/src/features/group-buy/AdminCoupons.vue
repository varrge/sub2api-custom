<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('groupBuy.couponAdminHint') }}</p>
      <button class="btn btn-primary" @click="edit()">{{ t('groupBuy.couponNew') }}</button>
    </div>
    <p v-if="error" class="gb-notice gb-notice-error p-3" role="alert">{{ error }}</p>
    <p v-if="loading" class="text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</p>
    <div v-else class="gb-panel overflow-x-auto">
      <table class="gb-table w-full text-left text-sm text-gray-700 dark:text-gray-300">
        <thead><tr>
          <th v-for="key in ['couponCode', 'couponDiscount', 'couponProduct', 'couponExpires', 'couponUsed', 'couponActive']" :key="key">{{ t(`groupBuy.${key}`) }}</th>
          <th>{{ t('common.actions') }}</th>
        </tr></thead>
        <tbody><tr v-for="coupon in coupons" :key="coupon.id">
          <td><span class="gb-code">{{ coupon.code }}</span></td>
          <td>{{ coupon.kind === 'fixed' ? cny(coupon.value) : t('groupBuy.couponPercentValue', { value: coupon.value }) }}</td>
          <td>{{ coupon.product_id ? products.find(product => product.id === coupon.product_id)?.name || `#${coupon.product_id}` : t('groupBuy.couponAllProducts') }}</td>
          <td>{{ coupon.expires_at ? exactDate(coupon.expires_at) : t('groupBuy.couponNoExpiry') }}</td>
          <td>{{ coupon.used_count }} / {{ coupon.max_uses || t('groupBuy.couponUnlimited') }}</td>
          <td>
            <span class="gb-pill" :class="coupon.active ? 'gb-pill-teal' : 'gb-pill-gray'">{{ coupon.active ? t('common.yes') : t('common.no') }}</span>
          </td>
          <td><button class="btn btn-secondary btn-sm" @click="edit(coupon)">{{ t('common.edit') }}</button></td>
        </tr></tbody>
      </table>
      <p v-if="!coupons.length" class="p-8 text-center text-gray-500 dark:text-gray-400">{{ t('groupBuy.couponEmpty') }}</p>
    </div>
    <BaseDialog :show="!!draft" :title="t(draft?.id ? 'groupBuy.couponEdit' : 'groupBuy.couponNew')" @close="close">
      <form v-if="draft" class="space-y-4" @submit.prevent="save">
        <label class="block space-y-1">
          <span>{{ t('groupBuy.couponCode') }}</span>
          <input v-model="draft.code" class="input" required minlength="2" maxlength="64" pattern="[A-Za-z0-9_-]{2,64}" :disabled="!!draft.id || saving" :placeholder="t('groupBuy.couponCodeHint')" />
        </label>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label class="block space-y-1"><span>{{ t('groupBuy.couponKind') }}</span>
            <select v-model="draft.kind" class="input" :disabled="saving"><option value="fixed">{{ t('groupBuy.couponFixed') }}</option><option value="percent">{{ t('groupBuy.couponPercent') }}</option></select>
          </label>
          <label class="block space-y-1"><span>{{ t(draft.kind === 'fixed' ? 'groupBuy.couponValueCNY' : 'groupBuy.couponValuePercent') }}</span>
            <input v-model.number="draft.value" type="number" min="0.01" :max="draft.kind === 'percent' ? 99.99 : 999999999999.99" step="0.01" required class="input" :disabled="saving" />
          </label>
        </div>
        <p v-if="draft.kind === 'percent'" class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.couponPercentHint') }}</p>
        <label class="block space-y-1"><span>{{ t('groupBuy.couponProduct') }}</span>
          <select v-model="draft.product_id" class="input" :disabled="saving"><option :value="null">{{ t('groupBuy.couponAllProducts') }}</option><option v-for="product in products" :key="product.id" :value="product.id">{{ product.name }}</option></select>
        </label>
        <label class="block space-y-1"><span>{{ t('groupBuy.couponExpires') }}</span><input v-model="expiresLocal" type="datetime-local" class="input" :disabled="saving" /><span class="block text-xs text-gray-500">{{ t('groupBuy.couponExpiryHint') }}</span></label>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label class="block space-y-1"><span>{{ t('groupBuy.couponMaxUses') }}</span><input v-model.number="draft.max_uses" type="number" min="0" max="2147483647" step="1" required class="input" :disabled="saving" /></label>
          <label class="block space-y-1"><span>{{ t('groupBuy.couponPerUser') }}</span><input v-model.number="draft.per_user_limit" type="number" min="1" max="2147483647" step="1" required class="input" :disabled="saving" /></label>
        </div>
        <label class="flex items-center gap-2"><input v-model="draft.active" type="checkbox" :disabled="saving" />{{ t('groupBuy.couponActive') }}</label>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.couponLimitsHint') }}</p>
        <p v-if="formError" class="gb-notice gb-notice-error p-3" role="alert">{{ formError }}</p>
        <div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="saving">{{ saving ? t('common.processing') : t('common.save') }}</button></div>
      </form>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import type { GroupBuyProduct, MonthCardCoupon } from '@/types/groupBuy'
import { extractApiErrorMessage } from '@/utils/apiError'
import { cny, exactDate } from './model'
import './glass.css'

defineProps<{ products: GroupBuyProduct[] }>()
const { t } = useI18n()
const coupons = ref<MonthCardCoupon[]>([])
const draft = ref<MonthCardCoupon | null>(null)
const expiresLocal = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const formError = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try { coupons.value = await adminGroupBuyAPI.coupons() }
  catch (err) { error.value = extractApiErrorMessage(err, t('groupBuy.loadFailed')) }
  finally { loading.value = false }
}
function edit(coupon?: MonthCardCoupon) {
  formError.value = ''
  draft.value = coupon ? { ...coupon } : { id: 0, code: '', kind: 'fixed', value: 10, product_id: null, active: true, expires_at: null, max_uses: 0, per_user_limit: 1, used_count: 0 }
  const date = coupon?.expires_at ? new Date(coupon.expires_at) : null
  expiresLocal.value = date ? new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16) : ''
}
function close() { if (!saving.value) draft.value = null }
async function save() {
  if (!draft.value || saving.value) return
  saving.value = true
  formError.value = ''
  try {
    await adminGroupBuyAPI.saveCoupon({ ...draft.value, code: draft.value.code.trim().toUpperCase(), expires_at: expiresLocal.value ? new Date(expiresLocal.value).toISOString() : null })
    draft.value = null
    await load()
  } catch (err) { formError.value = extractApiErrorMessage(err, t('groupBuy.couponSaveFailed')) }
  finally { saving.value = false }
}
onMounted(load)
</script>
