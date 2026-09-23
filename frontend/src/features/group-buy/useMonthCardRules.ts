import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { monthCardRulesAPI } from '@/api/monthCardRules'
import type { AdminMonthCardRules, MonthCardRuleDraft, MonthCardRules, PublishedMonthCardRule } from '@/types/monthCardRules'
import { extractApiErrorMessage } from '@/utils/apiError'

export function useMonthCardRules() {
  const { t } = useI18n()
  const state = ref<MonthCardRules | null>(null)
  const loading = ref(false)
  const error = ref('')
  const accepted = ref(false)
  const readingId = ref('')
  let generation = 0
  const documents = computed(() => state.value?.documents ?? [])
  const ready = computed(() => !loading.value && !error.value && documents.value.length > 0 && documents.value.every(d => !!d.read_at))
  const canPay = computed(() => ready.value && accepted.value)
  async function load() {
    const request = ++generation
    accepted.value = false
    loading.value = true
    error.value = ''
    try {
      const result = await monthCardRulesAPI.get()
      if (request === generation) state.value = result
    } catch (err) {
      if (request === generation) { state.value = null; error.value = extractApiErrorMessage(err, t('groupBuy.loadFailed')) }
    } finally { if (request === generation) loading.value = false }
  }
  async function read(document: PublishedMonthCardRule) {
    if (readingId.value || loading.value) return
    const request = generation
    readingId.value = document.id
    error.value = ''
    try {
      await monthCardRulesAPI.read(document.id, document.version)
      const result = await monthCardRulesAPI.get()
      if (request === generation) {
        if (state.value?.publication !== result.publication) accepted.value = false
        state.value = result
      }
    } catch (err) {
      if (request === generation) { accepted.value = false; error.value = extractApiErrorMessage(err, t('groupBuy.loadFailed')) }
    } finally { readingId.value = '' }
  }
  function setAccepted(value: boolean) { accepted.value = value && ready.value }
  function reset() { generation++; state.value = null; accepted.value = false; loading.value = false; error.value = '' }
  return { state, documents, loading, error, accepted, readingId, ready, canPay, load, read, setAccepted, reset }
}

export function useAdminMonthCardRules() {
  const { t } = useI18n()
  const state = ref<AdminMonthCardRules | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  async function load() {
    if (loading.value || saving.value) return
    loading.value = true; error.value = ''
    try { state.value = await monthCardRulesAPI.admin() }
    catch (err) { error.value = extractApiErrorMessage(err, t('groupBuy.loadFailed')) }
    finally { loading.value = false }
  }
  async function update(action: () => Promise<AdminMonthCardRules>) {
    if (!state.value || loading.value || saving.value) return false
    saving.value = true; error.value = ''
    try { state.value = await action(); return true }
    catch (err) { error.value = extractApiErrorMessage(err, t('groupBuy.loadFailed')); return false }
    finally { saving.value = false }
  }
  const save = (documents: MonthCardRuleDraft[]) => update(() => monthCardRulesAPI.save(state.value!.draft_revision, documents))
  const publish = () => update(() => monthCardRulesAPI.publish(state.value!.draft_revision))
  return { state, loading, saving, error, load, save, publish }
}
