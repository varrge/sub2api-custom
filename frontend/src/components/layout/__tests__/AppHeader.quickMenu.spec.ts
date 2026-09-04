import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'

import AppHeader from '../AppHeader.vue'

const state = vi.hoisted(() => ({
  routePath: '/dashboard',
  routeName: 'Dashboard',
  isAdmin: false,
  imageGenerationEnabled: true,
  modelPlazaEnabled: true,
  supportTicketsEnabled: true,
  paymentEnabled: true,
  userUnread: 7,
  adminUnread: 3,
  balance: 42.5 as number | undefined,
  frozenBalance: 7.5,
  userRefreshStatus: 'success' as 'idle' | 'loading' | 'success' | 'error',
  settings: {
    top_quick_menu_items: [] as string[],
    custom_menu_items: [],
  },
}))

const routerPush = vi.hoisted(() => vi.fn())

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
  useRoute: () => ({
    get path() { return state.routePath },
    get name() { return state.routeName },
    params: {},
    meta: {},
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    contactInfo: '',
    docUrl: '',
    cachedPublicSettings: state.settings,
    toggleMobileSidebar: vi.fn(),
  }),
  useAuthStore: () => ({
    user: {
      email: 'user@example.com',
      role: state.isAdmin ? 'admin' : 'user',
      balance: state.balance,
      frozen_balance: state.frozenBalance,
    },
    get userRefreshStatus() { return state.userRefreshStatus },
    get isAdmin() { return state.isAdmin },
    isSimpleMode: false,
    logout: vi.fn(),
  }),
  useOnboardingStore: () => ({ replay: vi.fn() }),
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/stores/supportTickets', () => ({
  useSupportTicketStore: () => ({
    get userUnreadCount() { return state.userUnread },
    get adminUnreadCount() { return state.adminUnread },
  }),
}))

vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: {
    imageGeneration: 'imageGeneration',
    modelPlaza: 'modelPlaza',
    supportTicket: 'supportTicket',
    payment: 'payment',
  },
  isFeatureFlagEnabled: (flag: string) =>
    flag === 'imageGeneration'
      ? state.imageGenerationEnabled
      : flag === 'modelPlaza'
        ? state.modelPlazaEnabled
        : flag === 'payment'
          ? state.paymentEnabled
          : state.supportTicketsEnabled,
}))

const RouterLinkStub = defineComponent({
  props: ['to'],
  setup(props, { attrs, slots }) {
    return () => h('a', { ...attrs, 'data-to': JSON.stringify(props.to) }, slots.default?.())
  },
})

let wrapper: VueWrapper | undefined

function mountHeader() {
  wrapper = mount(AppHeader, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        AnnouncementBell: true,
        LocaleSwitcher: true,
        SubscriptionProgressMini: true,
        Icon: { template: '<i />' },
      },
    },
  })
  return wrapper
}

describe('AppHeader top quick menu', () => {
  beforeEach(() => {
    state.routePath = '/dashboard'
    state.routeName = 'Dashboard'
    state.isAdmin = false
    state.imageGenerationEnabled = true
    state.modelPlazaEnabled = true
    state.supportTicketsEnabled = true
    state.paymentEnabled = true
    state.balance = 42.5
    state.frozenBalance = 7.5
    state.userRefreshStatus = 'success'
    state.settings.top_quick_menu_items = []
    routerPush.mockReset()
    localStorage.clear()
    document.documentElement.classList.remove('dark')
  })

  afterEach(() => {
    wrapper?.unmount()
    document.documentElement.classList.remove('dark')
  })

  it('renders configured items in order and moves model plaza out of its legacy slot', () => {
    state.settings.top_quick_menu_items = ['model_plaza', 'support_tickets', 'api_keys']
    const view = mountHeader()

    expect(view.find('[data-testid="model-plaza-legacy-entry"]').exists()).toBe(false)
    expect(
      view.findAll('[data-testid^="top-quick-menu-"]').map((node) => node.attributes('data-testid')),
    ).toEqual([
      'top-quick-menu-dashboard',
      'top-quick-menu-model_plaza',
      'top-quick-menu-support_tickets',
      'top-quick-menu-api_keys',
    ])
    expect(view.get('[data-testid="top-quick-menu-support_tickets"]').text()).toContain('7')
  })

  it('uses admin routes and highlights ticket child pages', () => {
    state.isAdmin = true
    state.routePath = '/admin/tickets/42'
    state.settings.top_quick_menu_items = ['support_tickets', 'usage']
    const view = mountHeader()

    const dashboard = view.get('[data-testid="top-quick-menu-dashboard"]')
    const tickets = view.get('[data-testid="top-quick-menu-support_tickets"]')
    const usage = view.get('[data-testid="top-quick-menu-usage"]')
    expect(dashboard.attributes('data-to')).toBe('"/admin/dashboard"')
    expect(tickets.attributes('data-to')).toBe('"/admin/tickets"')
    expect(tickets.attributes('aria-current')).toBe('page')
    expect(tickets.text()).toContain('3')
    expect(usage.attributes('data-to')).toBe('"/admin/usage"')
  })

  it('hides disabled configured features and retains the legacy model plaza entry when unselected', () => {
    state.supportTicketsEnabled = false
    state.settings.top_quick_menu_items = ['support_tickets', 'api_keys']
    const view = mountHeader()

    expect(view.find('[data-testid="top-quick-menu-support_tickets"]').exists()).toBe(false)
    expect(view.find('[data-testid="top-quick-menu-api_keys"]').exists()).toBe(true)
    expect(view.find('[data-testid="model-plaza-legacy-entry"]').exists()).toBe(true)
  })

  it('hides image generation from the quick menu when the feature is disabled', () => {
    state.imageGenerationEnabled = false
    state.settings.top_quick_menu_items = ['image_generation', 'api_keys']
    const view = mountHeader()

    expect(view.find('[data-testid="top-quick-menu-image_generation"]').exists()).toBe(false)
    expect(view.find('[data-testid="top-quick-menu-api_keys"]').exists()).toBe(true)
  })

  it.each([
    ['/docs/image-generation', 'ImageGeneration', 'image_generation'],
    ['/docs/batch-image', 'BatchImageGuide', 'batch_image'],
  ])('highlights %s through its legacy alias', (path, routeName, itemID) => {
    state.routePath = path
    state.routeName = routeName
    state.settings.top_quick_menu_items = ['image_generation', 'batch_image']
    const view = mountHeader()

    expect(view.get(`[data-testid="top-quick-menu-${itemID}"]`).attributes('aria-current')).toBe('page')
  })

  it('ends the header actions with announcement, theme, and available balance', async () => {
    const view = mountHeader()

    expect(
      view.findAll('[data-testid^="header-"]').slice(0, 3).map((node) => node.attributes('data-testid')),
    ).toEqual(['header-announcement', 'header-theme-toggle', 'header-balance'])
    expect(view.get('[data-testid="header-balance"]').classes()).not.toContain('hidden')
    expect(view.get('[data-testid="header-balance"]').text()).toContain('$42.50')

    await view.get('[data-testid="header-balance"]').trigger('click')
    expect(routerPush).toHaveBeenCalledWith('/purchase')
  })

  it('keeps balance visible but inert when payment is disabled', async () => {
    state.paymentEnabled = false
    const view = mountHeader()
    const balance = view.get('[data-testid="header-balance"]')

    expect(balance.attributes('aria-disabled')).toBe('true')
    expect(balance.text()).toContain('$42.50')
    await balance.trigger('click')
    expect(routerPush).not.toHaveBeenCalled()
  })

  it('formats a real zero balance as $0.00', () => {
    state.balance = 0
    const view = mountHeader()

    expect(view.get('[data-testid="header-balance"]').text()).toContain('$0.00')
  })

  it('uses a skeleton during refresh even with cached balance and $-- after failure', () => {
    state.userRefreshStatus = 'loading'
    const loadingView = mountHeader()
    expect(loadingView.get('[data-testid="header-balance"]').find('.animate-pulse').exists()).toBe(true)
    expect(loadingView.find('[data-testid="header-balance-details"]').exists()).toBe(false)
    loadingView.unmount()

    state.balance = undefined
    state.userRefreshStatus = 'error'
    const errorView = mountHeader()
    expect(errorView.get('[data-testid="header-balance"]').text()).toContain('$--')
  })

  it('toggles and persists the theme from the header', async () => {
    const view = mountHeader()

    await view.get('[data-testid="header-theme-toggle"]').trigger('click')

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
  })
})
