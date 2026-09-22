<template>
  <div class="space-y-3">
    <!-- 一级:平台(单选) -->
    <div class="flex items-start gap-2">
      <span class="w-16 shrink-0 pt-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
        {{ t('modelPlaza.filters.platformLabel') }}
      </span>
      <div class="flex flex-wrap items-center gap-2">
        <button
          v-for="p in ['all', ...platforms]"
          :key="`platform-${p}`"
          type="button"
          class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-40 disabled:grayscale"
          :class="p === 'all' ? chipClass(platform === 'all') : platform === p ? 'chip-tinted-active' : 'chip-tinted'"
          :style="p === 'all' ? undefined : { '--chip-accent': platformAccentColor(p) }"
          :disabled="p !== 'all' && !platformEnabled(p)"
          :aria-pressed="platform === p"
          @click="$emit('update:platform', p)"
        >
          <PlatformIcon v-if="p !== 'all'" :platform="p as GroupPlatform" size="xs" />
          {{ p === 'all' ? t('modelPlaza.filters.all') : p }}
        </button>
      </div>
    </div>

    <!-- 二级:分组(多选,按所属平台着色,当前平台/倍率下无结果的置灰) -->
    <div class="flex items-start gap-2">
      <span class="w-16 shrink-0 pt-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
        {{ t('modelPlaza.filters.groupLabel') }}
      </span>
      <div class="min-w-0 flex-1 space-y-1.5">
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="rounded-lg px-3 py-1.5 text-sm font-medium transition"
            :class="chipClass(groupIds === 'all')"
            :aria-pressed="groupIds === 'all'"
            @click="$emit('update:groupIds', 'all')"
          >
            {{ t('modelPlaza.filters.selectAll') }}
          </button>
          <button
            v-for="g in groups"
            :key="`group-${g.id}`"
            type="button"
            class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-40 disabled:grayscale"
            :class="isGroupSelected(g.id) ? 'chip-tinted-active' : 'chip-tinted'"
            :style="{ '--chip-accent': platformAccentColor(g.platform) }"
            :disabled="!groupEnabled(g) && !isGroupSelected(g.id)"
            :aria-pressed="isGroupSelected(g.id)"
            @click="toggleGroup(g.id)"
          >
            <span
              class="flex h-4 w-4 shrink-0 items-center justify-center rounded border transition"
              :class="isGroupSelected(g.id) ? 'border-white/70 bg-white/20' : 'border-current bg-transparent opacity-40'"
              aria-hidden="true"
            >
              <Icon v-if="isGroupSelected(g.id)" name="check" size="xs" class="h-3 w-3" />
            </span>
            <span class="min-w-0 break-words text-left">{{ g.name }}</span>
          </button>
          <button
            type="button"
            class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-400 ring-1 ring-inset ring-gray-200 transition hover:bg-gray-50 hover:text-gray-600 dark:text-dark-400 dark:ring-dark-700 dark:hover:bg-dark-800 dark:hover:text-dark-200"
            @click="$emit('update:groupIds', [])"
          >
            <Icon name="x" size="xs" class="h-3 w-3" />
            {{ t('modelPlaza.filters.clearGroups') }}
          </button>
        </div>
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1 pt-0.5 text-xs text-gray-400 dark:text-dark-500">
          <span>{{ t('modelPlaza.filters.multiSelectHint') }}</span>
          <span>{{ t('modelPlaza.filters.selectedGroups', { selected: selectedGroupCount, total: visibleGroups.length }) }}</span>
        </div>
      </div>
    </div>

    <!-- 三级:倍率(单选,当前组合下不存在的置灰) -->
    <div class="flex items-start gap-2">
      <span class="w-16 shrink-0 pt-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
        {{ t('modelPlaza.filters.rateLabel') }}
      </span>
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="rounded-lg px-3 py-1.5 text-sm font-medium transition"
          :class="chipClass(rate === 'all')"
          :aria-pressed="rate === 'all'"
          @click="$emit('update:rate', 'all')"
        >
          {{ t('modelPlaza.filters.all') }}
        </button>
        <button
          v-for="r in rates"
          :key="`rate-${r}`"
          type="button"
          class="rounded-lg px-3 py-1.5 font-mono text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-40 disabled:grayscale"
          :class="chipClass(rate === r)"
          :disabled="!rateEnabled(r)"
          :aria-pressed="rate === r"
          @click="$emit('update:rate', r)"
        >
          {{ r }}x
        </button>
      </div>
    </div>

    <!-- 四级:模型名搜索(纯前端过滤) -->
    <div class="flex flex-wrap items-start gap-2">
      <span class="w-16 shrink-0 pt-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
        {{ t('modelPlaza.filters.modelLabel') }}
      </span>
      <div class="relative w-full sm:w-72">
        <Icon
          name="search"
          size="sm"
          class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-500"
        />
        <input
          :value="search"
          type="text"
          :placeholder="t('modelPlaza.filters.searchPlaceholder')"
          :aria-label="t('modelPlaza.filters.searchPlaceholder')"
          class="input rounded-lg py-1.5 pl-9 pr-9"
          @input="$emit('update:search', ($event.target as HTMLInputElement).value)"
        />
        <button
          v-if="search"
          type="button"
          :aria-label="t('modelPlaza.filters.clearSearch')"
          class="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 transition-colors hover:text-gray-600 dark:text-dark-500 dark:hover:text-gray-300"
          @click="$emit('update:search', '')"
        >
          <Icon name="x" size="xs" class="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformAccentColor } from '@/utils/platformColors'
import type { GroupPlatform } from '@/types'

type GroupOption = { id: number; name: string; platform: string; rate: number }

const props = defineProps<{
  /** 数据中出现的平台(去重排序后)。 */
  platforms: string[]
  /** 全量分组(含平台与生效倍率),三个维度的置灰联动由此推导。 */
  groups: GroupOption[]
  /** 全量生效倍率去重升序。 */
  rates: number[]
  platform: string
  /** 'all'=分组维度不设限;[]=清空选择;数字数组=已选分组。 */
  groupIds: number[] | 'all'
  rate: number | 'all'
  /** 模型名搜索词(纯前端过滤)。 */
  search: string
}>()

const emit = defineEmits<{
  'update:platform': [value: string]
  'update:groupIds': [value: number[] | 'all']
  'update:rate': [value: number | 'all']
  'update:search': [value: string]
}>()

const { t } = useI18n()

/** 分组维度约束:'all' 与清空([])均不限制平台/倍率的探索,仅数组非空时才按并集约束。 */
function matchesGroupSelection(g: { id: number }): boolean {
  return props.groupIds === 'all' || props.groupIds.length === 0 || props.groupIds.includes(g.id)
}

function isGroupSelected(id: number): boolean {
  return props.groupIds === 'all'
    ? visibleGroups.value.some((g) => g.id === id)
    : props.groupIds.includes(id)
}

/** 全选后点击勾选项只取消该项;数组态下增删该 id。 */
function toggleGroup(id: number): void {
  if (props.groupIds === 'all') {
    emit('update:groupIds', visibleGroups.value.filter((g) => g.id !== id).map((g) => g.id))
    return
  }
  emit(
    'update:groupIds',
    props.groupIds.includes(id)
      ? props.groupIds.filter((g) => g !== id)
      : [...props.groupIds, id]
  )
}

/** 当前平台/倍率下可见(可点)的分组;全选与已选计数都以它为准。 */
const visibleGroups = computed(() => props.groups.filter((g) => groupEnabled(g)))

/** 已选计数只统计当前平台/倍率下可见的分组,被其他维度过滤隐藏的组不计入。 */
const selectedGroupCount = computed(() => {
  if (props.groupIds === 'all') return visibleGroups.value.length
  const visible = new Set(visibleGroups.value.map((g) => g.id))
  return props.groupIds.filter((id) => visible.has(id)).length
})

/**
 * 三个维度互为约束(faceted):某选项可点 ⟺ 在「其他两维」当前选择下仍有分组命中。
 * 分组「全选」永远可点,作为解除本维约束的出口;可点项组合恒有结果,无需选择修正。
 */
function platformEnabled(p: string): boolean {
  return props.groups.some(
    (g) =>
      g.platform === p &&
      matchesGroupSelection(g) &&
      (props.rate === 'all' || g.rate === props.rate)
  )
}

/** 分组是否匹配平台/倍率;模板额外允许取消当前筛选范围外的已选项。 */
function groupEnabled(g: { platform: string; rate: number }): boolean {
  return (
    (props.platform === 'all' || g.platform === props.platform) &&
    (props.rate === 'all' || g.rate === props.rate)
  )
}

function rateEnabled(r: number): boolean {
  return props.groups.some(
    (g) =>
      g.rate === r &&
      (props.platform === 'all' || g.platform === props.platform) &&
      matchesGroupSelection(g)
  )
}

function chipClass(active: boolean): string {
  return active
    ? 'bg-gradient-to-r from-primary-500 to-primary-600 text-on-primary shadow-sm shadow-primary-500/30'
    : 'bg-white text-gray-600 ring-1 ring-inset ring-gray-200 enabled:hover:bg-gray-50 enabled:hover:text-gray-900 enabled:hover:ring-gray-300 dark:bg-dark-800/60 dark:text-dark-300 dark:ring-dark-700 dark:enabled:hover:bg-dark-800 dark:enabled:hover:text-white'
}
</script>

<style scoped>
button:focus-visible {
  outline: 2px solid theme('colors.primary.500');
  outline-offset: 3px;
}

button {
  min-height: 2.25rem;
  max-width: 100%;
}

@media (pointer: coarse) {
  button {
    min-height: 2.75rem;
  }
}

/* 平台/分组 chip 的配色统一从 --chip-accent(平台主色)派生,新增平台无需扩展样式。
   激活态与非激活态在模板上互斥挂载,避免选择器优先级互相覆盖。 */
.chip-tinted {
  color: var(--chip-accent);
  color: color-mix(in srgb, var(--chip-accent) 78%, black);
  background-color: color-mix(in srgb, var(--chip-accent) 9%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--chip-accent) 25%, transparent);
}

.chip-tinted:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--chip-accent) 16%, transparent);
}

.dark .chip-tinted {
  color: color-mix(in srgb, var(--chip-accent) 72%, white);
  background-color: color-mix(in srgb, var(--chip-accent) 12%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--chip-accent) 30%, transparent);
}

.dark .chip-tinted:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--chip-accent) 18%, transparent);
}

.chip-tinted-active {
  color: #fff;
  background-color: var(--chip-accent);
  background-color: color-mix(in srgb, var(--chip-accent) 85%, black);
  box-shadow: 0 1px 2px 0 color-mix(in srgb, var(--chip-accent) 35%, transparent);
}

.chip-tinted-active:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--chip-accent) 75%, black);
}

.dark .chip-tinted-active {
  background-color: color-mix(in srgb, var(--chip-accent) 80%, transparent);
}

.dark .chip-tinted-active:not(:disabled):hover {
  background-color: var(--chip-accent);
}
</style>
