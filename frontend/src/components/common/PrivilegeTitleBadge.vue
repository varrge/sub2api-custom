<template>
  <span
    v-if="title"
    data-testid="user-privilege-title"
    class="inline-flex max-w-full items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-semibold"
    :title="title"
  >
    <svg class="h-3 w-3 flex-shrink-0" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M8 1.5 9.9 6.1 14.5 8 9.9 9.9 8 14.5 6.1 9.9 1.5 8 6.1 6.1 8 1.5Z" fill="currentColor" opacity="0.9" />
    </svg>
    <span class="truncate">{{ title }}</span>
  </span>
</template>

<script setup lang="ts">
import { useUserPrivilegeTitle } from '@/composables/useUserPrivilegeTitle'

const title = useUserPrivilegeTitle()
</script>

<style scoped>
span[data-testid='user-privilege-title'] {
  border-color: rgba(148, 104, 47, 0.35);
  background: rgba(255, 252, 247, 0.92);
  color: #6b4a1d;
  box-shadow: 0 2px 8px rgba(148, 104, 47, 0.12);
}

.dark span[data-testid='user-privilege-title'] {
  border-color: rgba(229, 185, 154, 0.3);
  background: rgba(30, 24, 18, 0.88);
  color: #e5b99a;
}
</style>
