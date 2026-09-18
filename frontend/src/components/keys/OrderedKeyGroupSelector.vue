<template>
  <div class="space-y-3">
    <p class="text-sm text-gray-600 dark:text-gray-400">{{ t('keys.multiGroup.orderHint') }}</p>
    <ol class="space-y-2" :aria-label="t('keys.multiGroup.selectedGroups')">
      <li v-for="(id, index) in modelValue" :key="id" :data-group-id="id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-600">
        <div class="flex items-start gap-2">
          <span class="pt-1 text-sm tabular-nums text-gray-500">{{ index + 1 }}.</span>
          <div class="min-w-0 flex-1">
            <GroupOptionItem v-if="groupsById.get(id)" v-bind="groupDisplayProps(groupsById.get(id)!)" :user-rate-multiplier="userRates[id]" :show-checkmark="false" />
            <span v-else class="text-sm">#{{ id }}</span>
            <p v-if="groupsById.get(id)" class="mt-1 text-xs text-gray-500">{{ chargingLabel(groupsById.get(id)!) }}</p>
            <p v-if="unavailableReason(id)" class="mt-1 text-xs text-amber-700 dark:text-amber-400">{{ unavailableReason(id) }}</p>
          </div>
        </div>
        <div class="mt-2 flex justify-end gap-2">
          <button type="button" class="btn btn-secondary px-2 py-1 text-xs" :disabled="disabled || index === 0" :aria-label="t('keys.multiGroup.moveUp', { name: groupName(id) })" @click="move(index, -1)">↑</button>
          <button type="button" class="btn btn-secondary px-2 py-1 text-xs" :disabled="disabled || index === modelValue.length - 1" :aria-label="t('keys.multiGroup.moveDown', { name: groupName(id) })" @click="move(index, 1)">↓</button>
          <button type="button" class="btn btn-secondary px-2 py-1 text-xs" :disabled="disabled" :aria-label="t('keys.multiGroup.remove', { name: groupName(id) })" @click="remove(id)">{{ t('common.remove') }}</button>
        </div>
      </li>
    </ol>
    <p v-if="modelValue.length === 0" :class="allowEmpty ? 'text-sm text-gray-500' : 'text-sm text-red-500'" role="status">{{ t(allowEmpty ? 'keys.multiGroup.adminEmpty' : 'keys.groupRequired') }}</p>
    <input v-model="search" type="search" class="input" :placeholder="t('keys.searchGroup')" :aria-label="t('keys.searchGroup')" :disabled="disabled" />
    <div class="max-h-56 space-y-1 overflow-y-auto">
      <button v-for="group in choices" :key="group.id" type="button" :data-add-group="group.id" :disabled="disabled" class="w-full rounded-lg border border-gray-200 p-3 text-left hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700" :aria-label="t('keys.multiGroup.add', { name: group.name })" @click="add(group.id)">
        <GroupOptionItem v-bind="groupDisplayProps(group)" :user-rate-multiplier="userRates[group.id]" :description="group.description" :show-checkmark="false" />
        <p class="mt-1 text-xs text-gray-500">{{ chargingLabel(group) }}</p>
      </button>
      <p v-if="choices.length === 0" class="py-2 text-sm text-gray-500">{{ t('keys.noGroupFound') }}</p>
    </div>
    <p class="rounded-lg bg-amber-50 p-3 text-xs text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">{{ t('keys.multiGroup.priceWarning') }}</p>
    <p class="text-xs text-gray-500">{{ t(multiGroupEnabled ? 'keys.multiGroup.strictScope' : 'keys.multiGroup.enableScope') }}</p>
    <p class="text-xs text-gray-500">{{ t('keys.multiGroup.sharedQuota') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Group } from '@/types'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import { groupDisplayProps } from './keyGroups'

const props = withDefaults(defineProps<{
  modelValue: number[]
  availableGroups: Group[]
  existingGroups?: Group[]
  additionGroupIds?: number[]
  userRates?: Record<number, number>
  multiGroupEnabled?: boolean
  allowEmpty?: boolean
  disabled?: boolean
}>(), {
  existingGroups: () => [],
  userRates: () => ({}),
  multiGroupEnabled: false,
  allowEmpty: false,
  disabled: false
})
const emit = defineEmits<{ 'update:modelValue': [number[]] }>()
const { t } = useI18n()
const search = ref('')
const groupsById = computed(() => new Map([...props.existingGroups, ...props.availableGroups].map(group => [group.id, group])))
const eligibleIds = computed(() => new Set(props.availableGroups.filter(group => group.status === 'active').map(group => group.id)))
const choices = computed(() => {
  const query = search.value.trim().toLowerCase()
  return props.availableGroups.filter(group => (!props.additionGroupIds || props.additionGroupIds.includes(group.id)) && eligibleIds.value.has(group.id) && !props.modelValue.includes(group.id) &&
    (!query || `${group.name} ${group.description ?? ''}`.toLowerCase().includes(query)))
})
const groupName = (id: number) => groupsById.value.get(id)?.name ?? `#${id}`
const chargingLabel = (group: Group) => t(group.subscription_type === 'subscription' ? 'keys.multiGroup.subscriptionCharge' : 'keys.multiGroup.balanceCharge')
const unavailableReason = (id: number) => {
  if (groupsById.value.get(id)?.status === 'inactive') return t('keys.multiGroup.inactive')
  return eligibleIds.value.has(id) ? '' : t('keys.multiGroup.ineligible')
}
const add = (id: number) => {
  if (!props.disabled && eligibleIds.value.has(id) && !props.modelValue.includes(id)) emit('update:modelValue', [...props.modelValue, id])
}
const remove = (id: number) => {
  if (!props.disabled) emit('update:modelValue', props.modelValue.filter(value => value !== id))
}
const move = (index: number, direction: number) => {
  const target = index + direction
  if (props.disabled || target < 0 || target >= props.modelValue.length) return
  const next = [...props.modelValue]
  ;[next[index], next[target]] = [next[target], next[index]]
  emit('update:modelValue', next)
}
</script>
