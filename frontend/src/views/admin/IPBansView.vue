<template>
  <AppLayout>
    <div class="space-y-4">
      <section class="card p-4 sm:p-5" data-testid="ip-ban-summary">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0 max-w-3xl">
            <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.ipBans.description') }}</p>
          </div>
          <div class="text-right">
            <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ipBans.summary.active') }}</div>
            <div class="text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ summary?.active_total ?? '—' }}</div>
          </div>
        </div>
        <div v-if="summary && Object.keys(summary.by_reason).length" class="mt-3 flex flex-wrap gap-2">
          <span
            v-for="(count, reason) in summary.by_reason"
            :key="reason"
            class="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-200"
          >
            {{ reasonLabel(String(reason)) }}
            <span class="font-semibold tabular-nums">{{ count }}</span>
          </span>
        </div>
        <p
          v-if="summary && !summary.trusted_proxy_configured"
          role="alert"
          class="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200"
          data-testid="ip-ban-proxy-warning"
        >
          {{ t('admin.ipBans.summary.proxyMissing') }}
        </p>
        <p
          v-else-if="summary && !summary.enabled"
          class="mt-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
        >
          {{ t('admin.ipBans.summary.autoOff') }}
        </p>
      </section>

      <div class="flex gap-1 border-b border-gray-200 dark:border-dark-700" role="tablist">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          role="tab"
          :aria-selected="activeTab === tab.key"
          :class="[
            '-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors',
            activeTab === tab.key
              ? 'border-primary-600 text-primary-700 dark:border-primary-400 dark:text-primary-300'
              : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-gray-200'
          ]"
          :data-testid="`ip-ban-tab-${tab.key}`"
          @click="switchTab(tab.key)"
        >
          {{ tab.label }}
        </button>
      </div>

      <TablePageLayout v-if="activeTab === 'list'">
        <template #filters>
          <div class="card p-4">
            <div class="flex flex-wrap items-end justify-between gap-3">
              <div class="flex flex-1 flex-wrap items-end gap-3">
                <div class="w-full sm:w-auto sm:min-w-[260px]">
                  <label class="input-label" for="ip-ban-search">{{ t('common.search') }}</label>
                  <input
                    id="ip-ban-search"
                    v-model.trim="filters.q"
                    type="text"
                    class="input"
                    :placeholder="t('admin.ipBans.filters.search')"
                    @keyup.enter="search"
                  />
                </div>
                <div class="w-full sm:w-auto sm:min-w-[140px]">
                  <label class="input-label">{{ t('admin.ipBans.filters.status') }}</label>
                  <Select v-model="filters.status" :options="statusOptions" @change="search" />
                </div>
                <div class="w-full sm:w-auto sm:min-w-[190px]">
                  <label class="input-label">{{ t('admin.ipBans.filters.reason') }}</label>
                  <Select v-model="filters.reason" :options="reasonOptions" @change="search" />
                </div>
              </div>
              <div class="flex flex-wrap gap-2">
                <button type="button" class="btn btn-secondary" :disabled="loading" @click="refreshAll">
                  {{ t('admin.ipBans.actions.refresh') }}
                </button>
                <button type="button" class="btn btn-primary" data-testid="ip-ban-add" @click="openCreate">
                  {{ t('admin.ipBans.actions.add') }}
                </button>
              </div>
            </div>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="bans" :loading="loading" row-key="id">
            <template #cell-ip="{ row }">
              <div class="font-mono text-sm text-gray-900 dark:text-white">{{ row.ip }}</div>
              <div class="mt-0.5 text-xs text-gray-400">{{ sourceLabel(row.source) }} · #{{ row.id }}</div>
            </template>
            <template #cell-reason="{ row }">
              <span class="whitespace-nowrap text-sm text-gray-700 dark:text-gray-200">{{ reasonLabel(row.reason) }}</span>
            </template>
            <template #cell-detail="{ row }">
              <div class="max-w-xs space-y-0.5 text-xs text-gray-500 dark:text-gray-400">
                <div v-for="line in detailLines(row)" :key="line" class="truncate" :title="line">{{ line }}</div>
                <div v-if="row.note" class="truncate text-gray-600 dark:text-gray-300" :title="row.note">{{ row.note }}</div>
              </div>
            </template>
            <template #cell-created_at="{ row }">
              <span class="whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">{{ formatDate(row.created_at) }}</span>
            </template>
            <template #cell-hits="{ row }">
              <div class="tabular-nums text-sm text-gray-700 dark:text-gray-200">{{ row.hits }}</div>
              <div v-if="row.last_hit_at" class="whitespace-nowrap text-xs text-gray-400">
                {{ t('admin.ipBans.detail.lastHit', { time: formatDate(row.last_hit_at) }) }}
              </div>
            </template>
            <template #cell-status="{ row }">
              <span
                v-if="!row.revoked_at"
                class="inline-flex items-center gap-1 rounded-full bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-900/30 dark:text-red-300"
              >{{ t('admin.ipBans.status.active') }}</span>
              <div v-else class="text-xs text-gray-500 dark:text-gray-400">
                <span class="inline-flex rounded-full bg-gray-100 px-2 py-0.5 font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ t('admin.ipBans.status.revoked') }}</span>
                <div class="mt-0.5 whitespace-nowrap">{{ t('admin.ipBans.status.revokedBy', { by: row.revoked_by || '—', time: formatDate(row.revoked_at) }) }}</div>
              </div>
            </template>
            <template #cell-actions="{ row }">
              <button
                v-if="!row.revoked_at"
                type="button"
                class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                :data-testid="`ip-ban-revoke-${row.id}`"
                @click="openRevoke(row)"
              >
                {{ t('admin.ipBans.actions.revoke') }}
              </button>
            </template>
            <template #empty>
              <div class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.ipBans.empty') }}</div>
            </template>
          </DataTable>
        </template>

        <template #pagination>
          <Pagination
            v-if="total > 0"
            :total="total"
            :page="page"
            :page-size="pageSize"
            @update:page="onPageChange"
            @update:pageSize="onPageSizeChange"
          />
        </template>
      </TablePageLayout>

      <section v-else class="card p-5 sm:p-6" data-testid="ip-ban-rules">
        <div v-if="settingsLoading && !settings" class="flex justify-center py-8">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>
        <form v-else-if="settings" class="max-w-3xl space-y-6" @submit.prevent="saveRules">
          <label class="flex items-start gap-3">
            <input v-model="settings.enabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300" data-testid="ip-ban-enabled" />
            <span>
              <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.ipBans.rules.enabled') }}</span>
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ipBans.rules.enabledHint') }}</span>
            </span>
          </label>

          <div>
            <label class="input-label" for="ip-ban-honeypots">{{ t('admin.ipBans.rules.honeypot') }}</label>
            <textarea id="ip-ban-honeypots" v-model="honeypotText" rows="6" class="input font-mono text-sm" spellcheck="false"></textarea>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ipBans.rules.honeypotHint') }}</p>
            <p
              v-if="settings.honeypot_conflicts?.length"
              role="alert"
              class="mt-2 text-xs text-amber-700 dark:text-amber-300"
              data-testid="ip-ban-honeypot-conflict"
            >
              {{ t('admin.ipBans.rules.honeypotConflict', { emails: settings.honeypot_conflicts.join('、') }) }}
            </p>
          </div>

          <label class="flex items-start gap-3">
            <input v-model="settings.ban_sensitive_probes" type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300" />
            <span>
              <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.ipBans.rules.probes') }}</span>
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ipBans.rules.probesHint') }}</span>
            </span>
          </label>

          <div class="space-y-3">
            <div class="flex flex-wrap items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
              <span class="min-w-[180px] font-medium">{{ t('admin.ipBans.rules.adminLogin') }}</span>
              <input v-model.number="settings.admin_login_window_minutes" type="number" min="1" max="1440" class="input w-20" />
              <span>{{ t('admin.ipBans.rules.within') }}</span>
              <input v-model.number="settings.admin_login_failures" type="number" min="0" max="1000" class="input w-20" data-testid="ip-ban-admin-login" />
              <span>{{ t('admin.ipBans.rules.times') }}</span>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
              <span class="min-w-[180px] font-medium">{{ t('admin.ipBans.rules.adminUnauthorized') }}</span>
              <input v-model.number="settings.admin_unauthorized_window_minutes" type="number" min="1" max="1440" class="input w-20" />
              <span>{{ t('admin.ipBans.rules.within') }}</span>
              <input v-model.number="settings.admin_unauthorized" type="number" min="0" max="1000" class="input w-20" />
              <span>{{ t('admin.ipBans.rules.times') }}</span>
            </div>
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ipBans.rules.zeroDisables') }}</p>
          </div>

          <div>
            <label class="input-label" for="ip-ban-allowlist">{{ t('admin.ipBans.rules.allowlist') }}</label>
            <textarea id="ip-ban-allowlist" v-model="allowlistText" rows="4" class="input font-mono text-sm" spellcheck="false"></textarea>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ipBans.rules.allowlistHint') }}</p>
          </div>

          <div class="flex justify-end">
            <button type="submit" class="btn btn-primary" :disabled="savingRules" data-testid="ip-ban-save-rules">
              {{ savingRules ? t('common.saving') : t('admin.ipBans.rules.save') }}
            </button>
          </div>
        </form>
      </section>
    </div>

    <BaseDialog :show="createVisible" :title="t('admin.ipBans.create.title')" @close="createVisible = false">
      <form id="ip-ban-create-form" class="space-y-4" @submit.prevent="submitCreate">
        <div>
          <label class="input-label" for="ip-ban-create-ip">{{ t('admin.ipBans.create.ip') }}</label>
          <input id="ip-ban-create-ip" v-model.trim="createForm.ip" type="text" class="input font-mono" autocomplete="off" required />
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ipBans.create.ipHint') }}</p>
        </div>
        <div>
          <label class="input-label" for="ip-ban-create-note">{{ t('admin.ipBans.create.note') }}</label>
          <input id="ip-ban-create-note" v-model.trim="createForm.note" type="text" maxlength="500" class="input" />
        </div>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="createVisible = false">{{ t('common.cancel') }}</button>
        <button type="submit" form="ip-ban-create-form" class="btn btn-danger" :disabled="submitting || !createForm.ip" data-testid="ip-ban-create-submit">
          {{ t('admin.ipBans.create.submit') }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="!!revokeTarget" :title="t('admin.ipBans.revoke.title')" @close="revokeTarget = null">
      <div v-if="revokeTarget" class="space-y-4">
        <p class="text-sm text-gray-700 dark:text-gray-200">{{ t('admin.ipBans.revoke.message', { ip: revokeTarget.ip }) }}</p>
        <div>
          <label class="input-label" for="ip-ban-revoke-note">{{ t('admin.ipBans.revoke.note') }}</label>
          <input id="ip-ban-revoke-note" v-model.trim="revokeNote" type="text" maxlength="500" class="input" />
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="revokeTarget = null">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary" :disabled="submitting" data-testid="ip-ban-revoke-submit" @click="submitRevoke">
          {{ t('admin.ipBans.revoke.submit') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { Column } from '@/components/common/types'
import { ipBansAPI, type IPBan, type IPBanSettings, type IPBanSummary } from '@/api/admin/ipBans'
import { useAppStore } from '@/stores'
import { formatDate } from '@/utils/format'

const { t, te } = useI18n()
const appStore = useAppStore()

type Tab = 'list' | 'rules'
const activeTab = ref<Tab>('list')
const tabs = computed(() => [
  { key: 'list' as Tab, label: t('admin.ipBans.tabs.list') },
  { key: 'rules' as Tab, label: t('admin.ipBans.tabs.rules') }
])

const summary = ref<IPBanSummary | null>(null)
const bans = ref<IPBan[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const filters = reactive({ q: '', status: 'active', reason: '' })

const reasonKeys = ['honeypot_email', 'sensitive_path_probe', 'admin_login_failures', 'admin_api_unauthorized', 'manual']

const statusOptions = computed(() => [
  { value: 'active', label: t('admin.ipBans.filters.active') },
  { value: 'revoked', label: t('admin.ipBans.filters.revoked') },
  { value: 'all', label: t('admin.ipBans.filters.all') }
])
const reasonOptions = computed(() => [
  { value: '', label: t('admin.ipBans.filters.all') },
  ...reasonKeys.map((key) => ({ value: key, label: reasonLabel(key) }))
])
const columns = computed<Column[]>(() => [
  { key: 'ip', label: t('admin.ipBans.columns.ip') },
  { key: 'reason', label: t('admin.ipBans.columns.reason') },
  { key: 'detail', label: t('admin.ipBans.columns.detail') },
  { key: 'created_at', label: t('admin.ipBans.columns.createdAt') },
  { key: 'hits', label: t('admin.ipBans.columns.hits') },
  { key: 'status', label: t('admin.ipBans.columns.status') },
  { key: 'actions', label: t('common.actions') }
])

function reasonLabel(reason: string): string {
  const key = `admin.ipBans.reasons.${reason}`
  return te(key) ? t(key) : reason
}
function sourceLabel(source: string): string {
  const key = `admin.ipBans.sources.${source}`
  return te(key) ? t(key) : source
}

function detailLines(row: IPBan): string[] {
  const d = row.detail || {}
  const lines: string[] = []
  const email = d.attempted_email
  const emails = d.attempted_emails
  if (typeof email === 'string' && email) lines.push(t('admin.ipBans.detail.email', { email }))
  else if (Array.isArray(emails) && emails.length) lines.push(t('admin.ipBans.detail.email', { email: emails.join(', ') }))
  if (typeof d.path === 'string' && d.path) lines.push(t('admin.ipBans.detail.path', { path: d.path }))
  if (d.count != null && d.window_minutes != null) lines.push(t('admin.ipBans.detail.count', { count: d.count, minutes: d.window_minutes }))
  if (row.source === 'backfill' && d.attempts != null) lines.push(t('admin.ipBans.detail.attempts', { count: d.attempts }))
  if (row.source === 'manual' && row.created_by) lines.push(t('admin.ipBans.detail.by', { by: row.created_by }))
  return lines
}

function errorMessage(err: unknown, fallback: string): string {
  const message = (err as { message?: string } | null)?.message
  return message || fallback
}

async function loadSummary() {
  try {
    summary.value = await ipBansAPI.summary()
  } catch {
    summary.value = null
  }
}

async function fetchBans() {
  loading.value = true
  try {
    const res = await ipBansAPI.list({
      page: page.value,
      page_size: pageSize.value,
      status: filters.status as 'active' | 'revoked' | 'all',
      reason: filters.reason || undefined,
      q: filters.q || undefined
    })
    bans.value = res.items
    total.value = res.total
  } catch (err) {
    appStore.showError(errorMessage(err, t('admin.ipBans.loadFailed')))
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  fetchBans()
}
function refreshAll() {
  loadSummary()
  fetchBans()
}
function onPageChange(p: number) {
  page.value = p
  fetchBans()
}
function onPageSizeChange(size: number) {
  pageSize.value = size
  page.value = 1
  fetchBans()
}

// Manual ban
const createVisible = ref(false)
const createForm = reactive({ ip: '', note: '' })
const submitting = ref(false)
function openCreate() {
  createForm.ip = ''
  createForm.note = ''
  createVisible.value = true
}
async function submitCreate() {
  if (!createForm.ip || submitting.value) return
  submitting.value = true
  try {
    const ban = await ipBansAPI.create(createForm.ip, createForm.note)
    appStore.showSuccess(t('admin.ipBans.create.success', { ip: ban.ip }))
    createVisible.value = false
    refreshAll()
  } catch (err) {
    appStore.showError(errorMessage(err, t('admin.ipBans.loadFailed')))
  } finally {
    submitting.value = false
  }
}

// Revoke
const revokeTarget = ref<IPBan | null>(null)
const revokeNote = ref('')
function openRevoke(row: IPBan) {
  revokeNote.value = ''
  revokeTarget.value = row
}
async function submitRevoke() {
  const target = revokeTarget.value
  if (!target || submitting.value) return
  submitting.value = true
  try {
    await ipBansAPI.revoke(target.id, revokeNote.value)
    appStore.showSuccess(t('admin.ipBans.revoke.success', { ip: target.ip }))
    revokeTarget.value = null
    refreshAll()
  } catch (err) {
    appStore.showError(errorMessage(err, t('admin.ipBans.loadFailed')))
  } finally {
    submitting.value = false
  }
}

// Rules
const settings = ref<IPBanSettings | null>(null)
const settingsLoading = ref(false)
const savingRules = ref(false)
const honeypotText = ref('')
const allowlistText = ref('')

function applySettings(value: IPBanSettings) {
  settings.value = { ...value }
  honeypotText.value = (value.honeypot_emails || []).join('\n')
  allowlistText.value = (value.allowlist || []).join('\n')
}
function lines(text: string): string[] {
  return text.split(/[\n,，]/).map((v) => v.trim()).filter(Boolean)
}
async function loadSettings() {
  settingsLoading.value = true
  try {
    applySettings(await ipBansAPI.getSettings())
  } catch (err) {
    appStore.showError(errorMessage(err, t('admin.ipBans.rules.loadFailed')))
  } finally {
    settingsLoading.value = false
  }
}
async function saveRules() {
  if (!settings.value || savingRules.value) return
  savingRules.value = true
  try {
    const saved = await ipBansAPI.saveSettings({
      ...settings.value,
      honeypot_emails: lines(honeypotText.value),
      allowlist: lines(allowlistText.value)
    })
    applySettings(saved)
    appStore.showSuccess(t('admin.ipBans.rules.saved'))
    loadSummary()
  } catch (err) {
    appStore.showError(errorMessage(err, t('admin.ipBans.rules.loadFailed')))
  } finally {
    savingRules.value = false
  }
}

function switchTab(tab: Tab) {
  activeTab.value = tab
  if (tab === 'rules' && !settings.value) loadSettings()
}

onMounted(() => {
  loadSummary()
  fetchBans()
})
</script>
