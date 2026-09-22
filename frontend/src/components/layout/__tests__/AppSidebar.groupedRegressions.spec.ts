import { defineComponent, h, nextTick, reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import AppSidebar from '../AppSidebar.vue'
import type { SidebarGroupsConfig } from '@/types'

const state = reactive({
  routePath: '/dashboard',
  mobileOpen: false,
  isAdmin: false,
  sidebarCollapsed: false,
  sidebarGroups: { groups: [] } as SidebarGroupsConfig,
  adminSidebarGroups: { groups: [] } as SidebarGroupsConfig,
})

const spies = vi.hoisted(() => ({
  setSidebarCollapsed: vi.fn(),
  setMobileOpen: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ get path() { return state.routePath } }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    backendModeEnabled: false,
    get cachedPublicSettings() {
      return { custom_menu_items: [], sidebar_groups: state.sidebarGroups }
    },
    contactInfo: '',
    get mobileOpen() { return state.mobileOpen },
    publicSettingsLoaded: true,
    setMobileOpen: spies.setMobileOpen,
    setSidebarCollapsed: spies.setSidebarCollapsed,
    get sidebarCollapsed() { return state.sidebarCollapsed },
    sidebarScrollTop: 0,
    siteLogo: '',
    siteName: 'Sub2API',
    siteVersion: '1.0.0',
  }),
  useAuthStore: () => ({
    get isAdmin() { return state.isAdmin },
    isSimpleMode: false,
    logout: vi.fn(),
    get user() {
      return { avatar_url: '', email: 'u@example.com', role: state.isAdmin ? 'admin' : 'user', username: 'U' }
    },
  }),
  useAdminSettingsStore: () => ({
    customMenuItems: [],
    fetch: vi.fn(),
    opsMonitoringEnabled: true,
    paymentEnabled: true,
    get sidebarGroups() { return state.adminSidebarGroups },
  }),
  useOnboardingStore: () => ({ isCurrentStep: () => false, nextStep: vi.fn(), replay: vi.fn() }),
  useSupportTicketStore: () => ({ adminUnreadCount: 0, userUnreadCount: 0 }),
}))

vi.mock('@/utils/featureFlags', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/utils/featureFlags')>(),
  makeSidebarFlag: () => () => true,
}))

vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({
    canUseBatchImage: { value: true },
    refreshBatchImageAccess: vi.fn().mockResolvedValue(true),
  }),
  useImageGenerationAccess: () => ({ canUseImageGeneration: { value: true } }),
}))

const RouterLinkStub = defineComponent({
  props: ['to'],
  setup(props, { attrs, slots }) {
    return () => h('a', { ...attrs, 'data-to': String(props.to) }, slots.default?.())
  },
})

let wrapper: VueWrapper | undefined

function mountSidebar() {
  wrapper = mount(AppSidebar, {
    global: {
      mocks: { $t: (key: string) => key },
      stubs: { Icon: { template: '<i />' }, RouterLink: RouterLinkStub, VersionBadge: true },
    },
  })
  return wrapper
}

function groupButton(view: VueWrapper, label: string) {
  const button = view.findAll('button[aria-expanded]').find(item => item.text() === label)
  if (!button) throw new Error(`Sidebar group not found: ${label}`)
  return button
}

describe('AppSidebar grouped navigation regressions', () => {
  beforeEach(() => {
    state.routePath = '/dashboard'
    state.isAdmin = false
    state.sidebarCollapsed = false
    state.mobileOpen = false
    state.sidebarGroups = { groups: [] }
    state.adminSidebarGroups = { groups: [] }
    vi.clearAllMocks()
    spies.setSidebarCollapsed.mockImplementation((value: boolean) => { state.sidebarCollapsed = value })
    spies.setMobileOpen.mockImplementation((value: boolean) => { state.mobileOpen = value })
    vi.stubGlobal('innerWidth', 1440)
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    vi.unstubAllGlobals()
  })

  it.each([
    { role: 'user', isAdmin: false, detailPath: '/tickets/123', menuPath: '/tickets' },
    { role: 'admin', isAdmin: true, detailPath: '/admin/tickets/123', menuPath: '/admin/tickets' },
  ])('expands and highlights grouped $role ticket details, preserving activity when folded', async ({ isAdmin, detailPath, menuPath }) => {
    state.isAdmin = isAdmin
    state.routePath = detailPath
    const config: SidebarGroupsConfig = {
      groups: [{ id: 'support', label: '工单服务', visibility: isAdmin ? 'admin' : 'user', items: [menuPath] }],
    }
    if (isAdmin) state.adminSidebarGroups = config
    else state.sidebarGroups = config

    const view = mountSidebar()
    await flushPromises()

    const folder = groupButton(view, '工单服务')
    expect(folder.attributes('aria-expanded')).toBe('true')
    expect(view.get(`[data-to="${menuPath}"]`).classes()).toContain('sidebar-link-active')
    expect(folder.classes()).not.toContain('sidebar-link-active')

    await folder.trigger('click')
    expect(folder.attributes('aria-expanded')).toBe('false')
    expect(view.find(`[data-to="${menuPath}"]`).exists()).toBe(false)
    expect(folder.classes()).toContain('sidebar-link-active')
  })

  it('highlights only plans within the native orders folder nested in a configured group', async () => {
    state.isAdmin = true
    state.routePath = '/admin/orders/plans'
    state.adminSidebarGroups = {
      groups: [{ id: 'operations', label: '运营管理', visibility: 'admin', items: ['/admin/orders'] }],
    }
    const view = mountSidebar()
    await flushPromises()

    const outerFolder = groupButton(view, '运营管理')
    const ordersFolder = groupButton(view, 'nav.orderManagement')
    expect(outerFolder.attributes('aria-expanded')).toBe('true')
    expect(ordersFolder.attributes('aria-expanded')).toBe('true')
    expect(view.get('[data-to="/admin/orders/plans"]').classes()).toContain('sidebar-link-active')
    expect(view.get('[data-to="/admin/orders"]').classes()).not.toContain('sidebar-link-active')
    expect(view.get('[data-to="/admin/orders/dashboard"]').classes()).not.toContain('sidebar-link-active')

    await ordersFolder.trigger('click')
    expect(ordersFolder.classes()).toContain('sidebar-link-active')
    expect(view.find('[data-to="/admin/orders/plans"]').exists()).toBe(false)

    await outerFolder.trigger('click')
    expect(outerFolder.classes()).toContain('sidebar-link-active')
    expect(outerFolder.attributes('aria-expanded')).toBe('false')
  })

  it.each([
    { role: 'user', isAdmin: false, path: '/keys', selector: '[data-tour="sidebar-my-keys"]', width: 1440 },
    { role: 'user', isAdmin: false, path: '/keys', selector: '[data-tour="sidebar-my-keys"]', width: 390 },
    { role: 'admin', isAdmin: true, path: '/admin/groups', selector: '#sidebar-group-manage', width: 1440 },
    { role: 'admin', isAdmin: true, path: '/admin/groups', selector: '#sidebar-group-manage', width: 390 },
  ])('reveals the grouped $role onboarding target from dashboard at width $width', async ({ isAdmin, path, selector, width }) => {
    state.isAdmin = isAdmin
    state.routePath = isAdmin ? '/admin/dashboard' : '/dashboard'
    vi.stubGlobal('innerWidth', width)
    const config: SidebarGroupsConfig = {
      groups: [{ id: 'onboarding', label: '入门工具', visibility: isAdmin ? 'admin' : 'user', items: [path] }],
    }
    if (isAdmin) state.adminSidebarGroups = config
    else state.sidebarGroups = config
    const view = mountSidebar()
    await flushPromises()

    const folder = groupButton(view, '入门工具')
    expect(folder.attributes('aria-expanded')).toBe('false')
    expect(view.find(selector).exists()).toBe(false)

    // Simulate the sidebar being collapsed after mount, before replaying the tour.
    state.sidebarCollapsed = true
    await nextTick()
    vi.clearAllMocks()
    window.dispatchEvent(new CustomEvent('sub2api:reveal-sidebar-item', { detail: selector }))
    await flushPromises()

    expect(spies.setSidebarCollapsed).toHaveBeenCalledWith(false)
    expect(view.get('aside').classes()).toContain('w-64')
    expect(folder.attributes('aria-expanded')).toBe('true')
    expect(view.get(selector).attributes('data-to')).toBe(path)
    if (width < 1024) {
      expect(spies.setMobileOpen).toHaveBeenCalledWith(true)
      expect(view.get('aside').classes()).not.toContain('-translate-x-full')
    } else {
      expect(spies.setMobileOpen).not.toHaveBeenCalled()
    }
  })
})
