<template>
  <AdminAnnouncementEditor
    v-model="content"
    :loading="loading"
    :disabled="!loaded"
    :saving="saving"
    :error="error"
    :success="success"
    :dirty="loaded && content !== savedContent"
    @save="save"
    @reset="reset"
    @retry="retry"
  />
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import { extractApiErrorMessage } from '@/utils/apiError'
import AdminAnnouncementEditor from './AdminAnnouncementEditor.vue'

const { t } = useI18n()
const content = ref('')
const savedContent = ref('')
const loaded = ref(false)
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const success = ref('')
async function load() {
  if (saving.value) return
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    const value = await adminGroupBuyAPI.announcement()
    content.value = savedContent.value = value.content
    loaded.value = true
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('groupBuy.announcementLoadFailed'))
  } finally {
    loading.value = false
  }
}
async function save() {
  if (!loaded.value || loading.value || saving.value || content.value === savedContent.value) return
  if (Array.from(content.value).length > 5000) {
    error.value = t('groupBuy.announcementTooLong')
    return
  }
  saving.value = true
  error.value = ''
  success.value = ''
  try {
    const value = await adminGroupBuyAPI.saveAnnouncement({ content: content.value })
    content.value = savedContent.value = value.content
    success.value = t('groupBuy.announcementSaved')
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('groupBuy.announcementSaveFailed'))
  } finally {
    saving.value = false
  }
}
function reset() {
  if (loading.value || saving.value) return
  content.value = ''
  success.value = ''
  error.value = ''
}
function refresh() {
  // Refresh must not discard an unsaved announcement.
  if (loading.value || saving.value) return
  if (loaded.value && content.value !== savedContent.value) {
    error.value = t('groupBuy.announcementUnsaved')
    return
  }
  return load()
}
function retry() {
  return loaded.value && content.value !== savedContent.value ? save() : refresh()
}
defineExpose({ refresh })
onMounted(load)
</script>
