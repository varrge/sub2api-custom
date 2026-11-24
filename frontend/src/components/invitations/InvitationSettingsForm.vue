<template>
  <form class="space-y-6" novalidate data-testid="invitation-settings" @submit.prevent="save">
    <div class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
      <div class="flex items-start justify-between gap-4 rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <div>
          <p id="invitation-enabled-label" class="text-sm font-medium text-gray-900 dark:text-white">{{ t('invitations.settings.enabled') }}</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t(draft.enabled ? 'invitations.settings.enabledOn' : 'invitations.settings.enabledOff') }}</p>
        </div>
        <Toggle v-model="draft.enabled" aria-labelledby="invitation-enabled-label" data-testid="invitation-enabled" />
      </div>
      <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <label for="invitation-validity" class="input-label">{{ t('invitations.settings.validityDays') }}</label>
        <input
          id="invitation-validity"
          v-model.number="draft.validity_days"
          type="number"
          inputmode="numeric"
          min="1"
          max="3650"
          step="1"
          class="input w-40"
          :class="shown('validity_days') ? 'input-error' : ''"
          :aria-invalid="!!shown('validity_days')"
          aria-describedby="invitation-validity-hint"
          data-testid="invitation-validity"
          @blur="touch('validity_days')"
        />
        <p id="invitation-validity-hint" class="input-hint">
          <span v-if="shown('validity_days')" class="text-red-600 dark:text-red-400">{{ t(`invitations.issues.${shown('validity_days')}`) }}. </span>{{ t('invitations.settings.validityHint') }}
        </p>
      </div>
      <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700 md:col-span-2 xl:col-span-1">
        <label for="invitation-auto-approve" class="input-label">{{ t('invitations.settings.autoApprove') }}</label>
        <input
          id="invitation-auto-approve"
          v-model.number="draft.auto_approve_hours"
          type="number"
          inputmode="numeric"
          min="0"
          :max="MAX_AUTO_APPROVE_HOURS"
          step="1"
          class="input w-40"
          :class="shown('auto_approve_hours') ? 'input-error' : ''"
          :aria-invalid="!!shown('auto_approve_hours')"
          aria-describedby="invitation-auto-approve-hint"
          data-testid="invitation-auto-approve"
          @blur="touch('auto_approve_hours')"
        />
        <p id="invitation-auto-approve-hint" class="input-hint">
          <span v-if="shown('auto_approve_hours')" class="text-red-600 dark:text-red-400" data-testid="invitation-auto-approve-issue">{{ t(`invitations.issues.${shown('auto_approve_hours')}`) }}. </span>{{ t(draft.auto_approve_hours === 0 ? 'invitations.settings.autoApproveOff' : 'invitations.settings.autoApproveOn') }}
        </p>
      </div>
    </div>

    <div>
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('invitations.settings.levels') }}</h3>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-dark-400">{{ t('invitations.settings.levelsHint') }}</p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="draft.levels.length >= 100" data-testid="invitation-add-level" @click="addLevel">
          <Icon name="plus" size="sm" />
          <span>{{ t('invitations.settings.addLevel') }}</span>
        </button>
      </div>

      <p v-if="issues.general && attempted" role="alert" class="mt-3 text-sm text-red-700 dark:text-red-300" data-testid="invitation-general-issue">
        {{ t(`invitations.issues.${issues.general}`) }}
      </p>

      <ul class="mt-4 space-y-3">
        <li
          v-for="(level, index) in draft.levels"
          :key="keys[index]"
          class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"
          :data-testid="`invitation-level-${index}`"
        >
          <fieldset>
            <legend class="sr-only">{{ t('invitations.settings.levelN', { n: index + 1 }) }}</legend>
            <div class="grid gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-6">
              <div v-for="field in fields" :key="field.key" :class="field.span">
                <label :for="`invitation-level-${keys[index]}-${field.key}`" class="input-label">{{ t(`invitations.settings.${field.label}`) }}</label>
                <!-- v-model.number keeps half-typed input such as "1." intact while the field has focus. -->
                <input
                  v-if="field.numeric"
                  v-model.number="level[field.key]"
                  type="number"
                  inputmode="decimal"
                  min="0"
                  :max="field.key === 'rebate_rate' ? 100 : undefined"
                  :step="field.step"
                  :id="`invitation-level-${keys[index]}-${field.key}`"
                  class="input"
                  :class="[shown(`${keys[index]}.${field.key}`, index, field.key) ? 'input-error' : '', field.key === 'id' ? 'font-mono' : '', field.key === 'id' && isLocked(index) ? 'bg-gray-50 text-gray-500 dark:bg-dark-900 dark:text-dark-400' : '']"
                  :aria-invalid="!!shown(`${keys[index]}.${field.key}`, index, field.key)"
                  :aria-describedby="`invitation-level-${keys[index]}-${field.key}-hint`"
                  :data-testid="`invitation-level-${index}-${field.key}`"
                  @input="saveError = ''"
                  @blur="touch(`${keys[index]}.${field.key}`)"
                />
                <input
                  v-else
                  v-model="level[field.key]"
                  type="text"
                  :maxlength="field.maxlength"
                  :readonly="field.key === 'id' && isLocked(index)"
                  :id="`invitation-level-${keys[index]}-${field.key}`"
                  class="input"
                  :class="[shown(`${keys[index]}.${field.key}`, index, field.key) ? 'input-error' : '', field.key === 'id' ? 'font-mono' : '', field.key === 'id' && isLocked(index) ? 'bg-gray-50 text-gray-500 dark:bg-dark-900 dark:text-dark-400' : '']"
                  :aria-invalid="!!shown(`${keys[index]}.${field.key}`, index, field.key)"
                  :aria-describedby="`invitation-level-${keys[index]}-${field.key}-hint`"
                  :data-testid="`invitation-level-${index}-${field.key}`"
                  @input="saveError = ''"
                  @blur="touch(`${keys[index]}.${field.key}`)"
                />
                <p :id="`invitation-level-${keys[index]}-${field.key}-hint`" class="input-hint min-h-[1rem]">
                  <span v-if="shown(`${keys[index]}.${field.key}`, index, field.key)" class="text-red-600 dark:text-red-400" :data-testid="`invitation-level-${index}-${field.key}-issue`">
                    {{ t(`invitations.issues.${shown(`${keys[index]}.${field.key}`, index, field.key)}`) }}
                  </span>
                  <span v-else-if="field.key === 'id' && isLocked(index)">{{ t('invitations.settings.idHint') }}</span>
                  <span v-else-if="field.limit && level[field.key] === 0">{{ t('invitations.unlimited') }}</span>
                </p>
              </div>
            </div>
            <div class="mt-1 flex justify-end">
              <button
                type="button"
                class="btn btn-ghost btn-sm text-red-700 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900/20"
                :disabled="draft.levels.length <= 1"
                :data-testid="`invitation-remove-level-${index}`"
                @click="removeLevel(index)"
              >
                <Icon name="trash" size="sm" />
                <span>{{ level.name ? t('invitations.settings.removeLevel', { name: level.name }) : t('invitations.settings.removeUnnamed') }}</span>
              </button>
            </div>
          </fieldset>
        </li>
      </ul>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-5 dark:border-dark-700">
      <p class="min-w-0 flex-1 text-sm" aria-live="polite">
        <span v-if="saveError" role="alert" class="text-red-700 dark:text-red-300" data-testid="invitation-settings-error">
          <span class="font-medium">{{ t('invitations.settings.saveFailed') }}</span>{{ t('invitations.sep') }}{{ saveError }}
        </span>
        <span v-else-if="attempted && issues.count" class="text-red-700 dark:text-red-300" data-testid="invitation-settings-issues">{{ t('invitations.settings.fixIssues', { count: issues.count }) }}</span>
        <span v-else-if="dirty" class="text-amber-800 dark:text-amber-300">{{ t('invitations.settings.unsaved') }}</span>
      </p>
      <div class="flex flex-wrap gap-3">
        <button type="button" class="btn btn-secondary" :disabled="!dirty || saving" data-testid="invitation-settings-reset" @click="reset">{{ t('invitations.settings.reset') }}</button>
        <button type="submit" class="btn btn-primary" :disabled="!dirty || saving" data-testid="invitation-settings-save">
          {{ saving ? t('invitations.settings.saving') : t('invitations.settings.save') }}
        </button>
      </div>
    </div>
  </form>
  <TotpStepUpDialog :controller="stepUp" />
</template>

<script setup lang="ts">
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { isStepUpCancelled, useStepUp } from '@/composables/useStepUp'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { invitationsAPI, type InvitationLevel, type InvitationSettings } from '@/api/invitations'
import { useAppStore } from '@/stores/app'
import { invitationErrorMessage } from '@/utils/invitationErrors'
import {
  MAX_AUTO_APPROVE_HOURS,
  cloneSettings,
  sameSettings,
  suggestLevelId,
  validateInvitationSettings,
  type LevelField,
  type SettingsIssue
} from '@/utils/invitationSettings'

const props = defineProps<{ settings: InvitationSettings }>()
const emit = defineEmits<{ saved: [settings: InvitationSettings]; dirty: [value: boolean] }>()

const { t } = useI18n()
const appStore = useAppStore()
const stepUp = useStepUp()

const fields: { key: LevelField; label: string; numeric?: boolean; step?: string; maxlength?: number; limit?: boolean; span: string }[] = [
  { key: 'name', label: 'name', maxlength: 100, span: 'lg:col-span-2' },
  { key: 'id', label: 'id', maxlength: 64, span: 'lg:col-span-2' },
  { key: 'spend_threshold', label: 'threshold', numeric: true, step: '0.01', span: 'lg:col-span-2' },
  { key: 'lifetime_invite_limit', label: 'lifetimeLimit', numeric: true, step: '1', limit: true, span: 'lg:col-span-2' },
  { key: 'unused_invite_limit', label: 'unusedLimit', numeric: true, step: '1', limit: true, span: 'lg:col-span-2' },
  { key: 'rebate_rate', label: 'rate', numeric: true, step: '0.01', span: 'lg:col-span-2' }
]

const draft = reactive<InvitationSettings>(cloneSettings(props.settings))
// Stable row keys: indexes shift when a level is removed, which would move focus and touched state.
const keys = ref<number[]>([])
let nextKey = 0
const touched = reactive(new Set<string>())
const attempted = ref(false)
const saving = ref(false)
const saveError = ref('')

// Rows that exist on the server keep their identifier: user overrides and past rebates refer to it.
// Tracked per row, so typing an existing identifier into a new level never locks that new row.
const lockedKeys = reactive(new Set<number>())
const lockedIds = computed(() => draft.levels.filter((_, index) => isLocked(index)).map((level) => level.id))
const issues = computed(() => validateInvitationSettings(draft, lockedIds.value))
const dirty = computed(() => !sameSettings(draft, props.settings))

function isLocked(index: number): boolean {
  return lockedKeys.has(keys.value[index])
}
function touch(key: string) {
  touched.add(key)
}
/** The issue to display for a field, once the user has left it or tried to save. */
function shown(key: string, index?: number, field?: LevelField): SettingsIssue | undefined {
  if (!attempted.value && !touched.has(key)) return undefined
  if (index === undefined || !field) return issues.value[key as 'validity_days' | 'auto_approve_hours']
  return issues.value.levels[index]?.[field]
}
function load(settings: InvitationSettings) {
  Object.assign(draft, cloneSettings(settings))
  keys.value = settings.levels.map(() => nextKey++)
  lockedKeys.clear()
  keys.value.forEach((key) => lockedKeys.add(key))
  touched.clear()
  attempted.value = false
  saveError.value = ''
}
function addLevel() {
  const top = draft.levels.reduce((max, level) => Math.max(max, Number(level.spend_threshold) || 0), 0)
  const level: InvitationLevel = {
    id: suggestLevelId(draft.levels.map((item) => item.id)),
    name: '',
    spend_threshold: draft.levels.length ? top + 100 : 0,
    lifetime_invite_limit: 0,
    unused_invite_limit: 0,
    rebate_rate: 0
  }
  draft.levels.push(level)
  keys.value.push(nextKey++)
}
function removeLevel(index: number) {
  if (draft.levels.length <= 1) return
  draft.levels.splice(index, 1)
  keys.value.splice(index, 1)
  saveError.value = ''
}
function reset() {
  load(props.settings)
}
async function save() {
  if (saving.value || !dirty.value) return
  attempted.value = true
  saveError.value = ''
  if (issues.value.count) return
  saving.value = true
  try {
    const payload = cloneSettings(draft)
    payload.levels = payload.levels.map((level) => ({ ...level, id: level.id.trim(), name: level.name.trim() }))
    const saved = await stepUp.run(() => invitationsAPI.saveSettings(payload))
    // Adopt the server's copy (trimmed, sorted by threshold) so the form is clean again.
    load(saved)
    appStore.showSuccess(t('invitations.settings.saved'))
    emit('saved', saved)
  } catch (error) {
    if (isStepUpCancelled(error)) return
    saveError.value = invitationErrorMessage(error, t, t('invitations.settings.saveFailed'))
  } finally {
    saving.value = false
  }
}

// The parent replaces `settings` after a save or reload; adopt it unless the user is mid-edit.
watch(() => props.settings, (next, previous) => {
  if (!previous || sameSettings(draft, previous)) load(next)
}, { immediate: true })
watch(dirty, (value) => emit('dirty', value), { immediate: true })
</script>
