<template>
  <span class="flex flex-wrap items-center gap-1.5">
    <span v-for="(id, index) in ids" :key="id" class="inline-flex items-center gap-1">
      <span class="text-xs tabular-nums text-gray-500">{{ index + 1 }}.</span>
      <GroupBadge v-if="groupsById.get(id)" v-bind="groupDisplayProps(groupsById.get(id)!)" :user-rate-multiplier="userRates[id]" />
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
