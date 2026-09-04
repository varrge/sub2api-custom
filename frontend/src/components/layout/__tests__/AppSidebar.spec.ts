import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import AppSidebar from '../AppSidebar.vue'

const sidebarState = vi.hoisted(() => ({
  isAdmin: true,
  isSimpleMode: false,
  mobileOpen: false,
  contactInfo: 'support@example.com',
}))

const sidebarSpies = vi.hoisted(() => ({
  routerPush: vi.fn(),
  setMobileOpen: vi.fn((open: boolean) => { sidebarState.mobileOpen = open }),
  setSidebarCollapsed: vi.fn(),
  logout: vi.fn().mockResolvedValue(undefined),
  replay: vi.fn(),
  nextStep: vi.fn(),
  fetchAdminSettings: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/dashboard' }),
  useRouter: () => ({ push: sidebarSpies.routerPush }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    backendModeEnabled: false,
    cachedPublicSettings: { custom_menu_items: [] },
    get contactInfo() { return sidebarState.contactInfo },
    get mobileOpen() { return sidebarState.mobileOpen },
    publicSettingsLoaded: true,
    setMobileOpen: sidebarSpies.setMobileOpen,
    setSidebarCollapsed: sidebarSpies.setSidebarCollapsed,
    sidebarCollapsed: false,
    sidebarScrollTop: 0,
    siteLogo: '',
    siteName: 'Sub2API',
    siteVersion: '1.0.0',
  }),
  useAuthStore: () => ({
    get isAdmin() { return sidebarState.isAdmin },
    get isSimpleMode() { return sidebarState.isSimpleMode },
    logout: sidebarSpies.logout,
    get user() {
      return {
        avatar_url: '',
        email: 'admin@example.com',
        role: sidebarState.isAdmin ? 'admin' : 'user',
        username: 'Admin',
      }
    },
  }),
  useAdminSettingsStore: () => ({
    customMenuItems: [],
    fetch: sidebarSpies.fetchAdminSettings,
    opsMonitoringEnabled: true,
    paymentEnabled: true,
  }),
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: sidebarSpies.nextStep,
    replay: sidebarSpies.replay,
  }),
  useSupportTicketStore: () => ({ adminUnreadCount: 0, userUnreadCount: 0 }),
}))

vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: {
    affiliate: 'affiliate',
    availableChannels: 'availableChannels',
    channelMonitor: 'channelMonitor',
    imageGeneration: 'imageGeneration',
    payment: 'payment',
    pluginManagement: 'pluginManagement',
    riskControl: 'riskControl',
    supportTicket: 'supportTicket',
  },
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

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar support ticket navigation', () => {
  it('keeps My Tickets feature-gated but available in simple mode', () => {
    const userItem = componentSource.match(/\{ path: '\/tickets'[^\n]+\}/)?.[0]

    expect(userItem).toContain('featureFlag: flagSupportTicket')
    expect(userItem).toContain('badge: () => supportTicketStore.userUnreadCount')
    expect(userItem).not.toContain('hideInSimpleMode')
    expect(componentSource).toContain("personalNavItems.value.filter((item) => item.path === '/tickets')")
  })

  it('always exposes separate admin ticket management and unread state', () => {
    const adminItem = componentSource.match(/\{ path: '\/admin\/tickets'[^\n]+\}/)?.[0]

    expect(adminItem).toContain('supportTickets.management')
    expect(adminItem).toContain('badge: () => supportTicketStore.adminUnreadCount')
    expect(adminItem).not.toContain('featureFlag')
  })
})

describe('AppSidebar account menu', () => {
  it('pins the account trigger below navigation and removes sidebar utility controls', () => {
    expect(componentSource.indexOf('data-testid="sidebar-account-trigger"')).toBeGreaterThan(
      componentSource.indexOf('</nav>'),
    )
    expect(componentSource).not.toContain('toggleTheme')
    expect(componentSource).not.toContain('toggleSidebar')
  })

  it('preserves account actions and closes on outside click or Escape', () => {
    expect(componentSource).toContain('to="/profile"')
    expect(componentSource).toContain('to="/keys"')
    expect(componentSource).toContain('handleLogout')
    expect(componentSource).toContain("event.key !== 'Escape'")
    expect(componentSource).toContain("document.addEventListener('click', handleAccountMenuClickOutside)")
  })

  it('opens above the mobile trigger and to its right on desktop', () => {
    expect(componentSource).toContain('bottom-full left-0 mb-2 w-full')
    expect(componentSource).toContain('lg:bottom-0 lg:left-full lg:mb-0 lg:ml-2 lg:w-56')
  })
})

let sidebarWrapper: VueWrapper | undefined
let sidebarHost: HTMLElement | undefined

function mountSidebar() {
  sidebarHost = document.createElement('div')
  document.body.appendChild(sidebarHost)
  sidebarWrapper = mount(AppSidebar, {
    attachTo: sidebarHost,
    global: {
      mocks: { $t: (key: string) => key },
      stubs: {
        Icon: { template: '<i />' },
        RouterLink: RouterLinkStub,
        VersionBadge: true,
      },
    },
  })
  return sidebarWrapper
}

describe('AppSidebar account menu interactions', () => {
  beforeEach(() => {
    sidebarState.isAdmin = true
    sidebarState.isSimpleMode = false
    sidebarState.mobileOpen = false
    sidebarState.contactInfo = 'support@example.com'
    vi.clearAllMocks()
  })

  afterEach(() => {
    sidebarWrapper?.unmount()
    sidebarHost?.remove()
    sidebarWrapper = undefined
    sidebarHost = undefined
    vi.useRealTimers()
  })

  it('toggles the menu from the account trigger', async () => {
    const view = mountSidebar()
    const trigger = view.get('[data-testid="sidebar-account-trigger"]')

    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(view.find('[data-testid="sidebar-account-dropdown"]').exists()).toBe(false)

    await trigger.trigger('click')
    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(view.find('[data-testid="sidebar-account-dropdown"]').exists()).toBe(true)

    await trigger.trigger('click')
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })

  it('closes on Escape with focus restored, and closes on outside click', async () => {
    const view = mountSidebar()
    const trigger = view.get('[data-testid="sidebar-account-trigger"]')

    await trigger.trigger('click')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()

    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(trigger.element)

    await trigger.trigger('click')
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()

    expect(trigger.attributes('aria-expanded')).toBe('false')
  })

  it.each(['/profile', '/keys'])('closes the mobile drawer after selecting %s', async (path) => {
    vi.useFakeTimers()
    sidebarState.mobileOpen = true
    const view = mountSidebar()
    const trigger = view.get('[data-testid="sidebar-account-trigger"]')

    await trigger.trigger('click')
    await view.get('[data-testid="sidebar-account-dropdown"]').get(`[data-to="${path}"]`).trigger('click')

    expect(trigger.attributes('aria-expanded')).toBe('false')
    vi.runAllTimers()
    expect(sidebarSpies.setMobileOpen).toHaveBeenCalledWith(false)
  })

  it('renders and runs the preserved admin account actions', async () => {
    sidebarState.mobileOpen = true
    const view = mountSidebar()
    const trigger = view.get('[data-testid="sidebar-account-trigger"]')

    expect(trigger.text()).toContain('Admin')
    expect(trigger.text()).toContain('admin.users.roles.admin')

    await trigger.trigger('click')
    expect(view.text()).toContain('admin@example.com')
    expect(view.text()).toContain('support@example.com')
    const githubLink = view.get('a[href="https://github.com/Wei-Shaw/sub2api"]')
    expect(githubLink.text()).toContain('nav.github')
    await githubLink.trigger('click')
    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(sidebarSpies.setMobileOpen).toHaveBeenCalledWith(false)

    await trigger.trigger('click')
    const guideButton = view.findAll('button.dropdown-item')
      .find((button) => button.text().includes('onboarding.restartTour'))
    expect(guideButton).toBeDefined()
    await guideButton!.trigger('click')
    expect(sidebarSpies.replay).toHaveBeenCalledOnce()
    expect(sidebarSpies.setMobileOpen).toHaveBeenCalledWith(false)

    await trigger.trigger('click')
    const logoutButton = view.findAll('button.dropdown-item')
      .find((button) => button.text().includes('nav.logout'))
    expect(logoutButton).toBeDefined()
    await logoutButton!.trigger('click')
    await flushPromises()

    expect(sidebarSpies.logout).toHaveBeenCalledOnce()
    expect(sidebarSpies.routerPush).toHaveBeenCalledWith('/login')
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })
})
