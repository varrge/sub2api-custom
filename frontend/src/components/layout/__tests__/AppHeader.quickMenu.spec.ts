import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'

import AppHeader from '../AppHeader.vue'

const state = vi.hoisted(() => ({
  routePath: '/dashboard',
  routeName: 'Dashboard',
  isAdmin: false,
  modelPlazaEnabled: true,
  supportTicketsEnabled: true,
  userUnread: 7,
  adminUnread: 3,
  settings: {
    top_quick_menu_items: [] as string[],
    custom_menu_items: [],
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
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
    user: { email: 'user@example.com', role: state.isAdmin ? 'admin' : 'user', balance: 0 },
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
    modelPlaza: 'modelPlaza',
    supportTicket: 'supportTicket',
  },
  isFeatureFlagEnabled: (flag: string) =>
    flag === 'modelPlaza' ? state.modelPlazaEnabled : state.supportTicketsEnabled,
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
    state.modelPlazaEnabled = true
    state.supportTicketsEnabled = true
    state.settings.top_quick_menu_items = []
  })

  afterEach(() => wrapper?.unmount())

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
})
