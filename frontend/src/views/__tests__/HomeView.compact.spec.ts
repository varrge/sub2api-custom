import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

import HomeView from '../HomeView.vue'

const { appStore, authStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Fallback site',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    isSimpleMode: false,
    user: null as { email?: string } | null,
    checkAuth: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountHome(settings: Record<string, unknown> = {}) {
  appStore.cachedPublicSettings = {
    site_name: 'Test site',
    site_subtitle: 'Test subtitle',
    ...settings,
  }

  return mount(HomeView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: { template: '<div data-testid="locale-switcher" />' },
        Icon: { template: '<span data-testid="icon" />' },
      },
    },
  })
}

function compactDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper.get('[data-testid="compact-home"]').findComponent(RouterLinkStub).props('to')
}

function modelPlazaDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper
    .findAllComponents(RouterLinkStub)
    .find((link) => link.props('to') === '/model-plaza')
    ?.props('to')
}

describe('HomeView compact mode', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.isSimpleMode = false
    document.documentElement.classList.remove('dark')
    authStore.user = null
    authStore.checkAuth.mockClear()
    appStore.fetchPublicSettings.mockClear()
    localStorage.clear()
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })

  it('renders custom HTML ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: '<section id="custom-home">Custom home</section>',
    })

    expect(wrapper.get('#custom-home').text()).toBe('Custom home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('renders custom URL content ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: ' https://example.com/home ',
    })

    expect(wrapper.get('iframe').attributes('src')).toBe('https://example.com/home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('treats whitespace-only custom content as empty and selects compact mode', () => {
    const wrapper = mountHome({ compact_home_enabled: true, home_content: ' \n\t ' })

    expect(wrapper.get('[data-testid="compact-home"]').text()).toContain('Test site')
  })

  it.each([undefined, false])('selects the default home when compact mode is %s', (enabled) => {
    const settings = enabled === undefined ? {} : { compact_home_enabled: enabled }
    const wrapper = mountHome(settings)

    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="marketing-home"]').exists()).toBe(true)
  })

  it('links unauthenticated visitors to login', () => {
    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/login')
  })

  it('links authenticated users to their dashboard', () => {
    authStore.isAuthenticated = true

    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/dashboard')
  })

  it('links administrators to the admin dashboard', () => {
    authStore.isAuthenticated = true
    authStore.isAdmin = true

    const wrapper = mountHome({ compact_home_enabled: true })
    expect(compactDestination(wrapper)).toBe('/admin/dashboard')
    expect(authStore.checkAuth).toHaveBeenCalledOnce()
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it.each([true, false])('shows the public model plaza in compact=%s', (compact) => {
    const wrapper = mountHome({
      compact_home_enabled: compact,
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it.each([true, false])('hides the private model plaza from visitors in compact=%s', (compact) => {
    const wrapper = mountHome({
      compact_home_enabled: compact,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it.each([true, false])('shows the private model plaza to signed-in users in compact=%s', (compact) => {
    authStore.isAuthenticated = true

    const wrapper = mountHome({
      compact_home_enabled: compact,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('shows the model plaza link in the default home header', () => {
    const wrapper = mountHome({
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it.each([true, false])('hides the disabled model plaza in compact=%s', (compact) => {
    const wrapper = mountHome({
      compact_home_enabled: compact,
      model_plaza_enabled: false,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it.each([
    [false, false, '/login'],
    [true, false, '/dashboard'],
    [true, true, '/admin/dashboard'],
  ])('routes marketing CTA for authenticated=%s admin=%s', (authenticated, admin, destination) => {
    authStore.isAuthenticated = authenticated as boolean
    authStore.isAdmin = admin as boolean
    const wrapper = mountHome()
    expect(wrapper.getComponent('[data-testid="home-primary-cta"]').props('to')).toBe(destination)
  })

  it('offers the existing purchase route when payments are enabled', () => {
    const wrapper = mountHome({ payment_enabled: true, registration_enabled: false })
    expect(wrapper.getComponent('[data-testid="home-purchase-cta"]').props('to')).toBe('/purchase')
    expect(wrapper.findAllComponents(RouterLinkStub).some((link) => link.props('to') === '/register')).toBe(false)
  })

  it.each(['disabled', 'admin', 'simple'])('hides the purchase CTA for %s', (reason) => {
    authStore.isAdmin = reason === 'admin'
    authStore.isSimpleMode = reason === 'simple'
    const wrapper = mountHome({ payment_enabled: reason !== 'disabled' })
    expect(wrapper.find('[data-testid="home-purchase-cta"]').exists()).toBe(false)
  })

  it('preserves site branding and only renders safe documentation links', () => {
    const wrapper = mountHome({ doc_url: 'https://example.com/docs', site_logo: '/brand.svg' })
    expect(wrapper.get('.brand').text()).toContain('Test site')
    expect(wrapper.get('.brand img').attributes('src')).toBe('/brand.svg')
    expect(wrapper.get('.hero .eyebrow').text()).toBe('Test subtitle')
    expect(wrapper.findAll('a[href="https://example.com/docs"]').length).toBeGreaterThan(0)
    const unsafe = mountHome({ doc_url: 'javascript:alert(1)' })
    expect(unsafe.find('a[href^="javascript:"]').exists()).toBe(false)
  })

  it('switches illustrative scenarios without making a model request', async () => {
    const wrapper = mountHome()
    const buttons = wrapper.findAll('.scenarios button')
    await buttons[1].trigger('click')
    expect(buttons[0].attributes('aria-pressed')).toBe('false')
    expect(buttons[1].attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('#home-demo-content').text()).toContain('home.marketing.demo.write.result')
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it('applies and persists the theme preference from the marketing header', async () => {
    const wrapper = mountHome()
    await wrapper.get('.theme-button').trigger('click')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(wrapper.get('[data-testid="marketing-home"]').classes()).toContain('is-dark')
  })
})
