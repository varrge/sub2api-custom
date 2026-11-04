<template>
  <AppLayout>
    <VotingManager
      v-model:form="form"
      :polls="polls" :poll="poll" :loading="loading" :detail-loading="detailLoading"
      :saving="saving" :error="error" :page="page" :pages="pages"
      :form-open="formOpen" :editing-id="editingId" :form-vote-count="formVoteCount"
      @create="create" @edit="edit" @inspect="inspect" @save="save"
      @close-form="closeForm" @delete="askDelete" @page-change="changePage" @refresh="load"
    />
    <ConfirmDialog
      :show="!!deleteTarget" :title="t('votingEntry.deleteTitle')"
      :message="t('votingEntry.deleteConfirm', { title: deleteTarget?.title || '' })" danger
      @confirm="remove" @cancel="deleteTarget = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import VotingManager from '@/components/voting/VotingManager.vue'
import { adminVotingAPI, type Poll, type PollInput, type PollSummary } from '@/api/voting'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const app = useAppStore()
const polls = ref<PollSummary[]>([])
const poll = ref<Poll | null>(null)
const loading = ref(false)
const detailLoading = ref(false)
const saving = ref(false)
const error = ref('')
const page = ref(1)
const pages = ref(0)
const formOpen = ref(false)
const editingId = ref<number | null>(null)
const formVoteCount = ref(0)
const deleteTarget = ref<PollSummary | null>(null)
const blank = (): PollInput => ({ title: '', description: '', image: '', status: 'draft', revision: 0, options: [{ title: '', image: '' }, { title: '', image: '' }] })
const form = ref<PollInput>(blank())
let listController: AbortController | undefined
let detailController: AbortController | undefined
let disposed = false

function message(e: unknown) { return (e as { message?: string })?.message || t('votingEntry.failed') }
async function load() {
  if (disposed) return
  listController?.abort()
  const controller = listController = new AbortController()
  loading.value = true
  error.value = ''
  try {
    const result = await adminVotingAPI.list(page.value, controller.signal)
    if (controller.signal.aborted) return
    polls.value = result.items
    pages.value = result.pages
    if (!result.items.length && page.value > 1) { page.value--; await load() }
  } catch (e) {
    if (!controller.signal.aborted) error.value = message(e)
  } finally {
    if (!controller.signal.aborted) loading.value = false
  }
}
async function fetchPoll(id: number): Promise<Poll | null> {
  detailController?.abort()
  const controller = detailController = new AbortController()
  detailLoading.value = true
  poll.value = null
  error.value = ''
  try {
    const result = await adminVotingAPI.get(id, controller.signal)
    if (controller.signal.aborted) return null
    poll.value = result
    return result
  } catch (e) {
    if (!controller.signal.aborted) error.value = message(e)
    return null
  } finally {
    if (!controller.signal.aborted) detailLoading.value = false
  }
}
function create() {
  if (saving.value) return
  detailController?.abort()
  detailLoading.value = false
  error.value = ''
  editingId.value = null
  formVoteCount.value = 0
  form.value = blank()
  formOpen.value = true
}
async function edit(id: number) {
  if (saving.value) return
  formOpen.value = false
  const result = await fetchPoll(id)
  if (!result) return
  editingId.value = id
  formVoteCount.value = result.voter_count
  form.value = { title: result.title, description: result.description, image: result.image, status: result.status, revision: result.revision, options: result.options.map(o => ({ title: o.title, image: o.image })) }
  formOpen.value = true
}
async function inspect(id: number) {
  if (saving.value) return
  formOpen.value = false
  await fetchPoll(id)
}
function closeForm() { if (!saving.value) formOpen.value = false }
async function save() {
  if (saving.value || !formOpen.value) return
  const input = form.value
  const titles = input.options.map(o => o.title.trim())
  if (!input.title.trim() || [...input.title.trim()].length > 200 || [...input.description.trim()].length > 2000 ||
      titles.length < 2 || titles.length > 12 || titles.some(s => !s || [...s].length > 200) || new Set(titles).size !== titles.length) {
    error.value = t('votingEntry.invalid')
    return
  }
  saving.value = true
  error.value = ''
  try {
    const result = editingId.value ? await adminVotingAPI.update(editingId.value, input) : await adminVotingAPI.create(input)
    formOpen.value = false
    app.showSuccess(t('votingEntry.saved'))
    if (!editingId.value) page.value = 1
    await load()
    if (!disposed) await fetchPoll(result.id)
  } catch (e) { error.value = message(e) }
  finally { saving.value = false }
}
function askDelete(id: number) {
  if (saving.value) return
  const target = polls.value.find(p => p.id === id)
  if (target && target.voter_count === 0) deleteTarget.value = target
}
async function remove() {
  const target = deleteTarget.value
  if (!target || saving.value) return
  detailController?.abort()
  detailLoading.value = false
  if (editingId.value === target.id) formOpen.value = false
  deleteTarget.value = null
  saving.value = true
  try {
    await adminVotingAPI.delete(target.id, target.revision)
    if (poll.value?.id === target.id) poll.value = null
    app.showSuccess(t('votingEntry.deleted'))
    await load()
  } catch (e) { error.value = message(e) }
  finally { saving.value = false }
}
function changePage(value: number) {
  if (saving.value || value < 1 || value > pages.value) return
  page.value = value
  void load()
}
onMounted(load)
onBeforeUnmount(() => { disposed = true; listController?.abort(); detailController?.abort() })
</script>
