<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex items-center gap-2">
        <Icon name="menu" size="md" class="text-primary-500" />
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ zh ? '侧栏分组' : 'Sidebar Groups' }}
        </h2>
      </div>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{
          zh
            ? '将侧栏页面收进自定义分组，分别控制用户端与管理端的展示与顺序；未分组的页面保持原位显示。'
            : 'Organize sidebar pages into custom groups with separate user/admin visibility and ordering. Ungrouped pages stay where they are.'
        }}
      </p>
    </div>

    <div class="space-y-5 p-4 sm:p-6">
      <!-- Loading -->
      <div v-if="loading" class="flex items-center gap-2 text-gray-500" data-testid="sidebar-groups-loading">
        <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
        {{ zh ? '加载中…' : 'Loading…' }}
      </div>

      <!-- Load failure: retry, never expose saveable defaults -->
      <div
        v-else-if="loadFailed"
        role="alert"
        class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300"
        data-testid="sidebar-groups-load-error"
      >
        <span>{{ error }}</span>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          data-testid="sidebar-groups-retry"
          @click="load"
        >
          {{ zh ? '重试' : 'Retry' }}
        </button>
      </div>

      <template v-else>
        <!-- Save / validation error keeps the draft visible -->
        <div
          v-if="error"
          role="alert"
          class="rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300"
          data-testid="sidebar-groups-error"
        >
          {{ error }}
        </div>

        <!-- Empty state -->
        <div
          v-if="draft.groups.length === 0"
          class="rounded-lg border-2 border-dashed border-amber-300/70 bg-amber-50/40 p-6 text-center dark:border-amber-500/30 dark:bg-amber-500/5"
          data-testid="sidebar-groups-empty"
        >
          <p class="text-sm text-gray-600 dark:text-gray-300">
            {{ zh ? '还没有侧栏分组。创建分组后，可以把常用页面收纳到一起。' : 'No sidebar groups yet. Create one to tuck related pages together.' }}
          </p>
          <button
            type="button"
            class="btn btn-primary btn-sm mt-3"
            data-testid="sidebar-groups-add-empty"
            @click="addGroup"
          >
            {{ zh ? '新建分组' : 'New group' }}
          </button>
        </div>

        <!-- Group cards -->
        <div
          v-for="(group, groupIndex) in draft.groups"
          :key="group.id"
          class="rounded-lg border border-amber-200/70 bg-white dark:border-amber-500/20 dark:bg-dark-800"
          data-testid="sidebar-group-card"
        >
          <!-- Group header: order, name, visibility, delete -->
          <div
            class="flex flex-wrap items-center gap-3 border-b border-amber-100 bg-gradient-to-r from-amber-50/80 to-transparent px-4 py-3 dark:border-amber-500/10 dark:from-amber-500/10"
          >
            <span class="text-sm font-medium text-amber-800 dark:text-amber-300">
              {{ zh ? `分组 ${groupIndex + 1}` : `Group ${groupIndex + 1}` }}
            </span>
            <input
              v-model="group.label"
              type="text"
              maxlength="64"
              class="input min-w-0 flex-1 text-sm"
              :placeholder="zh ? '分组名称' : 'Group name'"
              :aria-label="zh ? `分组 ${groupIndex + 1} 名称` : `Group ${groupIndex + 1} name`"
              data-testid="sidebar-group-label"
              @keydown.enter.prevent
            />
            <select
              :value="group.visibility"
              :aria-label="zh ? `分组 ${groupIndex + 1} 显示范围` : `Group ${groupIndex + 1} visibility`"
              class="input w-auto text-sm"
              data-testid="sidebar-group-visibility"
              @change="changeVisibility(group, ($event.target as HTMLSelectElement).value)"
            >
              <option value="user">{{ zh ? '用户端可见' : 'User sidebar' }}</option>
              <option value="admin">{{ zh ? '管理端可见' : 'Admin sidebar' }}</option>
            </select>
            <div class="flex items-center gap-1">
              <button
                type="button"
                class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-700"
                :disabled="groupIndex === 0"
                :title="zh ? '上移' : 'Move up'"
                data-testid="sidebar-group-move-up"
                @click="moveGroup(group.id, -1)"
              >
                <Icon name="chevronUp" size="sm" />
              </button>
              <button
                type="button"
                class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-700"
                :disabled="groupIndex === draft.groups.length - 1"
                :title="zh ? '下移' : 'Move down'"
                data-testid="sidebar-group-move-down"
                @click="moveGroup(group.id, 1)"
              >
                <Icon name="chevronDown" size="sm" />
              </button>
              <button
                type="button"
                class="rounded p-1 text-red-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                :title="zh ? '删除分组' : 'Delete group'"
                data-testid="sidebar-group-remove"
                @click="removeGroup(group.id)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>

          <!-- Group items -->
          <div class="space-y-2 p-4">
            <p
              v-if="group.items.length === 0"
              class="text-xs text-gray-400 dark:text-gray-500"
              data-testid="sidebar-group-items-empty"
            >
              {{ zh ? '尚未选择页面，从下方添加。' : 'No pages selected yet. Add one below.' }}
            </p>
            <div
              v-for="(path, itemIndex) in group.items"
              :key="path"
              class="flex items-center gap-2 rounded-md bg-gray-50 px-3 py-1.5 dark:bg-dark-700/60"
              data-testid="sidebar-group-item"
            >
              <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-200">
                {{ itemLabel(path) }}
              </span>
              <button
                type="button"
                class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-600"
                :disabled="itemIndex === 0"
                :title="zh ? '上移' : 'Move up'"
                data-testid="sidebar-group-item-move-up"
                @click="moveItem(group.id, path, -1)"
              >
                <Icon name="chevronUp" size="xs" />
              </button>
              <button
                type="button"
                class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-600"
                :disabled="itemIndex === group.items.length - 1"
                :title="zh ? '下移' : 'Move down'"
                data-testid="sidebar-group-item-move-down"
                @click="moveItem(group.id, path, 1)"
              >
                <Icon name="chevronDown" size="xs" />
              </button>
              <button
                type="button"
                class="rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20"
                :title="zh ? '移出分组' : 'Remove from group'"
                data-testid="sidebar-group-item-remove"
                @click="removeItem(group.id, path)"
              >
                <Icon name="x" size="xs" />
              </button>
            </div>

            <!-- Add page picker -->
            <select
              class="input w-full text-sm"
              data-testid="sidebar-group-item-add"
              :aria-label="zh ? `为分组 ${groupIndex + 1} 添加页面` : `Add page to group ${groupIndex + 1}`"
              :disabled="availableItems(group).length === 0"
              @change="pickItem(group, $event)"
            >
              <option value="" selected>
                {{
                  availableItems(group).length === 0
                    ? zh
                      ? '没有可添加的页面'
                      : 'No pages available'
                    : zh
                      ? '添加页面到分组…'
                      : 'Add a page to this group…'
                }}
              </option>
              <option
                v-for="option in availableItems(group)"
                :key="option.path"
                :value="option.path"
              >
                {{ option.label }}
              </option>
            </select>
          </div>
        </div>

        <!-- Footer actions -->
        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="draft.groups.length >= 32"
            data-testid="sidebar-groups-add"
            @click="addGroup"
          >
            <Icon name="plus" size="sm" />
            {{ zh ? '新建分组' : 'New group' }}
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="saving"
            data-testid="sidebar-groups-save"
            @click="save"
          >
            <svg
              v-if="saving"
              class="mr-1 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ saving ? (zh ? '保存中…' : 'Saving…') : (zh ? '保存分组' : 'Save groups') }}
          </button>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useSidebarGroupsEditor } from '@/features/sidebar/useSidebarGroupsEditor'
import type { CustomMenuItem, SidebarGroup } from '@/types'

const props = withDefaults(defineProps<{
  customMenuItems?: CustomMenuItem[]
}>(), {
  customMenuItems: () => [],
})

const { locale } = useI18n()
const zh = computed(() => locale.value.startsWith('zh'))

const {
  draft,
  loading,
  saving,
  error,
  load,
  save,
  addGroup,
  removeGroup,
  moveGroup,
  availableItems,
  addItem,
  removeItem,
  moveItem,
  itemLabel,
} = useSidebarGroupsEditor(() => props.customMenuItems)

// A failed load must not render saveable defaults; a failed save keeps the
// draft. loading only toggles during load cycles, never during save.
const loadFailed = ref(false)
watch(loading, (value, previous) => {
  if (previous && !value) loadFailed.value = !!error.value
})

// Changing scope clears the items so user pages never leak into admin groups.
function changeVisibility(group: SidebarGroup, visibility: string) {
  if (visibility !== 'user' && visibility !== 'admin') return
  if (group.visibility === visibility) return
  group.visibility = visibility
  group.items = []
}

function pickItem(group: SidebarGroup, event: Event) {
  const select = event.target as HTMLSelectElement
  if (select.value) addItem(group.id, select.value)
  select.value = ''
}
</script>
