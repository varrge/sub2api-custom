<template>
  <div class="space-y-6">
    <!-- Header & Entry Hint -->
    <div class="space-y-2">
      <div class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-200/80 pb-5 dark:border-dark-700/80">
        <div>
          <h2 class="text-xl font-bold tracking-tight text-gray-900 dark:text-dark-50 sm:text-2xl">
            {{ t('voting.manager.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('voting.manager.subtitle') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary btn-sm inline-flex items-center gap-1.5"
            :disabled="loading || saving"
            @click="emit('refresh')"
          >
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            <span>{{ t('voting.manager.refresh') }}</span>
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm inline-flex items-center gap-1.5 shadow-sm transition-all active:scale-[0.98]"
            :disabled="loading || saving"
            @click="emit('create')"
          >
            <Icon name="plus" size="sm" />
            <span>{{ t('voting.manager.createPoll') }}</span>
          </button>
        </div>
      </div>

      <!-- Discreet hint about entry toggle -->
      <div class="flex items-center gap-2 rounded-xl bg-gray-50 px-3.5 py-2.5 text-xs text-gray-500 dark:bg-dark-800/60 dark:text-dark-400">
        <Icon name="infoCircle" size="sm" class="shrink-0 text-gray-400 dark:text-dark-400" />
        <span>{{ t('voting.manager.entryHint') }}</span>
      </div>
    </div>

    <!-- Error Banner -->
    <div
      v-if="error"
      role="alert"
      class="flex items-center justify-between rounded-xl border border-red-200 bg-red-50 p-3.5 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-300"
    >
      <div class="flex items-center gap-2">
        <Icon name="exclamationCircle" size="sm" class="shrink-0" />
        <span>{{ error }}</span>
      </div>
      <button
        type="button"
        class="text-xs underline hover:no-underline"
        @click="emit('refresh')"
      >
        {{ t('voting.board.retry') }}
      </button>
    </div>

    <!-- Management Table Card -->
    <div class="overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-sm dark:border-dark-700/80 dark:bg-dark-800">
      <!-- Loading State -->
      <div v-if="loading && !polls.length" class="space-y-3 p-12 text-center text-sm text-gray-500 dark:text-dark-400">
        <Icon name="refresh" size="md" class="mx-auto animate-spin text-primary-500 dark:text-primary-400" />
        <p>{{ t('voting.manager.loading') }}</p>
      </div>

      <!-- Empty List State -->
      <div v-else-if="!polls.length" class="p-12 text-center">
        <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-dark-400">
          <Icon name="chartBar" size="md" />
        </div>
        <p class="mt-3 text-sm font-medium text-gray-900 dark:text-dark-100">{{ t('voting.manager.noPolls') }}</p>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('voting.manager.noPollsHint') }}</p>
      </div>

      <!-- Table View -->
      <div v-else class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 text-left text-sm dark:divide-dark-700">
          <thead class="bg-gray-50/70 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:bg-dark-900 dark:text-dark-400">
            <tr>
              <th scope="col" class="px-6 py-3.5">{{ t('voting.manager.colTitle') }}</th>
              <th scope="col" class="px-4 py-3.5">{{ t('voting.manager.colStatus') }}</th>
              <th scope="col" class="px-4 py-3.5">{{ t('voting.manager.colVoters') }}</th>
              <th scope="col" class="px-4 py-3.5">{{ t('voting.manager.colUpdated') }}</th>
              <th scope="col" class="px-6 py-3.5 text-right">{{ t('voting.manager.colActions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700/60">
            <tr
              v-for="item in polls"
              :key="item.id"
              class="transition-colors hover:bg-gray-50/60 dark:hover:bg-dark-700/30"
            >
              <td class="max-w-xs px-6 py-4 sm:max-w-sm md:max-w-md">
                <div class="font-medium text-gray-900 dark:text-dark-100 truncate" :title="item.title">
                  {{ item.title }}
                </div>
                <div v-if="item.description" class="mt-0.5 line-clamp-1 text-xs text-gray-500 dark:text-dark-400">
                  {{ item.description }}
                </div>
              </td>
              <td class="whitespace-nowrap px-4 py-4">
                <span :class="['inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset', getStatusClass(item.status)]">
                  {{ getStatusText(item.status) }}
                </span>
              </td>
              <td class="whitespace-nowrap px-4 py-4 text-xs text-gray-600 dark:text-dark-300">
                <span class="inline-flex items-center gap-1.5">
                  <Icon name="users" size="sm" class="h-3.5 w-3.5 text-gray-400" />
                  {{ item.voter_count }}
                </span>
              </td>
              <td class="whitespace-nowrap px-4 py-4 text-xs text-gray-500 dark:text-dark-400">
                {{ formatDate(item.updated_at) }}
              </td>
              <td class="whitespace-nowrap px-6 py-4 text-right">
                <div class="inline-flex items-center justify-end gap-1.5">
                  <!-- Inspect / Results button -->
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm p-1.5 text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-dark-50"
                    :title="t('voting.manager.inspect')"
                    :aria-label="t('voting.manager.inspect')"
                    :disabled="saving"
                    @click="inspect(item.id)"
                  >
                    <Icon name="chartBar" size="sm" />
                  </button>
                  <!-- Edit button -->
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm p-1.5 text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-dark-50"
                    :title="t('voting.manager.edit')"
                    :aria-label="t('voting.manager.edit')"
                    :disabled="saving"
                    @click="emit('edit', item.id)"
                  >
                    <Icon name="edit" size="sm" />
                  </button>
                  <!-- Delete button (disabled if voter_count > 0) -->
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm p-1.5 text-red-600 hover:text-red-700 disabled:opacity-30 dark:text-red-400"
                    :disabled="item.voter_count > 0 || saving"
                    :title="item.voter_count > 0 ? t('voting.manager.deleteDisabledHint') : t('voting.manager.delete')"
                    :aria-label="t('voting.manager.delete')"
                    @click="emit('delete', item.id)"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div
        v-if="pages > 1"
        class="flex items-center justify-between border-t border-gray-100 bg-gray-50/50 px-6 py-3.5 dark:border-dark-700/60 dark:bg-dark-800/80"
      >
        <span class="text-xs text-gray-500 dark:text-dark-400">
          {{ t('voting.manager.pagination', { page, pages }) }}
        </span>
        <div class="flex items-center gap-1">
          <button
            type="button"
            class="btn btn-secondary btn-sm p-1.5"
            :disabled="page <= 1 || loading || saving"
            :aria-label="t('voting.manager.prevPage')"
            @click="emit('page-change', page - 1)"
          >
            <Icon name="chevronLeft" size="sm" />
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm p-1.5"
            :disabled="page >= pages || loading || saving"
            :aria-label="t('voting.manager.nextPage')"
            @click="emit('page-change', page + 1)"
          >
            <Icon name="chevronRight" size="sm" />
          </button>
        </div>
      </div>
    </div>

    <!-- Inspect Dialog -->
    <BaseDialog
      :show="inspectModalOpen"
      :title="t('voting.manager.dialogInspectTitle')"
      width="wide"
      @close="inspectModalOpen = false"
    >
      <p v-if="error" role="alert" class="mb-3 text-sm text-red-600 dark:text-red-400">{{ error }}</p>
      <div v-if="detailLoading" class="p-8 text-center text-sm text-gray-500 dark:text-dark-400">
        <Icon name="refresh" size="md" class="mx-auto animate-spin text-primary-500" />
        <p class="mt-2">{{ t('voting.board.loadingDetail') }}</p>
      </div>
      <div v-else-if="poll" class="space-y-6">
        <div>
          <h3 class="text-lg font-bold text-gray-900 dark:text-dark-50">{{ poll.title }}</h3>
          <p v-if="poll.description" class="mt-1 text-xs text-gray-500 dark:text-dark-400 whitespace-pre-line">
            {{ poll.description }}
          </p>
          <div class="mt-3 flex items-center gap-2 text-xs text-gray-600 dark:text-dark-300">
            <span :class="['inline-flex items-center rounded px-1.5 py-0.5 font-medium ring-1 ring-inset', getStatusClass(poll.status)]">
              {{ getStatusText(poll.status) }}
            </span>
            <span>{{ t('voting.manager.inspectTotalVoters', { count: poll.voter_count }) }}</span>
          </div>
        </div>

        <div class="space-y-3">
          <div
            v-for="opt in poll.options"
            :key="opt.id"
            class="relative overflow-hidden rounded-xl border border-gray-200 bg-gray-50/50 p-4 dark:border-dark-700 dark:bg-dark-900"
          >
            <div
              class="absolute inset-y-0 left-0 bg-primary-500/10 dark:bg-primary-400/15"
              :style="{ width: `${calcInspectPercentage(opt.votes, poll.voter_count)}%` }"
            />
            <div class="relative flex items-center justify-between gap-3">
              <div class="flex items-center gap-3">
                <img v-if="opt.image" :src="opt.image" :alt="opt.title" class="h-10 w-10 shrink-0 rounded-lg object-cover" />
                <span class="text-sm font-medium text-gray-900 dark:text-dark-100">{{ opt.title }}</span>
              </div>
              <span class="text-xs font-semibold text-gray-700 dark:text-dark-300 shrink-0">
                {{ t('voting.manager.inspectOptionVotes', { votes: opt.votes, pct: calcInspectPercentage(opt.votes, poll.voter_count) }) }}
              </span>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button type="button" class="btn btn-secondary btn-sm" @click="inspectModalOpen = false">
            {{ t('voting.board.cancel') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Create / Edit Form Dialog -->
    <BaseDialog
      :show="formOpen"
      :title="editingId ? t('voting.manager.dialogEditTitle') : t('voting.manager.dialogCreateTitle')"
      width="wide"
      @close="emit('close-form')"
    >
      <form @submit.prevent="handleFormSubmit">
        <fieldset :disabled="saving" class="space-y-6">
        <!-- Validation Banner -->
        <div
          v-if="validationError || error"
          role="alert"
          class="flex items-center gap-2 rounded-xl border border-red-200 bg-red-50 p-3 text-xs text-red-700 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-300"
        >
          <Icon name="exclamationCircle" size="sm" class="shrink-0" />
          <span>{{ validationError || error }}</span>
        </div>

        <!-- Locked Hint if voters exist -->
        <div
          v-if="formVoteCount > 0"
          class="flex items-center gap-2.5 rounded-xl border border-amber-200/80 bg-amber-50/80 p-3.5 text-xs text-amber-800 dark:border-amber-900/40 dark:bg-amber-950/30 dark:text-amber-300"
        >
          <Icon name="infoCircle" size="sm" class="shrink-0 text-amber-600 dark:text-amber-400" />
          <span>{{ t('voting.manager.optionLockedNotice', { count: formVoteCount }) }}</span>
        </div>

        <!-- Title Field -->
        <div>
          <label for="voting-title" class="block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-dark-200">
            {{ t('voting.manager.fieldTitle') }} *
          </label>
          <input
            id="voting-title"
            type="text"
            maxlength="200"
            required
            class="input mt-1.5 w-full"
            :placeholder="t('voting.manager.fieldTitlePlaceholder')"
            :value="form.title"
            @input="updateTitle(($event.target as HTMLInputElement).value)"
          />
          <div class="mt-1 flex justify-end text-[11px] text-gray-400 dark:text-dark-500">
            {{ (form.title || '').length }} / 200
          </div>
        </div>

        <!-- Description Field -->
        <div>
          <label for="voting-description" class="block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-dark-200">
            {{ t('voting.manager.fieldDescription') }}
          </label>
          <textarea
            id="voting-description"
            rows="3"
            maxlength="2000"
            class="input mt-1.5 w-full"
            :placeholder="t('voting.manager.fieldDescriptionPlaceholder')"
            :value="form.description"
            @input="updateDescription(($event.target as HTMLTextAreaElement).value)"
          />
          <div class="mt-1 flex justify-end text-[11px] text-gray-400 dark:text-dark-500">
            {{ (form.description || '').length }} / 2000
          </div>
        </div>

        <!-- Cover Image -->
        <div>
          <label class="block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-dark-200">
            {{ t('voting.manager.fieldCover') }}
          </label>
          <div class="mt-1.5">
            <ImageUpload
              :model-value="form.image"
              :hint="t('voting.manager.fieldCoverHint')"
              :max-size="300 * 1024"
              @update:model-value="updateImage"
            />
          </div>
        </div>

        <!-- Status Select -->
        <div>
          <label for="voting-status" class="block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-dark-200">
            {{ t('voting.manager.fieldStatus') }}
          </label>
          <select
            id="voting-status"
            class="input mt-1.5 w-full"
            :value="form.status"
            @change="updateStatus(($event.target as HTMLSelectElement).value as PollStatus)"
          >
            <!-- Do not offer draft when editing an already published poll -->
            <option v-if="canSelectDraft" value="draft">{{ t('voting.manager.statusDraft') }}</option>
            <option value="open">{{ t('voting.manager.statusOpen') }}</option>
            <option value="closed">{{ t('voting.manager.statusClosed') }}</option>
          </select>
        </div>

        <!-- Options Section -->
        <div class="space-y-3 border-t border-gray-100 pt-5 dark:border-dark-700/60">
          <div class="flex items-center justify-between">
            <div>
              <label class="block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-dark-200">
                {{ t('voting.manager.optionsSectionTitle') }} *
              </label>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('voting.manager.optionsSectionHint') }}</p>
            </div>
            <button
              v-if="formVoteCount === 0"
              type="button"
              class="btn btn-secondary btn-sm inline-flex items-center gap-1"
              :disabled="form.options.length >= 12"
              @click="addOption"
            >
              <Icon name="plus" size="sm" />
              <span>{{ t('voting.manager.addOption') }}</span>
            </button>
          </div>

          <!-- Options list items -->
          <div class="space-y-4">
            <div
              v-for="(opt, index) in form.options"
              :key="index"
              class="rounded-xl border border-gray-200/90 bg-gray-50/50 p-4 dark:border-dark-700 dark:bg-dark-900/60"
            >
              <div class="flex items-start gap-3">
                <span class="mt-2.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gray-200 text-xs font-semibold text-gray-700 dark:bg-dark-700 dark:text-dark-300">
                  {{ index + 1 }}
                </span>

                <fieldset :disabled="formVoteCount > 0" class="min-w-0 flex-1 space-y-3">
                  <input
                    type="text"
                    required
                    maxlength="200"
                    :disabled="formVoteCount > 0"
                    class="input w-full"
                    :placeholder="t('voting.manager.optionPlaceholder')"
                    :aria-label="`${t('voting.manager.optionPlaceholder')} ${index + 1}`"
                    :value="opt.title"
                    @input="updateOptionTitle(index, ($event.target as HTMLInputElement).value)"
                  />

                  <div>
                    <ImageUpload
                      :model-value="opt.image"
                      size="sm"
                      :hint="t('voting.manager.optionImageHint')"
                      :max-size="300 * 1024"
                      @update:model-value="(val) => updateOptionImage(index, val)"
                    />
                  </div>
                </fieldset>

                <button
                  v-if="formVoteCount === 0"
                  type="button"
                  class="btn btn-secondary btn-sm p-1.5 text-red-600 hover:text-red-700 disabled:opacity-30 dark:text-red-400"
                  :disabled="form.options.length <= 2"
                  :title="t('voting.manager.removeOption')"
                  :aria-label="t('voting.manager.removeOption')"
                  @click="removeOption(index)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </div>
            </div>
          </div>
        </div>
        </fieldset>
      </form>

      <template #footer>
        <div class="flex items-center justify-end gap-2">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="saving"
            @click="emit('close-form')"
          >
            {{ t('voting.manager.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm inline-flex items-center gap-1.5 shadow-sm"
            :disabled="saving"
            @click="handleFormSubmit"
          >
            <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
            <span>{{ saving ? t('voting.manager.saving') : t('voting.manager.save') }}</span>
          </button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ImageUpload from '@/components/common/ImageUpload.vue'
import type { Poll, PollSummary, PollInput, PollStatus } from '@/api/voting'

const props = withDefaults(
  defineProps<{
    polls: PollSummary[]
    poll: Poll | null
    loading?: boolean
    detailLoading?: boolean
    saving?: boolean
    error?: string
    page?: number
    pages?: number
    form: PollInput
    formOpen: boolean
    editingId: number | null
    formVoteCount?: number
  }>(),
  {
    loading: false,
    detailLoading: false,
    saving: false,
    error: '',
    page: 1,
    pages: 1,
    formVoteCount: 0
  }
)

const emit = defineEmits<{
  (e: 'create'): void
  (e: 'edit', id: number): void
  (e: 'inspect', id: number): void
  (e: 'save'): void
  (e: 'close-form'): void
  (e: 'delete', id: number): void
  (e: 'update:form', value: PollInput): void
  (e: 'page-change', page: number): void
  (e: 'refresh'): void
}>()

const { t, locale } = useI18n()
const validationError = ref('')
const inspectModalOpen = ref(false)

function inspect(id: number) {
  inspectModalOpen.value = true
  emit('inspect', id)
}

watch(
  () => props.formOpen,
  (isOpen) => {
    if (isOpen) {
      validationError.value = ''
      inspectModalOpen.value = false
    }
  }
)

const editingPollOrigin = computed<PollSummary | undefined>(() => {
  if (!props.editingId) return undefined
  if (props.poll?.id === props.editingId) return props.poll
  return props.polls.find((p) => p.id === props.editingId)
})

const canSelectDraft = computed(() => {
  if (!props.editingId) return true
  const original = editingPollOrigin.value
  if (!original) return false
  return original.status === 'draft'
})

function updateTitle(title: string) {
  emit('update:form', { ...props.form, title })
}

function updateDescription(description: string) {
  emit('update:form', { ...props.form, description })
}

function updateImage(image: string) {
  emit('update:form', { ...props.form, image })
}

function updateStatus(status: PollStatus) {
  emit('update:form', { ...props.form, status })
}

function addOption() {
  if (props.form.options.length >= 12 || props.formVoteCount > 0) return
  const newOptions = [...props.form.options, { title: '', image: '' }]
  emit('update:form', { ...props.form, options: newOptions })
}

function removeOption(index: number) {
  if (props.form.options.length <= 2 || props.formVoteCount > 0) return
  const newOptions = props.form.options.filter((_, i) => i !== index)
  emit('update:form', { ...props.form, options: newOptions })
}

function updateOptionTitle(index: number, title: string) {
  if (props.formVoteCount > 0) return
  const newOptions = props.form.options.map((opt, i) => (i === index ? { ...opt, title } : opt))
  emit('update:form', { ...props.form, options: newOptions })
}

function updateOptionImage(index: number, image: string) {
  if (props.formVoteCount > 0) return
  const newOptions = props.form.options.map((opt, i) => (i === index ? { ...opt, image } : opt))
  emit('update:form', { ...props.form, options: newOptions })
}

function handleFormSubmit() {
  validationError.value = ''
  const title = (props.form.title || '').trim()
  if (!title) {
    validationError.value = t('voting.manager.validationTitleRequired')
    return
  }
  if (title.length > 200) {
    validationError.value = t('voting.manager.validationTitleTooLong')
    return
  }
  if ((props.form.description || '').length > 2000) {
    validationError.value = t('voting.manager.validationDescTooLong')
    return
  }
  if (props.form.options.length < 2) {
    validationError.value = t('voting.manager.validationMinOptions')
    return
  }
  for (let i = 0; i < props.form.options.length; i++) {
    if (!props.form.options[i].title.trim()) {
      validationError.value = t('voting.manager.validationOptionEmpty', { index: i + 1 })
      return
    }
  }

  emit('save')
}

function calcInspectPercentage(votes: number, totalVoters: number): string {
  if (!totalVoters || totalVoters <= 0) return '0.0'
  return ((votes / totalVoters) * 100).toFixed(1)
}

function formatDate(dt?: string): string {
  if (!dt) return '-'
  try {
    return new Date(dt).toLocaleString(locale.value, {
      year: 'numeric',
      month: 'numeric',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
  } catch {
    return dt
  }
}

function getStatusText(status: PollStatus): string {
  switch (status) {
    case 'draft':
      return t('voting.board.statusDraft')
    case 'open':
      return t('voting.board.statusOpen')
    case 'closed':
      return t('voting.board.statusClosed')
    default:
      return status
  }
}

function getStatusClass(status: PollStatus): string {
  switch (status) {
    case 'draft':
      return 'bg-gray-100 text-gray-700 ring-gray-500/20 dark:bg-dark-700 dark:text-dark-300 dark:ring-dark-500/30'
    case 'open':
      return 'bg-emerald-50 text-emerald-700 ring-emerald-600/20 dark:bg-emerald-950/30 dark:text-emerald-400 dark:ring-emerald-500/30'
    case 'closed':
      return 'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-950/30 dark:text-amber-400 dark:ring-amber-500/30'
    default:
      return 'bg-gray-100 text-gray-700 ring-gray-500/20'
  }
}
</script>
