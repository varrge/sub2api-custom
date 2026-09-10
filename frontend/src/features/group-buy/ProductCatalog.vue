<template>
  <section class="space-y-6">
    <!-- Header: title, subtitle and hall link -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-xl sm:text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
          {{ t('groupBuy.purchase') }}
        </h2>
        <p class="mt-1 text-sm text-slate-500 dark:text-dark-400">
          {{ t('groupBuy.subtitle') }}
        </p>
      </div>
      <RouterLink
        to="/group-buy"
        class="inline-flex items-center justify-center gap-1.5 rounded-xl border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm transition-colors hover:bg-slate-50 hover:text-slate-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teal-500 dark:border-dark-700 dark:bg-dark-800 dark:text-slate-200 dark:hover:bg-dark-700 dark:hover:text-white shrink-0 self-start sm:self-auto"
      >
        <svg aria-hidden="true" class="h-4 w-4 text-teal-600 dark:text-teal-400" viewBox="0 0 20 20" fill="currentColor">
          <path fill-rule="evenodd" d="M10 2a1 1 0 00-.707.293l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h4a1 1 0 001-1v-3a1 1 0 011-1h0a1 1 0 011 1v3a1 1 0 001 1h4a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7A1 1 0 0010 2z" clip-rule="evenodd" />
        </svg>
        <span>{{ t('groupBuy.hall') }}</span>
        <svg aria-hidden="true" class="h-3.5 w-3.5 text-slate-400 dark:text-dark-400" viewBox="0 0 20 20" fill="currentColor">
          <path fill-rule="evenodd" d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z" clip-rule="evenodd" />
        </svg>
      </RouterLink>
    </div>

    <!-- Rule Disclosure Banner -->
    <div class="rounded-xl border border-slate-200/80 bg-slate-50/80 p-3.5 text-xs text-slate-600 dark:border-dark-700/80 dark:bg-dark-800/50 dark:text-slate-300">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs">
          <span class="inline-flex items-center gap-1.5 font-medium text-slate-700 dark:text-slate-200">
            <svg aria-hidden="true" class="h-3.5 w-3.5 text-teal-600 dark:text-teal-400 shrink-0" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd" />
            </svg>
            {{ t('groupBuy.ruleHighlight1') }}
          </span>
          <span class="inline-flex items-center gap-1.5 font-medium text-slate-700 dark:text-slate-200">
            <svg aria-hidden="true" class="h-3.5 w-3.5 text-teal-600 dark:text-teal-400 shrink-0" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd" />
            </svg>
            {{ t('groupBuy.ruleHighlight2') }}
          </span>
          <span class="inline-flex items-center gap-1.5 font-medium text-slate-700 dark:text-slate-200">
            <svg aria-hidden="true" class="h-3.5 w-3.5 text-teal-600 dark:text-teal-400 shrink-0" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd" />
            </svg>
            {{ t('groupBuy.ruleHighlight3') }}
          </span>
        </div>
        <button
          type="button"
          class="inline-flex items-center gap-1 font-medium text-teal-600 hover:text-teal-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teal-500 rounded text-xs dark:text-teal-400 dark:hover:text-teal-300"
          :aria-expanded="showRules"
          @click="showRules = !showRules"
        >
          <span>{{ showRules ? t('groupBuy.hideRules') : t('groupBuy.viewRules') }}</span>
          <svg
            aria-hidden="true"
            :class="['h-3.5 w-3.5 transition-transform duration-200', showRules ? 'rotate-180' : '']"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
          </svg>
        </button>
      </div>
      <div v-show="showRules" class="mt-2.5 border-t border-slate-200/60 pt-2.5 leading-relaxed text-slate-500 dark:border-dark-700/60 dark:text-dark-400 space-y-1">
        <p>{{ t('groupBuy.rules') }}</p>
        <p>{{ t('groupBuy.independent') }} {{ t('groupBuy.createHint') }}</p>
      </div>
    </div>

    <!-- Error State -->
    <div
      v-if="error"
      class="rounded-2xl border border-red-200 bg-red-50/70 p-6 text-center text-sm text-red-600 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-400"
      role="alert"
    >
      <p>{{ error }}</p>
      <button
        type="button"
        class="mt-3 inline-flex items-center gap-1.5 rounded-lg bg-red-100 px-3 py-1.5 text-xs font-semibold text-red-700 hover:bg-red-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500 dark:bg-red-900/60 dark:text-red-200 dark:hover:bg-red-900"
        @click="load"
      >
        {{ t('groupBuy.retry') }}
      </button>
    </div>

    <!-- Loading State -->
    <div v-else-if="loading" class="flex flex-col items-center justify-center py-16 text-center">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-teal-600 border-t-transparent dark:border-teal-400 dark:border-t-transparent" />
      <p class="mt-3 text-sm text-slate-500 dark:text-dark-400">{{ t('common.loading') }}</p>
    </div>

    <!-- Empty State -->
    <div
      v-else-if="!filteredProducts.length"
      class="flex flex-col items-center justify-center rounded-2xl border border-dashed border-slate-200 p-12 text-center text-slate-500 dark:border-dark-700 dark:text-dark-400"
    >
      <p class="text-sm font-medium">{{ t('groupBuy.noProducts') }}</p>
    </div>

    <!-- Product List -->
    <div
      v-else
      :class="[
        'grid gap-6 items-stretch',
        filteredProducts.length === 1 ? 'mx-auto max-w-xl grid-cols-1' : 'grid-cols-1 md:grid-cols-2'
      ]"
    >
      <ProductCard
        v-for="product in filteredProducts"
        :key="product.id"
        :product="product"
        @select="(selection) => $emit('select', selection)"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { groupBuyAPI } from '@/api/groupBuy'
import type { GroupBuyProduct, MonthCardSelection } from '@/types/groupBuy'
import ProductCard from './ProductCard.vue'

const props = defineProps<{ groupId?: number }>()
defineEmits<{ select: [selection: MonthCardSelection] }>()
const { t } = useI18n()

const products = ref<GroupBuyProduct[]>([])
const loading = ref(true)
const error = ref('')
const showRules = ref(false)

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
