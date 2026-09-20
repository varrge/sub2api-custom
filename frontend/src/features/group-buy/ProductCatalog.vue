<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold text-gray-900 dark:text-white">
        {{ t('groupBuy.purchase') }}
      </h2>
      <RouterLink to="/group-buy" class="btn btn-secondary">
        {{ t('groupBuy.hall') }}
      </RouterLink>
    </div>
    <p class="text-sm leading-6 text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.rules') }} {{ t('groupBuy.independent') }}
    </p>
    <p v-if="error" class="gb-notice gb-notice-error p-3" role="alert">
      {{ error }}
      <button class="underline underline-offset-2" @click="load">
        {{ t('groupBuy.retry') }}
      </button>
    </p>
    <p v-else-if="loading" class="py-8 text-center text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </p>
    <p
      v-else-if="!filteredProducts.length"
      class="gb-panel p-8 text-center text-gray-500 dark:text-gray-400"
    >
      {{ t('groupBuy.noProducts') }}
    </p>
    <div class="grid gap-5 sm:grid-cols-2">
      <article
        v-for="product in filteredProducts"
        :key="product.id"
        class="gb-panel gb-panel-hover flex flex-col p-5 sm:p-6"
      >
        <p class="gb-label">
          {{ product.group_name }} · {{ product.platform }}
        </p>
        <h3 class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
          {{ product.name }}
        </h3>
        <p class="gb-accent mt-2 text-3xl font-bold">
          {{ cny(product.price_cny) }}
        </p>
        <p
          v-if="product.description"
          class="mt-2 text-sm text-gray-500 dark:text-gray-400"
        >
          {{ product.description }}
        </p>
        <dl class="my-4 space-y-2 text-sm">
          <div class="flex justify-between">
            <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.baseQuota') }}</dt>
            <dd class="font-semibold text-gray-900 dark:text-white">{{ usd(product.base_quota_usd) }}</dd>
          </div>
          <div class="flex justify-between">
            <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.weeklyQuota') }}</dt>
            <dd class="text-gray-700 dark:text-gray-300">{{ usd(product.base_quota_usd / 4) }}</dd>
          </div>
          <div class="flex justify-between">
            <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.recruitmentHours') }}</dt>
            <dd class="text-gray-700 dark:text-gray-300">{{ product.recruitment_hours }}</dd>
          </div>
        </dl>
        <QuotaLadder class="mb-4" :product="product" />
        <div class="mt-auto grid grid-cols-2 gap-2 pt-2">
          <button
            class="btn btn-secondary"
            @click="$emit('select', { product, mode: 'solo' })"
          >
            {{ t('groupBuy.solo') }}
          </button>
          <button
            class="btn btn-primary"
            @click="$emit('select', { product, mode: 'create' })"
          >
            {{ t('groupBuy.create') }}
          </button>
        </div>
        <p class="mt-3 text-xs leading-5 text-gray-500 dark:text-gray-400">
          {{ t('groupBuy.soloHint') }} {{ t('groupBuy.createHint') }}
        </p>
      </article>
    </div>
  </section>
</template>
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { groupBuyAPI } from '@/api/groupBuy'
import type { GroupBuyProduct, MonthCardSelection } from '@/types/groupBuy'
import { cny, usd } from './model'
import QuotaLadder from './QuotaLadder.vue'
import './glass.css'
const props = defineProps<{ groupId?: number }>()
defineEmits<{ select: [selection: MonthCardSelection] }>()
const { t } = useI18n()
const products = ref<GroupBuyProduct[]>([])
const loading = ref(true)
const error = ref('')
const filteredProducts = computed(() =>
  products.value.filter(
    (product) =>
      product.for_sale && (!props.groupId || product.group_id === props.groupId)
  )
)
async function load() {
  loading.value = true
  error.value = ''
  try {
    products.value = await groupBuyAPI.products()
  } catch {
    error.value = t('groupBuy.loadFailed')
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>
