<template>
  <AppLayout>
    <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t(admin ? 'supportTickets.adminTitle' : 'supportTickets.userTitle') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t(admin ? 'supportTickets.adminDescription' : 'supportTickets.userDescription') }}
        </p>
      </div>
      <router-link v-if="!admin" to="/tickets/new" class="btn btn-primary">
        <Icon name="plus" size="md" class="mr-1" />
        {{ t('supportTickets.actions.new') }}
      </router-link>
    </div>

    <TablePageLayout>
      <template #filters>
        <form class="flex flex-wrap items-end gap-3" data-test="ticket-filters" @submit.prevent="applyFilters">
          <div class="min-w-52 flex-1">
            <Input
              id="ticket-title-filter"
              v-model="filters.title"
              :label="t('supportTickets.columns.title')"
              :placeholder="t('supportTickets.filters.title')"
              data-test="ticket-title-filter"
            />
          </div>
          <div v-if="admin" class="min-w-52 flex-1">
            <Input
              id="ticket-user-filter"
              v-model="filters.user_search"
              :label="t('supportTickets.columns.user')"
              :placeholder="t('supportTickets.filters.user')"
              data-test="ticket-user-filter"
            />
          </div>
          <div class="w-44">
            <label class="input-label mb-1.5 block">{{ t('supportTickets.columns.category') }}</label>
            <Select
              id="ticket-category-filter"
              v-model="filters.category"
              :aria-label="t('supportTickets.columns.category')"
              :options="categoryFilterOptions"
              data-test="ticket-category-filter"
            />
          </div>
          <div class="w-44">
            <label class="input-label mb-1.5 block">{{ t('supportTickets.columns.status') }}</label>
            <Select
              id="ticket-status-filter"
              v-model="filters.status"
              :aria-label="t('supportTickets.columns.status')"
              :options="statusFilterOptions"
              data-test="ticket-status-filter"
            />
          </div>
          <div class="w-44">
            <label class="input-label mb-1.5 block">{{ t('supportTickets.columns.priority') }}</label>
            <Select
              id="ticket-priority-filter"
              v-model="filters.priority"
              :aria-label="t('supportTickets.columns.priority')"
              :options="priorityFilterOptions"
              data-test="ticket-priority-filter"
            />
          </div>
          <button type="submit" class="btn btn-primary" data-test="apply-ticket-filters">
            {{ t('supportTickets.actions.search') }}
          </button>
          <button type="button" class="btn btn-secondary" @click="resetFilters">
            {{ t('supportTickets.actions.reset') }}
          </button>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="loading"
            :title="t('supportTickets.actions.refresh')"
            data-test="refresh-tickets"
            @click="refreshAll"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </form>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="tickets"
          :loading="loading"
          row-key="id"
          clickable-rows
          @row-click="openTicket"
        >
          <template #cell-title="{ row }">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span
                  v-if="row.unread"
                  class="h-2 w-2 flex-none rounded-full bg-primary-500"
                  :aria-label="t('supportTickets.unreadBadge', { count: 1 })"
                  data-test="ticket-unread"
                ></span>
                <span class="max-w-80 truncate font-medium text-gray-900 dark:text-white">
                  {{ row.title }}
                </span>
              </div>
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">#{{ row.id }}</span>
            </div>
          </template>

          <template v-if="admin" #cell-user="{ row }">
            <div class="max-w-56">
              <div class="truncate font-medium">{{ row.user?.username || `#${row.user_id}` }}</div>
              <div v-if="row.user?.email" class="truncate text-xs text-gray-500 dark:text-gray-400">
                {{ row.user.email }}
              </div>
            </div>
          </template>

          <template #cell-category="{ value }">
            {{ ticketLabel('category', value) }}
          </template>
          <template #cell-priority="{ value }">
            <span class="badge" :class="priorityClass(value)">{{ ticketLabel('priority', value) }}</span>
          </template>
          <template #cell-status="{ value }">
            <span class="badge" :class="statusClass(value)">{{ ticketLabel('status', value) }}</span>
          </template>
          <template #cell-updated_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ formatDateTimeToMinute(value) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <button type="button" class="btn btn-ghost btn-sm" @click.stop="openTicket(row)">
              {{ t('supportTickets.actions.view') }}
            </button>
          </template>
          <template #empty>
            <EmptyState
              :title="t('supportTickets.empty.title')"
              :description="t(admin ? 'supportTickets.empty.adminDescription' : 'supportTickets.empty.description')"
              :action-text="admin ? undefined : t('supportTickets.actions.new')"
              :action-to="admin ? undefined : '/tickets/new'"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :page-size="pagination.page_size"
          :total="pagination.total"
          @update:page="changePage"
          @update:page-size="changePageSize"
        />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore, useAuthStore, useSupportTicketStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import { isSupportTicketFeatureDisabled, supportTicketsAdminAPI, supportTicketsUserAPI } from './api'
import {
  supportTicketCategories,
  supportTicketPriorities,
  supportTicketStatuses,
  type SupportTicket,
  type SupportTicketCategory,
  type SupportTicketPriority,
  type SupportTicketStatus,
} from './types'

const props = withDefaults(defineProps<{ admin?: boolean }>(), { admin: false })
const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const unreadStore = useSupportTicketStore()

const tickets = ref<SupportTicket[]>([])
const loading = ref(false)
const filters = reactive<{
  title: string
  user_search: string
  category: SupportTicketCategory | ''
  status: SupportTicketStatus | ''
  priority: SupportTicketPriority | ''
}>({ title: '', user_search: '', category: '', status: '', priority: '' })
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 1 })
let loadSequence = 0

const columns = computed<Column[]>(() => [
  { key: 'title', label: t('supportTickets.columns.title') },
  ...(props.admin ? [{ key: 'user', label: t('supportTickets.columns.user') }] : []),
  { key: 'category', label: t('supportTickets.columns.category') },
  { key: 'priority', label: t('supportTickets.columns.priority') },
  { key: 'status', label: t('supportTickets.columns.status') },
  { key: 'updated_at', label: t('supportTickets.columns.updatedAt') },
  { key: 'actions', label: t('supportTickets.columns.actions') },
])

const categoryFilterOptions = computed(() => [
  { value: '', label: t('supportTickets.filters.allCategories') },
  ...supportTicketCategories.map((value) => ({ value, label: ticketLabel('category', value) })),
])
const statusFilterOptions = computed(() => [
  { value: '', label: t('supportTickets.filters.allStatuses') },
  ...supportTicketStatuses.map((value) => ({ value, label: ticketLabel('status', value) })),
])
const priorityFilterOptions = computed(() => [
  { value: '', label: t('supportTickets.filters.allPriorities') },
  ...supportTicketPriorities.map((value) => ({ value, label: ticketLabel('priority', value) })),
])

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

async function redirectDisabled(error: unknown): Promise<boolean> {
  if (props.admin || !isSupportTicketFeatureDisabled(error)) return false
  await appStore.fetchPublicSettings(true).catch(() => undefined)
  appStore.showWarning(t('supportTickets.featureDisabled'))
  await router.replace(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
  return true
}

async function loadTickets(): Promise<void> {
  const sequence = ++loadSequence
  loading.value = true
  try {
    const api = props.admin ? supportTicketsAdminAPI : supportTicketsUserAPI
    const page = await api.list({
      page: pagination.page,
      page_size: pagination.page_size,
      title: filters.title,
      user_search: props.admin ? filters.user_search : undefined,
      category: filters.category || undefined,
      status: filters.status || undefined,
      priority: filters.priority || undefined,
    })
    if (sequence !== loadSequence) return
    tickets.value = page.items
    Object.assign(pagination, page)
  } catch (error) {
    if (sequence !== loadSequence || await redirectDisabled(error)) return
    appStore.showError(extractApiErrorMessage(error, t('supportTickets.errors.load')))
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

async function refreshUnread(): Promise<void> {
  try {
    if (props.admin) await unreadStore.refreshAdminUnread()
    else await unreadStore.refreshUserUnread()
  } catch (error) {
    await redirectDisabled(error)
  }
}

async function refreshAll(): Promise<void> {
  await Promise.all([loadTickets(), refreshUnread()])
}

function applyFilters(): void {
  pagination.page = 1
  void loadTickets()
}

function resetFilters(): void {
  Object.assign(filters, { title: '', user_search: '', category: '', status: '', priority: '' })
  pagination.page = 1
  void loadTickets()
}

function changePage(page: number): void {
  pagination.page = page
  void loadTickets()
}

function changePageSize(pageSize: number): void {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadTickets()
}

function openTicket(ticket: SupportTicket): void {
  void router.push(props.admin ? `/admin/tickets/${ticket.id}` : `/tickets/${ticket.id}`)
}

onMounted(loadTickets)
</script>
