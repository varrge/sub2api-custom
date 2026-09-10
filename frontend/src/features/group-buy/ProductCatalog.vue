<template>
  <section class="space-y-4">
    <div class="flex flex-wrap justify-between gap-3">
      <h2 class="text-xl font-bold">{{ t('groupBuy.purchase') }}</h2>
      <RouterLink to="/group-buy" class="btn btn-secondary">
        {{ t('groupBuy.hall') }}
      </RouterLink>
    </div>
    <p class="text-sm leading-6 text-gray-500">
      {{ t('groupBuy.rules') }} {{ t('groupBuy.independent') }}
    </p>
    <p v-if="error" class="text-sm text-red-600" role="alert">
      {{ error }}
      <button class="underline" @click="load">{{ t('groupBuy.retry') }}</button>
    </p>
    <p v-else-if="loading" class="py-8 text-center">
      {{ t('common.loading') }}
    </p>
    <p
      v-else-if="!filteredProducts.length"
      class="card p-8 text-center text-gray-500"
    >
      {{ t('groupBuy.noProducts') }}
    </p>
    <div class="grid gap-5 sm:grid-cols-2">
      <article
        v-for="product in filteredProducts"
        :key="product.id"
        class="rounded-2xl border-2 border-emerald-100 bg-white p-5 dark:border-emerald-900 dark:bg-dark-800"
      >
        <p class="text-xs text-gray-500">
          {{ product.group_name }} · {{ product.platform }}
        </p>
        <h3 class="mt-1 text-lg font-semibold">{{ product.name }}</h3>
        <p class="mt-2 text-3xl font-bold text-emerald-600">
          {{ cny(product.price_cny) }}
        </p>
        <p v-if="product.description" class="mt-2 text-sm text-gray-500">
          {{ product.description }}
        </p>
        <dl class="my-4 space-y-2 text-sm">
          <div class="flex justify-between">
            <dt>{{ t('groupBuy.baseQuota') }}</dt>
            <dd class="font-semibold">{{ usd(product.base_quota_usd) }}</dd>
          </div>
          <div class="flex justify-between">
            <dt>{{ t('groupBuy.weeklyQuota') }}</dt>
            <dd>{{ usd(product.base_quota_usd / 4) }}</dd>
          </div>
          <div class="flex justify-between">
            <dt>{{ t('groupBuy.recruitmentHours') }}</dt>
            <dd>{{ product.recruitment_hours }}</dd>
          </div>
        </dl>
        <p class="text-xs font-medium text-gray-500">
          {{ t('groupBuy.tiers') }}
        </p>
        <div class="my-3 flex flex-wrap gap-2">
          <span
            v-for="tier in product.tiers"
            :key="tier.members"
            class="rounded-md bg-emerald-50 px-2 py-1 text-xs text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"
          >
            {{
              t('groupBuy.tier', {
                members: tier.members,
                amount: usd(tier.quota_usd)
              })
            }}
          </span>
        </div>
        <div class="grid grid-cols-2 gap-2">
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
        <p class="mt-3 text-xs leading-5 text-gray-500">
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
