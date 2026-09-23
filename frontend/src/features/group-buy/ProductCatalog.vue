<template>
  <section class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold text-gray-900 dark:text-white">{{ t('groupBuy.purchase') }}</h2>
      <RouterLink to="/group-buy" class="btn btn-secondary">{{ t('groupBuy.hall') }}</RouterLink>
    </div>
    <p class="text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('groupBuy.independent') }}</p>
    <p v-if="error" class="gb-notice gb-notice-error p-3" role="alert">
      {{ error }} <button class="underline underline-offset-2" @click="load">{{ t('groupBuy.retry') }}</button>
    </p>
    <p v-else-if="loading" class="py-8 text-center text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</p>
    <p v-else-if="!filteredProducts.length" class="gb-panel p-8 text-center text-gray-500 dark:text-gray-400">{{ t('groupBuy.noProducts') }}</p>
    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <article v-for="product in filteredProducts" :key="product.id" class="gb-panel gb-panel-hover flex h-full min-w-0 flex-col p-4">
        <p class="gb-label truncate">{{ product.group_name }} · {{ product.platform }}</p>
        <h3 class="mt-0.5 break-words text-base font-semibold text-gray-900 dark:text-white">{{ product.name }}</h3>
        <p class="mt-1 flex flex-wrap items-baseline gap-x-2">
          <span class="gb-accent text-xl font-bold">{{ cny(product.price_cny) }}</span>
          <span class="text-xs text-gray-500 dark:text-gray-400">/ {{ t('groupBuy.cardValidity') }}</span>
        </p>
        <dl class="mt-2 space-y-1 text-sm">
          <div class="flex items-baseline justify-between gap-2"><dt class="shrink-0 text-gray-500 dark:text-gray-400">{{ t('groupBuy.baseQuota') }}</dt><dd class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ usd(product.base_quota_usd) }}</dd></div>
          <div class="flex items-baseline justify-between gap-2"><dt class="shrink-0 text-gray-500 dark:text-gray-400">{{ t('groupBuy.weeklyQuota') }}</dt><dd class="tabular-nums text-gray-700 dark:text-gray-300">{{ usd(product.base_quota_usd / 4) }}</dd></div>
        </dl>
        <div v-if="product.tiers.length" class="mt-2.5">
          <p class="gb-label">{{ t('groupBuy.quotaPreview') }}</p>
          <div class="gbp-grid mt-1">
            <div v-for="(step, index) in previewSteps(product)" :key="step.members" class="gbp-cell" :class="{ 'gbp-cell-initial': index === 0 }">
              <span class="gbp-cell-label">{{ index === 0 ? t('groupBuy.previewInitial') : t('groupBuy.previewMembers', { members: step.members }) }}</span>
              <span class="gbp-cell-amount" :class="{ 'gb-accent': index === 0 }">{{ usdCompact(step.quota_usd) }}</span>
            </div>
            <button v-if="hasMoreTiers(product)" type="button" class="gbp-cell gbp-more" @click="openDetails(product)"><span>{{ t('groupBuy.inspect') }}</span><span aria-hidden="true">›</span></button>
          </div>
        </div>
        <div class="mt-auto pt-3">
          <button type="button" class="gbp-rules" @click="openDetails(product)">{{ t('groupBuy.viewRules') }}</button>
          <div class="mt-2 grid grid-cols-2 gap-2">
            <button class="btn btn-secondary" @click="$emit('select', { product, mode: 'solo' })">{{ t('groupBuy.solo') }}</button>
            <button class="btn btn-primary" @click="$emit('select', { product, mode: 'create' })">{{ t('groupBuy.create') }}</button>
          </div>
        </div>
      </article>
    </div>
    <ProductDetailsDialog :show="detailsOpen" :product="detailProduct" @close="detailsOpen = false" />
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { groupBuyAPI } from '@/api/groupBuy'
import type { GroupBuyProduct, MonthCardSelection } from '@/types/groupBuy'
import { cny, quotaSteps, usd } from './model'
import ProductDetailsDialog from './ProductDetailsDialog.vue'
import './glass.css'

const props = defineProps<{ groupId?: number }>()
defineEmits<{ select: [selection: MonthCardSelection] }>()
const { t } = useI18n()
const products = ref<GroupBuyProduct[]>([])
const loading = ref(true)
const error = ref('')
const detailsOpen = ref(false)
const detailProduct = ref<GroupBuyProduct | null>(null)
const filteredProducts = computed(() => products.value.filter(p => p.for_sale && (!props.groupId || p.group_id === props.groupId)))
const previewFormat = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 0, maximumFractionDigits: 8 })
const usdCompact = (amount: number) => previewFormat.format(amount)
function previewSteps(product: GroupBuyProduct) {
  const steps = quotaSteps(product).map(({ members, quota_usd }) => ({ members, quota_usd }))
  if (steps.length <= 4) return steps
  const tiers = steps.slice(1)
  const mid = Math.floor((tiers.length - 1) / 2)
  const indexes = [...new Set([0, 1, mid, tiers.length - 1])]
  return [steps[0], ...indexes.map(index => tiers[index])]
}
const hasMoreTiers = (product: GroupBuyProduct) => product.tiers.length > previewSteps(product).length - 1
function openDetails(product: GroupBuyProduct) { detailProduct.value = product; detailsOpen.value = true }
async function load() {
  loading.value = true; error.value = ''
  try { products.value = await groupBuyAPI.products() } catch { error.value = t('groupBuy.loadFailed') } finally { loading.value = false }
}
onMounted(load)
</script>

<style scoped>
.gbp-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: .375rem; }
.gbp-cell { display: flex; min-width: 0; flex-direction: column; align-items: center; justify-content: center; gap: .0625rem; border-radius: .5rem; padding: .375rem .25rem; text-align: center; background: rgb(250 244 237 / .8); border: 1px solid rgb(242 215 192 / .4); }
.gbp-cell-initial { border-color: rgb(201 151 111 / .55); background: linear-gradient(135deg, rgb(250 244 237 / .95), rgb(247 230 216 / .85)); }
.dark .gbp-cell { background: rgb(61 40 18 / .35); border-color: rgb(107 69 32 / .5); }
.gbp-cell-label, .gbp-cell-amount { max-width: 100%; overflow-wrap: anywhere; line-height: 1.25; }
.gbp-cell-label { font-size: .6875rem; color: rgb(110 110 118); }
.gbp-cell-amount { font-size: .75rem; font-weight: 600; font-variant-numeric: tabular-nums; color: rgb(15 23 42); }
.dark .gbp-cell-label { color: rgb(161 161 170); } .dark .gbp-cell-amount { color: #f8fafc; }
.gbp-more { cursor: pointer; color: rgb(138 92 36); } .dark .gbp-more, .dark .gbp-rules { color: rgb(229 185 154); }
.gbp-more:focus-visible, .gbp-rules:focus-visible { outline: 2px solid rgb(201 151 111 / .6); outline-offset: 2px; }
.gbp-rules { width: 100%; text-align: left; font-size: .75rem; font-weight: 500; color: rgb(138 92 36); text-decoration: underline dotted; text-underline-offset: 3px; }
</style>
