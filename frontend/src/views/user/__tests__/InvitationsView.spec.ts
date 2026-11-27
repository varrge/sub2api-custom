import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import InvitationsView from '../InvitationsView.vue'
import type { InvitationCode, InvitationOverview } from '@/api/invitations'

const { api, copyToClipboard, showSuccess, auth } = vi.hoisted(() => ({
  api: { overview: vi.fn(), apply: vi.fn(), codes: vi.fn(), rebates: vi.fn() },
  copyToClipboard: vi.fn(),
  showSuccess: vi.fn(),
  auth: { user: { id: 7 } as { id: number } | null, isEnterprise: false }
}))

vi.mock('@/api/invitations', () => ({ invitationsAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError: vi.fn() }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key) })
  }
})

const STORAGE_KEY = 'sub2api_invitation_apply:7'
const page = <T>(items: T[]) => ({ items, total: items.length, page: 1, page_size: 20, pages: 1 })

function overview(patch: Partial<InvitationOverview> = {}): InvitationOverview {
  return {
    enabled: true,
    validity_days: 30,
    auto_approve_hours: 0,
    levels: [
      { id: 'default', name: 'Starter', spend_threshold: 0, lifetime_invite_limit: 5, unused_invite_limit: 2, rebate_rate: 10 },
      { id: 'vip', name: 'VIP', spend_threshold: 100, lifetime_invite_limit: 0, unused_invite_limit: 0, rebate_rate: 15 }
    ],
    level: { id: 'default', name: 'Starter', spend_threshold: 0, lifetime_invite_limit: 5, unused_invite_limit: 2, rebate_rate: 10 },
    recharge_total: 40,
    next_level: { id: 'vip', name: 'VIP', spend_threshold: 100, lifetime_invite_limit: 0, unused_invite_limit: 0, rebate_rate: 15 },
    next_level_remaining: 60,
    override_level_id: '',
    lifetime_used: 1,
    unused_count: 1,
    summary: { pending: 2, paid: 5.5, rejected_or_revoked: 1 },
    ...patch
  }
}
function invite(patch: Partial<InvitationCode> = {}): InvitationCode {
  return {
    id: 1, code: 'NI0123456789ABCDEF', inviter_id: 7, inviter_email: 'me@example.com', invitee_id: null, invitee_email: '',
    status: 'unused', created_at: '2026-11-20T00:00:00Z', expires_at: new Date(Date.now() + 10 * 86_400_000).toISOString(), used_at: null,
    ...patch
  }
}

function mountView() {
  return mount(InvitationsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true,
        InvitationRecords: { props: ['kind', 'refreshKey'], template: '<div :data-testid="`records-${kind}`" :data-refresh="refreshKey" />' }
      }
    }
  })
}

describe('user InvitationsView', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.clearAllMocks()
    localStorage.clear()
    auth.user = { id: 7 }
    auth.isEnterprise = false
    api.overview.mockResolvedValue(overview())
    api.codes.mockResolvedValue(page([invite()]))
    api.apply.mockResolvedValue(invite({ id: 2, code: 'NIAAAABBBBCCCCDDDD' }))
    copyToClipboard.mockResolvedValue(true)
  })

  it('shows level, quota, upgrade gap, reward totals and the newest unused code', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(api.codes).toHaveBeenCalledWith({ page: 1, page_size: 1, status: 'unused' })
    expect(wrapper.get('[data-testid="invitation-level"]').text()).toContain('Starter')
    expect(wrapper.get('[data-testid="invitation-rate"]').text()).toBe('10%')
    expect(wrapper.get('[data-testid="invitation-next-level"]').text()).toContain('"level":"VIP"')
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('40')
    expect(wrapper.get('[data-testid="invitation-quota-lifetime"]').text()).toContain('"used":1,"limit":5')
    expect(wrapper.get('[data-testid="invitation-quota-unused"]').text()).toContain('"used":1,"limit":2')
    const rewards = wrapper.get('[data-testid="invitation-rewards"]').text()
    expect(rewards).toContain('$2.00')
    expect(rewards).toContain('$5.50')
    expect(rewards).toContain('$1.00')
    expect(wrapper.get('[data-testid="invitation-ticket-code"]').attributes('aria-label')).toBe('NI0123456789ABCDEF')
    expect(wrapper.get('[data-testid="invitation-issue-note"]').text()).toContain('invitations.issue.validity')
  })

  it('lets enterprise users apply and share codes without rebate UI or requests', async () => {
    auth.isEnterprise = true
    // Even an old/stale response advertising a rate must not expose enterprise rebates.
    api.overview.mockResolvedValue(overview({ auto_approve_hours: 24 }))
    api.codes.mockResolvedValue(page([invite({ rebate_eligible: false })]))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="invitation-enterprise-note"]').text()).toBe('invitations.enterprise.note')
    expect(wrapper.get('[data-testid="invitation-no-rebate"]').text()).toBe('invitations.enterprise.noRebate')
    expect(wrapper.get('[data-testid="invitation-quota-unused"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="invitation-rate"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="invitation-rewards"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="invitation-tab-rebates"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="records-rebates"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="invitation-auto-note"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('invitations.steps.three')
    expect(wrapper.text()).not.toContain('invitations.level.nextRate')
    expect(wrapper.text()).not.toContain('10%')
    expect(wrapper.text()).not.toContain('15%')
    await wrapper.get('[data-testid="copy-invitation-link"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalled()
    await wrapper.get('[data-testid="apply-invitation"]').trigger('click')
    await flushPromises()
    expect(api.apply).toHaveBeenCalledOnce()
    expect(api.rebates).not.toHaveBeenCalled()
  })

  it('honors server rebate eligibility even before enterprise identity refresh', async () => {
    api.overview.mockResolvedValue(overview({ rebate_enabled: false }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="invitation-rewards"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="invitation-tab-rebates"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="apply-invitation"]').attributes('disabled')).toBeUndefined()
  })

  it('keeps enterprise code requests subject to the same quotas and feature switch', async () => {
    auth.isEnterprise = true
    for (const patch of [{ enabled: false }, { unused_count: 2 }, { lifetime_used: 5 }]) {
      api.overview.mockResolvedValue(overview({ ...patch, rebate_enabled: false }))
      const wrapper = mountView()
      await flushPromises()
      expect(wrapper.get('[data-testid="apply-invitation"]').attributes('disabled')).toBeDefined()
      expect(wrapper.get('[data-testid="records-codes"]').exists()).toBe(true)
      if (patch.enabled === false) expect(wrapper.get('[data-testid="invitation-issue-note"]').text()).toBe('invitations.enterprise.paused')
      wrapper.unmount()
    }
    expect(api.apply).not.toHaveBeenCalled()
  })

  it('copies the code and a same-origin registration link', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="copy-invitation-code"]').trigger('click')
    expect(copyToClipboard).toHaveBeenLastCalledWith('NI0123456789ABCDEF', 'invitations.issue.codeCopied')
    await wrapper.get('[data-testid="copy-invitation-link"]').trigger('click')
    const link = new URL(copyToClipboard.mock.calls.at(-1)![0])
    expect(link.origin).toBe(window.location.origin)
    expect(link.pathname).toBe('/register')
    expect(link.searchParams.get('invitation_code')).toBe('NI0123456789ABCDEF')
  })

  it('issues a code with a UUID, clears the pending key and refreshes', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="apply-invitation"]').trigger('click')
    await flushPromises()

    const requestId = api.apply.mock.calls[0][0]
    expect(requestId).toMatch(/^[0-9a-f-]{36}$/)
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(showSuccess).toHaveBeenCalledWith('invitations.issue.applied')
    expect(api.overview).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="records-codes"]').attributes('data-refresh')).toBe('1')
    expect(wrapper.find('[data-testid="invitation-pending"]').exists()).toBe(false)
  })

  it('keeps the same request id when the answer is lost, across a reload, until confirmed', async () => {
    api.apply.mockRejectedValueOnce({ status: 0, message: 'Network error' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="apply-invitation"]').trigger('click')
    await flushPromises()

    const requestId = api.apply.mock.calls[0][0]
    expect(localStorage.getItem(STORAGE_KEY)).toBe(requestId)
    expect(wrapper.get('[data-testid="invitation-pending"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="invitation-apply-error"]').text()).toContain('invitations.issue.uncertain')
    // The plain "issue" button is replaced so a second, different request cannot be started.
    expect(wrapper.find('[data-testid="apply-invitation"]').exists()).toBe(false)
    wrapper.unmount()

    // Reload: the quota now looks full because the lost request did create the code.
    api.overview.mockResolvedValue(overview({ unused_count: 2 }))
    const reloaded = mountView()
    await flushPromises()
    expect(reloaded.get('[data-testid="invitation-pending"]').exists()).toBe(true)
    await reloaded.get('[data-testid="confirm-invitation"]').trigger('click')
    await flushPromises()

    expect(api.apply).toHaveBeenCalledTimes(2)
    expect(api.apply.mock.calls[1][0]).toBe(requestId)
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(reloaded.find('[data-testid="invitation-pending"]').exists()).toBe(false)
  })

  it('treats a business rejection as final and does not keep the request', async () => {
    api.apply.mockRejectedValueOnce({ status: 409, reason: 'INVITATION_QUOTA_EXCEEDED', message: 'raw' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="apply-invitation"]').trigger('click')
    await flushPromises()

    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(wrapper.find('[data-testid="invitation-pending"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="invitation-apply-error"]').text()).toContain('invitations.errors.INVITATION_QUOTA_EXCEEDED')
    // The overview is reloaded so the quota shown matches the server.
    expect(api.overview).toHaveBeenCalledTimes(2)
  })

  it('reuses the in-memory request when browser storage is unavailable', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('storage blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('storage blocked') })
    api.apply.mockRejectedValueOnce({ status: 0, message: 'Network error' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="apply-invitation"]').trigger('click')
    await flushPromises()
    const requestId = api.apply.mock.calls[0][0]
    await wrapper.get('[data-testid="confirm-invitation"]').trigger('click')
    await flushPromises()
    expect(api.apply).toHaveBeenCalledTimes(2)
    expect(api.apply.mock.calls[1][0]).toBe(requestId)
    wrapper.unmount()
  })

  it('lets the user stop confirming a lost request', async () => {
    localStorage.setItem(STORAGE_KEY, 'ddfd252d-dfb4-4e00-98cc-96d29e86af00')
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="discard-invitation"]').trigger('click')
    await flushPromises()

    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(api.apply).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="apply-invitation"]').exists()).toBe(true)
  })

  it('does not replay a request stored for another account', async () => {
    localStorage.setItem('sub2api_invitation_apply:8', 'ddfd252d-dfb4-4e00-98cc-96d29e86af00')
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="invitation-pending"]').exists()).toBe(false)
  })

  it('explains why issuing is unavailable and keeps history readable', async () => {
    api.codes.mockResolvedValue(page([]))
    for (const [patch, message] of [
      [{ enabled: false }, 'invitations.issue.paused'],
      [{ lifetime_used: 5 }, 'invitations.quota.lifetimeFull'],
      [{ unused_count: 2 }, 'invitations.quota.unusedFull']
    ] as const) {
      api.overview.mockResolvedValue(overview(patch))
      const wrapper = mountView()
      await flushPromises()
      expect(wrapper.get('[data-testid="invitation-issue-note"]').text()).toBe(message)
      expect(wrapper.get('[data-testid="apply-invitation"]').attributes('disabled')).toBeDefined()
      expect(wrapper.get('[data-testid="invitation-no-code"]').exists()).toBe(true)
      expect(wrapper.get('[data-testid="records-codes"]').exists()).toBe(true)
      await wrapper.get('[data-testid="apply-invitation"]').trigger('click')
      expect(api.apply).not.toHaveBeenCalled()
      wrapper.unmount()
    }
  })

  it('treats a limit of 0 as unlimited', async () => {
    api.overview.mockResolvedValue(overview({
      level: { id: 'vip', name: 'VIP', spend_threshold: 0, lifetime_invite_limit: 0, unused_invite_limit: 0, rebate_rate: 15 },
      lifetime_used: 500, unused_count: 500, next_level: null, next_level_remaining: 0
    }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="apply-invitation"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="invitation-quota-lifetime"]').text()).toContain('invitations.unlimited')
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="invitation-level"]').text()).toContain('invitations.level.highest')
  })

  it('shows a retryable error when the overview fails and recovers', async () => {
    api.overview.mockRejectedValueOnce({ status: 403, reason: 'INVITATION_PERSONAL_ONLY', message: 'raw' })
    const wrapper = mountView()
    await flushPromises()
    const error = wrapper.get('[data-testid="invitation-load-error"]')
    expect(error.text()).toContain('invitations.errors.INVITATION_PERSONAL_ONLY')
    expect(wrapper.find('[data-testid="invitation-issue"]').exists()).toBe(false)

    await error.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="invitation-load-error"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="invitation-issue"]').exists()).toBe(true)
  })

  it('loads the rebate history only when its tab is opened', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="records-rebates"]').exists()).toBe(false)
    await wrapper.get('[data-testid="invitation-tab-rebates"]').trigger('click')
    expect(wrapper.get('[data-testid="records-rebates"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="invitation-tab-rebates"]').attributes('aria-selected')).toBe('true')
  })

  it('lists every level with its rate and limits, and marks the current one', async () => {
    const wrapper = mountView()
    await flushPromises()

    const starter = wrapper.get('[data-testid="invitation-ladder-default"]')
    expect(starter.attributes('aria-current')).toBe('true')
    expect(starter.text()).toContain('invitations.level.current')
    expect(starter.text()).toContain('10%')
    expect(starter.text()).toContain('"lifetime":"5","unused":"2"')
    expect(starter.text()).toContain('invitations.level.ladderStart')

    const vip = wrapper.get('[data-testid="invitation-ladder-vip"]')
    expect(vip.attributes('aria-current')).toBeUndefined()
    expect(vip.text()).not.toContain('invitations.level.current')
    expect(vip.text()).toContain('15%')
    expect(vip.text()).toContain('$100.00')
    // A limit of 0 reads as unlimited, never as "0 codes".
    expect(vip.text()).toContain('"lifetime":"invitations.unlimited","unused":"invitations.unlimited"')
  })

  it('marks the level an administrator set, and hides the ladder when there is only one level', async () => {
    const levels = overview().levels
    api.overview.mockResolvedValue(overview({ level: levels[1], override_level_id: 'vip' }))
    let wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="invitation-ladder-vip"]').attributes('aria-current')).toBe('true')
    expect(wrapper.get('[data-testid="invitation-ladder-default"]').attributes('aria-current')).toBeUndefined()

    api.overview.mockResolvedValue(overview({ levels: [levels[0]], next_level: null, next_level_remaining: 0 }))
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="invitation-ladder"]').exists()).toBe(false)
  })

  it('says how long a rebate can wait only when automatic payment is on', async () => {
    let wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="invitation-auto-note"]').exists()).toBe(false)

    api.overview.mockResolvedValue(overview({ auto_approve_hours: 72 }))
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="invitation-auto-note"]').text()).toBe('invitations.steps.autoDays {"count":3}')

    api.overview.mockResolvedValue(overview({ auto_approve_hours: 36 }))
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="invitation-auto-note"]').text()).toBe('invitations.steps.autoHours {"count":36}')
  })
})
