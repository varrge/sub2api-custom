import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { modelKeyAccessAPI, type ModelAccessGroup, type ModelAccessKey, type ModelAccessRow } from '@/api/modelKeyAccess'
import { keysAPI } from '@/api/keys'
import { userGroupsAPI } from '@/api/groups'
import { useAppStore } from '@/stores/app'
import type { ApiKeyModelAllowlist } from '@/types'

export function modelPolicyAllows(policy: ApiKeyModelAllowlist, model: string): boolean {
  if (!policy?.enabled) return true
  if (policy.mode && policy.mode !== 'allow' && policy.mode !== 'deny') return false
  const listed = (policy.models ?? []).some(id => id.trim() === model.trim())
  return policy.mode === 'deny' ? !listed : listed
}

export function validModelAccessID(value: string): boolean {
  const model = value.trim()
  // Match the backend's concrete, case-sensitive public-ID validation.
  return model.length > 0 && Array.from(model).length <= 256 &&
    !Array.from(value).some(char => {
      const code = char.codePointAt(0)!
      return code <= 31 || (code >= 127 && code <= 159)
    }) && !/[*?\[\]]/.test(model)
}

export function useModelKeyAccess() {
  const { t } = useI18n()
  const app = useAppStore()
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  const catalogWarning = ref(false)
  const selectedModel = ref('')
  const modelSearch = ref('')
  const keySearch = ref('')
  const groupFilter = ref<number | null>(null)
  const keys = ref<ModelAccessKey[]>([])
  const draft = ref<Record<number, boolean>>({})
  const catalog = ref<Map<string, Set<number>>>(new Map())
  const loadedGroupIDs = ref<Set<number>>(new Set())
  const customModels = ref<string[]>([])
  const switchPending = ref(false)
  let pendingAction: (() => void | Promise<void>) | null = null
  let pendingCancel: (() => void) | null = null
  let alive = true
  let generation = 0
  let controller: AbortController | null = null

  const groups = computed(() => {
    const byID = new Map<number, ModelAccessGroup>()
    for (const key of keys.value) for (const group of key.groups) byID.set(group.id, group)
    return [...byID.values()].sort((a, b) => a.name.localeCompare(b.name))
  })
  const allModels = computed(() => [...new Set([
    ...catalog.value.keys(), ...customModels.value,
    ...keys.value.flatMap(key => key.model_allowlist.models ?? [])
  ])].sort())
  const models = computed(() => allModels.value.filter(model => model.toLowerCase().includes(modelSearch.value.trim().toLowerCase())))

  const allRows = computed<ModelAccessRow[]>(() => keys.value.map(key => {
    const sourceGroups = catalog.value.get(selectedModel.value)
    const listed = key.group_ids.some(id => sourceGroups?.has(id))
    const known = key.group_ids.length > 0 && key.group_ids.every(id => loadedGroupIDs.value.has(id))
    const originalAllowed = modelPolicyAllows(key.model_allowlist, selectedModel.value)
    return {
      id: key.id, name: key.name, status: key.status, expires_at: key.expires_at,
      groups: key.groups, group_ids: key.group_ids,
      allowed: draft.value[key.id] ?? originalAllowed, originalAllowed,
      catalogState: listed ? 'listed' : known ? 'unlisted' : 'unknown'
    }
  }))
  const rows = computed(() => {
    const search = keySearch.value.trim().toLowerCase()
    return allRows.value.filter(row => (!search || row.name.toLowerCase().includes(search) || String(row.id).includes(search)) &&
      (groupFilter.value === null || row.group_ids.includes(groupFilter.value)))
  })
  const totalCount = computed(() => keys.value.length)
  const allowedCount = computed(() => selectedModel.value ? allRows.value.filter(row => row.allowed).length : 0)
  const addedCount = computed(() => allRows.value.filter(row => row.allowed && !row.originalAllowed).length)
  const removedCount = computed(() => allRows.value.filter(row => !row.allowed && row.originalAllowed).length)
  const dirty = computed(() => !!selectedModel.value && addedCount.value + removedCount.value > 0)

  function resetDraft() {
    draft.value = Object.fromEntries(keys.value.map(key => [key.id, modelPolicyAllows(key.model_allowlist, selectedModel.value)]))
  }

  async function load() {
    if (saving.value) return
    const request = ++generation
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    error.value = ''
    const [snapshot, available] = await Promise.allSettled([
      modelKeyAccessAPI.list(controller.signal), userGroupsAPI.getAvailable()
    ])
    if (!alive || request !== generation) return
    if (snapshot.status === 'rejected') {
      error.value = t('modelKeyAccess.loadFailed')
      loading.value = false
      return
    }
    keys.value = snapshot.value.keys
    resetDraft()
    catalog.value = new Map()
    loadedGroupIDs.value = new Set()
    catalogWarning.value = available.status === 'rejected'
    if (available.status === 'fulfilled') {
      const ids = available.value.map(group => group.id)
      const chunks: number[][] = []
      for (let offset = 0; offset < ids.length; offset += 100) chunks.push(ids.slice(offset, offset + 100))
      const results = await Promise.allSettled(chunks.map(chunk => keysAPI.getModelOptions(chunk)))
      if (!alive || request !== generation) return
      results.forEach((result, index) => {
        if (result.status === 'rejected') {
          catalogWarning.value = true
          return
        }
        chunks[index].forEach(id => loadedGroupIDs.value.add(id))
        for (const model of result.value.models) {
          const sources = catalog.value.get(model.id) ?? new Set<number>()
          model.group_ids.forEach(id => sources.add(id))
          catalog.value.set(model.id, sources)
        }
      })
    }
    loading.value = false
  }

  function requestAction(action: () => void | Promise<void>, cancel?: () => void) {
    if (saving.value || loading.value || switchPending.value) {
      cancel?.()
      return
    }
    if (!dirty.value) {
      void action()
      return
    }
    pendingAction = action
    pendingCancel = cancel ?? null
    switchPending.value = true
  }

  function selectModel(value: string) {
    if (!validModelAccessID(value)) {
      error.value = t('modelKeyAccess.invalidModel')
      return
    }
    const model = value.trim()
    if (model === selectedModel.value) return
    requestAction(() => {
      selectedModel.value = model
      if (!allModels.value.includes(model)) customModels.value.push(model)
      resetDraft()
      error.value = ''
    })
  }

  function toggleKey(id: number, allowed: boolean) {
    if (!selectedModel.value || saving.value || loading.value || switchPending.value || !keys.value.some(key => key.id === id)) return
    draft.value[id] = allowed
  }

  function setVisible(allowed: boolean) {
    if (!selectedModel.value || saving.value || loading.value || switchPending.value) return
    draft.value = { ...draft.value, ...Object.fromEntries(rows.value.map(row => [row.id, allowed])) }
  }

  function discard() {
    if (saving.value || loading.value) return
    resetDraft()
    error.value = ''
  }

  async function save(): Promise<boolean> {
    if (saving.value || loading.value || !selectedModel.value) return false
    if (!dirty.value) return true
    saving.value = true
    error.value = ''
    const revisions = new Map(keys.value.map(key => [key.id, key.revision]))
    try {
      // Submit every snapshotted key, including hidden rows. The backend checks
      // all policy revisions under row locks before committing the batch.
      const result = await modelKeyAccessAPI.update(selectedModel.value, allRows.value.map(row => ({
        id: row.id, allowed: row.allowed, revision: revisions.get(row.id)!
      })))
      if (!alive) return false
      const committed = new Map(result.keys.map(key => [key.id, key]))
      keys.value = keys.value.map(key => {
        const next = committed.get(key.id)
        return next ? { ...key, model_allowlist: next.model_allowlist, revision: next.revision } : key
      })
      resetDraft()
      app.showSuccess(t('modelKeyAccess.saved'))
      return true
    } catch (cause) {
      if (alive) {
        const failure = cause as { status?: number; reason?: string; response?: { status?: number; data?: { reason?: string } } }
        error.value = t(failure.status === 409 || failure.response?.status === 409 ||
          failure.reason === 'API_KEY_MODEL_ACCESS_CONFLICT' || failure.response?.data?.reason === 'API_KEY_MODEL_ACCESS_CONFLICT'
          ? 'modelKeyAccess.conflict' : 'modelKeyAccess.saveFailed')
      }
      return false
    } finally {
      if (alive) saving.value = false
    }
  }

  async function resolveSwitch(action: 'save' | 'discard' | 'cancel') {
    if (saving.value || !switchPending.value) return
    if (action === 'save' && !(await save())) return
    const proceed = pendingAction
    const cancel = pendingCancel
    pendingAction = null
    pendingCancel = null
    switchPending.value = false
    if (action === 'cancel') {
      cancel?.()
      return
    }
    if (action === 'discard') discard()
    await proceed?.()
  }

  function reload() { requestAction(load) }

  onBeforeRouteLeave(() => {
    if (saving.value) return false
    if (!dirty.value) return true
    return new Promise<boolean>(resolve => requestAction(() => { resolve(true) }, () => { resolve(false) }))
  })
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (dirty.value || saving.value) {
      event.preventDefault()
      event.returnValue = ''
    }
  }
  onMounted(() => {
    window.addEventListener('beforeunload', beforeUnload)
    void load()
  })
  onBeforeUnmount(() => {
    alive = false
    generation++
    controller?.abort()
    pendingCancel?.()
    window.removeEventListener('beforeunload', beforeUnload)
  })

  return {
    loading, saving, error, catalogWarning, selectedModel, modelSearch, keySearch, groupFilter,
    models, rows, groups, totalCount, allowedCount, addedCount, removedCount, dirty, switchPending,
    selectModel, toggleKey, setVisible, save, discard, reload, resolveSwitch
  }
}
