import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import { keysAPI } from '@/api/keys'
import { userGroupsAPI } from '@/api/groups'
import { modelKeyAccessAPI, type ModelAccessKey } from '@/api/modelKeyAccess'
import { tutorialAPI, type TutorialCatalog } from '@/api/tutorial'
import type { ApiKey, ApiKeyModelOptions, Group, GroupPlatform } from '@/types'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { useKeyConfig } from '@/composables/useKeyConfig'
import { safeTutorialEndpoint, tutorialClientModelAllowed, tutorialKeyUsable, tutorialModels, tutorialShareURL } from '@/utils/tutorial'

export function useTutorialCenter() {
  const route = useRoute()
  const { t } = useI18n()
  const app = useAppStore()
  const { copyToClipboard } = useClipboard()
  const query = (name: string) => typeof route.query[name] === 'string' ? route.query[name] as string : ''
  const initial = { platform: query('platform'), group: Number(query('group')), model: query('model'), client: query('client'), os: query('os'), route: query('route'), key: Number(query('key_id')) }
  const initialModelGroup = initial.group
  const loading = ref(true), keyLoading = ref(false), modelsLoading = ref(false)
  const error = ref(''), keyError = ref(''), modelsError = ref('')
  const catalog = ref<TutorialCatalog>({ revision: 0, articles: [], routes: [] })
  const available = ref<Group[]>([])
  const keys = ref<ModelAccessKey[]>([])
  const selectedKeyId = ref<number | null>(null)
  const selectedKey = ref<ApiKey | null>(null)
  const revealKey = ref(false)
  const platform = ref<GroupPlatform | ''>('')
  const groupID = ref<number | null>(null), model = ref(''), routeID = ref('default')
  const options = ref<ApiKeyModelOptions>({ models: [] })
  const fileIndex = ref(0)
  const copied = ref('')
  let alive = true, loadGeneration = 0, keyGeneration = 0, modelGeneration = 0
  const boundGroups = computed(() => {
    if (!selectedKeyId.value) return available.value
    if (!selectedKey.value || keyLoading.value || keyError.value || !tutorialKeyUsable(selectedKey.value)) return []
    const ids = selectedKey.value.group_ids ?? [selectedKey.value.group_id]
    return available.value.filter(g => ids.includes(g.id))
  })
  const platforms = computed(() => [...new Set(boundGroups.value.map(g => g.platform))])
  const groups = computed(() => boundGroups.value.filter(g => g.platform === platform.value))
  const group = computed(() => groups.value.find(g => g.id === groupID.value))
  const models = computed(() => group.value && !keyLoading.value && !keyError.value
    ? tutorialModels(options.value, group.value.id, selectedKey.value).filter(id => tutorialClientModelAllowed(id, config.activeClientTab.value, platform.value)) : [])
  const routes = computed(() => {
    const configured = app.cachedPublicSettings?.api_base_url?.replace(/\/+$/, '') || window.location.origin
    const defaults = safeTutorialEndpoint(configured) ? [{ id: 'default', title: t('tutorial.defaultRoute'), url: configured }] : []
    return [...defaults, ...catalog.value.routes.filter(r => !r.platform || r.platform === platform.value).sort((a,b) => a.order-b.order)]
  })
  const endpoint = computed(() => routes.value.find(r => r.id === routeID.value)?.url || '')
  const config = useKeyConfig(reactive({
    show: true,
    get apiKey() { return revealKey.value && selectedKey.value ? selectedKey.value.key : 'YOUR_API_KEY' },
    get baseUrl() { return endpoint.value },
    get platform() { return group.value?.platform || null },
    get claudeCodeOnly() { return group.value?.claude_code_only },
    get allowMessagesDispatch() { return group.value?.allow_messages_dispatch },
    get model() { return model.value }
  }))
  const ready = computed(() => !loading.value && !error.value && !keyLoading.value && !keyError.value && !modelsLoading.value && !modelsError.value && !!group.value && !!endpoint.value && models.value.includes(model.value))
  const files = computed(() => ready.value ? config.currentFiles.value : [])
  const file = computed(() => files.value[fileIndex.value] || files.value[0])
  const articles = computed(() => catalog.value.articles.filter(a => (!a.platform || a.platform === platform.value) && (!a.client || a.client === config.activeClientTab.value)).sort((a,b) => a.order-b.order))
  const guides = computed(() => articles.value.filter(a => a.kind === 'tutorial'))
  const faqs = computed(() => articles.value.filter(a => a.kind === 'faq'))
  watch(platforms, value => { if (!value.includes(platform.value as GroupPlatform)) platform.value = value[0] || '' })
  watch(groups, value => { if (!value.some(g => g.id === groupID.value)) groupID.value = value[0]?.id || null })
  watch(routes, value => { if (!value.some(r => r.id === routeID.value)) routeID.value = value[0]?.id || '' })
  watch(models, value => { if (!value.includes(model.value)) model.value = value[0] || '' })
  watch([() => config.activeClientTab.value, () => config.activeTab.value, groupID], () => { fileIndex.value = 0 })
  watch([file, endpoint], () => { copied.value = '' })
  watch(groupID, () => { void loadModels() })
  watch(selectedKeyId, () => { void loadKey() })

  async function loadModels() {
    const request = ++modelGeneration
    options.value = { models: [] }; modelsError.value = ''; modelsLoading.value = false
    const id = groupID.value
    if (!id) return
    modelsLoading.value = true
    try {
      const result = await keysAPI.getModelOptions([id])
      if (!alive || request !== modelGeneration) return
      options.value = result
      await nextTick()
      if (initial.model && (!initialModelGroup || id === initialModelGroup) && (!selectedKeyId.value || selectedKey.value)) {
        if (models.value.includes(initial.model)) model.value = initial.model
        initial.model = ''
      }
    } catch {
      if (alive && request === modelGeneration) modelsError.value = t('tutorial.modelsFailed')
    } finally {
      if (alive && request === modelGeneration) modelsLoading.value = false
    }
  }
  async function loadKey() {
    const request = ++keyGeneration
    selectedKey.value = null; revealKey.value = false; keyError.value = ''; keyLoading.value = false
    const id = selectedKeyId.value
    if (!id) return
    keyLoading.value = true
    try {
      const key = await keysAPI.getById(id)
      if (!alive || request !== keyGeneration) return
      if (!tutorialKeyUsable(key)) { keyError.value = t('tutorial.keyUnavailable'); return }
      selectedKey.value = key
    } catch {
      if (alive && request === keyGeneration) keyError.value = t('tutorial.keyFailed')
    } finally {
      if (alive && request === keyGeneration) {
        keyLoading.value = false
        await nextTick()
        await restoreSelection()
      }
    }
  }
  async function restoreSelection() {
    if (platforms.value.includes(initial.platform as GroupPlatform)) platform.value = initial.platform as GroupPlatform
    await nextTick()
    if (groups.value.some(g => g.id === initial.group)) groupID.value = initial.group
    await nextTick()
    if (config.clientTabs.value.some(c => c.id === initial.client)) config.activeClientTab.value = initial.client
    await nextTick()
    if (config.currentTabs.value.some(s => s.id === initial.os)) config.activeTab.value = initial.os
    if (routes.value.some(r => r.id === initial.route)) routeID.value = initial.route
    initial.platform = ''; initial.group = 0; initial.client = ''; initial.os = ''; initial.route = ''
  }
  async function load() {
    const request = ++loadGeneration
    loading.value = true; error.value = ''
    try {
      const [content, groups, snapshot] = await Promise.all([tutorialAPI.get(), userGroupsAPI.getAvailable(), modelKeyAccessAPI.list()])
      if (!alive || request !== loadGeneration) return
      catalog.value = content; available.value = groups; keys.value = snapshot.keys
      await nextTick()
      if (initial.key) {
        if (keys.value.some(k => k.id === initial.key)) selectedKeyId.value = initial.key
        else keyError.value = t('tutorial.keyFailed')
        initial.key = 0
      } else await restoreSelection()
    } catch (cause) {
      if (alive && request === loadGeneration) error.value = (cause as { message?: string })?.message || t('tutorial.loadFailed')
    } finally {
      if (alive && request === loadGeneration) loading.value = false
    }
  }
  async function copy(content: string, id: string) {
    if (await copyToClipboard(content, t('tutorial.copied'))) copied.value = id
  }
  async function share() {
    await copy(tutorialShareURL(window.location.origin, { platform: platform.value, group: groupID.value, model: model.value, client: config.activeClientTab.value, os: config.activeTab.value, route: routeID.value }), 'share')
  }
  function download() {
    if (!file.value) return
    const names: Record<string,string> = { Terminal: 'environment.sh', PowerShell: 'environment.ps1', 'Command Prompt': 'environment.cmd' }
    const name = names[file.value.path] || file.value.path.split(/[/\\]/).pop()!.replace(/ \(.*\)$/, '')
    saveAs(new Blob([file.value.content], { type: 'text/plain;charset=utf-8' }), name)
  }
  onMounted(load)
  onBeforeUnmount(() => { alive = false; loadGeneration++; keyGeneration++; modelGeneration++; selectedKey.value = null })
  return { loading, error, load, keys, selectedKeyId, selectedKey, revealKey, keyLoading, keyError, loadKey, platforms, platform, groups, groupID, group, models, model, modelsLoading, modelsError, loadModels, routes, routeID, endpoint, config, files, file, fileIndex, guides, faqs, copy, copied, share, download, ready }
}
