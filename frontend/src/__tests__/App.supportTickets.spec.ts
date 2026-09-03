import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  app: null as any,
  auth: null as any,
  supportTickets: null as any,
  fetchSubscriptions: vi.fn(),
  startPolling: vi.fn(),
  clearSubscriptions: vi.fn(),
  fetchAnnouncements: vi.fn(),
  resetAnnouncements: vi.fn(),
  fetchCompliance: vi.fn(),
  resetCompliance: vi.fn(),
  requireAcknowledgement: vi.fn(),
  afterEach: vi.fn(),
  replace: vi.fn(),
  getSetupStatus: vi.fn(),
}))

vi.mock('vue-router', () => ({
  RouterView: { template: '<div />' },
  useRoute: () => reactive({ fullPath: '/', path: '/', name: 'Home', params: {}, meta: {} }),
  useRouter: () => ({ afterEach: mocks.afterEach, replace: mocks.replace }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => mocks.app,
  useAuthStore: () => mocks.auth,
  useSupportTicketStore: () => mocks.supportTickets,
  useSubscriptionStore: () => ({
    fetchActiveSubscriptions: mocks.fetchSubscriptions,
    startPolling: mocks.startPolling,
    clear: mocks.clearSubscriptions,
  }),
  useAnnouncementStore: () => ({
    fetchAnnouncements: mocks.fetchAnnouncements,
    reset: mocks.resetAnnouncements,
  }),
  useAdminComplianceStore: () => ({
    fetchStatus: mocks.fetchCompliance,
    reset: mocks.resetCompliance,
    requireAcknowledgement: mocks.requireAcknowledgement,
  }),
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/api/setup', () => ({ getSetupStatus: mocks.getSetupStatus }))
vi.mock('@/utils/branding', () => ({ updateFavicon: vi.fn() }))

import App from '../App.vue'

describe('App support ticket unread lifecycle', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.getSetupStatus.mockResolvedValue({ needs_setup: false })
    mocks.fetchSubscriptions.mockResolvedValue(undefined)
    mocks.fetchAnnouncements.mockResolvedValue(undefined)
    mocks.fetchCompliance.mockResolvedValue(undefined)
    mocks.app = reactive({
      cachedPublicSettings: { support_ticket_enabled: true, custom_menu_items: [] },
      siteLogo: '',
      siteName: 'Sub2API',
      fetchPublicSettings: vi.fn().mockResolvedValue(undefined),
    })
    mocks.auth = reactive({ isAuthenticated: true, isAdmin: true })
    const initializeUserUnread = vi.fn(async () => {
      mocks.supportTickets.userUnreadLoaded = true
      return 2
    })
    const resetUserUnread = vi.fn(() => {
      mocks.supportTickets.userUnreadCount = 0
      mocks.supportTickets.userUnreadLoaded = false
    })
    mocks.supportTickets = reactive({
      userUnreadCount: 2,
      adminUnreadCount: 7,
      userUnreadLoaded: false,
      adminUnreadLoaded: true,
      initializeUserUnread,
      initializeAdminUnread: vi.fn().mockResolvedValue(7),
      resetUserUnread,
      reset: vi.fn(),
    })
  })

  it('resets the user cache on disable and fetches it again on re-enable without resetting admin state', async () => {
    const wrapper = mount(App, {
      global: {
        stubs: {
          Toast: true,
          NavigationProgress: true,
          AdminComplianceDialog: true,
          AnnouncementPopup: true,
        },
      },
    })
    await flushPromises()
    mocks.supportTickets.initializeUserUnread.mockClear()

    mocks.app.cachedPublicSettings.support_ticket_enabled = false
    await nextTick()
    expect(mocks.supportTickets.resetUserUnread).toHaveBeenCalledOnce()
    expect(mocks.supportTickets.adminUnreadCount).toBe(7)
    expect(mocks.supportTickets.adminUnreadLoaded).toBe(true)

    mocks.app.cachedPublicSettings.support_ticket_enabled = true
    await flushPromises()
    expect(mocks.supportTickets.initializeUserUnread).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
})
