<template>
  <div class="space-y-5" data-testid="invitation-user-level">
    <form class="flex flex-wrap items-end gap-3" role="search" @submit.prevent="search">
      <div class="min-w-0 flex-1 basis-64">
        <label for="invitation-user-search" class="input-label">{{ t('invitations.users.search') }}</label>
        <input id="invitation-user-search" v-model="query" type="search" class="input" :placeholder="t('invitations.users.searchPlaceholder')" data-testid="invitation-user-search" />
      </div>
      <button type="submit" class="btn btn-secondary" :disabled="searching || !query.trim()" data-testid="invitation-user-search-submit">
        <Icon name="search" size="sm" />
        <span>{{ searching ? t('invitations.users.searching') : t('invitations.users.searchButton') }}</span>
      </button>
    </form>

    <p v-if="searchError" role="alert" class="text-sm text-red-700 dark:text-red-300">
      <span class="font-medium">{{ t('invitations.users.searchFailed') }}</span>{{ t('invitations.sep') }}{{ searchError }}
    </p>
    <ul v-if="results.length" class="flex flex-wrap gap-2" data-testid="invitation-user-results">
      <li v-for="user in results" :key="user.id">
        <button
          type="button"
          class="btn btn-sm"
          :class="user.id === userId ? 'btn-primary' : 'btn-secondary'"
          :aria-pressed="user.id === userId"
          :data-testid="`invitation-user-${user.id}`"
          @click="emit('select', user.id)"
        >
          <span class="break-all">{{ user.email }}</span>
          <span class="opacity-70">#{{ user.id }}</span>
        </button>
      </li>
    </ul>
    <p v-else-if="searched && !searching && !searchError" class="text-sm text-gray-500 dark:text-dark-400" data-testid="invitation-user-none">{{ t('invitations.users.noUsers') }}</p>

    <div v-if="!userId" class="rounded-2xl border border-dashed border-gray-300 px-5 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400">
      <p>{{ t('invitations.users.pick') }}</p>
    </div>
    <template v-else>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('invitations.users.selected', { id: userId }) }}</h3>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary btn-sm" data-testid="invitation-user-codes" @click="emit('records', 'codes', userId)">{{ t('invitations.users.viewCodes') }}</button>
          <button v-if="overview?.rebate_enabled !== false" type="button" class="btn btn-secondary btn-sm" data-testid="invitation-user-rebates" @click="emit('records', 'review', userId)">{{ t('invitations.users.viewRebates') }}</button>
        </div>
      </div>

      <div v-if="loading" class="flex justify-center py-8" role="status" :aria-label="t('common.loading')">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>
      <div v-else-if="loadError" role="alert" class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-200" data-testid="invitation-user-error">
        <span><span class="font-medium">{{ t('invitations.users.loadFailed') }}</span>{{ t('invitations.sep') }}{{ loadError }}</span>
        <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('invitations.records.retry') }}</button>
      </div>
      <template v-else-if="overview">
        <p v-if="overview.rebate_enabled === false" class="text-sm text-gray-600 dark:text-dark-300">{{ t('invitations.enterprise.note') }}</p>
        <InvitationOverviewPanel :overview="overview" :show-ladder="false" />
        <form class="rounded-xl border border-gray-200 p-4 dark:border-dark-700" @submit.prevent="saveLevel">
          <div class="flex flex-wrap items-end gap-3">
            <div class="w-72 max-w-full">
              <label for="invitation-override" class="input-label">{{ t('invitations.users.override') }}</label>
              <Select id="invitation-override" v-model="levelId" :options="levelOptions" />
            </div>
            <button type="submit" class="btn btn-primary" :disabled="saving || levelId === overview.override_level_id" data-testid="invitation-user-save">
              {{ saving ? t('common.processing') : t('invitations.users.saveLevel') }}
            </button>
          </div>
          <p class="input-hint">{{ t('invitations.users.overrideHint') }}</p>
          <p v-if="saveError" role="alert" class="mt-2 text-sm text-red-700 dark:text-red-300" data-testid="invitation-user-save-error">
            <span class="font-medium">{{ t('invitations.users.levelFailed') }}</span>{{ t('invitations.sep') }}{{ saveError }}
          </p>
        </form>
      </template>
    </template>

    <section class="border-t border-gray-100 pt-5 dark:border-dark-700" data-testid="invitation-overrides">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('invitations.users.overrides') }}
        <span v-if="overridesTotal" class="ml-1 font-normal tabular-nums text-gray-500 dark:text-dark-400">{{ overridesTotal }}</span>
      </h3>
      <p v-if="overridesError" role="alert" class="mt-2 flex flex-wrap items-center gap-3 text-sm text-red-700 dark:text-red-300" data-testid="invitation-overrides-error">
        <span><span class="font-medium">{{ t('invitations.users.overridesFailed') }}</span>{{ t('invitations.sep') }}{{ overridesError }}</span>
        <button type="button" class="btn btn-secondary btn-sm" @click="loadOverrides(true)">{{ t('invitations.records.retry') }}</button>
      </p>
      <p v-else-if="overridesLoaded && !overrides.length" class="mt-2 text-sm text-gray-500 dark:text-dark-400" data-testid="invitation-overrides-empty">{{ t('invitations.users.overridesEmpty') }}</p>
      <ul v-if="overrides.length" class="mt-2 divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="item in overrides" :key="item.user_id">
          <button
            type="button"
            class="flex w-full flex-wrap items-center justify-between gap-x-4 gap-y-1 py-2.5 text-left text-sm hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:hover:bg-dark-800"
            :aria-current="item.user_id === userId ? 'true' : undefined"
            :data-testid="`invitation-override-${item.user_id}`"
            @click="emit('select', item.user_id)"
          >
            <span class="min-w-0">
              <span class="break-all" :class="item.user_id === userId ? 'font-semibold text-primary-700 dark:text-primary-300' : 'text-gray-900 dark:text-gray-100'">{{ item.email }}</span>
              <span class="text-gray-500 dark:text-dark-400"> #{{ item.user_id }}</span>
            </span>
            <span class="flex flex-wrap items-center gap-x-3 gap-y-1">
              <span class="badge badge-primary">{{ levelName(item.level_id) }}</span>
              <span class="whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">{{ t('invitations.users.overrideSince', { date: formatDateOnly(item.updated_at) }) }}</span>
            </span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm mb-2"
            :disabled="saving"
            :data-testid="`invitation-override-clear-${item.user_id}`"
            @click="clearOverride(item.user_id)"
          >{{ t('invitations.users.clearOverride') }}</button>
        </li>
      </ul>
      <button v-if="overrides.length < overridesTotal" type="button" class="btn btn-secondary btn-sm mt-3" :disabled="overridesLoading" data-testid="invitation-overrides-more" @click="loadOverrides(false)">
        {{ overridesLoading ? t('common.loading') : t('invitations.users.overridesMore') }}
      </button>
    </section>
  </div>
  <TotpStepUpDialog :controller="stepUp" />
</template>

<script setup lang="ts">
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { isStepUpCancelled, useStepUp } from '@/composables/useStepUp'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import InvitationOverviewPanel from '@/components/invitations/InvitationOverview.vue'
import { invitationsAPI, type InvitationLevel, type InvitationOverride, type InvitationOverview } from '@/api/invitations'
import { list as listUsers } from '@/api/admin/users'
import { useAppStore } from '@/stores/app'
import { invitationErrorMessage } from '@/utils/invitationErrors'
import { formatDateOnly } from '@/utils/format'
import { formatRate } from '@/utils/invitationFormat'

const props = defineProps<{
  userId?: number
  levels: InvitationLevel[]
  /** Bump to reload the selected user after rules or rebates changed. */
  refreshKey?: number
}>()
const emit = defineEmits<{
  select: [id: number]
  records: [tab: 'codes' | 'review', id: number]
  changed: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const stepUp = useStepUp()
const query = ref('')
const results = ref<{ id: number; email: string }[]>([])
const searching = ref(false)
const searched = ref(false)
const searchError = ref('')
const overview = ref<InvitationOverview | null>(null)
const loading = ref(false)
const loadError = ref('')
const levelId = ref('')
const saving = ref(false)
const saveError = ref('')
let loadVersion = 0
const OVERRIDES_PAGE = 20
const overrides = ref<InvitationOverride[]>([])
const overridesTotal = ref(0)
const overridesLoading = ref(false)
const overridesLoaded = ref(false)
const overridesError = ref('')
let overridesVersion = 0

const levelOptions = computed(() => [
  { value: '', label: t('invitations.users.automatic') },
  ...props.levels.map((level) => ({ value: level.id, label: overview.value?.rebate_enabled === false ? level.name : `${level.name} (${formatRate(level.rebate_rate)})` }))
])

function levelName(id: string): string {
  // Removing a level that users are set to is refused by the server, so the id is a last resort.
  return props.levels.find((level) => level.id === id)?.name ?? id
}

/** reset=true reloads the first page (after a level change); otherwise appends the next page. */
async function loadOverrides(reset: boolean) {
  const version = ++overridesVersion
  const page = reset ? 1 : Math.floor(overrides.value.length / OVERRIDES_PAGE) + 1
  overridesLoading.value = true
  overridesError.value = ''
  try {
    const result = await invitationsAPI.overrides({ page, page_size: OVERRIDES_PAGE })
    if (version !== overridesVersion) return
    const items = result.items || []
    overrides.value = reset ? items : [...overrides.value, ...items.filter((item) => !overrides.value.some((o) => o.user_id === item.user_id))]
    overridesTotal.value = result.total
    overridesLoaded.value = true
  } catch (error) {
    if (version === overridesVersion) overridesError.value = invitationErrorMessage(error, t, t('invitations.users.overridesFailed'))
  } finally {
    if (version === overridesVersion) overridesLoading.value = false
  }
}

async function search() {
  const term = query.value.trim()
  if (!term || searching.value) return
  searching.value = true
  searchError.value = ''
  try {
    const result = await listUsers(1, 20, { search: term })
    results.value = (result.items || []).map((user) => ({ id: user.id, email: user.email }))
    searched.value = true
  } catch (error) {
    results.value = []
    searchError.value = invitationErrorMessage(error, t, t('invitations.users.searchFailed'))
  } finally {
    searching.value = false
  }
}

async function load() {
  const id = props.userId
  const version = ++loadVersion
  saveError.value = ''
  if (!id) {
    overview.value = null
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const result = await invitationsAPI.userOverview(id)
    if (version !== loadVersion) return
    overview.value = result
    levelId.value = result.override_level_id
  } catch (error) {
    if (version !== loadVersion) return
    overview.value = null
    loadError.value = invitationErrorMessage(error, t, t('invitations.users.loadFailed'))
  } finally {
    if (version === loadVersion) loading.value = false
  }
}

async function saveLevel() {
  const id = props.userId
  if (!id || saving.value || !overview.value || levelId.value === overview.value.override_level_id) return
  saving.value = true
  saveError.value = ''
  const selectedLevel = levelId.value
  try {
    const result = await stepUp.run(() => invitationsAPI.setLevel(id, selectedLevel))
    if (props.userId === id) {
      overview.value = result
      levelId.value = result?.override_level_id ?? ''
    }
    appStore.showSuccess(t('invitations.users.levelSaved'))
    emit('changed')
    void loadOverrides(true)
  } catch (error) {
    if (isStepUpCancelled(error)) return
    saveError.value = invitationErrorMessage(error, t, t('invitations.users.levelFailed'))
  } finally {
    saving.value = false
  }
}

async function clearOverride(id: number) {
  if (saving.value) return
  saving.value = true
  try {
    await stepUp.run(() => invitationsAPI.setLevel(id, ''))
    appStore.showSuccess(t('invitations.users.levelSaved'))
    emit('changed')
    if (props.userId === id) void load()
    await loadOverrides(true)
  } catch (error) {
    if (!isStepUpCancelled(error)) appStore.showError(invitationErrorMessage(error, t, t('invitations.users.levelFailed')))
  } finally {
    saving.value = false
  }
}

watch([() => props.userId, () => props.refreshKey], () => void load(), { immediate: true })
watch(() => props.refreshKey, () => void loadOverrides(true), { immediate: true })
</script>
