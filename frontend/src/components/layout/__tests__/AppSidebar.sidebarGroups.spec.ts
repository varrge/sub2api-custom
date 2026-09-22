import { defineComponent, h } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import AppSidebar from '../AppSidebar.vue'
import type { SidebarGroupsConfig } from '@/types'

const state = vi.hoisted(() => ({
  isAdmin: false,
  sidebarCollapsed: false,
  sidebarGroups: { groups: [] } as SidebarGroupsConfig,
  adminSidebarGroups: { groups: [] } as SidebarGroupsConfig,
}))

const spies = vi.hoisted(() => ({
  setSidebarCollapsed: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/dashboard' }),
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
    mobileOpen: false,
    publicSettingsLoaded: true,
    setMobileOpen: vi.fn(),
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

describe('AppSidebar custom groups', () => {
  beforeEach(() => {
    state.isAdmin = false
    state.sidebarCollapsed = false
    state.sidebarGroups = { groups: [] }
    state.adminSidebarGroups = { groups: [] }
    vi.clearAllMocks()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })

  it('groups configured user pages under an expandable folder and keeps the rest', async () => {
    state.sidebarGroups = {
      groups: [{ id: 'g1', label: '常用工具', visibility: 'user', items: ['/keys', '/usage'] }],
    }
    const view = mountSidebar()
    await flushPromises()

    const groupButton = view.findAll('button').find(b => b.text().includes('常用工具'))
    expect(groupButton).toBeDefined()

    // Active route (/dashboard) is not in the group -> collapsed by default
    expect(view.find('[data-to="/keys"]').exists()).toBe(false)
    // Ungrouped entries stay visible
    expect(view.find('[data-to="/dashboard"]').exists()).toBe(true)

    await groupButton!.trigger('click')
    expect(view.get('[data-to="/keys"]').text()).toContain('nav.apiKeys')
    expect(view.get('[data-to="/usage"]').text()).toContain('nav.usage')

    // Group renders above the ungrouped remainder
    const text = view.get('nav').text()
    expect(text.indexOf('常用工具')).toBeLessThan(text.indexOf('nav.dashboard'))
  })

  it('auto-expands the group containing the active route', async () => {
    state.sidebarGroups = {
      groups: [{ id: 'g1', label: '常用工具', visibility: 'user', items: ['/dashboard', '/keys'] }],
    }
    const view = mountSidebar()
    await flushPromises()
    expect(view.find('[data-to="/dashboard"]').exists()).toBe(true)
    expect(view.find('[data-to="/keys"]').exists()).toBe(true)
  })

  it('expands the collapsed sidebar when a group icon is clicked', async () => {
    state.sidebarCollapsed = true
    state.sidebarGroups = {
      groups: [{ id: 'g1', label: '常用工具', visibility: 'user', items: ['/keys'] }],
    }
    const view = mountSidebar()
    await flushPromises()

    const groupButton = view.findAll('button').find(b => b.attributes('title') === '常用工具')
    expect(groupButton).toBeDefined()
    await groupButton!.trigger('click')

    expect(spies.setSidebarCollapsed).toHaveBeenCalledWith(false)
  })

  it('applies admin-scope groups to the admin navigation only', async () => {
    state.isAdmin = true
    state.adminSidebarGroups = {
      groups: [{ id: 'ga', label: '运营管理', visibility: 'admin', items: ['/admin/users', '/admin/groups'] }],
    }
    state.sidebarGroups = {
      groups: [{ id: 'gu', label: '常用工具', visibility: 'user', items: ['/keys', '/profile'] }],
    }
    const view = mountSidebar()
    await flushPromises()

    const buttons = view.findAll('button')
    const adminGroup = buttons.find(b => b.text().includes('运营管理'))
    const userGroup = buttons.find(b => b.text().includes('常用工具'))
    expect(adminGroup).toBeDefined()
    expect(userGroup).toBeDefined()

    await adminGroup!.trigger('click')
    expect(view.find('[data-to="/admin/users"]').exists()).toBe(true)
    // Grouped personal entries are hidden until the personal group opens
    expect(view.find('[data-to="/keys"]').exists()).toBe(false)
    await userGroup!.trigger('click')
    expect(view.find('[data-to="/keys"]').exists()).toBe(true)
  })

  it('ignores groups whose scope does not match the section', async () => {
    state.sidebarGroups = {
      groups: [{ id: 'ga', label: '管理专属', visibility: 'admin', items: ['/keys'] }],
    }
    const view = mountSidebar()
    await flushPromises()

    expect(view.text()).not.toContain('管理专属')
    // /keys stays as a plain entry, not swallowed by the wrong-scope group
    expect(view.find('[data-to="/keys"]').exists()).toBe(true)
  })
})
