<template>
  <section class="catalog-shell overflow-hidden rounded-3xl border border-gray-200/80 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900" data-testid="model-catalog">
    <header class="flex flex-wrap items-center justify-between gap-5 border-b border-gray-100 px-5 py-6 dark:border-dark-700 md:px-7">
      <div class="flex flex-wrap items-center gap-3">
        <h2 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">{{ t('modelPlaza.title') }}</h2>
        <span class="rounded-xl bg-gradient-to-r from-amber-400 to-orange-400 px-3 py-1.5 text-sm font-bold text-white" :title="t('modelPlaza.catalog.creditHint')">{{ t('modelPlaza.catalog.creditBadge') }}</span>
      </div>
      <div class="flex gap-3">
        <div class="min-w-24 rounded-2xl border border-gray-200 px-4 py-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.catalog.totalModels') }}</p>
          <p class="mt-1 text-2xl font-semibold tabular-nums text-gray-950 dark:text-white">{{ allModels.length }}</p>
        </div>
        <div class="min-w-24 rounded-2xl border border-gray-200 px-4 py-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.catalog.suppliers') }}</p>
          <p class="mt-1 text-2xl font-semibold tabular-nums text-gray-950 dark:text-white">{{ allSupplierCount }}</p>
        </div>
      </div>
    </header>

    <details v-if="descriptionHtml" class="border-b border-gray-100 px-5 py-3 dark:border-dark-700 md:px-7">
      <summary class="cursor-pointer text-sm font-medium text-gray-600 dark:text-dark-300">{{ t('modelPlaza.catalog.billingNotes') }}</summary>
      <div class="plaza-description mt-3 text-sm" v-html="descriptionHtml"></div>
    </details>
    <p v-if="!isAuthenticated" class="px-5 pt-4 text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.anonymousHint') }}</p>

    <div class="catalog-workspace">
      <aside class="catalog-filter-rail hidden border-r border-gray-100 p-5 dark:border-dark-700 xl:block">
        <PlazaFilters v-bind="filterProps" id-prefix="catalog-desktop" v-on="filterEvents" />
      </aside>
      <div class="min-w-0">
        <div class="flex items-center gap-3 border-b border-gray-100 p-5 dark:border-dark-700">
          <button type="button" class="btn btn-secondary shrink-0 xl:hidden" :aria-label="t('modelPlaza.catalog.filters')" @click="openFilters">
            <Icon name="menu" size="sm" /><span class="hidden sm:inline">{{ t('modelPlaza.catalog.filters') }}</span>
          </button>
          <div class="relative min-w-0 flex-1">
            <Icon name="search" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model="searchQuery" type="search" class="input w-full rounded-2xl pl-10 text-sm" :aria-label="t('modelPlaza.searchPlaceholder')" :placeholder="t('modelPlaza.searchPlaceholder')" />
          </div>
          <span class="hidden shrink-0 text-xs tabular-nums text-gray-400 sm:block" aria-live="polite">{{ t('modelPlaza.catalog.results', { count: filteredModels.length, total: allModels.length }) }}</span>
        </div>
        <p class="flex flex-wrap items-center justify-between gap-2 px-5 pt-4 text-xs text-gray-500 dark:text-dark-400">
          <span>{{ t('modelPlaza.catalog.priceSource', { group: priceGroup?.name ?? t('modelPlaza.catalog.noGroups') }) }}</span>
          <span class="sm:hidden" aria-live="polite">{{ t('modelPlaza.catalog.results', { count: filteredModels.length, total: allModels.length }) }}</span>
        </p>
        <div v-if="loading" class="flex min-h-64 items-center justify-center" role="status" :aria-label="t('modelPlaza.loading')">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500/20 border-t-primary-500"></div>
        </div>
        <p v-else-if="error" role="alert" class="m-5 rounded-2xl bg-red-50 p-8 text-center text-sm text-red-600 dark:bg-red-900/20 dark:text-red-300">{{ t('modelPlaza.loadFailed') }}</p>
        <div v-else-if="filteredModels.length" class="catalog-grid p-5">
          <PlazaModelCard v-for="model in filteredModels" :key="`${selectionRevision}:${model.id}`" :model="model" :price-group-id="priceGroupId" @open-group-detail="activeDetail = $event" />
        </div>
        <p v-else class="m-5 rounded-2xl border border-dashed border-gray-200 px-5 py-14 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">{{ searchActive ? t('modelPlaza.noSearchResult') : t('modelPlaza.empty') }}</p>
      </div>
    </div>

    <dialog ref="filterDrawer" class="catalog-drawer m-0 h-dvh max-h-none w-[min(88vw,340px)] max-w-none border-r border-gray-200 bg-white p-0 text-gray-900 shadow-2xl dark:border-dark-700 dark:bg-dark-900 dark:text-white" :aria-label="t('modelPlaza.catalog.filters')" @click="onDrawerBackdrop">
      <div class="flex h-full flex-col">
        <div class="flex justify-end border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <button type="button" class="btn-ghost btn-icon" :aria-label="t('common.close')" @click="closeFilters"><Icon name="x" size="md" /></button>
        </div>
        <div class="flex-1 overflow-y-auto p-5"><PlazaFilters v-bind="filterProps" id-prefix="catalog-mobile" v-on="filterEvents" /></div>
        <div class="border-t border-gray-100 p-4 dark:border-dark-700"><button type="button" class="btn btn-primary w-full" @click="closeFilters">{{ t('modelPlaza.catalog.showResults', { count: filteredModels.length }) }}</button></div>
      </div>
    </dialog>
    <BaseDialog :show="activeDetail !== null" :title="activeDetail ? `${activeDetail.group.name} · ${activeDetail.model.name}` : t('modelPlaza.pricingDetail')" width="extra-wide" @close="activeDetail = null">
      <PlazaGroupSection v-if="activeDetail" :group="{ ...activeDetail.group, models: [activeDetail.model] }" />
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { effectiveGroupRate, useTemporaryRateNow } from '@/utils/temporary-rate'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import { aggregatePlazaModels, plazaModelType, type PlazaModelType, type PlazaBrandInfo, type GroupModelVariant } from './plaza-models'
import PlazaModelCard from './PlazaModelCard.vue'
import PlazaFilters from './PlazaFilters.vue'
import PlazaGroupSection from './PlazaGroupSection.vue'

const props = defineProps<{ response: ModelPlazaResponse | null; loading: boolean; error?: boolean; embedded?: boolean }>()
const { t } = useI18n()
const authStore = useAuthStore()
const isAuthenticated = computed(() => authStore.isAuthenticated)
const now = useTemporaryRateNow()
const groupId = ref<number | null>(null)
const referenceGroupId = ref<number | null>(null)
const supplierId = ref('all')
const modelType = ref<PlazaModelType | 'all'>('all')
const searchQuery = ref('')
const activeDetail = ref<GroupModelVariant | null>(null)
const filterDrawer = ref<HTMLDialogElement | null>(null)
const selectionRevision = ref(0)

const groups = computed(() => [...(props.response?.groups ?? [])].sort((a, b) => a.id - b.id))
const groupOptions = computed(() => groups.value.map(group => ({ id: group.id, name: group.name, rate: effectiveGroupRate(group, group.user_rate_multiplier, now.value) })))
watch(groups, list => {
  if (!list.some(group => group.id === groupId.value)) groupId.value = null
  if (!list.some(group => group.id === referenceGroupId.value)) referenceGroupId.value = list[0]?.id ?? null
}, { immediate: true })
watch([groupId, referenceGroupId], () => { selectionRevision.value++ })
const priceGroupId = computed(() => groupId.value ?? referenceGroupId.value)
const priceGroup = computed(() => groups.value.find(group => group.id === priceGroupId.value))
const allModels = computed(() => aggregatePlazaModels(groups.value, now.value))
const allSupplierCount = computed(() => new Set(allModels.value.map(model => model.brand.id)).size)

const matchingModels = computed(() => allModels.value.filter(model => {
  const variants = groupId.value === null ? model.variants : model.variants.filter(variant => variant.group.id === groupId.value)
  return variants.length > 0 &&
    (modelType.value === 'all' || variants.some(variant => plazaModelType(variant.model) === modelType.value)) &&
    model.name.toLowerCase().includes(searchQuery.value.trim().toLowerCase())
}))
const suppliers = computed(() => {
  const counts = new Map<string, PlazaBrandInfo & { count: number }>()
  for (const model of matchingModels.value) {
    const supplier = counts.get(model.brand.id) ?? { ...model.brand, count: 0 }
    supplier.count++
    counts.set(supplier.id, supplier)
  }
  return [...counts.values()].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
})
// Keep a selected supplier visible when other filters reduce its matching count to zero.
const supplierOptions = computed(() => {
  if (supplierId.value === 'all' || suppliers.value.some(supplier => supplier.id === supplierId.value)) return suppliers.value
  const brand = allModels.value.find(model => model.brand.id === supplierId.value)?.brand
  return brand ? [...suppliers.value, { ...brand, count: 0 }] : suppliers.value
})
watch(allModels, list => {
  if (!list.some(model => model.brand.id === supplierId.value)) supplierId.value = 'all'
})
const filteredModels = computed(() => matchingModels.value.filter(model => supplierId.value === 'all' || model.brand.id === supplierId.value))
const searchActive = computed(() => !!searchQuery.value.trim() || groupId.value !== null || supplierId.value !== 'all' || modelType.value !== 'all')
const descriptionHtml = computed(() => DOMPurify.sanitize(marked.parse(props.response?.description?.trim() ?? '') as string))
const filterProps = computed(() => ({
  groups: groupOptions.value, groupId: groupId.value, referenceGroupId: referenceGroupId.value,
  modelType: modelType.value, suppliers: supplierOptions.value, supplierTotal: matchingModels.value.length, supplierId: supplierId.value
}))
const filterEvents = {
  'update:groupId': (value: number | null) => { groupId.value = value },
  'update:referenceGroupId': (value: number | null) => { referenceGroupId.value = value },
  'update:modelType': (value: PlazaModelType | 'all') => { modelType.value = value },
  'update:supplierId': (value: string) => { supplierId.value = value },
  reset: () => { groupId.value = null; supplierId.value = 'all'; modelType.value = 'all'; searchQuery.value = ''; selectionRevision.value++ }
}
function openFilters() {
  if (!filterDrawer.value || filterDrawer.value.open) return
  filterDrawer.value.showModal()
}
function closeFilters() {
  if (filterDrawer.value?.open) filterDrawer.value.close()
}
function onDrawerBackdrop(event: MouseEvent) {
  if (event.target === filterDrawer.value) closeFilters()
}
onBeforeUnmount(closeFilters)
</script>

<style scoped>
.catalog-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 285px), 1fr)); gap: 18px; align-items: stretch; }
.catalog-workspace { display: grid; grid-template-columns: minmax(0, 1fr); }
@media (min-width: 1280px) { .catalog-workspace { grid-template-columns: 230px minmax(0, 1fr); } }
.catalog-drawer::backdrop { background: rgb(15 23 42 / 40%); backdrop-filter: blur(3px); }
:global(body:has(.catalog-drawer[open])) { overflow: hidden; }
.plaza-description { line-height: 1.7; overflow-wrap: anywhere; }
.plaza-description :deep(h1), .plaza-description :deep(h2), .plaza-description :deep(h3) { @apply mb-2 mt-3 font-semibold text-gray-900 first:mt-0 dark:text-white; }
.plaza-description :deep(p), .plaza-description :deep(li) { @apply mb-2 text-gray-700 last:mb-0 dark:text-dark-200; }
.plaza-description :deep(a) { @apply text-primary-600 underline underline-offset-4 dark:text-primary-300; }
.plaza-description :deep(ul) { @apply mb-2 list-disc pl-5; }
.plaza-description :deep(ol) { @apply mb-2 list-decimal pl-5; }
.plaza-description :deep(code) { @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs dark:bg-dark-800; }
.plaza-description :deep(blockquote) { @apply my-2 border-l-4 border-gray-300 pl-3 text-gray-600 dark:border-dark-600 dark:text-dark-300; }
</style>
