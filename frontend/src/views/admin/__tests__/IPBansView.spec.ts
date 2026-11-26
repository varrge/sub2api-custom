import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

import IPBansView from '../IPBansView.vue'

const api = vi.hoisted(() => ({
  list: vi.fn(),
  summary: vi.fn(),
  create: vi.fn(),
  revoke: vi.fn(),
  getSettings: vi.fn(),
  saveSettings: vi.fn(),
}))
const toast = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))

vi.mock('@/api/admin/ipBans', () => ({ ipBansAPI: api, default: api }))
vi.mock('@/api/client', () => ({ apiClient: {} }))
vi.mock('@/stores', () => ({ useAppStore: () => toast }))
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))
vi.mock('vue-i18n', () => ({
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'zh' } }, install: () => {} }),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key),
    te: () => true,
  }),
}))

enableAutoUnmount(afterEach)

const ban = {
  id: 7, ip: '57.181.38.31', reason: 'honeypot_email', source: 'auto',
  detail: { attempted_email: 'admin@sub2api.local', path: '/api/v1/auth/login' },
  note: '', created_by: 'system', created_at: '2026-11-24T11:00:00Z', hits: 3,
  last_hit_at: '2026-11-24T11:05:00Z', revoked_at: null, revoked_by: '', revoke_note: '',
}

function mountView() {
  return mount(IPBansView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Pagination: true,
        Select: { props: ['modelValue', 'options'], template: '<select></select>' },
        DataTable: {
          props: ['data'],
          template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-ip" :row="row" /><slot name="cell-detail" :row="row" /><slot name="cell-actions" :row="row" /></div></div>',
        },
        BaseDialog: {
          props: ['show', 'title'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
      },
    },
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  api.summary.mockResolvedValue({ active_total: 1, by_reason: { honeypot_email: 1 }, trusted_proxy_configured: false, enabled: true })
  api.list.mockResolvedValue({ items: [ban], total: 1, page: 1, page_size: 20, pages: 1 })
  api.revoke.mockResolvedValue(undefined)
  api.create.mockResolvedValue({ ...ban, id: 8, ip: '203.0.113.9', reason: 'manual', source: 'manual' })
  api.getSettings.mockResolvedValue({
    enabled: true, honeypot_emails: ['admin@sub2api.local'], ban_sensitive_probes: true,
    admin_login_failures: 5, admin_login_window_minutes: 15, admin_unauthorized: 10,
    admin_unauthorized_window_minutes: 10, allowlist: [], honeypot_conflicts: ['admin@admin.com'],
  })
  api.saveSettings.mockImplementation(async (s: Record<string, unknown>) => ({ ...s, honeypot_conflicts: [] }))
})

describe('admin IPBansView', () => {
  it('lists active bans with the attempted account and warns when trusted proxies are missing', async () => {
    const w = mountView()
    await flushPromises()
    expect(api.list).toHaveBeenCalledWith(expect.objectContaining({ status: 'active', page: 1 }))
    expect(w.text()).toContain('57.181.38.31')
    expect(w.text()).toContain('admin@sub2api.local')
    expect(w.find('[data-testid="ip-ban-proxy-warning"]').exists()).toBe(true)
  })

  it('lifts a ban from the admin panel', async () => {
    const w = mountView()
    await flushPromises()
    await w.get('[data-testid="ip-ban-revoke-7"]').trigger('click')
    await w.get('#ip-ban-revoke-note').setValue('误封')
    await w.get('[data-testid="ip-ban-revoke-submit"]').trigger('click')
    await flushPromises()
    expect(api.revoke).toHaveBeenCalledWith(7, '误封')
    expect(toast.showSuccess).toHaveBeenCalled()
    expect(api.list).toHaveBeenCalledTimes(2)
  })

  it('bans an address manually', async () => {
    const w = mountView()
    await flushPromises()
    await w.get('[data-testid="ip-ban-add"]').trigger('click')
    await w.get('#ip-ban-create-ip').setValue('203.0.113.9')
    await w.get('#ip-ban-create-note').setValue('scanner')
    await w.get('#ip-ban-create-form').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith('203.0.113.9', 'scanner')
  })

  it('edits rules, shows decoy conflicts and saves one entry per line', async () => {
    const w = mountView()
    await flushPromises()
    await w.get('[data-testid="ip-ban-tab-rules"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="ip-ban-honeypot-conflict"]').text()).toContain('admin@admin.com')
    await w.get('#ip-ban-honeypots').setValue('admin@sub2api.local\nroot@sub2api.local\n')
    await w.get('#ip-ban-allowlist').setValue('198.51.100.7, 203.0.113.0/24')
    await w.get('[data-testid="ip-ban-admin-login"]').setValue('3')
    await w.get('[data-testid="ip-ban-rules"] form').trigger('submit')
    await flushPromises()
    expect(api.saveSettings).toHaveBeenCalledWith(expect.objectContaining({
      honeypot_emails: ['admin@sub2api.local', 'root@sub2api.local'],
      allowlist: ['198.51.100.7', '203.0.113.0/24'],
      admin_login_failures: 3,
    }))
    expect(w.find('[data-testid="ip-ban-honeypot-conflict"]').exists()).toBe(false)
  })
})
