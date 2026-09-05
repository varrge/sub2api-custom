<template>
  <div class="space-y-7">
    <div>
      <h2 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('modelPlaza.catalog.filters') }}</h2>
      <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-400">{{ t('modelPlaza.catalog.filtersHint') }}</p>
    </div>
    <fieldset>
      <legend class="mb-3 text-sm font-semibold text-gray-700 dark:text-dark-200">{{ t('modelPlaza.catalog.type') }}</legend>
      <div class="grid grid-cols-2 gap-2">
        <button v-for="type in types" :key="type" type="button" class="filter-option justify-center" :class="{ 'filter-option-active': modelType === type }" :aria-pressed="modelType === type" @click="$emit('update:modelType', type)">
          {{ t(`modelPlaza.catalog.types.${type}`) }}
        </button>
      </div>
    </fieldset>
    <div>
      <label :for="`${idPrefix}-group`" class="mb-3 block text-sm font-semibold text-gray-700 dark:text-dark-200">{{ t('modelPlaza.catalog.group') }}</label>
      <select :id="`${idPrefix}-group`" :value="groupId ?? ''" class="input w-full text-sm" @change="$emit('update:groupId', selectedId($event))">
        <option value="">{{ t('modelPlaza.catalog.allGroups') }}</option>
        <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} ×{{ group.rate }}</option>
      </select>
    </div>
    <div v-if="groupId === null">
      <label :for="`${idPrefix}-reference`" class="mb-3 block text-sm font-semibold text-gray-700 dark:text-dark-200">{{ t('modelPlaza.catalog.referenceGroup') }}</label>
      <select :id="`${idPrefix}-reference`" :value="referenceGroupId ?? ''" class="input w-full text-sm" :disabled="!groups.length" @change="$emit('update:referenceGroupId', selectedId($event))">
        <option v-if="!groups.length" value="">{{ t('modelPlaza.catalog.noGroups') }}</option>
        <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} ×{{ group.rate }}</option>
      </select>
      <p class="mt-2 text-xs leading-5 text-gray-400 dark:text-dark-400">{{ t('modelPlaza.catalog.referenceHint') }}</p>
    </div>
    <fieldset>
      <legend class="mb-3 text-sm font-semibold text-gray-700 dark:text-dark-200">{{ t('modelPlaza.catalog.suppliers') }}</legend>
      <div class="space-y-2">
        <button type="button" class="filter-option" :class="{ 'filter-option-active': supplierId === 'all' }" :aria-pressed="supplierId === 'all'" @click="$emit('update:supplierId', 'all')">
          <span>{{ t('modelPlaza.catalog.allSuppliers') }}</span><span class="supplier-count">{{ supplierTotal }}</span>
        </button>
        <button v-for="supplier in suppliers" :key="supplier.id" type="button" class="filter-option" :class="{ 'filter-option-active': supplierId === supplier.id }" :aria-pressed="supplierId === supplier.id" @click="$emit('update:supplierId', supplier.id)">
          <span class="flex min-w-0 items-center gap-2.5"><PlatformIcon :platform="supplier.platform" size="lg" :class="supplier.id === 'claude' ? 'text-orange-500' : ''" /><span class="truncate">{{ supplier.name }}</span></span>
          <span class="supplier-count">{{ supplier.count }}</span>
        </button>
      </div>
    </fieldset>
    <button type="button" class="rounded-lg text-sm text-gray-500 underline underline-offset-4 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-dark-300" @click="$emit('reset')">{{ t('modelPlaza.catalog.reset') }}</button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { PlazaBrandInfo, PlazaModelType } from './plaza-models'

defineProps<{
  idPrefix: string
  groups: { id: number; name: string; rate: number }[]
  groupId: number | null
  referenceGroupId: number | null
  modelType: PlazaModelType | 'all'
  suppliers: (PlazaBrandInfo & { count: number })[]
  supplierTotal: number
  supplierId: string
}>()
defineEmits<{
  'update:groupId': [number | null]
  'update:referenceGroupId': [number | null]
  'update:modelType': [PlazaModelType | 'all']
  'update:supplierId': [string]
  reset: []
}>()
const { t } = useI18n()
const types = ['all', 'text', 'image', 'video'] as const
function selectedId(event: Event): number | null {
  const value = (event.target as HTMLSelectElement).value
  return value === '' ? null : Number(value)
}
</script>

<style scoped>
.filter-option {
  @apply flex min-h-11 w-full items-center justify-between gap-2 rounded-2xl border border-gray-100 bg-gray-50/60 px-3.5 py-2.5 text-left text-sm font-medium text-gray-600 transition hover:border-primary-300 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 dark:border-dark-700 dark:bg-dark-900/40 dark:text-dark-200;
}
.filter-option-active {
  @apply border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-700 dark:bg-primary-900/20 dark:text-primary-300;
}
.supplier-count {
  @apply rounded-full bg-white px-2 py-0.5 text-xs tabular-nums dark:bg-dark-700;
}
</style>
