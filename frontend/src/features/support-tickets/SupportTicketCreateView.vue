<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl">
      <router-link to="/tickets" class="mb-4 inline-flex items-center text-sm text-primary-600 hover:underline dark:text-primary-400">
        ← {{ t('supportTickets.actions.back') }}
      </router-link>
      <div class="card p-5 sm:p-7">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('supportTickets.newTitle') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('supportTickets.newDescription') }}</p>

        <form class="mt-6 space-y-5" data-test="create-ticket-form" @submit.prevent="submit">
          <div>
            <Input
              id="ticket-title"
              v-model="form.title"
              :label="t('supportTickets.form.title')"
              :placeholder="t('supportTickets.form.titlePlaceholder')"
              :error="titleError"
              required
            />
            <p v-if="!titleError" class="mt-1 text-right text-xs text-gray-500 dark:text-gray-400">
              {{ t('supportTickets.form.characters', { count: titleCount, max: SUPPORT_TICKET_TITLE_MAX }) }}
            </p>
          </div>

          <div class="grid gap-5 sm:grid-cols-2">
            <div>
              <label for="ticket-category" class="input-label mb-1.5 block">{{ t('supportTickets.form.category') }}</label>
              <Select id="ticket-category" v-model="form.category" :aria-label="t('supportTickets.form.category')" :options="categoryOptions" />
            </div>
            <div>
              <label for="ticket-priority" class="input-label mb-1.5 block">{{ t('supportTickets.form.priority') }}</label>
              <Select id="ticket-priority" v-model="form.priority" :aria-label="t('supportTickets.form.priority')" :options="priorityOptions" />
            </div>
          </div>

          <div>
            <TextArea
              id="ticket-content"
              v-model="form.content"
              :label="t('supportTickets.form.content')"
              :placeholder="t('supportTickets.form.contentPlaceholder')"
              :error="contentError"
              rows="8"
              required
            />
            <p v-if="!contentError" class="mt-1 text-right text-xs text-gray-500 dark:text-gray-400">
              {{ t('supportTickets.form.characters', { count: contentCount, max: SUPPORT_TICKET_CONTENT_MAX }) }}
            </p>
          </div>

          <SupportTicketImagePicker v-model="form.images" :disabled="submitting" />

          <div class="flex justify-end gap-3 border-t border-gray-100 pt-5 dark:border-dark-700">
            <button type="button" class="btn btn-secondary" :disabled="submitting" @click="resetForm">
              {{ t('supportTickets.actions.reset') }}
            </button>
            <button type="submit" class="btn btn-primary" :disabled="submitting" data-test="submit-ticket">
              <span v-if="submitting" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
              {{ t('supportTickets.actions.submit') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Input from '@/components/common/Input.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import { useAppStore, useAuthStore, useSupportTicketStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import SupportTicketImagePicker from './SupportTicketImagePicker.vue'
import { isSupportTicketFeatureDisabled, supportTicketsUserAPI } from './api'
import {
  SUPPORT_TICKET_CONTENT_MAX,
  SUPPORT_TICKET_TITLE_MAX,
  supportTicketCategories,
  supportTicketCharacterCount,
  supportTicketPriorities,
  supportTicketTextIsValid,
  type SupportTicketCategory,
  type SupportTicketPriority,
} from './types'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const unreadStore = useSupportTicketStore()
const submitting = ref(false)
const titleError = ref('')
const contentError = ref('')
const form = reactive<{
  title: string
  category: SupportTicketCategory
  priority: SupportTicketPriority
  content: string
  images: File[]
}>({
  title: '',
  category: 'account',
  priority: 'normal',
  content: '',
  images: [],
})

const titleCount = computed(() => supportTicketCharacterCount(form.title))
const contentCount = computed(() => supportTicketCharacterCount(form.content))
const categoryOptions = computed(() => supportTicketCategories.map((value) => ({
  value,
  label: t(`supportTickets.category.${value}`),
})))
const priorityOptions = computed(() => supportTicketPriorities.map((value) => ({
  value,
  label: t(`supportTickets.priority.${value}`),
})))

function validate(): boolean {
  titleError.value = supportTicketTextIsValid(form.title, SUPPORT_TICKET_TITLE_MAX)
    ? ''
    : t('supportTickets.errors.title')
  contentError.value = supportTicketTextIsValid(form.content, SUPPORT_TICKET_CONTENT_MAX)
    ? ''
    : t('supportTickets.errors.content')
  return !titleError.value && !contentError.value
}

function resetForm(): void {
  Object.assign(form, {
    title: '',
    category: 'account',
    priority: 'normal',
    content: '',
    images: [],
  })
  titleError.value = ''
  contentError.value = ''
}

async function redirectDisabled(error: unknown): Promise<boolean> {
  if (!isSupportTicketFeatureDisabled(error)) return false
  await appStore.fetchPublicSettings(true).catch(() => undefined)
  appStore.showWarning(t('supportTickets.featureDisabled'))
  await router.replace(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
  return true
}

async function submit(): Promise<void> {
  if (!validate()) return
  submitting.value = true
  try {
    const ticket = await supportTicketsUserAPI.create({ ...form })
    form.images = []
    appStore.showSuccess(t('supportTickets.success.created'))
    await unreadStore.refreshUserUnread().catch(() => undefined)
    await router.push(`/tickets/${ticket.id}`)
  } catch (error) {
    if (await redirectDisabled(error)) return
    appStore.showError(extractApiErrorMessage(error, t('supportTickets.errors.create')))
  } finally {
    submitting.value = false
  }
}
</script>
