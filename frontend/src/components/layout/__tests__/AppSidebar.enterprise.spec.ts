import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick, reactive } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'

import AppSidebar from '../AppSidebar.vue'

const featureSettings = reactive({
  payment_enabled: true,
  support_ticket_enabled: true as boolean | undefined,
  custom_menu_items: [],
})

const state = vi.hoisted(() => ({
  isAdmin: false,
  isEnterprise: false,
  isSimpleMode: false,
  mobileOpen: false,
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/usage' }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    backendModeEnabled: false,
    cachedPublicSettings: featureSettings,
    contactInfo: '',
    get mobileOpen() { return state.mobileOpen },
    publicSettingsLoaded: true,
    setMobileOpen: vi.fn(),
    setSidebarCollapsed: vi.fn(),
    sidebarCollapsed: false,
    sidebarScrollTop: 0,
    siteLogo: '',
    siteName: 'Sub2API',
    siteVersion: '1.0.0',
  }),
  useAuthStore: () => ({
    get isAdmin() { return state.isAdmin },
    get isEnterprise() { return state.isEnterprise },
    get isSimpleMode() { return state.isSimpleMode },
    logout: vi.fn().mockResolvedValue(undefined),
    get user() {
      return {
        avatar_url: '',
        email: 'user@example.com',
        role: state.isAdmin ? 'admin' : 'user',
        username: 'User',
        account_type: state.isEnterprise ? 'enterprise' : 'standard',
      }
    },
  }),
  useAdminSettingsStore: () => ({
    customMenuItems: [],
    fetch: vi.fn(),
    opsMonitoringEnabled: true,
    paymentEnabled: true,
  }),
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn(),
    replay: vi.fn(),
  }),
  useSupportTicketStore: () => ({ adminUnreadCount: 0, userUnreadCount: 0 }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: featureSettings }),
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
let host: HTMLElement | undefined

function mountSidebar() {
  host = document.createElement('div')
  document.body.appendChild(host)
  wrapper = mount(AppSidebar, {
    attachTo: host,
    global: {
      mocks: { $t: (key: string) => key },
      stubs: {
        Icon: { template: '<i />' },
        RouterLink: RouterLinkStub,
        VersionBadge: true,
      },
    },
  })
  return wrapper
}

function navPaths(view: VueWrapper): string[] {
  return view.findAll('nav .sidebar-section a').map((a) => a.attributes('data-to'))
}

describe('AppSidebar enterprise navigation', () => {
  beforeEach(() => {
    state.isAdmin = false
    state.isEnterprise = false
    state.isSimpleMode = false
    state.mobileOpen = false
    featureSettings.payment_enabled = true
    featureSettings.support_ticket_enabled = true
    vi.clearAllMocks()
  })

  afterEach(() => {
    wrapper?.unmount()
    host?.remove()
    wrapper = undefined
    host = undefined
  })

  it('renders six first-level entries in order when payment and tickets are enabled', () => {
    state.isEnterprise = true
    const view = mountSidebar()

    expect(view.find('[data-testid="enterprise-nav-section"]').exists()).toBe(true)
    expect(navPaths(view)).toEqual(['/usage', '/monitor', '/keys', '/purchase', '/profile', '/tickets'])
  })

  it('excludes orders, model plaza, dashboard and admin entries for enterprise users', () => {
    state.isEnterprise = true
    const view = mountSidebar()
    const paths = navPaths(view)

    expect(paths).not.toContain('/orders')
    expect(paths).not.toContain('/model-plaza')
    expect(paths).not.toContain('/dashboard')
    expect(paths.some((p) => p.startsWith('/admin'))).toBe(false)
    expect(view.find('#sidebar-channel-manage').exists()).toBe(false)
    expect(view.find('#sidebar-group-manage').exists()).toBe(false)
    expect(view.find('#sidebar-wallet').exists()).toBe(false)
  })

  it('keeps the enterprise nav fixed regardless of run mode', () => {
    state.isEnterprise = true
    state.isSimpleMode = true
    const view = mountSidebar()

    expect(navPaths(view)).toEqual(['/usage', '/monitor', '/keys', '/purchase', '/profile', '/tickets'])
  })

  it.each([
    [false, true, ['/usage', '/monitor', '/keys', '/profile', '/tickets']],
    [true, false, ['/usage', '/monitor', '/keys', '/purchase', '/profile']],
    [false, false, ['/usage', '/monitor', '/keys', '/profile']],
    [true, undefined, ['/usage', '/monitor', '/keys', '/purchase', '/profile']],
  ])('filters enterprise navigation for payment=%s and tickets=%s', (payment, tickets, expected) => {
    state.isEnterprise = true
    featureSettings.payment_enabled = payment as boolean
    featureSettings.support_ticket_enabled = tickets as boolean | undefined

    expect(navPaths(mountSidebar())).toEqual(expected)
  })

  it('updates enterprise entries when public feature settings change after mount', async () => {
    state.isEnterprise = true
    state.isSimpleMode = true
    const view = mountSidebar()

    featureSettings.payment_enabled = false
    featureSettings.support_ticket_enabled = false
    await nextTick()
    expect(navPaths(view)).toEqual(['/usage', '/monitor', '/keys', '/profile'])

    featureSettings.payment_enabled = true
    featureSettings.support_ticket_enabled = true
    await nextTick()
    expect(navPaths(view)).toEqual(['/usage', '/monitor', '/keys', '/purchase', '/profile', '/tickets'])
  })

  it('does not render the enterprise nav for regular users', () => {
    state.isEnterprise = false
    const view = mountSidebar()

    expect(view.find('[data-testid="enterprise-nav-section"]').exists()).toBe(false)
    const paths = navPaths(view)
    expect(paths).toContain('/dashboard')
    expect(paths).toContain('/keys')
  })

  it('keeps the admin navigation unchanged for admins (admin branch takes precedence)', () => {
    state.isAdmin = true
    state.isEnterprise = true
    const view = mountSidebar()

    expect(view.find('[data-testid="enterprise-nav-section"]').exists()).toBe(false)
    const paths = navPaths(view)
    expect(paths).toContain('/admin/dashboard')
    expect(paths).toContain('/admin/accounts')
  })
})
