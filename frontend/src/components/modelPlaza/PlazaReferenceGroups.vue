<template>
  <fieldset :aria-describedby="`${idPrefix}-reference-hint`" class="min-w-0">
    <legend class="mb-3 text-sm font-semibold text-gray-700 dark:text-dark-200">{{ t('modelPlaza.catalog.referenceGroup') }}</legend>
    <div class="mb-2 flex flex-wrap items-center gap-2">
      <button type="button" class="reference-action" :disabled="!groups.length" :aria-pressed="modelValue === 'all'" @click="$emit('update:modelValue', 'all')">{{ t('modelPlaza.catalog.referenceSelectAll') }}</button>
      <button type="button" class="reference-action" :disabled="selectedCount === 0" @click="$emit('update:modelValue', [])">{{ t('modelPlaza.catalog.referenceClear') }}</button>
    </div>
    <div class="max-h-60 space-y-1.5 overflow-y-auto rounded-xl p-1">
      <label v-for="group in groups" :key="group.id" :for="`${idPrefix}-reference-${group.id}`" class="reference-option" :class="{ 'reference-option-active': isSelected(group.id) }">
        <input :id="`${idPrefix}-reference-${group.id}`" type="checkbox" :checked="isSelected(group.id)" class="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 accent-primary-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500" @change="toggle(group.id)" />
        <span class="min-w-0 flex-1">
          <span class="block break-words [overflow-wrap:anywhere]">{{ group.name }}</span>
          <span class="mt-0.5 block text-xs font-normal opacity-75">{{ t('modelPlaza.catalog.multiplier', { rate: group.rate }) }}</span>
        </span>
      </label>
      <p v-if="!groups.length" class="py-3 text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.catalog.noGroups') }}</p>
    </div>
    <p class="mt-2 text-xs tabular-nums text-gray-500 dark:text-dark-300" aria-live="polite">{{ t('modelPlaza.catalog.referenceSelected', { count: selectedCount, total: groups.length }) }}</p>
    <p :id="`${idPrefix}-reference-hint`" class="mt-2 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t('modelPlaza.catalog.referenceHint') }}</p>
  </fieldset>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  idPrefix: string
  groups: { id: number; name: string; rate: number }[]
  modelValue: number[] | 'all'
}>()
const emit = defineEmits<{ 'update:modelValue': [number[] | 'all'] }>()
const { t } = useI18n()
function isSelected(id: number) {
  return props.modelValue === 'all' || props.modelValue.includes(id)
}
const selectedCount = computed(() => props.groups.filter(group => isSelected(group.id)).length)
function toggle(id: number) {
  const selected = props.modelValue === 'all' ? props.groups.map(group => group.id) : props.modelValue
  emit('update:modelValue', selected.includes(id) ? selected.filter(value => value !== id) : [...selected, id])
}
</script>

<style scoped>
.reference-action { @apply min-h-9 rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-medium text-gray-600 transition hover:border-primary-300 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 disabled:cursor-not-allowed disabled:opacity-40 dark:border-dark-600 dark:text-dark-200; }
.reference-option { @apply flex min-h-11 cursor-pointer items-start gap-2.5 rounded-xl border border-gray-100 bg-gray-50/60 px-2.5 py-2 text-sm font-medium text-gray-600 transition hover:border-primary-300 dark:border-dark-700 dark:bg-dark-900/40 dark:text-dark-200; }
.reference-option-active { @apply border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-700 dark:bg-primary-900/20 dark:text-primary-300; }
</style>
