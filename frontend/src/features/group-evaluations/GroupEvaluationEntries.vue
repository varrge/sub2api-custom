<template>
  <section v-if="groups.length || error" class="card my-5 !rounded-2xl p-4 sm:p-5">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('groupEvaluations.title') }}</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('groupEvaluations.description') }}</p>
      </div>
      <button type="button" class="btn btn-ghost btn-sm" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
    </div>
    <p v-if="error" class="text-sm text-gray-500" role="status">{{ error }}</p>
    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <button v-for="group in groups" :key="group.group_id" type="button" class="flex items-center justify-between gap-3 rounded-xl border border-gray-100 p-3 text-left transition hover:border-primary-300 hover:bg-primary-50/50 dark:border-dark-700 dark:hover:bg-dark-700" :aria-label="`${group.group_name} · ${t('groupEvaluations.title')}`" @click="selected = group">
        <span class="min-w-0">
          <strong class="block truncate text-sm text-gray-900 dark:text-white">{{ group.group_name }}</strong>
          <small class="mt-1 block text-gray-500">{{ [group.candy_enabled ? t('groupEvaluations.candy') : '', group.pelican_enabled ? t('groupEvaluations.pelican') : ''].filter(Boolean).join(' / ') }}</small>
        </span>
        <span class="shrink-0 text-xs text-primary-600 dark:text-primary-400">{{ group.running ? t('groupEvaluations.running') : t('groupEvaluations.viewResults') }}</span>
      </button>
    </div>
  </section>
  <GroupEvaluationDialog :show="selected !== null" :group-id="selected?.group_id ?? null" :group-name="selected?.group_name" @close="selected = null" />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { listEvaluationGroups, type EvaluationGroup } from '@/api/groupEvaluations'
import GroupEvaluationDialog from './GroupEvaluationDialog.vue'

const { t } = useI18n()
const emit = defineEmits<{ 'groups-loaded': [groups: EvaluationGroup[]] }>()
const groups = ref<EvaluationGroup[]>([])
const selected = ref<EvaluationGroup | null>(null)
const loading = ref(false)
const error = ref('')
let controller: AbortController | null = null

function openGroup(groupId: number) {
  const group = groups.value.find(item => item.group_id === groupId)
  if (group) selected.value = group
}
defineExpose({ openGroup })

async function load() {
  controller?.abort()
  const current = new AbortController()
  controller = current
  loading.value = true
  error.value = ''
  try {
    const result = await listEvaluationGroups(current.signal)
    if (current.signal.aborted) return
    groups.value = result.items.filter(group => group.enabled && (group.candy_enabled || group.pelican_enabled))
    emit('groups-loaded', groups.value)
    if (selected.value && !groups.value.some(group => group.group_id === selected.value?.group_id)) selected.value = null
  } catch (err) {
    if (!current.signal.aborted) {
      groups.value = []
      emit('groups-loaded', [])
      selected.value = null
      const status = (err as { status?: number })?.status
      if (status !== 403 && status !== 404) error.value = t('groupEvaluations.loadFailed')
    }
  } finally {
    if (controller === current) loading.value = false
  }
}

onMounted(() => void load())
onBeforeUnmount(() => controller?.abort())
</script>
