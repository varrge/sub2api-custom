<template>
  <section class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-600" data-test="model-allowlist">
    <h3 class="font-medium text-gray-900 dark:text-white">{{ t('keys.modelRestriction.title') }}</h3>
    <label class="flex items-center gap-2 text-sm">
      <input
        type="checkbox"
        class="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
        data-test="model-restriction-enabled"
        :checked="modelValue.enabled"
        :disabled="disabled"
        @change="setEnabled(($event.target as HTMLInputElement).checked)"
      />
      {{ t('keys.modelRestriction.enable') }}
    </label>
    <p class="input-hint">{{ t(modelValue.enabled ? 'keys.modelRestriction.allowedHint' : 'keys.modelRestriction.allHint') }}</p>

    <p v-if="loading" class="text-sm text-gray-500" role="status">{{ t('keys.modelRestriction.loading') }}</p>
    <div v-else-if="failed" class="text-sm text-red-600 dark:text-red-400" role="alert">
      {{ t('keys.modelRestriction.loadFailed') }}
      <button type="button" class="btn btn-secondary ml-2" :disabled="disabled" @click="refreshOptions">{{ t('keys.modelRestriction.retry') }}</button>
    </div>
    <template v-else>
      <input v-model="search" type="search" class="input" :aria-label="t('keys.modelRestriction.search')" :placeholder="t('keys.modelRestriction.search')" />
      <div class="flex flex-wrap items-center gap-3 text-sm">
        <button type="button" class="text-primary-600 disabled:opacity-50" :disabled="disabled || !modelValue.enabled || !options.length" @click="selectAll">{{ t('keys.modelRestriction.selectAll') }}</button>
        <button type="button" class="text-primary-600 disabled:opacity-50" :disabled="disabled || !modelValue.enabled || !selectedModels.length" @click="setModels([])">{{ t('keys.modelRestriction.clear') }}</button>
        <span v-if="modelValue.enabled" class="text-gray-500">{{ t('keys.modelRestriction.selectedCount', { count: selectedModels.length }) }}</span>
      </div>
      <div v-if="filteredOptions.length" class="max-h-60 space-y-1 overflow-y-auto rounded-lg border border-gray-200 p-2 dark:border-dark-600">
        <label v-for="model in filteredOptions" :key="model.id" class="flex items-start gap-2 rounded p-2 text-sm hover:bg-gray-50 dark:hover:bg-dark-700">
          <input
            type="checkbox"
            class="mt-0.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            :value="model.id"
            :checked="!modelValue.enabled || selectedModels.includes(model.id)"
            :disabled="disabled || !modelValue.enabled"
            @change="toggleModel(model.id, ($event.target as HTMLInputElement).checked)"
          />
          <span class="break-all font-mono">{{ model.id }}</span>
        </label>
      </div>
      <p v-else class="text-sm text-gray-500">{{ t(options.length ? 'keys.modelRestriction.noMatches' : 'keys.modelRestriction.noModels') }}</p>
    </template>

    <div v-if="retainedModels.length" class="space-y-2 rounded-lg bg-amber-50 p-3 text-sm dark:bg-amber-900/10" data-test="retained-models">
      <p>{{ t(loaded ? 'keys.modelRestriction.unavailableHint' : 'keys.modelRestriction.retainedHint') }}</p>
      <label v-for="model in retainedModels" :key="model" class="flex items-start gap-2">
        <input
          type="checkbox"
          class="mt-0.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          :value="model"
          checked
          :disabled="disabled || !modelValue.enabled"
          @change="toggleModel(model, ($event.target as HTMLInputElement).checked)"
        />
        <span class="break-all font-mono">{{ model }}</span>
      </label>
    </div>
    <p v-if="modelValue.enabled && !selectedModels.length" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ t('keys.modelRestriction.required') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApiKeyModelAllowlist, ApiKeyModelOptions } from '@/types'

const props = defineProps<{
  modelValue: ApiKeyModelAllowlist
  groupIds: number[]
  loadOptions: (groupIds: number[]) => Promise<ApiKeyModelOptions>
  disabled?: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [value: ApiKeyModelAllowlist] }>()
const { t } = useI18n()
const options = ref<ApiKeyModelOptions['models']>([])
const loading = ref(false)
const failed = ref(false)
const loaded = ref(false)
const search = ref('')
let generation = 0

const selectedModels = computed(() => [...new Set(props.modelValue.models ?? [])])
const filteredOptions = computed(() => options.value.filter(model => model.id.toLowerCase().includes(search.value.trim().toLowerCase())))
const retainedModels = computed(() => selectedModels.value.filter(model => !options.value.some(option => option.id === model)))

const setEnabled = (enabled: boolean) => emit('update:modelValue', { enabled, models: [...selectedModels.value] })
const setModels = (models: string[]) => emit('update:modelValue', { enabled: props.modelValue.enabled, models: [...new Set(models)] })
const toggleModel = (model: string, allowed: boolean) => setModels(allowed ? [...selectedModels.value, model] : selectedModels.value.filter(id => id !== model))
const selectAll = () => setModels([...selectedModels.value, ...options.value.map(model => model.id)])

const refreshOptions = async () => {
  const request = ++generation
  options.value = []
  failed.value = false
  loaded.value = false
  loading.value = true
  try {
    const result = props.groupIds.length ? await props.loadOptions([...props.groupIds]) : { models: [] }
    if (request !== generation) return
    const unique = new Map<string, ApiKeyModelOptions['models'][number]>()
    for (const model of result.models) {
      const previous = unique.get(model.id)
      unique.set(model.id, { id: model.id, group_ids: [...new Set([...(previous?.group_ids ?? []), ...model.group_ids])] })
    }
    options.value = [...unique.values()]
    loaded.value = true
  } catch {
    if (request === generation) failed.value = true
  } finally {
    if (request === generation) loading.value = false
  }
}

watch(() => [props.loadOptions, ...props.groupIds], refreshOptions, { immediate: true })
onBeforeUnmount(() => { generation++ })
</script>
