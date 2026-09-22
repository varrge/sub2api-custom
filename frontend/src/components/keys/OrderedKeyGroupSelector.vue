<template>
  <div class="key-group-selector min-w-0 space-y-2">
    <p class="text-xs text-gray-600 dark:text-gray-400">{{ t('keys.multiGroup.orderHint') }}</p>

    <!-- Selected groups: compact list -->
    <ol v-if="modelValue.length > 0" class="key-group-list" :aria-label="t('keys.multiGroup.selectedGroups')">
      <li
        v-for="(id, index) in modelValue"
        :key="id"
        :data-group-id="id"
        class="key-group-row"
      >
        <span class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-primary-100 text-[11px] font-semibold tabular-nums text-primary-700 dark:bg-dark-700 dark:text-primary-300">{{ index + 1 }}</span>
        <div class="min-w-0 flex-1">
          <GroupOptionItem
            v-if="groupsById.get(id)"
            class="key-group-option"
            v-bind="groupDisplayProps(groupsById.get(id)!)"
            :user-rate-multiplier="userRates[id]"
            :show-checkmark="false"
          />
          <span v-else class="text-sm">#{{ id }}</span>
          <span v-if="groupsById.get(id)" class="block text-[10px] leading-4 text-gray-500 dark:text-gray-400" data-test="group-billing-type">{{ t(groupsById.get(id)!.subscription_type === 'subscription' ? 'groups.subscription' : 'dashboard.balance') }}</span>
          <p v-if="unavailableReason(id)" class="mt-0.5 text-xs text-amber-700 dark:text-amber-400">{{ unavailableReason(id) }}</p>
        </div>
        <div class="flex shrink-0 items-center gap-1">
          <button
            type="button"
            class="btn-icon"
            :disabled="disabled || index === 0"
            :aria-label="t('keys.multiGroup.moveUp', { name: groupName(id) })"
            @click="move(index, -1)"
          >
            <Icon name="arrowUp" size="xs" />
          </button>
          <button
            type="button"
            class="btn-icon"
            :disabled="disabled || index === modelValue.length - 1"
            :aria-label="t('keys.multiGroup.moveDown', { name: groupName(id) })"
            @click="move(index, 1)"
          >
            <Icon name="arrowDown" size="xs" />
          </button>
          <button
            type="button"
            class="btn-icon text-red-600 hover:text-red-700 dark:text-red-400"
            :disabled="disabled"
            :aria-label="t('keys.multiGroup.remove', { name: groupName(id) })"
            @click="remove(id)"
          >
            <Icon name="x" size="xs" />
          </button>
        </div>
      </li>
    </ol>
    <p
      v-else
      :class="allowEmpty ? 'text-sm text-gray-500' : 'text-sm text-red-500'"
      role="status"
    >
      {{ t(allowEmpty ? 'keys.multiGroup.adminEmpty' : 'keys.groupRequired') }}
    </p>

    <!-- Candidates: collapsible -->
    <div class="pt-2">
      <button
        v-if="!showCandidates && eligibleChoices.length > 0"
        type="button"
        class="flex w-full items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:border-primary-300 hover:bg-primary-50/40 dark:border-dark-600 dark:text-gray-300 dark:hover:border-primary-700 dark:hover:bg-dark-700"
        :disabled="disabled" :aria-expanded="false" :aria-controls="candidatePanelId" data-test="expand-groups" @click="showCandidates = true"
      >
        <span>{{ t('keys.multiGroup.addGroup', { count: eligibleChoices.length }) }}</span>
        <Icon name="plus" size="sm" />
      </button>

      <div v-else-if="eligibleChoices.length > 0 || showCandidates" :id="candidatePanelId" class="space-y-2">
        <div class="flex items-center gap-2">
          <input
            v-model="search"
            type="search"
            class="input flex-1"
            :placeholder="t('keys.searchGroup')"
            :aria-label="t('keys.searchGroup')"
            :disabled="disabled"
          />
          <button
            v-if="showCandidates"
            type="button"
            class="btn btn-secondary px-3 py-1 text-xs"
            :aria-expanded="true" :aria-controls="candidatePanelId" data-test="collapse-groups" @click="showCandidates = false"
          >
            {{ t('common.collapse') }}
          </button>
        </div>

        <div v-if="choices.length > 0" class="key-candidates-list max-h-64 overflow-y-auto p-0.5">
          <button
            v-for="group in choices"
            :key="group.id"
            type="button"
            :data-add-group="group.id"
            :disabled="disabled"
            class="key-candidate-row"
            :aria-label="t('keys.multiGroup.add', { name: group.name })"
            @click="add(group.id)"
          >
            <div class="min-w-0 flex-1">
            <GroupOptionItem
              class="key-group-option"
              v-bind="groupDisplayProps(group)"
              :user-rate-multiplier="userRates[group.id]"
              :description="group.description?.trim() === group.name.trim() ? undefined : group.description"
              :show-checkmark="false"
            />
            <span class="block text-[10px] leading-4 text-gray-500 dark:text-gray-400" data-test="group-billing-type">{{ t(group.subscription_type === 'subscription' ? 'groups.subscription' : 'dashboard.balance') }}</span>
            </div>
            <Icon name="plus" size="sm" class="shrink-0 text-gray-400" />
          </button>
        </div>
        <p v-else-if="search" class="py-2 text-sm text-gray-500">{{ t('keys.noGroupFound') }}</p>
      </div>
    </div>

    <!-- Shared charging info -->
    <div class="space-y-1 pt-2">
      <p class="text-xs text-gray-500">
        <span class="font-medium">{{ t('groups.subscription') }}: </span>{{ t('keys.multiGroup.subscriptionCharge') }}
      </p>
      <p class="text-xs text-gray-500">
        <span class="font-medium">{{ t('dashboard.balance') }}: </span>{{ t('keys.multiGroup.balanceCharge') }}
      </p>
      <p class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">{{ t('keys.multiGroup.priceWarning') }}</p>
      <p class="text-xs text-gray-500">{{ t(multiGroupEnabled ? 'keys.multiGroup.strictScope' : 'keys.multiGroup.enableScope') }}</p>
      <p class="text-xs text-gray-500">{{ t('keys.multiGroup.sharedQuota') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Group } from '@/types'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import Icon from '@/components/icons/Icon.vue'
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
const candidatePanelId = useId()
const showCandidates = ref(props.modelValue.length === 0)
watch(showCandidates, open => { if (!open) search.value = '' })
watch(() => props.modelValue.length, count => { if (count === 0) showCandidates.value = true })
const groupsById = computed(() => new Map([...props.existingGroups, ...props.availableGroups].map(group => [group.id, group])))
const eligibleIds = computed(() => new Set(props.availableGroups.filter(group => group.status === 'active').map(group => group.id)))
const eligibleChoices = computed(() => props.availableGroups.filter(group =>
  (!props.additionGroupIds || props.additionGroupIds.includes(group.id)) &&
  eligibleIds.value.has(group.id) && !props.modelValue.includes(group.id)))
const choices = computed(() => {
  const query = search.value.trim().toLowerCase()
  return eligibleChoices.value.filter(group => !query || `${group.name} ${group.description ?? ''}`.toLowerCase().includes(query))
})
const groupName = (id: number) => groupsById.value.get(id)?.name ?? `#${id}`
const unavailableReason = (id: number) => {
  if (groupsById.value.get(id)?.status === 'inactive') return t('keys.multiGroup.inactive')
  return eligibleIds.value.has(id) ? '' : t('keys.multiGroup.ineligible')
}
const add = (id: number) => {
  if (!props.disabled && eligibleChoices.value.some(group => group.id === id)) emit('update:modelValue', [...props.modelValue, id])
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

<style scoped>
.key-group-selector {
  container-type: inline-size;
}

/* Selected groups: compact rows — order badge / name+rate / actions on one line.
   Short names stay within ~64px; long names wrap (never truncated);
   on narrow screens the action buttons drop to their own row. */
.key-group-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.key-group-row {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  min-height: 2.5rem;
  padding: 0.375rem 0.5rem;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  background-color: #fff;
}

.dark .key-group-row {
  border-color: #4a4a51; /* dark-600 */
  background-color: #27272d; /* dark-800 */
}

/* Long group names wrap instead of being truncated by GroupBadge/GroupOptionItem */
.key-group-option :deep(.truncate) {
  overflow: visible;
  text-overflow: clip;
  white-space: normal;
  overflow-wrap: anywhere;
}

.key-group-option :deep(.groupOptionItemBadge) {
  max-width: 100%;
}

.key-group-option :deep(.groupOptionItemBadge > svg) {
  flex-shrink: 0;
}

/* Rate windows and temporary prices must fit narrow/shared modal variants. */
.key-group-option :deep(> div:nth-child(2)),
.key-group-option :deep(> div:nth-child(2) > div) {
  min-width: 0;
  max-width: 100%;
}

.key-group-option :deep(.whitespace-nowrap) {
  max-width: 100%;
  white-space: normal;
  overflow-wrap: anywhere;
}

/* Compact icon buttons inside selected rows */
.key-group-row .btn-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.75rem;
  height: 1.75rem;
  padding: 0.375rem;
  border-radius: 0.5rem;
}

.key-group-row .btn-icon:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

@media (max-width: 480px) {
  .key-group-row {
    flex-wrap: wrap;
  }

  .key-group-row > :last-child {
    flex-basis: 100%;
    justify-content: flex-end;
    padding-top: 0.125rem;
  }
}

/* Candidate groups: at most two columns (one on narrow screens),
   capped height with internal scrolling. */
.key-candidates-list {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.375rem;
  max-height: 256px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

@container (min-width: 36rem) {
  .key-candidates-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.key-candidate-row {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  min-width: 0;
  padding: 0.5rem;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  background-color: #fff;
  text-align: left;
  transition: border-color 0.15s, background-color 0.15s;
}

.key-candidate-row:hover:not(:disabled) {
  @apply border-primary-300 bg-primary-50/40;
}

.key-candidate-row:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.dark .key-candidate-row {
  border-color: #4a4a51; /* dark-600 */
  background-color: #27272d; /* dark-800 */
}

.dark .key-candidate-row:hover:not(:disabled) {
  @apply border-primary-700 bg-dark-700;
}

/* In the candidate grid, long names must not stretch the column */
.key-candidate-row .key-group-option :deep(.truncate) {
  max-width: 100%;
}
</style>
