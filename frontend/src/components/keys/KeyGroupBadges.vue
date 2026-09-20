<template>
  <span class="key-group-badges flex min-w-0 max-w-full flex-wrap items-start gap-1.5 whitespace-normal">
    <span v-for="(id, index) in ids" :key="id" class="inline-flex min-w-0 max-w-full items-start gap-1">
      <span class="shrink-0 pt-1 text-xs tabular-nums text-gray-500">{{ index + 1 }}.</span>
      <GroupBadge v-if="groupsById.get(id)" class="key-group-badge" v-bind="groupDisplayProps(groupsById.get(id)!)" :user-rate-multiplier="userRates[id]" />
      <span v-else class="text-xs text-gray-500">#{{ id }}</span>
    </span>
    <span v-if="ids.length === 0" class="text-sm text-gray-400">{{ t('keys.noGroup') }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApiKey } from '@/types'
import GroupBadge from '@/components/common/GroupBadge.vue'
import { groupDisplayProps, keyGroupIds, keyGroups } from './keyGroups'

const props = withDefaults(defineProps<{ apiKey: ApiKey; userRates?: Record<number, number> }>(), {
  userRates: () => ({})
})
const { t } = useI18n()
const ids = computed(() => keyGroupIds(props.apiKey))
const groupsById = computed(() => new Map(keyGroups(props.apiKey).map(group => [group.id, group])))
</script>

<style scoped>
.key-group-badge {
  min-width: 0;
  max-width: 100%;
  flex-wrap: wrap;
  white-space: normal;
  overflow-wrap: anywhere;
}

.key-group-badge :deep(.truncate) {
  min-width: 0;
  white-space: normal;
  overflow-wrap: anywhere;
}
</style>
