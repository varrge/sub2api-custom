<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-5">
      <router-link :to="listPath" class="inline-flex items-center text-sm text-primary-600 hover:underline dark:text-primary-400">
        ← {{ t('supportTickets.actions.back') }}
      </router-link>

      <div v-if="loading && !ticket" class="card flex justify-center py-16">
        <span class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></span>
      </div>

      <template v-else-if="ticket">
        <section class="card p-5 sm:p-6">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('supportTickets.detailTitle', { id: ticket.id }) }}
              </p>
              <h1 class="mt-1 break-words text-2xl font-bold text-gray-900 dark:text-white">{{ ticket.title }}</h1>
              <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
                {{ ticketLabel('category', ticket.category) }} ·
                {{ t('supportTickets.detail.createdAt', { date: formatDateTimeToMinute(ticket.created_at) }) }}
              </p>
              <p v-if="admin && ticket.user" class="mt-2 text-sm text-gray-700 dark:text-gray-300">
                {{ ticket.user.email
                  ? t('supportTickets.detail.userIdentity', { username: ticket.user.username, email: ticket.user.email })
                  : ticket.user.username }}
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <span class="badge" :class="priorityClass(ticket.priority)">{{ ticketLabel('priority', ticket.priority) }}</span>
              <span class="badge" :class="statusClass(ticket.status)">{{ ticketLabel('status', ticket.status) }}</span>
            </div>
          </div>

          <div v-if="admin" class="mt-5 flex flex-wrap items-end gap-3 border-t border-gray-100 pt-5 dark:border-dark-700">
            <div class="w-44">
              <label for="ticket-priority" class="input-label mb-1.5 block">{{ t('supportTickets.detail.priorityLabel') }}</label>
              <Select
                id="ticket-priority"
                :model-value="ticket.priority"
                :aria-label="t('supportTickets.detail.priorityLabel')"
                :options="priorityOptions"
                :disabled="updating"
                data-test="ticket-priority"
                @change="changePriority"
              />
            </div>
            <button
              v-for="status in statusTransitions"
              :key="status"
              type="button"
              class="btn btn-secondary"
              :disabled="updating"
              :data-test="`ticket-status-${status}`"
              @click="changeStatus(status)"
            >
              {{ t('supportTickets.actions.changeStatus', { status: ticketLabel('status', status) }) }}
            </button>
          </div>
        </section>

        <section class="card p-5 sm:p-6">
          <div class="mb-5 flex items-center justify-between">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('supportTickets.detail.conversation') }}</h2>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="loading"
              data-test="refresh-ticket-detail"
              @click="loadDetail"
            >
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>

          <div class="space-y-4" data-test="ticket-conversation">
            <article
              v-for="message in ticket.messages || []"
              :key="message.id"
              class="rounded-2xl border p-4"
              :class="message.author_role === 'admin'
                ? 'border-primary-200 bg-primary-50/50 dark:border-primary-900 dark:bg-primary-900/10'
                : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800'"
            >
              <div class="mb-3 flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
                <span class="font-semibold text-gray-700 dark:text-gray-300">
                  {{ message.author?.username || t(message.author_role === 'admin' ? 'supportTickets.detail.support' : 'supportTickets.detail.user') }}
                </span>
                <time :datetime="message.created_at">{{ formatDateTimeToMinute(message.created_at) }}</time>
              </div>
              <p class="whitespace-pre-wrap break-words text-sm leading-6 text-gray-800 dark:text-gray-200">{{ message.content }}</p>
              <div v-if="message.attachments.length" class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3">
                <a
                  v-for="attachment in message.attachments"
                  :key="attachment.id"
                  :href="attachmentURLs[attachment.id]"
                  :class="[
                    'flex aspect-square items-center justify-center overflow-hidden rounded-xl border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-900',
                    attachmentURLs[attachment.id] ? 'cursor-zoom-in' : 'pointer-events-none',
                  ]"
                  target="_blank"
                  rel="noopener"
                >
                  <img
                    v-if="attachmentURLs[attachment.id]"
                    :src="attachmentURLs[attachment.id]"
                    :alt="t('supportTickets.form.images')"
                    class="h-full w-full object-contain"
                  />
                  <span v-else class="px-2 text-center text-xs text-gray-400">
                    {{ t('supportTickets.detail.attachmentUnavailable') }}
                  </span>
                </a>
              </div>
            </article>
          </div>
        </section>

        <section class="card p-5 sm:p-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('supportTickets.detail.reply') }}</h2>
          <p v-if="ticket.status === 'closed'" class="mt-2 rounded-lg bg-gray-100 p-3 text-sm text-gray-600 dark:bg-dark-700 dark:text-gray-300">
            {{ t('supportTickets.detail.closedHint') }}
          </p>
          <form class="mt-4 space-y-4" data-test="ticket-reply-form" @submit.prevent="reply">
            <div>
              <TextArea
                id="ticket-reply-content"
                v-model="replyForm.content"
                :placeholder="t('supportTickets.form.contentPlaceholder')"
                :error="contentError"
                :disabled="ticket.status === 'closed' || replying"
                rows="6"
              />
              <p v-if="!contentError" class="mt-1 text-right text-xs text-gray-500 dark:text-gray-400">
                {{ t('supportTickets.form.characters', { count: contentCount, max: SUPPORT_TICKET_CONTENT_MAX }) }}
              </p>
            </div>
            <SupportTicketImagePicker
              v-model="replyForm.images"
              :disabled="ticket.status === 'closed' || replying"
            />
            <div class="flex justify-end">
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="ticket.status === 'closed' || replying"
                data-test="submit-ticket-reply"
              >
                {{ t('supportTickets.actions.reply') }}
              </button>
            </div>
          </form>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore, useSupportTicketStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import SupportTicketImagePicker from './SupportTicketImagePicker.vue'
import { isSupportTicketFeatureDisabled, supportTicketsAdminAPI, supportTicketsUserAPI } from './api'
import {
  SUPPORT_TICKET_CONTENT_MAX,
  supportTicketCharacterCount,
  supportTicketPriorities,
  supportTicketTextIsValid,
  type SupportTicket,
  type SupportTicketPriority,
  type SupportTicketStatus,
} from './types'

const props = withDefaults(defineProps<{ admin?: boolean }>(), { admin: false })
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const unreadStore = useSupportTicketStore()
const ticket = ref<SupportTicket | null>(null)
const loading = ref(false)
const replying = ref(false)
const updating = ref(false)
const contentError = ref('')
const attachmentURLs = reactive<Record<number, string>>({})
const replyForm = reactive({ content: '', images: [] as File[] })
let loadSequence = 0

const listPath = computed(() => props.admin ? '/admin/tickets' : '/tickets')
const contentCount = computed(() => supportTicketCharacterCount(replyForm.content))
const priorityOptions = computed(() => supportTicketPriorities.map((value) => ({
  value,
  label: ticketLabel('priority', value),
})))
const statusTransitions = computed<SupportTicketStatus[]>(() => {
  if (ticket.value?.status === 'pending') return ['in_progress', 'closed']
  if (ticket.value?.status === 'in_progress') return ['closed']
  if (ticket.value?.status === 'closed') return ['in_progress']
  return []
})

function ticketLabel(group: 'category' | 'priority' | 'status', value: string): string {
  return t(`supportTickets.${group}.${value}`)
}

function priorityClass(priority: string): string {
  if (priority === 'urgent') return 'badge-danger'
  if (priority === 'high') return 'badge-warning'
  if (priority === 'low') return 'badge-gray'
  return 'badge-info'
}

function statusClass(status: string): string {
  if (status === 'closed') return 'badge-gray'
  if (status === 'in_progress') return 'badge-info'
  return 'badge-warning'
}

function revokeAttachmentURLs(): void {
  for (const url of Object.values(attachmentURLs)) URL.revokeObjectURL(url)
  for (const key of Object.keys(attachmentURLs)) delete attachmentURLs[Number(key)]
}

async function redirectDisabled(error: unknown): Promise<boolean> {
  if (props.admin || !isSupportTicketFeatureDisabled(error)) return false
  await appStore.fetchPublicSettings(true).catch(() => undefined)
  appStore.showWarning(t('supportTickets.featureDisabled'))
  await router.replace(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
  return true
}

async function refreshUnread(): Promise<void> {
  if (props.admin) await unreadStore.refreshAdminUnread()
  else await unreadStore.refreshUserUnread()
}

async function loadAttachments(detail: SupportTicket, sequence: number): Promise<void> {
  const api = props.admin ? supportTicketsAdminAPI : supportTicketsUserAPI
  const attachments = (detail.messages || []).flatMap((message) => message.attachments)
  const loaded = await Promise.all(attachments.map(async (attachment) => {
    try {
      const blob = await api.attachment(detail.id, attachment.id)
      return [attachment.id, URL.createObjectURL(blob)] as const
    } catch {
      return null
    }
  }))
  if (sequence !== loadSequence) {
    for (const entry of loaded) if (entry) URL.revokeObjectURL(entry[1])
    return
  }
  for (const entry of loaded) if (entry) attachmentURLs[entry[0]] = entry[1]
}

async function loadDetail(): Promise<void> {
  const sequence = ++loadSequence
  revokeAttachmentURLs()
  const id = Number(route.params.id)
  if (!Number.isInteger(id) || id <= 0) {
    await router.replace('/404')
    return
  }
  loading.value = true
  try {
    const api = props.admin ? supportTicketsAdminAPI : supportTicketsUserAPI
    const detail = await api.get(id)
    if (sequence !== loadSequence) return
    ticket.value = detail

    try {
      await api.markRead(id)
      if (sequence !== loadSequence) return
      ticket.value.unread = false
      await refreshUnread()
    } catch (error) {
      if (sequence !== loadSequence) return
      if (await redirectDisabled(error)) return
      appStore.showError(extractApiErrorMessage(error, t('supportTickets.errors.loadDetail')))
    }

    if (sequence === loadSequence) await loadAttachments(detail, sequence)
  } catch (error) {
    if (sequence !== loadSequence || await redirectDisabled(error)) return
    if ((error as { status?: number })?.status === 404) {
      await router.replace('/404')
      return
    }
    appStore.showError(extractApiErrorMessage(error, t('supportTickets.errors.loadDetail')))
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

async function reply(): Promise<void> {
  if (!ticket.value || ticket.value.status === 'closed') return
  contentError.value = supportTicketTextIsValid(replyForm.content, SUPPORT_TICKET_CONTENT_MAX)
    ? ''
    : t('supportTickets.errors.content')
  if (contentError.value) return

  replying.value = true
  const sequence = loadSequence
  try {
    const api = props.admin ? supportTicketsAdminAPI : supportTicketsUserAPI
    await api.reply(ticket.value.id, {
      content: replyForm.content,
      images: replyForm.images,
    })
    if (sequence !== loadSequence) return
    replyForm.content = ''
    replyForm.images = []
    contentError.value = ''
    appStore.showSuccess(t('supportTickets.success.replied'))
    await loadDetail()
  } catch (error) {
    if (sequence !== loadSequence) return
    if (await redirectDisabled(error)) return
    appStore.showError(extractApiErrorMessage(error, t('supportTickets.errors.reply')))
  } finally {
    replying.value = false
  }
}

async function changeStatus(status: SupportTicketStatus): Promise<void> {
  if (!props.admin || !ticket.value || !statusTransitions.value.includes(status)) return
  updating.value = true
  const sequence = loadSequence
  try {
    const updated = await supportTicketsAdminAPI.updateStatus(ticket.value.id, status)
    if (sequence !== loadSequence || !ticket.value) return
    ticket.value = { ...ticket.value, ...updated }
    await unreadStore.refreshAdminUnread().catch(() => undefined)
    appStore.showSuccess(t('supportTickets.success.statusUpdated'))
  } catch (error) {
    if (sequence !== loadSequence) return
    appStore.showError(extractApiErrorMessage(error, t('supportTickets.errors.update')))
  } finally {
    updating.value = false
  }
}

async function changePriority(value: string | number | boolean | null, _option: SelectOption | null): Promise<void> {
  if (!props.admin || !ticket.value || !supportTicketPriorities.includes(value as SupportTicketPriority)) return
  const priority = value as SupportTicketPriority
  if (priority === ticket.value.priority) return
  updating.value = true
  const sequence = loadSequence
  try {
    const updated = await supportTicketsAdminAPI.updatePriority(ticket.value.id, priority)
    if (sequence !== loadSequence || !ticket.value) return
    ticket.value = { ...ticket.value, ...updated }
    await unreadStore.refreshAdminUnread().catch(() => undefined)
    appStore.showSuccess(t('supportTickets.success.priorityUpdated'))
  } catch (error) {
    if (sequence !== loadSequence) return
    appStore.showError(extractApiErrorMessage(error, t('supportTickets.errors.update')))
  } finally {
    updating.value = false
  }
}

watch(
  [() => route.params.id, () => props.admin],
  () => {
    ticket.value = null
    replyForm.content = ''
    replyForm.images = []
    contentError.value = ''
    void loadDetail()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  loadSequence++
  revokeAttachmentURLs()
})
</script>
