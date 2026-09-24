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
          <td>{{ couponProductsLabel(coupon) }}</td>
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
          <input v-model="draft.code" class="input" required minlength="2" maxlength="64" pattern="[A-Za-z0-9_-]{2,64}" data-test="coupon-code" :disabled="!!draft.id || saving" :placeholder="t('groupBuy.couponCodeHint')" />
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
        <fieldset class="space-y-2"><legend>{{ t('groupBuy.couponProduct') }}</legend>
          <div class="flex flex-wrap gap-x-4 gap-y-1">
            <label class="flex items-center gap-2"><input v-model="productScope" type="radio" name="coupon-product-scope" value="all" data-test="coupon-products-all" :disabled="saving" />{{ t('groupBuy.couponAllProducts') }}</label>
            <label class="flex items-center gap-2"><input v-model="productScope" type="radio" name="coupon-product-scope" value="selected" data-test="coupon-products-selected" :disabled="saving" />{{ t('groupBuy.couponSelectedProducts') }}</label>
          </div>
          <div v-if="productScope === 'selected'" class="space-y-2">
            <input v-model="productSearch" class="input" data-test="coupon-product-search" :placeholder="t('groupBuy.couponProductSearch')" :aria-label="t('groupBuy.couponProductSearch')" :disabled="saving" />
            <div class="max-h-40 space-y-1 overflow-y-auto rounded-lg border border-gray-200 p-2 dark:border-gray-700">
              <label v-for="product in filteredProducts" :key="product.id" class="flex items-center gap-2">
                <input v-model="draft.product_ids" type="checkbox" class="shrink-0" :value="product.id" :data-product-id="product.id" :disabled="saving" /><span class="min-w-0 break-words">{{ product.name }}</span>
              </label>
              <p v-if="!filteredProducts.length" class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.couponNoMatchingProducts') }}</p>
            </div>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupBuy.couponSelectedCount', { count: draft.product_ids.length }) }}</p>
            <p v-if="!draft.product_ids.length" class="gb-notice gb-notice-error p-2 text-xs" role="alert">{{ t('groupBuy.couponSelectRequired') }}</p>
          </div>
        </fieldset>
        <label class="block space-y-1"><span>{{ t('groupBuy.couponExpires') }}</span><input v-model="expiresLocal" type="datetime-local" class="input" :disabled="saving" /><span class="block text-xs text-gray-500">{{ t('groupBuy.couponExpiryHint') }}</span></label>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label class="block space-y-1"><span>{{ t('groupBuy.couponMaxUses') }}</span><input v-model.number="draft.max_uses" type="number" min="0" max="2147483647" step="1" required class="input" :disabled="saving" /></label>
          <label class="block space-y-1"><span>{{ t('groupBuy.couponPerUser') }}</span><input v-model.number="draft.per_user_limit" type="number" min="0" max="2147483647" step="1" required class="input" data-test="coupon-per-user" :disabled="saving" /></label>
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
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import type { GroupBuyProduct, MonthCardCoupon } from '@/types/groupBuy'
import { extractApiErrorMessage } from '@/utils/apiError'
import { cny, exactDate } from './model'
import './glass.css'

const props = defineProps<{ products: GroupBuyProduct[] }>()
const { t } = useI18n()
const coupons = ref<MonthCardCoupon[]>([])
type CouponDraft = MonthCardCoupon & { product_ids: number[] }
const draft = ref<CouponDraft | null>(null)
const productScope = ref<'all' | 'selected'>('all')
const productSearch = ref('')
const filteredProducts = computed(() => {
  const choices = props.products.map(product => ({ id: product.id, name: product.name }))
  for (const id of draft.value?.product_ids ?? []) {
    if (!choices.some(product => product.id === id)) choices.push({ id, name: `#${id}` })
  }
  const query = productSearch.value.trim().toLowerCase()
  return choices.filter(product => product.name.toLowerCase().includes(query) || String(product.id).includes(query))
})
function couponProductsLabel(coupon: MonthCardCoupon) {
  const ids = coupon.product_ids ?? (coupon.product_id ? [coupon.product_id] : [])
  return ids.length ? ids.map(id => props.products.find(product => product.id === id)?.name || `#${id}`).join('、') : t('groupBuy.couponAllProducts')
}
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
  const ids = coupon?.product_ids ?? (coupon?.product_id ? [coupon.product_id] : [])
  draft.value = coupon ? { ...coupon, product_ids: [...ids] } : { id: 0, code: '', kind: 'fixed', value: 10, product_id: null, product_ids: [], active: true, expires_at: null, max_uses: 0, per_user_limit: 1, used_count: 0 }
  productScope.value = ids.length ? 'selected' : 'all'
  productSearch.value = ''
  const date = coupon?.expires_at ? new Date(coupon.expires_at) : null
  expiresLocal.value = date ? new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16) : ''
}
function close() { if (!saving.value) draft.value = null }
async function save() {
  if (!draft.value || saving.value) return
  formError.value = ''
  if (productScope.value === 'selected' && !draft.value.product_ids.length) return
  const productIDs = productScope.value === 'all' ? [] : [...new Set(draft.value.product_ids)].sort((a, b) => a - b)
  saving.value = true
  try {
    await adminGroupBuyAPI.saveCoupon({ ...draft.value, product_ids: productIDs, product_id: productIDs[0] ?? null, code: draft.value.code.trim().toUpperCase(), expires_at: expiresLocal.value ? new Date(expiresLocal.value).toISOString() : null })
    draft.value = null
    await load()
  } catch (err) { formError.value = extractApiErrorMessage(err, t('groupBuy.couponSaveFailed')) }
  finally { saving.value = false }
}
onMounted(load)
</script>
