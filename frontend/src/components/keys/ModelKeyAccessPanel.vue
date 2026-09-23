<template>
  <div class="space-y-6" data-test="model-key-access">
    <!-- Header -->
    <div class="space-y-2">
      <RouterLink
        to="/keys"
        class="inline-flex items-center gap-1.5 text-sm text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400"
      >
        <Icon name="arrowLeft" size="sm" />
        {{ t('modelKeyAccess.backToKeys') }}
      </RouterLink>
      <h1 class="text-xl font-bold text-gray-900 dark:text-white">{{ t('modelKeyAccess.title') }}</h1>
      <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('modelKeyAccess.subtitle') }}</p>
    </div>

    <!-- Catalog warning: catalog incomplete; only keys with confirmed matching groups are shown -->
    <div
      v-if="catalogWarning"
      role="status"
      class="flex items-start gap-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-300"
    >
      <Icon name="exclamationTriangle" size="md" class="mt-0.5 shrink-0" />
      <span>{{ t('modelKeyAccess.catalogWarning') }}</span>
    </div>

    <!-- Error: retry never clears the draft -->
    <div
      v-if="error"
      role="alert"
      class="flex flex-wrap items-center gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800/50 dark:bg-red-900/20 dark:text-red-300"
    >
      <Icon name="exclamationCircle" size="md" class="shrink-0" />
      <span class="min-w-0 flex-1 break-words">{{ error }}</span>
      <button type="button" class="btn btn-secondary btn-sm shrink-0" :disabled="busy" @click="emit('reload')">
        {{ t('modelKeyAccess.retry') }}
      </button>
    </div>

    <div class="grid min-w-0 gap-6 lg:grid-cols-[minmax(0,20rem)_minmax(0,1fr)]">
      <!-- Left: model picker -->
      <section class="card min-w-0 space-y-3 p-4 self-start" :aria-label="t('modelKeyAccess.modelSectionTitle')">
        <h2 class="font-medium text-gray-900 dark:text-white">{{ t('modelKeyAccess.modelSectionTitle') }}</h2>
        <SearchInput
          :model-value="modelSearch"
          :placeholder="t('modelKeyAccess.modelSearchPlaceholder')"
          @update:model-value="emit('update:modelSearch', $event)"
        />
        <button
          v-if="manualModelId"
          type="button"
          class="btn btn-secondary w-full flex-wrap justify-start gap-1.5 text-left"
          :disabled="busy"
          data-test="use-manual-model"
          @click="emit('select-model', manualModelId)"
        >
          <Icon name="check" size="sm" class="shrink-0" />
          <span class="shrink-0">{{ t('modelKeyAccess.useThisModel') }}</span>
          <span class="min-w-0 break-all font-mono text-xs">{{ manualModelId }}</span>
        </button>
        <p v-if="loading" class="text-sm text-gray-500 dark:text-dark-400" role="status">
          {{ t('modelKeyAccess.loading') }}
        </p>
        <ul v-if="models.length" class="max-h-96 space-y-1 overflow-y-auto" data-test="model-list">
          <li v-for="model in models" :key="model">
            <button
              type="button"
              class="w-full min-w-0 break-all rounded-lg px-3 py-2 text-left font-mono text-sm transition-colors"
              :class="model === selectedModel
                ? 'bg-primary-50 text-primary-700 ring-1 ring-primary-500 dark:bg-primary-900/20 dark:text-primary-300'
                : 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
              :aria-pressed="model === selectedModel"
              :disabled="busy"
              @click="emit('select-model', model)"
            >
              {{ model }}
            </button>
          </li>
        </ul>
        <p v-else-if="!loading" class="text-sm text-gray-500 dark:text-dark-400">
          {{ t(modelSearch.trim() ? 'modelKeyAccess.noModelMatches' : 'modelKeyAccess.noModels') }}
        </p>
      </section>

      <!-- Right: keys allowed to call the selected model -->
      <section class="min-w-0">
        <EmptyState
          v-if="!selectedModel"
          :title="t('modelKeyAccess.noModelSelectedTitle')"
          :description="t('modelKeyAccess.noModelSelectedHint')"
        />
        <template v-else>
          <div class="card min-w-0 space-y-4 p-4 sm:p-6">
            <header class="space-y-1">
              <h2 class="break-all font-mono text-base font-semibold text-gray-900 dark:text-white" data-test="selected-model">
                {{ selectedModel }}
              </h2>
              <p class="text-sm text-gray-500 dark:text-dark-400">
                {{ t('modelKeyAccess.stats', { total: totalCount, allowed: allowedCount }) }}
                <span class="text-gray-400 dark:text-dark-500">· {{ t('modelKeyAccess.filterNote') }}</span>
              </p>
            </header>

            <!-- Filters: display only, never narrow the save scope -->
            <div class="flex flex-wrap items-center gap-3">
              <SearchInput
                :model-value="keySearch"
                :placeholder="t('modelKeyAccess.keySearchPlaceholder')"
                class="w-full sm:w-64"
                @update:model-value="emit('update:keySearch', $event)"
              />
              <Select
                :model-value="groupFilter"
                :options="groupFilterOptions"
                class="w-44"
                @update:model-value="onGroupFilterChange"
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="busy || !rows.length"
                data-test="allow-visible"
                @click="emit('set-visible', true)"
              >
                {{ t('modelKeyAccess.allowVisible') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="busy || !rows.length"
                data-test="block-visible"
                @click="emit('set-visible', false)"
              >
                {{ t('modelKeyAccess.blockVisible') }}
              </button>
            </div>
            <p class="text-xs text-gray-400 dark:text-dark-500">{{ t('modelKeyAccess.visibleScopeHint') }}</p>

            <!-- Key rows -->
            <ul v-if="rows.length" class="divide-y divide-gray-100 dark:divide-dark-700" data-test="key-rows">
              <li
                v-for="row in rows"
                :key="row.id"
                class="flex flex-wrap items-center gap-x-3 gap-y-2 py-3"
                data-test="key-row"
              >
                <button
                  type="button"
                  role="switch"
                  :aria-checked="row.allowed"
                  :aria-label="t('modelKeyAccess.columnAllow') + ': ' + row.name"
                  :disabled="busy"
                  class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 disabled:cursor-not-allowed disabled:opacity-50"
                  :class="row.allowed ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'"
                  data-test="allow-toggle"
                  @click="emit('toggle-key', row.id, !row.allowed)"
                >
                  <span
                    class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                    :class="row.allowed ? 'translate-x-4' : 'translate-x-0'"
                  />
                </button>

                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="min-w-0 break-all font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
                    <span
                      v-if="row.allowed !== row.originalAllowed"
                      class="badge badge-warning shrink-0"
                      data-test="modified-badge"
                    >
                      {{ t('modelKeyAccess.modifiedBadge') }}
                    </span>
                  </div>
                  <div class="mt-1 flex flex-wrap gap-1">
                    <span
                      v-for="group in row.groups"
                      :key="group.id"
                      class="badge badge-gray max-w-full whitespace-normal"
                    >
                      <span class="min-w-0 break-all">{{ group.name }}</span>
                    </span>
                    <span v-if="!row.groups.length" class="text-xs text-gray-400 dark:text-dark-500">
                      {{ t('modelKeyAccess.noGroups') }}
                    </span>
                  </div>
                </div>

                <!-- Mobile: spans a second full-width line so the name/group column keeps
                     usable width; sm+ returns to a right-aligned column on the first line. -->
                <div class="flex w-full flex-wrap items-center gap-x-3 gap-y-1 sm:w-auto sm:shrink-0 sm:flex-col sm:items-end sm:gap-1 sm:text-right" data-test="key-row-status">
                  <span :class="['badge', statusBadgeClass(row.status)]">{{ statusLabel(row.status) }}</span>
                  <span
                    v-if="row.expires_at"
                    class="text-xs"
                    :class="isExpired(row.expires_at) ? 'text-red-500 dark:text-red-400' : 'text-gray-400 dark:text-dark-500'"
                  >
                    {{ t('modelKeyAccess.expiresAt', { time: formatDateTime(row.expires_at) }) }}
                  </span>
                </div>
              </li>
            </ul>
            <EmptyState
              v-else-if="!totalCount"
              :title="t('modelKeyAccess.noKeysTitle')"
              :description="t('modelKeyAccess.noKeysHint')"
            />
            <EmptyState
              v-else
              :title="t('modelKeyAccess.noMatchesTitle')"
              :description="t('modelKeyAccess.noMatchesHint')"
            />
          </div>

          <!-- Save bar: applies to every loaded key, not only filtered rows -->
          <div class="card sticky bottom-4 mt-4 flex flex-wrap items-center gap-3 p-4" data-test="save-bar">
            <div class="min-w-0 text-sm">
              <p v-if="dirty" class="flex flex-wrap gap-x-3">
                <span v-if="addedCount" class="font-medium text-emerald-600 dark:text-emerald-400">
                  {{ t('modelKeyAccess.added', { count: addedCount }) }}
                </span>
                <span v-if="removedCount" class="font-medium text-red-500 dark:text-red-400">
                  {{ t('modelKeyAccess.removed', { count: removedCount }) }}
                </span>
              </p>
              <p v-else class="text-gray-500 dark:text-dark-400">{{ t('modelKeyAccess.noChanges') }}</p>
              <p class="mt-0.5 text-xs text-gray-400 dark:text-dark-500">{{ t('modelKeyAccess.saveScopeNote') }}</p>
            </div>
            <div class="ml-auto flex shrink-0 gap-3">
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="busy || !dirty"
                data-test="discard"
                @click="emit('discard')"
              >
                {{ t('modelKeyAccess.discard') }}
              </button>
              <button
                type="button"
                class="btn btn-primary"
                :disabled="busy || !dirty"
                data-test="save"
                @click="emit('save')"
              >
                {{ saving ? t('modelKeyAccess.saving') : t('modelKeyAccess.save') }}
              </button>
            </div>
          </div>
        </template>
      </section>
    </div>

    <!-- Unsaved-changes guard: business layer resolves the choice -->
    <BaseDialog
      :show="switchPending"
      :title="t('modelKeyAccess.switchTitle')"
      width="narrow"
      :close-on-escape="false"
      :show-close-button="!saving"
      @close="emit('resolve-switch', 'cancel')"
    >
      <div class="space-y-3">
        <p class="break-all text-sm text-gray-600 dark:text-gray-400">
          {{ t('modelKeyAccess.switchMessage', { model: selectedModel }) }}
        </p>
        <!-- Save-and-continue failures surface here: the dialog stays open with the draft intact -->
        <div
          v-if="error"
          role="alert"
          class="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800/50 dark:bg-red-900/20 dark:text-red-300"
          data-test="dialog-error"
        >
          <Icon name="exclamationCircle" size="md" class="mt-0.5 shrink-0" />
          <span class="min-w-0 break-words">{{ error }}</span>
        </div>
      </div>
      <template #footer>
        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end sm:gap-3">
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="saving"
            data-test="switch-cancel"
            @click="emit('resolve-switch', 'cancel')"
          >
            {{ t('modelKeyAccess.stay') }}
          </button>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="saving"
            data-test="switch-discard"
            @click="emit('resolve-switch', 'discard')"
          >
            {{ t('modelKeyAccess.discardAndContinue') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="saving"
            data-test="switch-save"
            @click="emit('resolve-switch', 'save')"
          >
            {{ saving ? t('modelKeyAccess.saving') : t('modelKeyAccess.saveAndContinue') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelAccessGroup, ModelAccessRow } from '@/api/modelKeyAccess'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{
  loading: boolean
  saving: boolean
  error: string
  catalogWarning: boolean
  selectedModel: string
  modelSearch: string
  keySearch: string
  groupFilter: number | null
  models: string[]
  rows: ModelAccessRow[]
  groups: ModelAccessGroup[]
  totalCount: number
  allowedCount: number
  addedCount: number
  removedCount: number
  dirty: boolean
  switchPending: boolean
}>()

const emit = defineEmits<{
  'update:modelSearch': [value: string]
  'update:keySearch': [value: string]
  'update:groupFilter': [value: number | null]
  'select-model': [model: string]
  'toggle-key': [id: number, allowed: boolean]
  'set-visible': [allowed: boolean]
  save: []
  discard: []
  reload: []
  'resolve-switch': [action: 'save' | 'discard' | 'cancel']
}>()

const { t } = useI18n()

const busy = computed(() => props.loading || props.saving)

// A non-empty search that is not already selected can be used as a full model ID.
const manualModelId = computed(() => {
  const value = props.modelSearch.trim()
  return value && value !== props.selectedModel ? value : ''
})

const groupFilterOptions = computed(() => [
  { value: null, label: t('modelKeyAccess.allGroups') },
  ...props.groups.map(group => ({ value: group.id, label: group.name }))
])

const onGroupFilterChange = (value: string | number | boolean | null) => {
  emit('update:groupFilter', typeof value === 'number' ? value : null)
}

const statusLabel = (status: string) => {
  const key = `keys.status.${status}`
  const label = t(key)
  return label === key ? status : label
}

const statusBadgeClass = (status: string) =>
  status === 'active' ? 'badge-success'
    : status === 'quota_exhausted' ? 'badge-warning'
      : status === 'expired' ? 'badge-danger'
        : 'badge-gray'

const isExpired = (expiresAt: string) => new Date(expiresAt).getTime() < Date.now()
</script>
