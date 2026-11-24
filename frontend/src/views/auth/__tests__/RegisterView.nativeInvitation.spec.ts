import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import RegisterView from '@/views/auth/RegisterView.vue'

const { getPublicSettingsMock, validateInvitationCodeMock, registerMock, pushMock, showErrorMock, route } = vi.hoisted(() => ({
  showErrorMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
  validateInvitationCodeMock: vi.fn(),
  registerMock: vi.fn(),
  pushMock: vi.fn(),
  route: { query: {} as Record<string, unknown> }
}))

const publicSettings = {
  registration_enabled: true,
  email_verify_enabled: false,
  promo_code_enabled: false,
  // The global invite-only gate is off: a native invitation link must still work.
  invitation_code_enabled: false,
  affiliate_enabled: false,
  turnstile_enabled: false,
  turnstile_site_key: '',
  site_name: 'Sub2API',
  registration_email_suffix_whitelist: [],
  linuxdo_oauth_enabled: false,
  wechat_oauth_enabled: false,
  oidc_oauth_enabled: false,
  github_oauth_enabled: true,
  google_oauth_enabled: false
}

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
  useRoute: () => route
}))
vi.mock('vue-i18n', () => ({
  createI18n: () => ({ global: { t: (key: string) => key } }),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } })
}))
vi.mock('@/stores', () => ({
  useAuthStore: () => ({ register: (...args: unknown[]) => registerMock(...args) }),
  useAppStore: () => ({ showError: (...args: unknown[]) => showErrorMock(...args), showSuccess: vi.fn(), showWarning: vi.fn() })
}))
vi.mock('@/api/auth', async () => {
  const actual = await vi.importActual<typeof import('@/api/auth')>('@/api/auth')
  return {
    ...actual,
    getPublicSettings: (...args: unknown[]) => getPublicSettingsMock(...args),
    validateInvitationCode: (...args: unknown[]) => validateInvitationCodeMock(...args)
  }
})

function mountRegister() {
  return mount(RegisterView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        Icon: true,
        TurnstileWidget: true,
        LoginAgreementPrompt: true,
        EmailOAuthButtons: {
          emits: ['start'],
          template: '<button data-testid="oauth-start" @click="$emit(\'start\', { provider: \'github\', params: { redirect: \'/dashboard\' } })" />'
        },
        LinuxDoOAuthSection: true,
        WechatOAuthSection: true,
        OidcOAuthSection: true,
        RouterLink: true,
        teleport: true,
        transition: false
      }
    }
  })
}

async function settle() {
  await flushPromises()
  // The code is validated after a 500 ms debounce.
  await vi.advanceTimersByTimeAsync(600)
  await flushPromises()
}

describe('RegisterView native invitation link', () => {
  const originalLocation = window.location

  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    sessionStorage.clear()
    route.query = {}
    getPublicSettingsMock.mockResolvedValue(publicSettings)
    validateInvitationCodeMock.mockResolvedValue({ valid: true })
    registerMock.mockResolvedValue({})
  })
  afterEach(() => {
    vi.useRealTimers()
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
  })

  it('hides the invitation field when no code was supplied and the gate is off', async () => {
    const wrapper = mountRegister()
    await settle()
    expect(wrapper.find('#invitation_code').exists()).toBe(false)
    expect(validateInvitationCodeMock).not.toHaveBeenCalled()
  })

  it('prefills, validates and submits the code from the share link, then forgets it', async () => {
    route.query = { invitation_code: ' NI0123456789ABCDEF ' }
    const wrapper = mountRegister()
    await settle()

    const field = wrapper.get<HTMLInputElement>('#invitation_code')
    expect(field.element.value).toBe('NI0123456789ABCDEF')
    expect(wrapper.text()).toContain('common.optional')
    expect(validateInvitationCodeMock).toHaveBeenCalledWith('NI0123456789ABCDEF')
    expect(sessionStorage.getItem('registration_invitation_code')).toBe('NI0123456789ABCDEF')

    await wrapper.get('#email').setValue('new@example.com')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(registerMock).toHaveBeenCalledWith(expect.objectContaining({ email: 'new@example.com', invitation_code: 'NI0123456789ABCDEF' }))
    // It is a registration invitation, never an affiliate code.
    expect(registerMock.mock.calls[0][0]).not.toHaveProperty('aff_code')
    expect(sessionStorage.getItem('registration_invitation_code')).toBeNull()
  })

  it('restores the code after the page is reloaded without the query string', async () => {
    sessionStorage.setItem('registration_invitation_code', 'NI0123456789ABCDEF')
    const wrapper = mountRegister()
    await settle()
    expect(wrapper.get<HTMLInputElement>('#invitation_code').element.value).toBe('NI0123456789ABCDEF')
  })

  it('refuses to register with a code the server reports as invalid', async () => {
    validateInvitationCodeMock.mockResolvedValue({ valid: false, error_code: 'INVITATION_CODE_USED' })
    route.query = { invitation_code: 'NIUSED' }
    const wrapper = mountRegister()
    await settle()

    await wrapper.get('#email').setValue('new@example.com')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(registerMock).not.toHaveBeenCalled()
    // This form reports field problems through the toast, like its other fields.
    expect(showErrorMock).toHaveBeenCalledWith('auth.invitationCodeInvalid')
    expect(wrapper.get('#invitation_code').classes()).toContain('border-red-500')
    // The code stays so the user can see and correct it.
    expect(sessionStorage.getItem('registration_invitation_code')).toBe('NIUSED')
  })

  it('lets the user remove the code and register without it', async () => {
    route.query = { invitation_code: 'NI0123456789ABCDEF' }
    const wrapper = mountRegister()
    await settle()

    await wrapper.get('#invitation_code').setValue('')
    await wrapper.get('#email').setValue('new@example.com')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(registerMock).toHaveBeenCalledTimes(1)
    expect(registerMock.mock.calls[0][0].invitation_code).toBeUndefined()
    expect(sessionStorage.getItem('registration_invitation_code')).toBeNull()
  })

  it('carries the code into email verification', async () => {
    getPublicSettingsMock.mockResolvedValue({ ...publicSettings, email_verify_enabled: true })
    route.query = { invitation_code: 'NI0123456789ABCDEF' }
    const wrapper = mountRegister()
    await settle()

    await wrapper.get('#email').setValue('new@example.com')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(registerMock).not.toHaveBeenCalled()
    expect(pushMock).toHaveBeenCalledWith('/email-verify')
    expect(JSON.parse(sessionStorage.getItem('register_data') || '{}')).toMatchObject({
      email: 'new@example.com',
      invitation_code: 'NI0123456789ABCDEF'
    })
    // Kept until the account actually exists.
    expect(sessionStorage.getItem('registration_invitation_code')).toBe('NI0123456789ABCDEF')
  })

  it('sends the code when an OAuth sign-up is started', async () => {
    const location = { href: '', origin: 'http://localhost', pathname: '/register', search: '', hash: '' }
    Object.defineProperty(window, 'location', { configurable: true, value: location })
    route.query = { invitation_code: 'NI0123456789ABCDEF' }
    const wrapper = mountRegister()
    await settle()

    await wrapper.get('.auth-alternative-trigger').trigger('click')
    await wrapper.get('[data-testid="oauth-start"]').trigger('click')
    await flushPromises()

    const started = new URL(location.href, 'http://localhost')
    expect(started.pathname).toContain('/auth/oauth/github/start')
    expect(started.searchParams.get('invitation_code')).toBe('NI0123456789ABCDEF')
    expect(started.searchParams.get('redirect')).toBe('/dashboard')
    expect(sessionStorage.getItem('registration_invitation_code')).toBe('NI0123456789ABCDEF')
  })

  it('does not add an invitation parameter to OAuth when there is no code', async () => {
    const location = { href: '', origin: 'http://localhost', pathname: '/register', search: '', hash: '' }
    Object.defineProperty(window, 'location', { configurable: true, value: location })
    const wrapper = mountRegister()
    await settle()

    await wrapper.get('.auth-alternative-trigger').trigger('click')
    await wrapper.get('[data-testid="oauth-start"]').trigger('click')
    await flushPromises()
    expect(new URL(location.href, 'http://localhost').searchParams.has('invitation_code')).toBe(false)
  })
})
