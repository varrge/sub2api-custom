import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import InvitationsView from '../InvitationsView.vue'
import type { InvitationCode, InvitationRebate, InvitationSettings, InvitationStats } from '@/api/invitations'

const { api, listUsers, showSuccess, showError, leaveGuards } = vi.hoisted(() => ({
  api: {
    stats: vi.fn(), settings: vi.fn(), saveSettings: vi.fn(), codes: vi.fn(), rebates: vi.fn(),
    userOverview: vi.fn(), setLevel: vi.fn(), review: vi.fn(), disableCode: vi.fn(), overrides: vi.fn()
  },
  listUsers: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  leaveGuards: [] as Array<() => unknown>
}))

vi.mock('@/api/invitations', () => ({ invitationsAPI: api }))
vi.mock('@/api/admin/users', () => ({ list: listUsers, default: { list: listUsers } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))
vi.mock('vue-router', () => ({ onBeforeRouteLeave: (guard: () => unknown) => leaveGuards.push(guard) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key) })
  }
})

const page = <T>(items: T[]) => ({ items, total: items.length, page: 1, page_size: 20, pages: 1 })

function settings(): InvitationSettings {
  return {
    enabled: true,
    validity_days: 30,
    auto_approve_hours: 0,
    levels: [
      { id: 'default', name: 'Starter', spend_threshold: 0, lifetime_invite_limit: 3, unused_invite_limit: 3, rebate_rate: 10 },
      { id: 'vip', name: 'VIP', spend_threshold: 100, lifetime_invite_limit: 0, unused_invite_limit: 10, rebate_rate: 15 }
    ]
  }
}
function stats(patch: Partial<InvitationStats> = {}): InvitationStats {
  return {
    total_invites: 12, used_invites: 5, active_invites: 4, pending_count: 2, pending_amount: 7.5,
    approved_count: 3, approved_amount: 20, declined_count: 1, declined_amount: 4, overridden_users: 1, ...patch
  }
}
function rebate(patch: Partial<InvitationRebate> = {}): InvitationRebate {
  return {
    id: 31, invite_id: 9, invite_code: 'NI0123456789ABCDEF', inviter_id: 7, inviter_email: 'inviter@example.com',
    invitee_id: 8, invitee_email: 'invitee@example.com', source_type: 'payment', source_id: 55, source_currency: 'CNY', source_status: 'COMPLETED',
    source_occurred_at: '2026-11-20T00:00:00Z', first_amount: 50, rebate_rate: 10, rebate_amount: 5, level_name: 'Starter',
    status: 'pending', review_reason: '', auto_approved: false, created_at: '2026-11-20T00:00:00Z', reviewed_at: null, ...patch
  }
}
function code(patch: Partial<InvitationCode> = {}): InvitationCode {
  return {
    id: 14, code: 'NI7F3A9C21B4E05D68', inviter_id: 7, inviter_email: 'inviter@example.com', invitee_id: null, invitee_email: '',
    status: 'unused', created_at: '2026-11-20T00:00:00Z', expires_at: '2026-12-20T00:00:00Z', used_at: null, ...patch
  }
}

const stubs = {
  AppLayout: { template: '<main><slot /></main>' },
  Icon: true,
  Pagination: true,
  TotpStepUpDialog: {
    props: ['controller'],
    template: '<div v-if="controller.visible.value" data-testid="invitation-step-up"><button data-testid="step-up-verify" @click="controller.onVerified()">Verify</button><button data-testid="step-up-cancel" @click="controller.onCancel()">Cancel</button></div>'
  },
  BaseDialog: { props: ['show', 'title'], template: '<div v-if="show" role="dialog"><h3>{{ title }}</h3><slot /><slot name="footer" /></div>' },
  Select: {
    props: ['modelValue', 'options'],
    emits: ['update:modelValue'],
    template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option></select>'
  },
  DataTable: {
    props: ['columns', 'data', 'loading'],
    template: '<div><table><tr v-for="row in data" :key="row.id"><td v-for="c in columns" :key="c.key"><slot :name="`cell-${c.key}`" :row="row" /></td></tr></table><slot v-if="!loading && !data.length" name="empty" /></div>'
  }
}

async function mountView() {
  const wrapper = mount(InvitationsView, { global: { stubs }, attachTo: document.body })
  await flushPromises()
  return wrapper
}
async function openTab(wrapper: Awaited<ReturnType<typeof mountView>>, key: string) {
  await wrapper.get(`[data-testid="admin-invitation-tab-${key}"]`).trigger('click')
  await flushPromises()
}

describe('admin InvitationsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    leaveGuards.length = 0
    api.stats.mockResolvedValue(stats())
    api.settings.mockResolvedValue(settings())
    api.saveSettings.mockImplementation(async (value: InvitationSettings) => value)
    api.codes.mockResolvedValue(page([]))
    api.rebates.mockResolvedValue(page([rebate()]))
    api.disableCode.mockImplementation(async (id: number) => code({ id, status: 'disabled' }))
    api.overrides.mockResolvedValue(page([]))
    api.review.mockImplementation(async (id: number, action: string) => rebate({ id, status: action === 'approve' ? 'approved' : action === 'reject' ? 'rejected' : 'revoked' }))
  })
  afterEach(() => {
    document.body.innerHTML = ''
    vi.restoreAllMocks()
  })

  it('opens on the pending review queue and shows the work-queue totals', async () => {
    const wrapper = await mountView()

    expect(api.rebates).toHaveBeenCalledWith({ page: 1, page_size: 20, status: 'pending' }, true)
    expect(api.codes).not.toHaveBeenCalled()
    const summary = wrapper.get('[data-testid="invitation-stats"]').text()
    expect(summary).toContain('$7.50')
    expect(summary).toContain('"count":2')
    expect(summary).toContain('$20.00')
    expect(summary).toContain('"used":5,"active":4')
    expect(wrapper.get('[data-testid="admin-invitation-tab-review"]').text()).toContain('2')
    expect(wrapper.find('[data-testid="invitation-paused"]').exists()).toBe(false)
  })

  it('flags paused issuance without hiding records', async () => {
    api.settings.mockResolvedValue({ ...settings(), enabled: false })
    const wrapper = await mountView()
    expect(wrapper.get('[data-testid="invitation-paused"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="rebate-approve-31"]').exists()).toBe(true)
  })

  it('approves with an optional note, states the consequence, then refreshes list and totals', async () => {
    const wrapper = await mountView()
    await wrapper.get('[data-testid="rebate-approve-31"]').trigger('click')

    const consequence = wrapper.get('[data-testid="invitation-review-consequence"]').text()
    expect(consequence).toContain('invitations.review.approveBody')
    expect(consequence).toContain('inviter@example.com')
    expect(consequence).toContain('$5.00')
    // First amount stays in its own currency; the reward is the USD figure.
    expect(wrapper.get('[data-testid="invitation-review-form"]').text()).toContain('CN¥50.00')
    expect(wrapper.get('[data-testid="invitation-review-submit"]').text()).toContain('invitations.review.approveSubmit')

    await wrapper.get('[data-testid="invitation-review-form"]').trigger('submit')
    await flushPromises()

    expect(api.review).toHaveBeenCalledWith(31, 'approve', '')
    expect(showSuccess).toHaveBeenCalledWith('invitations.review.approveDone')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(api.rebates).toHaveBeenCalledTimes(2)
    expect(api.stats).toHaveBeenCalledTimes(2)
  })

  it('prompts for step-up before retrying an approval and preserves its payload', async () => {
    api.review.mockRejectedValueOnce({ reason: 'STEP_UP_REQUIRED' })
    const wrapper = await mountView()
    await wrapper.get('[data-testid="rebate-approve-31"]').trigger('click')
    await wrapper.get('[data-testid="invitation-review-reason"]').setValue('checked payment')
    await wrapper.get('[data-testid="invitation-review-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-testid="invitation-step-up"]').exists()).toBe(true)
    expect(api.review).toHaveBeenCalledTimes(1)
    await wrapper.get('[data-testid="step-up-verify"]').trigger('click')
    await flushPromises()
    expect(api.review).toHaveBeenCalledTimes(2)
    expect(api.review).toHaveBeenLastCalledWith(31, 'approve', 'checked payment')
    wrapper.unmount()
  })

  it('cancels step-up without retrying or displaying an operation failure', async () => {
    api.review.mockRejectedValueOnce({ reason: 'STEP_UP_REQUIRED' })
    const wrapper = await mountView()
    await wrapper.get('[data-testid="rebate-approve-31"]').trigger('click')
    await wrapper.get('[data-testid="invitation-review-form"]').trigger('submit')
    await flushPromises()
    await wrapper.get('[data-testid="step-up-cancel"]').trigger('click')
    await flushPromises()
    expect(api.review).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="invitation-review-error"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('removes an override without requiring an active personal account overview', async () => {
    api.overrides.mockResolvedValueOnce(page([{ user_id: 7, email: 'deleted@example.com', level_id: 'vip', updated_at: '2026-11-20T00:00:00Z' }]))
    api.setLevel.mockResolvedValueOnce(null)
    const wrapper = await mountView()
    await openTab(wrapper, 'users')
    await wrapper.get('[data-testid="invitation-override-clear-7"]').trigger('click')
    await flushPromises()
    expect(api.setLevel).toHaveBeenCalledWith(7, '')
    expect(api.userOverview).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('invitations.users.levelSaved')
    expect(wrapper.find('[data-testid="invitation-override-7"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('requires a reason to reject and to revoke', async () => {
    const wrapper = await mountView()
    await wrapper.get('[data-testid="rebate-reject-31"]').trigger('click')
    await wrapper.get('[data-testid="invitation-review-form"]').trigger('submit')
    await flushPromises()
    expect(api.review).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="invitation-review-reason"]').attributes('aria-invalid')).toBe('true')
    expect(wrapper.get('[role="dialog"]').text()).toContain('invitations.review.reasonRequired')

    await wrapper.get('[data-testid="invitation-review-reason"]').setValue('  duplicate account  ')
    await wrapper.get('[data-testid="invitation-review-form"]').trigger('submit')
    await flushPromises()
    expect(api.review).toHaveBeenCalledWith(31, 'reject', 'duplicate account')

    api.rebates.mockResolvedValue(page([rebate({ status: 'approved' })]))
    const second = await mountView()
    expect(second.find('[data-testid="rebate-approve-31"]').exists()).toBe(false)
    await second.get('[data-testid="rebate-revoke-31"]').trigger('click')
    expect(second.get('[data-testid="invitation-review-consequence"]').text()).toContain('invitations.review.revokeBody')
    await second.get('[data-testid="invitation-review-reason"]').setValue('refunded')
    await second.get('[data-testid="invitation-review-form"]').trigger('submit')
    await flushPromises()
    expect(api.review).toHaveBeenLastCalledWith(31, 'revoke', 'refunded')
  })

  it('keeps the dialog open on failure and refreshes when another admin already acted', async () => {
    api.review.mockRejectedValueOnce({ status: 500, message: 'database unavailable' })
    const wrapper = await mountView()
    await wrapper.get('[data-testid="rebate-approve-31"]').trigger('click')
    await wrapper.get('[data-testid="invitation-review-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="invitation-review-error"]').text()).toContain('database unavailable')
    expect(api.rebates).toHaveBeenCalledTimes(1)

    api.review.mockRejectedValueOnce({ status: 409, reason: 'INVITATION_CONFLICT', message: 'raw' })
    await wrapper.get('[data-testid="invitation-review-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="invitation-review-error"]').text()).toContain('invitations.review.stale')
    expect(api.rebates).toHaveBeenCalledTimes(2)
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('validates level rules next to the field and saves trimmed, numeric values', async () => {
    const wrapper = await mountView()
    await openTab(wrapper, 'rules')

    const save = wrapper.get('[data-testid="invitation-settings-save"]')
    expect(save.attributes('disabled')).toBeDefined()
    // Existing identifiers are locked: user overrides and past rebates refer to them.
    expect(wrapper.get('[data-testid="invitation-level-0-id"]').attributes('readonly')).toBeDefined()

    await wrapper.get('[data-testid="invitation-level-1-rebate_rate"]').setValue('120')
    await wrapper.get('[data-testid="invitation-level-1-spend_threshold"]').setValue('0')
    await wrapper.get('[data-testid="invitation-settings"]').trigger('submit')
    await flushPromises()
    expect(api.saveSettings).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="invitation-level-1-rebate_rate-issue"]').text()).toBe('invitations.issues.range')
    expect(wrapper.get('[data-testid="invitation-level-1-spend_threshold-issue"]').text()).toBe('invitations.issues.duplicate')
    expect(wrapper.get('[data-testid="invitation-settings-issues"]').text()).toContain('"count":2')
    expect(wrapper.get('[data-testid="admin-invitation-tab-rules"]').find('[role="img"]').attributes('aria-label')).toBe('invitations.tabs.unsaved')

    await wrapper.get('[data-testid="invitation-level-1-rebate_rate"]').setValue('12.5')
    await wrapper.get('[data-testid="invitation-level-1-spend_threshold"]').setValue('200')
    await wrapper.get('[data-testid="invitation-level-1-name"]').setValue('  Gold  ')
    await wrapper.get('[data-testid="invitation-validity"]').setValue('14')
    await wrapper.get('[data-testid="invitation-auto-approve"]').setValue('72')
    await wrapper.get('[data-testid="invitation-enabled"]').trigger('click')
    await wrapper.get('[data-testid="invitation-settings"]').trigger('submit')
    await flushPromises()

    expect(api.saveSettings).toHaveBeenCalledWith({
      enabled: false,
      validity_days: 14,
      auto_approve_hours: 72,
      levels: [
        { id: 'default', name: 'Starter', spend_threshold: 0, lifetime_invite_limit: 3, unused_invite_limit: 3, rebate_rate: 10 },
        { id: 'vip', name: 'Gold', spend_threshold: 200, lifetime_invite_limit: 0, unused_invite_limit: 10, rebate_rate: 12.5 }
      ]
    })
    expect(showSuccess).toHaveBeenCalledWith('invitations.settings.saved')
    expect(wrapper.get('[data-testid="invitation-settings-save"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="invitation-paused"]').exists()).toBe(true)
  })

  it('adds, removes and resets levels, and never removes the last one', async () => {
    const wrapper = await mountView()
    await openTab(wrapper, 'rules')

    await wrapper.get('[data-testid="invitation-add-level"]').trigger('click')
    expect(wrapper.get<HTMLInputElement>('[data-testid="invitation-level-2-id"]').element.value).toBe('level-3')
    expect(wrapper.get<HTMLInputElement>('[data-testid="invitation-level-2-spend_threshold"]').element.value).toBe('200')
    expect(wrapper.get('[data-testid="invitation-level-2-id"]').attributes('readonly')).toBeUndefined()
    // A new level without a name blocks saving with a field message.
    await wrapper.get('[data-testid="invitation-settings"]').trigger('submit')
    expect(wrapper.get('[data-testid="invitation-level-2-name-issue"]').text()).toBe('invitations.issues.required')
    expect(api.saveSettings).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="invitation-settings-reset"]').trigger('click')
    expect(wrapper.find('[data-testid="invitation-level-2"]').exists()).toBe(false)

    await wrapper.get('[data-testid="invitation-remove-level-1"]').trigger('click')
    expect(wrapper.find('[data-testid="invitation-level-1"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="invitation-remove-level-0"]').attributes('disabled')).toBeDefined()
  })

  it('explains a rejected save and keeps the edits', async () => {
    api.saveSettings.mockRejectedValueOnce({ status: 409, reason: 'INVITATION_LEVEL_IN_USE', message: 'raw' })
    const wrapper = await mountView()
    await openTab(wrapper, 'rules')
    await wrapper.get('[data-testid="invitation-remove-level-1"]').trigger('click')
    await wrapper.get('[data-testid="invitation-settings"]').trigger('submit')
    await flushPromises()

    expect(wrapper.get('[data-testid="invitation-settings-error"]').text()).toContain('invitations.errors.INVITATION_LEVEL_IN_USE')
    expect(wrapper.find('[data-testid="invitation-level-1"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="invitation-settings-save"]').attributes('disabled')).toBeUndefined()
  })

  it('asks before leaving with unsaved rules', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const wrapper = await mountView()
    expect(leaveGuards[0]()).toBe(true)
    expect(confirm).not.toHaveBeenCalled()

    await openTab(wrapper, 'rules')
    await wrapper.get('[data-testid="invitation-validity"]').setValue('7')
    expect(leaveGuards[0]()).toBe(false)
    expect(confirm).toHaveBeenCalledWith('invitations.settings.leaveConfirm')
  })

  it('finds a personal account, sets its level and jumps to its records', async () => {
    listUsers.mockResolvedValue(page([{ id: 7, email: 'inviter@example.com' }]))
    const overview = {
      enabled: true, validity_days: 30, auto_approve_hours: 0, levels: settings().levels, level: settings().levels[0], recharge_total: 10, next_level: settings().levels[1],
      next_level_remaining: 90, override_level_id: '', lifetime_used: 1, unused_count: 1, summary: { pending: 0, paid: 0, rejected_or_revoked: 0 }
    }
    api.userOverview.mockResolvedValue(overview)
    api.setLevel.mockResolvedValue({ ...overview, level: settings().levels[1], override_level_id: 'vip' })

    const wrapper = await mountView()
    await openTab(wrapper, 'users')
    await wrapper.get('[data-testid="invitation-user-search"]').setValue(' inviter ')
    await wrapper.get('[data-testid="invitation-user-level"] form').trigger('submit')
    await flushPromises()
    expect(listUsers).toHaveBeenCalledWith(1, 20, { search: 'inviter', account_type: 'personal' })

    await wrapper.get('[data-testid="invitation-user-7"]').trigger('click')
    await flushPromises()
    expect(api.userOverview).toHaveBeenCalledWith(7)
    const panel = wrapper.get('[data-testid="invitation-user-level"]')
    expect(panel.get('[data-testid="invitation-level"]').text()).toContain('Starter')
    expect(panel.get('[data-testid="invitation-user-save"]').attributes('disabled')).toBeDefined()

    await panel.get('select').setValue('vip')
    await panel.findAll('form')[1].trigger('submit')
    await flushPromises()
    // Submitting again with nothing changed must not write a second audit entry.
    await panel.findAll('form')[1].trigger('submit')
    await flushPromises()
    expect(api.setLevel).toHaveBeenCalledTimes(1)
    expect(api.setLevel).toHaveBeenCalledWith(7, 'vip')
    expect(panel.get('[data-testid="invitation-level"]').text()).toContain('VIP')
    expect(panel.get('[data-testid="invitation-level"]').text()).toContain('invitations.level.assigned')
    expect(showSuccess).toHaveBeenCalledWith('invitations.users.levelSaved')

    await panel.get('[data-testid="invitation-user-codes"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="admin-invitation-tab-codes"]').attributes('aria-selected')).toBe('true')
    expect(api.codes).toHaveBeenCalledWith({ page: 1, page_size: 20, user_id: 7 }, true)
    expect(wrapper.get('[data-testid="invitation-records-codes"] [data-testid="invitation-records-user"]').text()).toContain('"id":7')

    // A user's rebate history shows every status, not just the pending queue.
    await openTab(wrapper, 'users')
    await wrapper.get('[data-testid="invitation-user-rebates"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="admin-invitation-tab-review"]').attributes('aria-selected')).toBe('true')
    expect(api.rebates).toHaveBeenLastCalledWith({ page: 1, page_size: 20, user_id: 7 }, true)

    // The pending total returns to the queue across all inviters.
    await wrapper.get('[data-testid="invitation-stats-pending"]').trigger('click')
    await flushPromises()
    expect(api.rebates).toHaveBeenLastCalledWith({ page: 1, page_size: 20, status: 'pending' }, true)
  })

  it('does not save an emptied number box as 0 (0 would mean unlimited)', async () => {
    const wrapper = await mountView()
    await openTab(wrapper, 'rules')
    await wrapper.get('[data-testid="invitation-level-0-lifetime_invite_limit"]').setValue('')
    await wrapper.get('[data-testid="invitation-validity"]').setValue('')
    await wrapper.get('[data-testid="invitation-settings"]').trigger('submit')
    await flushPromises()

    expect(api.saveSettings).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="invitation-level-0-lifetime_invite_limit-issue"]').text()).toBe('invitations.issues.required')
    expect(wrapper.get('[data-testid="invitation-validity"]').attributes('aria-invalid')).toBe('true')
    expect(wrapper.get('[data-testid="invitation-settings-issues"]').text()).toContain('"count":2')
  })

  it('opens the inviter from a record and reports a missing account', async () => {
    api.userOverview.mockRejectedValue({ status: 404, reason: 'INVITATION_NOT_FOUND', message: 'raw' })
    const wrapper = await mountView()
    await wrapper.get('[data-testid="invitation-records-rebates"] td button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="admin-invitation-tab-users"]').attributes('aria-selected')).toBe('true')
    expect(wrapper.get('[data-testid="invitation-user-error"]').text()).toContain('invitations.errors.INVITATION_NOT_FOUND')
  })

  it('filters by status and search text, and offers to clear filters when nothing matches', async () => {
    vi.useFakeTimers()
    try {
      const wrapper = await mountView()
      const list = wrapper.get('[data-testid="invitation-records-rebates"]')
      api.rebates.mockResolvedValue(page([]))

      await list.get('select').setValue('approved')
      await flushPromises()
      expect(api.rebates).toHaveBeenLastCalledWith({ page: 1, page_size: 20, status: 'approved' }, true)

      await list.get('[data-testid="invitation-records-search"]').setValue('  inviter@example.com ')
      expect(api.rebates).toHaveBeenCalledTimes(2)
      await vi.advanceTimersByTimeAsync(400)
      await flushPromises()
      expect(api.rebates).toHaveBeenLastCalledWith({ page: 1, page_size: 20, status: 'approved', q: 'inviter@example.com' }, true)

      const empty = list.get('[data-testid="invitation-records-empty"]')
      expect(empty.text()).toContain('invitations.records.emptyFiltered')
      await empty.get('button').trigger('click')
      await flushPromises()
      expect(api.rebates).toHaveBeenLastCalledWith({ page: 1, page_size: 20 }, true)
      expect(list.get('[data-testid="invitation-records-empty"]').text()).toContain('invitations.records.emptyRebates')
    } finally {
      vi.useRealTimers()
    }
  })

  it('shows retryable errors for the totals and the rules', async () => {
    api.stats.mockRejectedValueOnce({ status: 500, message: 'stats down' })
    api.settings.mockRejectedValueOnce({ status: 500, message: 'settings down' })
    const wrapper = await mountView()
    expect(wrapper.get('[data-testid="invitation-stats"]').text()).toContain('stats down')
    await openTab(wrapper, 'rules')
    const error = wrapper.get('[data-testid="invitation-settings-load-error"]')
    expect(error.text()).toContain('settings down')

    await error.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="invitation-settings"]').exists()).toBe(true)
    await wrapper.get('[data-testid="invitation-stats"] button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="invitation-stats"]').text()).toContain('$7.50')
  })

  it('explains the waiting period, and never saves an emptied box as 0 (0 turns automatic payment off)', async () => {
    api.settings.mockResolvedValue({ ...settings(), auto_approve_hours: 24 })
    const wrapper = await mountView()
    await openTab(wrapper, 'rules')
    const hint = () => wrapper.get('#invitation-auto-approve-hint').text()
    expect(wrapper.get<HTMLInputElement>('[data-testid="invitation-auto-approve"]').element.value).toBe('24')
    expect(hint()).toContain('invitations.settings.autoApproveOn')

    await wrapper.get('[data-testid="invitation-auto-approve"]').setValue('')
    await wrapper.get('[data-testid="invitation-settings"]').trigger('submit')
    await flushPromises()
    expect(api.saveSettings).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="invitation-auto-approve-issue"]').text()).toContain('invitations.issues.required')
    expect(wrapper.get('[data-testid="invitation-auto-approve"]').attributes('aria-invalid')).toBe('true')

    await wrapper.get('[data-testid="invitation-auto-approve"]').setValue('9000')
    expect(wrapper.get('[data-testid="invitation-auto-approve-issue"]').text()).toContain('invitations.issues.range')

    await wrapper.get('[data-testid="invitation-auto-approve"]').setValue('0')
    expect(hint()).toContain('invitations.settings.autoApproveOff')
    await wrapper.get('[data-testid="invitation-settings"]').trigger('submit')
    await flushPromises()
    expect(api.saveSettings).toHaveBeenCalledWith(expect.objectContaining({ auto_approve_hours: 0 }))
  })

  it('warns before approving a rebate whose first payment was refunded or never completed', async () => {
    api.rebates.mockResolvedValue(page([
      rebate({ id: 31, source_status: 'REFUNDED' }),
      rebate({ id: 32, source_status: 'RECHARGING' }),
      rebate({ id: 33, source_status: 'COMPLETED' }),
      rebate({ id: 34, source_type: 'redeem_code', source_status: '' })
    ]))
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="rebate-source-concern-31"]').text()).toBe('invitations.records.sourceRefund {"status":"REFUNDED"}')
    // A refund reads as an error, an unfinished order as a caution.
    expect(wrapper.get('[data-testid="rebate-source-concern-31"]').classes()).toContain('text-red-700')
    expect(wrapper.get('[data-testid="rebate-source-concern-32"]').classes()).toContain('text-amber-700')
    expect(wrapper.get('[data-testid="rebate-source-concern-32"]').text()).toBe('invitations.records.sourceIncomplete {"status":"RECHARGING"}')
    expect(wrapper.find('[data-testid="rebate-source-concern-33"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="rebate-source-concern-34"]').exists()).toBe(false)

    await wrapper.get('[data-testid="rebate-approve-31"]').trigger('click')
    expect(wrapper.get('[data-testid="invitation-review-warning"]').text()).toBe('invitations.review.refundWarning {"status":"REFUNDED"}')
    // Rejecting a refunded one is the expected action and needs no warning.
    await wrapper.get('[data-testid="rebate-reject-31"]').trigger('click')
    expect(wrapper.find('[data-testid="invitation-review-warning"]').exists()).toBe(false)

    await wrapper.get('[data-testid="rebate-approve-32"]').trigger('click')
    expect(wrapper.get('[data-testid="invitation-review-warning"]').text()).toBe('invitations.review.incompleteWarning {"status":"RECHARGING"}')
    await wrapper.get('[data-testid="rebate-approve-33"]').trigger('click')
    expect(wrapper.find('[data-testid="invitation-review-warning"]').exists()).toBe(false)
  })

  it('labels a rebate that the waiting period paid, not an administrator', async () => {
    api.rebates.mockResolvedValue(page([
      rebate({ id: 41, status: 'approved', auto_approved: true, reviewed_at: '2026-11-22T00:00:00Z' }),
      rebate({ id: 42, status: 'approved', reviewed_at: '2026-11-22T00:00:00Z' })
    ]))
    const wrapper = await mountView()
    const labels = wrapper.findAll('[data-testid="invitation-auto-approved"]')
    expect(labels).toHaveLength(1)
    expect(labels[0].text()).toBe('invitations.records.autoApproved')
    // It can still be taken back.
    expect(wrapper.get('[data-testid="rebate-revoke-41"]').exists()).toBe(true)
  })

  it('voids only unused codes, after a confirmation that names the inviter', async () => {
    api.codes.mockResolvedValue(page([
      code({ id: 14 }),
      code({ id: 11, status: 'used', invitee_id: 31, invitee_email: 'invitee@example.com' }),
      code({ id: 9, status: 'expired' }),
      code({ id: 8, status: 'disabled' })
    ]))
    const wrapper = await mountView()
    await openTab(wrapper, 'codes')

    for (const id of [11, 9, 8]) expect(wrapper.find(`[data-testid="invite-disable-${id}"]`).exists(), String(id)).toBe(false)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)

    await wrapper.get('[data-testid="invite-disable-14"]').trigger('click')
    const dialog = wrapper.get('[role="dialog"]')
    expect(dialog.text()).toContain('invitations.records.disableBody {"email":"inviter@example.com"}')
    expect(dialog.text()).toContain('NI7F3A9C21B4E05D68')
    expect(api.disableCode).not.toHaveBeenCalled()

    // Cancel leaves the code alone.
    await dialog.findAll('button')[0].trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(api.disableCode).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="invite-disable-14"]').trigger('click')
    await wrapper.get('[role="dialog"]').findAll('button')[1].trigger('click')
    await flushPromises()
    expect(api.disableCode).toHaveBeenCalledTimes(1)
    expect(api.disableCode).toHaveBeenCalledWith(14)
    expect(showSuccess).toHaveBeenCalledWith('invitations.records.disabled')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    // The list and the totals above it are reloaded.
    expect(api.codes).toHaveBeenCalledTimes(2)
    expect(api.stats).toHaveBeenCalledTimes(2)
  })

  it('reports a code that was used a moment ago and reloads to show its real state', async () => {
    api.codes.mockResolvedValue(page([code({ id: 14 })]))
    api.disableCode.mockRejectedValueOnce({ status: 409, reason: 'INVITATION_CONFLICT', message: 'raw' })
    const wrapper = await mountView()
    await openTab(wrapper, 'codes')
    await wrapper.get('[data-testid="invite-disable-14"]').trigger('click')
    await wrapper.get('[role="dialog"]').findAll('button')[1].trigger('click')
    await flushPromises()

    expect(showSuccess).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledTimes(1)
    expect(showError.mock.calls[0][0]).toContain('invitations.records.disableFailed')
    expect(showError.mock.calls[0][0]).toContain('invitations.errors.INVITATION_CONFLICT')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(api.codes).toHaveBeenCalledTimes(2)
    // Totals did not change, so they are not refetched.
    expect(api.stats).toHaveBeenCalledTimes(1)
  })

  it('lists users with a set level, opens one, and pages through the rest', async () => {
    const first = Array.from({ length: 20 }, (_, i) => ({ user_id: 100 + i, email: `u${100 + i}@example.com`, level_id: 'vip', updated_at: '2026-11-20T00:00:00Z' }))
    api.overrides
      .mockResolvedValueOnce({ items: first, total: 21, page: 1, page_size: 20, pages: 2 })
      .mockResolvedValueOnce({ items: [{ user_id: 300, email: 'last@example.com', level_id: 'default', updated_at: '2026-11-01T00:00:00Z' }], total: 21, page: 2, page_size: 20, pages: 2 })
    api.userOverview.mockResolvedValue({
      enabled: true, validity_days: 30, auto_approve_hours: 0, levels: settings().levels, level: settings().levels[1], recharge_total: 0,
      next_level: settings().levels[1], next_level_remaining: 100, override_level_id: 'vip', lifetime_used: 0, unused_count: 0,
      summary: { pending: 0, paid: 0, rejected_or_revoked: 0 }
    })
    const wrapper = await mountView()
    await openTab(wrapper, 'users')

    expect(api.overrides).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    const section = wrapper.get('[data-testid="invitation-overrides"]')
    expect(section.findAll('li')).toHaveLength(20)
    // The level is shown by name, not by its identifier.
    expect(section.get('[data-testid="invitation-override-100"]').text()).toContain('VIP')
    expect(section.get('[data-testid="invitation-override-100"]').text()).toContain('u100@example.com')

    await section.get('[data-testid="invitation-overrides-more"]').trigger('click')
    await flushPromises()
    expect(api.overrides).toHaveBeenLastCalledWith({ page: 2, page_size: 20 })
    expect(section.findAll('li')).toHaveLength(21)
    expect(section.get('[data-testid="invitation-override-300"]').text()).toContain('Starter')
    expect(section.find('[data-testid="invitation-overrides-more"]').exists()).toBe(false)

    await section.get('[data-testid="invitation-override-100"]').trigger('click')
    await flushPromises()
    expect(api.userOverview).toHaveBeenCalledWith(100)
    expect(section.get('[data-testid="invitation-override-100"]').attributes('aria-current')).toBe('true')
  })

  it('says so when nobody has a set level, and offers a retry when the list fails', async () => {
    const wrapper = await mountView()
    await openTab(wrapper, 'users')
    expect(wrapper.get('[data-testid="invitation-overrides-empty"]').text()).toBe('invitations.users.overridesEmpty')

    api.overrides.mockRejectedValueOnce({ status: 500, message: 'overrides down' })
    const second = await mountView()
    await openTab(second, 'users')
    const error = second.get('[data-testid="invitation-overrides-error"]')
    expect(error.text()).toContain('overrides down')
    expect(second.find('[data-testid="invitation-overrides-empty"]').exists()).toBe(false)
    await error.get('button').trigger('click')
    await flushPromises()
    expect(second.find('[data-testid="invitation-overrides-error"]').exists()).toBe(false)
    expect(second.get('[data-testid="invitation-overrides-empty"]').exists()).toBe(true)
  })
})
