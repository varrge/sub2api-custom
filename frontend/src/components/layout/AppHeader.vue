<template>
  <header class="app-header glass sticky top-0 z-30 border-b border-gray-200/50 dark:border-dark-700/50">
    <div class="relative flex h-16 items-center justify-between gap-2 px-2 sm:px-4 md:px-6">
      <!-- Left: Mobile Menu Toggle + Page Title -->
      <div class="flex min-w-0 shrink-0 items-center gap-2 sm:gap-4">
        <button
          @click="toggleMobileSidebar"
          class="btn-ghost btn-icon lg:hidden"
          :aria-label="t('common.toggleMenu')"
        >
          <Icon name="menu" size="md" />
        </button>

        <div class="hidden min-w-0 max-w-[30vw] lg:block">
          <h1 class="truncate text-lg font-semibold text-gray-900 dark:text-white">
            {{ pageTitle }}
          </h1>
          <p v-if="pageDescription" class="truncate text-xs text-gray-500 dark:text-dark-400">
            {{ pageDescription }}
          </p>
        </div>
      </div>

      <nav
        v-if="user"
        class="quick-menu-slot"
        :aria-label="t('topQuickMenu.label')"
        data-testid="top-quick-menu"
      >
        <div class="quick-menu-pill inline-flex max-w-full items-center overflow-hidden rounded-full border border-gray-200/80 bg-gray-50/90 p-1 shadow-sm dark:border-dark-700 dark:bg-dark-800/80">
          <router-link
            :to="dashboardPath"
            class="quick-menu-link shrink-0"
            :class="isQuickMenuPathActive(dashboardPath) ? 'quick-menu-link-active' : 'quick-menu-link-idle'"
            :aria-current="isQuickMenuPathActive(dashboardPath) ? 'page' : undefined"
            :title="t('topQuickMenu.dashboard')"
            data-testid="top-quick-menu-dashboard"
          >
            <Icon name="home" size="sm" />
            <span class="quick-menu-dashboard-label">{{ t('topQuickMenu.dashboard') }}</span>
          </router-link>
          <router-link
            v-for="(item, index) in visibleTopQuickMenuItems"
            :key="item.id"
            :to="quickMenuTarget(item)"
            class="quick-menu-link quick-menu-optional shrink-0"
            :class="[
              `quick-menu-optional-${index + 1}`,
              isQuickMenuItemActive(item) ? 'quick-menu-link-active' : 'quick-menu-link-idle',
            ]"
            :aria-current="isQuickMenuItemActive(item) ? 'page' : undefined"
            :data-testid="`top-quick-menu-${item.id}`"
          >
            <Icon :name="item.icon" size="sm" />
            <span>{{ t(item.labelKey) }}</span>
            <span
              v-if="item.id === 'support_tickets' && supportTicketUnreadCount > 0"
              class="min-w-4 rounded-full bg-red-500 px-1 text-center text-[10px] font-semibold leading-4 text-white"
            >
              {{ supportTicketUnreadCount > 99 ? '99+' : supportTicketUnreadCount }}
            </span>
          </router-link>
        </div>
      </nav>

      <!-- Right: Docs + Language + Subscriptions + Announcements + Theme + Balance -->
      <div class="flex min-w-0 items-center gap-1 sm:gap-3">
        <!-- Docs Link -->
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="header-docs hidden items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white sm:flex"
        >
          <Icon name="book" size="sm" />
          <span class="hidden sm:inline">{{ t('nav.docs') }}</span>
        </a>

        <!-- Model Plaza Entry -->
        <router-link
          v-if="user && modelPlazaEnabled && !modelPlazaInQuickMenu"
          :to="{ path: '/model-plaza', query: { embedded: '1' } }"
          data-testid="model-plaza-legacy-entry"
          class="header-model-plaza hidden items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white sm:flex"
        >
          <Icon name="grid" size="sm" />
          <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
        </router-link>

        <div class="flex min-w-0 items-center gap-1 rounded-2xl border border-gray-200/70 bg-white/70 p-0.5 shadow-sm shadow-slate-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-white/[0.06] dark:shadow-black/20 sm:gap-1.5">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Subscription Progress (for users with active subscriptions) -->
          <SubscriptionProgressMini v-if="user" class="header-subscription" />

          <!-- Announcement Bell -->
          <AnnouncementBell v-if="user" data-testid="header-announcement" />

          <!-- Theme Toggle -->
          <button
            v-if="user"
            type="button"
            class="btn-ghost btn-icon shrink-0"
            :title="themeToggleLabel"
            :aria-label="themeToggleLabel"
            data-testid="header-theme-toggle"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="md" :class="isDark ? 'text-amber-500' : ''" />
          </button>

          <!-- Balance Display -->
          <div
            v-if="user"
            class="header-balance group relative min-w-0 shrink-0"
          >
            <button
              type="button"
              class="flex min-w-0 items-center gap-1.5 rounded-xl px-2 py-1.5 text-gray-700 transition-colors dark:text-dark-300 sm:px-3"
              :class="paymentEnabled
                ? 'hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-800 dark:hover:text-primary-400'
                : 'cursor-default'"
              :aria-disabled="!paymentEnabled"
              :aria-label="balanceAriaLabel"
              :title="balanceLoading ? undefined : balanceDisplay"
              data-testid="header-balance"
              @click="openPurchase"
            >
              <Icon name="dollar" size="sm" class="shrink-0" />
              <span
                v-if="balanceLoading"
                class="h-4 w-14 animate-pulse rounded bg-gray-200 dark:bg-dark-700"
                aria-hidden="true"
              ></span>
              <span v-else class="max-w-28 truncate text-sm font-semibold tabular-nums">
                {{ balanceDisplay }}
              </span>
            </button>
            <div
              v-if="!balanceLoading"
              class="pointer-events-none absolute right-0 top-full z-50 mt-2 hidden w-56 rounded-lg border border-gray-200 bg-white p-3 text-xs text-gray-700 shadow-lg dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300 lg:group-hover:block lg:group-focus-within:block"
              data-testid="header-balance-details"
            >
              <div class="flex items-center justify-between">
                <span class="text-gray-500 dark:text-dark-400">{{ balanceAvailableText }}</span>
                <span class="font-medium tabular-nums text-gray-900 dark:text-white">{{ balanceDisplay }}</span>
              </div>
              <div class="mt-2 flex items-center justify-between">
                <span class="text-gray-500 dark:text-dark-400">{{ balanceFrozenText }}</span>
                <span class="font-medium tabular-nums text-amber-700 dark:text-amber-200">{{ frozenBalanceDisplay }}</span>
              </div>
              <div class="mt-2 border-t border-gray-100 pt-2 dark:border-dark-700">
                <div class="flex items-center justify-between">
                  <span class="text-gray-500 dark:text-dark-400">{{ balanceTotalText }}</span>
                  <span class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ totalBalanceDisplay }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useSupportTicketStore } from '@/stores/supportTickets'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { TOP_QUICK_MENU_OPTIONS, normalizeTopQuickMenuItems } from '@/utils/topQuickMenu'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const supportTicketStore = useSupportTicketStore()

const user = computed(() => authStore.user)
const isDark = ref(document.documentElement.classList.contains('dark'))
const docUrl = computed(() => sanitizeUrl(appStore.docUrl))
const imageGenerationEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.imageGeneration))
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const supportTicketsEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.supportTicket))
const paymentEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.payment))
const configuredTopQuickMenuItems = computed(() =>
  normalizeTopQuickMenuItems(appStore.cachedPublicSettings?.top_quick_menu_items),
)
const modelPlazaInQuickMenu = computed(() => configuredTopQuickMenuItems.value.includes('model_plaza'))
const dashboardPath = computed(() => authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
const visibleTopQuickMenuItems = computed(() =>
  configuredTopQuickMenuItems.value
    .map((id) => TOP_QUICK_MENU_OPTIONS.find((item) => item.id === id))
    .filter((item): item is (typeof TOP_QUICK_MENU_OPTIONS)[number] => {
      if (!item) return false
      if (item.id === 'image_generation') return imageGenerationEnabled.value
      if (item.id === 'model_plaza') return modelPlazaEnabled.value
      if (item.id === 'support_tickets') return supportTicketsEnabled.value
      return true
    }),
)
const supportTicketUnreadCount = computed(() =>
  authStore.isAdmin ? supportTicketStore.adminUnreadCount : supportTicketStore.userUnreadCount,
)
const availableBalance = computed(() => Number(user.value?.balance))
const frozenBalance = computed(() => Number(user.value?.frozen_balance ?? 0))
const totalBalance = computed(() => availableBalance.value + frozenBalance.value)
const balanceLoading = computed(() => authStore.userRefreshStatus === 'loading')
const balanceFailed = computed(() =>
  authStore.userRefreshStatus === 'error' || !Number.isFinite(availableBalance.value),
)
const balanceAvailableText = computed(() => t('common.availableBalance') === 'common.availableBalance' ? '可用余额' : t('common.availableBalance'))
const balanceFrozenText = computed(() => t('common.frozenBalance') === 'common.frozenBalance' ? '冻结金额' : t('common.frozenBalance'))
const balanceTotalText = computed(() => t('common.totalBalance') === 'common.totalBalance' ? '总余额' : t('common.totalBalance'))
const balanceDisplay = computed(() => balanceFailed.value ? '$--' : formatHeaderMoney(availableBalance.value))
const frozenBalanceDisplay = computed(() => balanceFailed.value ? '$--' : formatHeaderMoney(frozenBalance.value))
const totalBalanceDisplay = computed(() => balanceFailed.value ? '$--' : formatHeaderMoney(totalBalance.value))
const themeToggleLabel = computed(() => t(isDark.value ? 'nav.lightMode' : 'nav.darkMode'))
const balanceAriaLabel = computed(() => {
  const label = `${balanceAvailableText.value}: ${balanceLoading.value ? t('common.loading') : balanceDisplay.value}`
  return paymentEnabled.value ? `${label}. ${t('nav.buySubscription')}` : label
})

const pageTitle = computed(() => {
  // For custom pages, use the menu item's label instead of generic "自定义页面"
  if (route.name === 'CustomPage') {
    const id = route.params.id as string
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const menuItem = publicItems.find((item) => item.id === id)
      ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
    if (menuItem?.label) return menuItem.label
  }
  const titleKey = route.meta.titleKey as string
  if (titleKey) {
    return t(titleKey)
  }
  return (route.meta.title as string) || ''
})

const pageDescription = computed(() => {
  const descKey = route.meta.descriptionKey as string
  if (descKey) {
    return t(descKey)
  }
  return (route.meta.description as string) || ''
})

function toggleMobileSidebar() {
  appStore.toggleMobileSidebar()
}

function quickMenuPath(item: (typeof TOP_QUICK_MENU_OPTIONS)[number]): string {
  return authStore.isAdmin && 'adminPath' in item ? item.adminPath : item.path
}

function quickMenuTarget(item: (typeof TOP_QUICK_MENU_OPTIONS)[number]) {
  const path = quickMenuPath(item)
  return item.id === 'model_plaza' ? { path, query: { embedded: '1' } } : path
}

function isQuickMenuPathActive(path: string): boolean {
  return route.path === path || route.path.startsWith(`${path}/`)
}

function isQuickMenuItemActive(item: (typeof TOP_QUICK_MENU_OPTIONS)[number]): boolean {
  return ('routeName' in item && route.name === item.routeName)
    || isQuickMenuPathActive(quickMenuPath(item))
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function formatHeaderMoney(value: number) {
  if (!Number.isFinite(value)) return '$0.00'
  return `$${value.toFixed(2)}`
}

function openPurchase() {
  if (paymentEnabled.value) void router.push('/purchase')
}
</script>

<style scoped>
.app-header {
  container: app-header / inline-size;
}

.header-model-plaza,
.header-docs,
.header-subscription {
  display: none;
}

@container app-header (min-width: 48rem) {
  .header-model-plaza {
    display: flex;
  }
}

@container app-header (min-width: 56rem) {
  .header-docs {
    display: flex;
  }
}

@container app-header (min-width: 62rem) {
  .header-subscription {
    display: block;
  }
}

.quick-menu-slot {
  position: absolute;
  left: 50%;
  display: flex;
  width: clamp(2.5rem, calc(100% - 28rem), 35rem);
  transform: translateX(-50%);
  justify-content: center;
  container-type: inline-size;
}

.quick-menu-link {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  border-radius: 9999px;
  padding: 0.375rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 500;
  line-height: 1rem;
  transition: color 150ms, background-color 150ms;
}

.quick-menu-link-active {
  background: rgb(37 99 235);
  color: white;
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.08);
}

.quick-menu-link-idle {
  color: rgb(75 85 99);
}

.quick-menu-link-idle:hover {
  background: rgb(229 231 235 / 0.75);
  color: rgb(17 24 39);
}

:global(.dark) .quick-menu-link-idle {
  color: rgb(156 163 175);
}

:global(.dark) .quick-menu-link-idle:hover {
  background: rgb(55 65 81 / 0.8);
  color: white;
}

.quick-menu-dashboard-label,
.quick-menu-optional {
  display: none;
}

@container (min-width: 8rem) {
  .quick-menu-dashboard-label {
    display: inline;
  }
}

@container (min-width: 15rem) {
  .quick-menu-optional-1 {
    display: flex;
  }
}

@container (min-width: 25rem) {
  .quick-menu-optional-2 {
    display: flex;
  }
}

@container (min-width: 35rem) {
  .quick-menu-optional-3 {
    display: flex;
  }
}

@media (max-width: 639px) {
  .quick-menu-dashboard-label,
  .quick-menu-optional { display: none; }
}

</style>
