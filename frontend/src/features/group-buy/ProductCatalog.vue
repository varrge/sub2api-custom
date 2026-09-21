<template>
  <section class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold text-gray-900 dark:text-white">
        {{ t('groupBuy.purchase') }}
      </h2>
      <RouterLink to="/group-buy" class="btn btn-secondary">
        {{ t('groupBuy.hall') }}
      </RouterLink>
    </div>
    <p class="text-sm leading-6 text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.independent') }}
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
    <div class="grid items-start gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <article
        v-for="product in filteredProducts"
        :key="product.id"
        class="gb-panel gb-panel-hover flex flex-col p-4"
      >
        <p class="gb-label">
          {{ product.group_name }} · {{ product.platform }}
        </p>
        <h3 class="mt-0.5 text-base font-semibold text-gray-900 dark:text-white">
          {{ product.name }}
        </h3>
        <p class="mt-1 flex flex-wrap items-baseline gap-x-2">
          <span class="gb-accent text-2xl font-bold">{{ cny(product.price_cny) }}</span>
          <span class="text-xs text-gray-500 dark:text-gray-400">/ {{ t('groupBuy.cardValidity') }}</span>
        </p>
        <dl class="mt-3 space-y-1.5 text-sm">
          <div class="flex justify-between gap-2">
            <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.baseQuota') }}</dt>
            <dd class="font-semibold text-gray-900 dark:text-white">{{ usd(product.base_quota_usd) }}</dd>
          </div>
          <div class="flex justify-between gap-2">
            <dt class="text-gray-500 dark:text-gray-400">{{ t('groupBuy.weeklyQuota') }}</dt>
            <dd class="text-gray-700 dark:text-gray-300">{{ usd(product.base_quota_usd / 4) }}</dd>
          </div>
        </dl>
        <div v-if="product.tiers.length" class="mt-3">
          <p class="gb-label">{{ t('groupBuy.quotaPreview') }}</p>
          <div
            class="gb-preview-scroll mt-1"
            role="group"
            tabindex="0"
            :aria-label="t('groupBuy.quotaPreview')"
          >
            <div class="gb-preview-track">
              <span
                class="gb-preview-line"
                :style="{ left: previewInset(product), right: previewInset(product) }"
                aria-hidden="true"
              />
              <div
                v-for="(step, index) in previewSteps(product)"
                :key="step.members"
                class="gb-preview-step"
              >
                <span
                  class="gb-preview-dot"
                  :class="index === 0 ? 'gb-preview-dot-filled' : 'gb-preview-dot-open'"
                  aria-hidden="true"
                />
                <span class="gb-preview-name">
                  {{
                    index === 0
                      ? t('groupBuy.previewInitial')
                      : t('groupBuy.previewMembers', { members: step.members })
                  }}
                </span>
                <span class="gb-preview-amount" :class="{ 'gb-accent': index === 0 }">
                  {{ usdCompact(step.quota_usd) }}
                </span>
              </div>
            </div>
          </div>
        </div>
        <details class="gb-details mt-3 text-sm">
          <summary>{{ t('groupBuy.cardDetails') }}</summary>
          <div class="gb-details-body">
            <p v-if="product.description" class="text-gray-500 dark:text-gray-400">
              {{ product.description }}
            </p>
            <p class="text-gray-500 dark:text-gray-400">
              {{ t('groupBuy.recruitmentHours') }}: {{ product.recruitment_hours }} · {{ t('groupBuy.maxMembers') }}: {{ product.max_members }}
            </p>
            <QuotaLadder :product="product" />
            <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
              {{ t('groupBuy.rules') }}
            </p>
            <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
              {{ t('groupBuy.soloHint') }} {{ t('groupBuy.createHint') }}
            </p>
          </div>
        </details>
        <div class="mt-auto grid grid-cols-2 gap-2 pt-3">
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
      </article>
    </div>
  </section>
</template>
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { groupBuyAPI } from '@/api/groupBuy'
import type { GroupBuyProduct, MonthCardSelection } from '@/types/groupBuy'
import { cny, quotaSteps, usd } from './model'
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
const previewFormat = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 0,
  maximumFractionDigits: 8
})
const usdCompact = (amount: number) => previewFormat.format(amount)
const previewSteps = (product: GroupBuyProduct) =>
  quotaSteps(product).map(({ members, quota_usd }) => ({ members, quota_usd }))
const previewInset = (product: GroupBuyProduct) =>
  `${50 / (product.tiers.length + 1)}%`
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
<style scoped>
.gb-preview-scroll {
  overflow-x: auto;
  border-radius: 0.5rem;
  scrollbar-width: thin;
}
.gb-preview-scroll:focus-visible {
  outline: 2px solid rgb(20 184 166 / 0.6);
  outline-offset: 2px;
}
.gb-preview-track {
  position: relative;
  display: flex;
  align-items: flex-start;
  width: 100%;
  min-width: max-content;
  padding-top: 0.25rem;
}
.gb-preview-line {
  position: absolute;
  top: calc(0.5rem - 1px);
  height: 2px;
  border-radius: 9999px;
  background: linear-gradient(90deg, #14b8a6 0%, #06b6d4 100%);
  opacity: 0.35;
}
.gb-preview-step {
  flex: 1 1 0;
  min-width: 3.25rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.125rem;
  text-align: center;
}
.gb-preview-dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
  flex: none;
}
.gb-preview-dot-filled {
  background: linear-gradient(135deg, #14b8a6 0%, #06b6d4 100%);
  box-shadow: 0 0 0 3px rgb(20 184 166 / 0.15);
}
.gb-preview-dot-open {
  background: #ffffff;
  border: 1.5px solid rgb(13 148 136 / 0.45);
}
.dark .gb-preview-dot-open {
  background: rgb(15 23 42);
  border-color: rgb(45 212 191 / 0.4);
}
.gb-preview-name {
  white-space: nowrap;
  font-size: 0.6875rem;
  line-height: 1.2;
  color: rgb(100 116 139);
}
.dark .gb-preview-name {
  color: rgb(148 163 184);
}
.gb-preview-amount {
  white-space: nowrap;
  font-size: 0.6875rem;
  font-weight: 600;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
  color: rgb(15 23 42);
}
.dark .gb-preview-amount {
  color: #f8fafc;
}
.gb-details > summary {
  cursor: pointer;
  font-size: 0.8125rem;
  font-weight: 500;
  color: rgb(13 148 136);
  list-style-position: inside;
}
.dark .gb-details > summary {
  color: rgb(45 212 191);
}
.gb-details > summary:focus-visible {
  outline: 2px solid rgb(20 184 166 / 0.6);
  outline-offset: 2px;
  border-radius: 0.25rem;
}
.gb-details-body {
  margin-top: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
</style>
