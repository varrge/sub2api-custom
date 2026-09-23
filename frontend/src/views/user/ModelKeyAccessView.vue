<template>
  <AppLayout>
    <ModelKeyAccessPanel
      v-bind="state"
      @update:model-search="modelSearch = $event"
      @update:key-search="keySearch = $event"
      @update:group-filter="groupFilter = $event"
      @select-model="selectModel"
      @toggle-key="toggleKey"
      @set-visible="setVisible"
      @save="save"
      @discard="discard"
      @reload="reload"
      @resolve-switch="resolveSwitch"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { reactive } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelKeyAccessPanel from '@/components/keys/ModelKeyAccessPanel.vue'
import { useModelKeyAccess } from '@/composables/useModelKeyAccess'

const access = useModelKeyAccess()
const state = reactive(access)
const { modelSearch, keySearch, groupFilter, selectModel, toggleKey, setVisible, save, discard, reload, resolveSwitch } = access
</script>
