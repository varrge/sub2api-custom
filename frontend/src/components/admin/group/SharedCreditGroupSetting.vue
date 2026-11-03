<template>
  <div v-if="groupId" class="rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800" data-testid="group-shared-credit-section">
    <div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-dark-700">
      <div class="flex items-center gap-2">
        <Icon name="creditCard" class="text-primary-500" />
        <h3 class="text-sm font-semibold text-gray-900 dark:text-dark-50">
          {{ t('sharedCredit.groupSetting.title') }}
        </h3>
      </div>
      <span
        v-if="globalState"
        class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
        :class="globalState.status === 'active' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-400'"
      >
        {{ t(`sharedCredit.activity.status.${globalState.status}`) }}
        <span class="ml-1 text-gray-400">(${{ globalState.remaining_usd }})</span>
      </span>
    </div>

    <!-- 409 Conflict Banner -->
    <div
      v-if="conflictDetected"
      class="mt-3 flex items-center justify-between rounded-lg border border-amber-300 bg-amber-50 p-3 text-xs text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200"
      role="alert"
    >
      <span>{{ t('sharedCredit.conflictDetected') }}</span>
      <button type="button" class="btn btn-secondary px-2 py-0.5 text-xs" @click="loadGlobalState(true)">
        {{ t('sharedCredit.overwriteAndReload') }}
      </button>
    </div>

    <!-- Error message -->
    <div
      v-if="error"
      class="mt-3 text-xs text-red-600 dark:text-red-400"
      role="alert"
    >
      {{ error }}
      <button type="button" class="btn btn-secondary" :disabled="loading || saving" @click="loadGlobalState(true)">{{ t('sharedCredit.retry') }}</button>
    </div>

    <div class="mt-4 flex flex-wrap items-center justify-between gap-4">
      <div class="space-y-1">
        <div class="text-sm font-medium text-gray-700 dark:text-dark-200">
          {{ t('sharedCredit.groupSetting.participateLabel') }}
        </div>
        <p class="text-xs text-gray-500 dark:text-dark-400">
          {{ t('sharedCredit.groupSetting.participateHint') }}
        </p>
        <p class="text-xs text-amber-700 dark:text-amber-300">{{ t('sharedCredit.activity.balanceNotice') }}</p>
      </div>

      <div class="flex items-center gap-3">
        <Toggle
          v-model="isParticipating"
          :disabled="loading || saving || !globalState || conflictDetected"
          data-testid="group-shared-credit-toggle"
          :aria-label="t('sharedCredit.groupSetting.participateLabel')"
        />
        <button
          type="button"
          class="btn btn-primary px-3 py-1.5 text-xs"
          :disabled="!dirty || saving || loading || !globalState || conflictDetected"
          @click="saveGroupParticipation"
          data-testid="group-shared-credit-save"
        >
          <svg v-if="saving" class="h-3.5 w-3.5 animate-spin text-white" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <span v-else>{{ t('sharedCredit.save') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { getState, setGroup, type SharedCreditState } from '@/api/admin/sharedCredit'

const props = defineProps<{
  groupId: number
}>()

const { t } = useI18n()

const globalState = ref<SharedCreditState | null>(null)
const isParticipating = ref(false)
const originalParticipation = ref(false)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const conflictDetected = ref(false)

const dirty = computed(() => {
  return isParticipating.value !== originalParticipation.value
})

let reqId = 0
const loadGlobalState = async (forceOverwriteDraft = false) => {
  if (!props.groupId || saving.value) return
  const hadDraft = dirty.value
  const currentReq = ++reqId
  loading.value = true
  error.value = ''
  try {
    const state = await getState()
    if (currentReq !== reqId) return
    globalState.value = state
    const participates = state.group_ids.includes(props.groupId)
    originalParticipation.value = participates
    if (!hadDraft || forceOverwriteDraft) {
      isParticipating.value = participates
      conflictDetected.value = false
    }
  } catch (err: any) {
    if (currentReq === reqId) {
      error.value = err?.message || t('sharedCredit.loadFailed')
    }
  } finally {
    if (currentReq === reqId) {
      loading.value = false
    }
  }
}

const saveGroupParticipation = async () => {
  if (!props.groupId || !globalState.value || saving.value || loading.value || conflictDetected.value) return
  saving.value = true
  error.value = ''
  conflictDetected.value = false
  try {
    const updated = await setGroup(props.groupId, {
      enabled: isParticipating.value,
      revision: globalState.value.revision
    })
    globalState.value = updated
    const participates = updated.group_ids.includes(props.groupId)
    originalParticipation.value = participates
    isParticipating.value = participates
  } catch (err: any) {
    if (err?.status === 409 || err?.response?.status === 409) {
      conflictDetected.value = true
    } else {
      error.value = err?.message || t('sharedCredit.saveFailed')
    }
  } finally {
    saving.value = false
  }
}

watch(() => props.groupId, () => {
  loadGlobalState(true)
})

onMounted(() => {
  loadGlobalState(true)
})
</script>
