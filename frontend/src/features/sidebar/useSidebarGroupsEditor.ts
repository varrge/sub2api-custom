import { computed, onMounted, ref, toValue, type MaybeRefOrGetter } from 'vue'
import { useI18n } from 'vue-i18n'
import { getSidebarGroups, updateSidebarGroups } from '@/api/admin/settings'
import { useAppStore } from '@/stores/app'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import type { CustomMenuItem, SidebarGroup, SidebarGroupsConfig } from '@/types'
import { sidebarPageOptions } from './sidebarGroups'

const copy = (config: SidebarGroupsConfig): SidebarGroupsConfig => ({
  groups: config.groups.map(group => ({ ...group, items: [...group.items] })),
})

export function useSidebarGroupsEditor(customMenus: MaybeRefOrGetter<CustomMenuItem[]>) {
  const { t, locale } = useI18n()
  const app = useAppStore()
  const admin = useAdminSettingsStore()
  const draft = ref<SidebarGroupsConfig>({ groups: [] })
  const loading = ref(true)
  const saving = ref(false)
  const error = ref('')
  let loaded = false
  const zh = computed(() => locale.value.startsWith('zh'))
  const options = computed(() => ({
    user: sidebarPageOptions('user', toValue(customMenus), t),
    admin: sidebarPageOptions('admin', toValue(customMenus), t),
  }))

  async function load() {
    if (saving.value) return
    loading.value = true
    loaded = false
    error.value = ''
    try {
      draft.value = copy(await getSidebarGroups())
      loaded = true
    } catch {
      error.value = zh.value ? '分组加载失败，请重试。' : 'Could not load groups. Please retry.'
    } finally {
      loading.value = false
    }
  }

  async function save(): Promise<boolean> {
    if (!loaded || loading.value || saving.value) return false
    const draftAtStart = JSON.stringify(draft.value)
    const snapshot = copy(draft.value)
    const assigned = { user: new Set<string>(), admin: new Set<string>() }
    for (const group of snapshot.groups) {
      group.label = group.label.trim()
      if (!group.label || [...group.label].length > 64) {
        error.value = zh.value ? '分组名称需为 1–64 个字。' : 'Group names must contain 1–64 characters.'
        return false
      }
      for (const path of group.items) {
        if (assigned[group.visibility].has(path)) {
          error.value = zh.value ? '同一页面只能放入一个分组。' : 'Each page can belong to only one group.'
          return false
        }
        assigned[group.visibility].add(path)
      }
    }
    error.value = ''
    saving.value = true
    try {
      const saved = await updateSidebarGroups(snapshot)
      const hasNewEdits = JSON.stringify(draft.value) !== draftAtStart
      if (!hasNewEdits) draft.value = copy(saved)
      admin.sidebarGroups = copy(saved)
      // Update the current sidebar immediately without waiting for an unrelated
      // settings request. Public data must never retain the admin-only groups.
      const publicConfig = { groups: saved.groups.filter(group => group.visibility === 'user') }
      if (app.cachedPublicSettings) {
        app.cachedPublicSettings = { ...app.cachedPublicSettings, sidebar_groups: copy(publicConfig) }
      }
      if (window.__APP_CONFIG__) {
        window.__APP_CONFIG__ = { ...window.__APP_CONFIG__, sidebar_groups: copy(publicConfig) }
      }
      app.showSuccess(hasNewEdits
        ? (zh.value ? '本次保存完成，后续修改尚未保存。' : 'Saved. Your newer edits are still unsaved.')
        : (zh.value ? '侧栏分组已保存' : 'Sidebar groups saved'))
      return true
    } catch {
      error.value = zh.value ? '保存失败，修改已保留，请重试。' : 'Save failed. Your changes are retained; please retry.'
      return false
    } finally {
      saving.value = false
    }
  }

  function addGroup() {
    if (draft.value.groups.length >= 32) {
      error.value = zh.value ? '最多可创建 32 个分组。' : 'You can create up to 32 groups.'
      return
    }
    draft.value.groups.push({
      id: `group_${crypto.randomUUID()}`, label: '', visibility: 'user', items: [],
    })
  }
  function removeGroup(id: string) {
    draft.value.groups = draft.value.groups.filter(group => group.id !== id)
  }
  function move<T>(items: T[], index: number, delta: number) {
    const target = index + delta
    if (index < 0 || target < 0 || target >= items.length) return
    const [item] = items.splice(index, 1)
    items.splice(target, 0, item)
  }
  function moveGroup(id: string, delta: number) {
    move(draft.value.groups, draft.value.groups.findIndex(group => group.id === id), delta)
  }
  function availableItems(group: SidebarGroup) {
    const assigned = new Set(draft.value.groups.filter(g => g.visibility === group.visibility).flatMap(g => g.items))
    return options.value[group.visibility].filter(item => !assigned.has(item.path))
  }
  function addItem(id: string, path: string) {
    const group = draft.value.groups.find(g => g.id === id)
    if (group && availableItems(group).some(item => item.path === path)) group.items.push(path)
  }
  function removeItem(id: string, path: string) {
    const group = draft.value.groups.find(g => g.id === id)
    if (group) group.items = group.items.filter(item => item !== path)
  }
  function moveItem(id: string, path: string, delta: number) {
    const group = draft.value.groups.find(g => g.id === id)
    if (group) move(group.items, group.items.indexOf(path), delta)
  }
  function itemLabel(path: string): string {
    return [...options.value.user, ...options.value.admin].find(item => item.path === path)?.label ?? path
  }

  onMounted(load)
  return { draft, loading, saving, error, load, save, addGroup, removeGroup, moveGroup,
    availableItems, addItem, removeItem, moveItem, itemLabel }
}
