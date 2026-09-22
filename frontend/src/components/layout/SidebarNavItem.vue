<template>
  <!-- Collapsible group (has children); recursion supports a native group nested inside a custom group -->
  <template v-if="item.children?.length">
    <button
      type="button"
      class="sidebar-link w-full"
      :class="[
        depth === 0 ? 'mb-1' : 'mb-0.5 py-1.5 text-sm',
        {
          'sidebar-link-active': isGroupActive(item) && !isExpanded(item),
          'sidebar-link-collapsed': collapsed
        }
      ]"
      :title="collapsed ? item.label : undefined"
      :aria-expanded="!collapsed && isExpanded(item)"
      @click="$emit('group-click', item)"
    >
      <span v-if="item.iconSvg" class="flex-shrink-0 sidebar-svg-icon" :class="iconSizeClass" v-html="sanitizeSvg(item.iconSvg)"></span>
      <component v-else :is="item.icon" class="flex-shrink-0" :class="iconSizeClass" />
      <span
        class="sidebar-label sidebar-label-flex"
        :class="{ 'sidebar-label-collapsed': collapsed }"
        :aria-hidden="collapsed ? 'true' : 'false'"
      >
        <span class="min-w-0 truncate">{{ item.label }}</span>
        <svg
          class="h-4 w-4 flex-shrink-0 transition-transform duration-200"
          :class="isExpanded(item) ? 'rotate-180' : ''"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="1.5"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
        </svg>
      </span>
    </button>
    <div v-if="!collapsed && isExpanded(item)" class="mb-1 ml-4 border-l border-gray-200 pl-2 dark:border-dark-600">
      <SidebarNavItem
        v-for="child in item.children"
        :key="child.path"
        :item="child"
        :collapsed="collapsed"
        :current-path="currentPath"
        :is-expanded="isExpanded"
        :is-group-active="isGroupActive"
        :is-active="isActive"
        :depth="depth + 1"
        @group-click="$emit('group-click', $event)"
        @menu-click="$emit('menu-click', $event)"
      />
    </div>
  </template>

  <!-- Normal item (no children) -->
  <router-link
    v-else
    :to="item.path"
    class="sidebar-link"
    :class="[
      depth === 0 ? 'mb-1' : 'mb-0.5 py-1.5 text-sm',
      {
        'sidebar-link-active': item.exact ? currentPath === item.path : isActive(item.path),
        'sidebar-link-collapsed': collapsed
      }
    ]"
    :title="collapsed ? item.label : undefined"
    :id="anchorId(item.path)"
    :data-tour="item.path === '/keys' ? 'sidebar-my-keys' : undefined"
    @click="$emit('menu-click', item.path)"
  >
    <span v-if="item.iconSvg" class="flex-shrink-0 sidebar-svg-icon" :class="iconSizeClass" v-html="sanitizeSvg(item.iconSvg)"></span>
    <component v-else :is="item.icon" class="flex-shrink-0" :class="iconSizeClass" />
    <span
      class="sidebar-label flex min-w-0 items-center gap-2"
      :class="{ 'sidebar-label-collapsed': collapsed }"
      :aria-hidden="collapsed ? 'true' : 'false'"
    >
      <span class="min-w-0 truncate">{{ item.label }}</span>
      <span v-if="item.badge?.()" class="ml-auto rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white">{{ item.badge() }}</span>
    </span>
  </router-link>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { sanitizeSvg } from '@/utils/sanitize'

export interface SidebarNavEntry {
  path: string
  label: string
  icon?: unknown
  iconSvg?: string
  children?: SidebarNavEntry[]
  expandOnly?: boolean
  exact?: boolean
  badge?: () => number
}

const props = withDefaults(defineProps<{
  item: SidebarNavEntry
  collapsed: boolean
  currentPath: string
  isExpanded: (item: SidebarNavEntry) => boolean
  isGroupActive: (item: SidebarNavEntry) => boolean
  isActive: (path: string) => boolean
  depth?: number
}>(), {
  depth: 0,
})

defineEmits<{
  'group-click': [item: SidebarNavEntry]
  'menu-click': [path: string]
}>()

const iconSizeClass = computed(() => (props.depth === 0 ? 'h-5 w-5' : 'h-4 w-4'))

// Onboarding anchors historically rendered on top-level admin links; kept here
// so the tour still finds them when the renderer moves into this component.
function anchorId(path: string): string | undefined {
  switch (path) {
    case '/admin/accounts':
      return 'sidebar-channel-manage'
    case '/admin/groups':
      return 'sidebar-group-manage'
    case '/admin/redeem':
      return 'sidebar-wallet'
    default:
      return undefined
  }
}
</script>

<style scoped>
.sidebar-link-collapsed {
  gap: 0;
  padding-left: 0.875rem;
  padding-right: 0.875rem;
}

.sidebar-label {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition:
    max-width 0.2s ease,
    opacity 0.12s ease,
    transform 0.12s ease;
  max-width: 12rem;
}

.sidebar-label-flex {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.sidebar-label-collapsed {
  max-width: 0;
  opacity: 0;
  transform: translateX(-4px);
  pointer-events: none;
}

/* Custom SVG icon in sidebar: constrain size without overriding uploaded SVG colors */
.sidebar-svg-icon {
  color: currentColor;
}

.sidebar-svg-icon :deep(svg) {
  display: block;
  width: 100%;
  height: 100%;
}
</style>
