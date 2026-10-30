<template>
  <fieldset v-if="selectedGroups.length" class="space-y-2" data-testid="account-fallback-groups">
    <legend class="input-label">{{ t('admin.accounts.fallbackGroups') }}</legend>
    <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.fallbackGroupsHint') }}</p>
    <label v-for="group in selectedGroups" :key="group.id" class="flex cursor-pointer items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
      <input
        type="checkbox"
        :value="group.id"
        :checked="modelValue.includes(group.id)"
        :aria-label="t('admin.accounts.fallbackForGroup', { group: group.name })"
        class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
        @change="toggle(group.id, ($event.target as HTMLInputElement).checked)"
      />
      <span class="min-w-0 break-words">{{ group.name }}</span>
      <span class="ml-auto shrink-0 text-xs text-gray-500">{{ t(modelValue.includes(group.id) ? 'admin.accounts.fallbackAccount' : 'admin.accounts.normalAccount') }}</span>
    </label>
  </fieldset>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  modelValue: number[]
  groupIds: number[]
  groups: { id: number; name: string }[]
}>()
const emit = defineEmits<{ 'update:modelValue': [value: number[]] }>()
const { t } = useI18n()
const selectedGroups = computed(() => props.groups.filter(group => props.groupIds.includes(group.id)))

function toggle(id: number, checked: boolean) {
  emit('update:modelValue', checked ? [...props.modelValue, id] : props.modelValue.filter(value => value !== id))
}

watch(() => props.groupIds, ids => {
  const selected = props.modelValue.filter(id => ids.includes(id))
  if (selected.length !== props.modelValue.length) emit('update:modelValue', selected)
}, { deep: true })
</script>
