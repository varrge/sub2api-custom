<template>
  <BaseDialog :show="show" :title="t('admin.users.userApiKeys')" width="wide" @close="handleClose">
    <div v-if="user" class="space-y-4">
      <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <p class="font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
        <p class="text-sm text-gray-500">{{ user.username }}</p>
      </div>
      <p v-if="loading" class="py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</p>
      <p v-else-if="apiKeys.length === 0" class="py-8 text-center text-sm text-gray-500">{{ t('admin.users.noApiKeys') }}</p>
      <div v-else class="max-h-96 space-y-3 overflow-y-auto">
        <div v-for="key in apiKeys" :key="key.id" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <div class="mb-1 flex items-center gap-2">
            <span class="font-medium text-gray-900 dark:text-white">{{ key.name }}</span>
            <span :class="['badge text-xs', key.status === 'active' ? 'badge-success' : 'badge-danger']">{{ key.status }}</span>
          </div>
          <p class="truncate font-mono text-sm text-gray-500">{{ key.key.substring(0, 20) }}...{{ key.key.substring(key.key.length - 8) }}</p>
          <button class="mt-3 flex w-full min-w-0 max-w-full items-start gap-2 whitespace-normal rounded-lg p-1 text-left hover:bg-gray-100 dark:hover:bg-dark-700" :aria-label="t('keys.multiGroup.editGroups', { name: key.name })" :disabled="groupsLoading || groupsFailed" @click="openGroupSelector(key)">
            <KeyGroupBadges class="flex-1" :api-key="key" :user-rates="user.group_rates ?? {}" />
            <Icon name="edit" size="sm" class="mt-1 shrink-0 text-gray-400" />
          </button>
          <p class="mt-2 text-xs text-gray-500">{{ t('admin.users.columns.created') }}: {{ formatDateTime(key.created_at) }}</p>
        </div>
      </div>
      <div v-if="groupsFailed" class="text-sm text-red-500" role="alert">
        {{ t('keys.multiGroup.loadFailed') }}
        <button type="button" class="btn btn-secondary ml-2" @click="loadGroups">{{ t('common.refresh') }}</button>
      </div>
    </div>
  </BaseDialog>
  <BaseDialog :show="selectedKey !== null" :title="t('keys.multiGroup.title')" width="normal" @close="selectedKey = null">
    <OrderedKeyGroupSelector
      v-if="selectedKey"
      v-model="selectedGroupIds"
      :available-groups="availableGroups"
      :existing-groups="keyGroups(selectedKey)"
      :user-rates="user?.group_rates ?? {}"
      :multi-group-enabled="selectedKey.multi_group_enabled"
      :disabled="saving"
      allow-empty
    />
    <KeyModelAllowlist
      v-if="selectedKey"
      v-model="selectedModelAllowlist"
      class="mt-4"
      :group-ids="selectedGroupIds"
      :load-options="loadModelOptions"
      :disabled="saving"
    />
    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" :disabled="saving" @click="selectedKey = null">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="saving" @click="saveGroups">{{ t(saving ? 'common.saving' : 'common.save') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import type { AdminUser, Group, ApiKey, ApiKeyModelAllowlist } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import KeyGroupBadges from '@/components/keys/KeyGroupBadges.vue'
import OrderedKeyGroupSelector from '@/components/keys/OrderedKeyGroupSelector.vue'
import KeyModelAllowlist from '@/components/keys/KeyModelAllowlist.vue'
import { keyGroupIds, keyGroups } from '@/components/keys/keyGroups'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close'])
const { t } = useI18n()
const appStore = useAppStore()
const apiKeys = ref<ApiKey[]>([])
const availableGroups = ref<Group[]>([])
const loading = ref(false)
const groupsLoading = ref(false)
const groupsFailed = ref(false)
const selectedKey = ref<ApiKey | null>(null)
const selectedGroupIds = ref<number[]>([])
const selectedModelAllowlist = ref<ApiKeyModelAllowlist>({ enabled: false, models: [] })
const saving = ref(false)
let loadGeneration = 0
let groupsGeneration = 0

const loadGroups = async () => {
  const userId = props.user?.id
  if (!props.show || !userId) return
  const generation = loadGeneration
  const request = ++groupsGeneration
  const isCurrentRequest = () => generation === loadGeneration && request === groupsGeneration
  groupsLoading.value = true
  groupsFailed.value = false
  availableGroups.value = []
  try {
    const groups = await adminAPI.users.getAvailableGroups(userId)
    if (isCurrentRequest()) availableGroups.value = groups
  } catch {
    if (isCurrentRequest()) groupsFailed.value = true
  } finally {
    if (isCurrentRequest()) groupsLoading.value = false
  }
}

watch(() => [props.show, props.user?.id] as const, async ([show, userId], _, onCleanup) => {
  onCleanup(() => { loadGeneration++ })
  const generation = ++loadGeneration
  selectedKey.value = null
  apiKeys.value = []
  availableGroups.value = []
  groupsLoading.value = false
  groupsFailed.value = false
  loading.value = false
  if (!show || !userId) return
  loading.value = true
  const groupsPromise = loadGroups()
  try {
    const result = await adminAPI.users.getUserApiKeys(userId)
    if (generation === loadGeneration) apiKeys.value = result.items || []
  } catch {
    if (generation === loadGeneration) appStore.showError(t('keys.failedToLoad'))
  } finally {
    if (generation === loadGeneration) loading.value = false
    await groupsPromise
  }
}, { immediate: true })

const openGroupSelector = (key: ApiKey) => {
  selectedKey.value = key
  selectedGroupIds.value = keyGroupIds(key)
  selectedModelAllowlist.value = {
    ...key.model_allowlist,
    enabled: key.model_allowlist?.enabled ?? false,
    models: [...(key.model_allowlist?.models ?? [])]
  }
}

const loadModelOptions = (groupIds: number[]) => adminAPI.users.getApiKeyModelOptions(props.user!.id, groupIds)

const saveGroups = async () => {
  const key = selectedKey.value
  if (!key || saving.value) return
  saving.value = true
  try {
    const result = await adminAPI.apiKeys.updateApiKeyGroups(key.id, selectedGroupIds.value, {
      ...selectedModelAllowlist.value,
      enabled: selectedModelAllowlist.value.enabled,
      models: [...(selectedModelAllowlist.value.models ?? [])]
    }, key.model_allowlist_revision)
    const index = apiKeys.value.findIndex(item => item.id === key.id)
    if (index !== -1) apiKeys.value[index] = result.api_key
    selectedKey.value = null
    appStore.showSuccess(t('admin.users.groupChangedSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.users.groupChangeFailed'))
  } finally {
    saving.value = false
  }
}

const handleClose = () => {
  selectedKey.value = null
  emit('close')
}
</script>
