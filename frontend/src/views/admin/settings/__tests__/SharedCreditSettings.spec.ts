import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import SharedCreditSettings from '../SharedCreditSettings.vue'
import SharedCreditGroupSetting from '@/components/admin/group/SharedCreditGroupSetting.vue'

const api = vi.hoisted(() => ({ getState: vi.fn(), updateState: vi.fn(), getUsage: vi.fn(), setGroup: vi.fn(), groups: vi.fn() }))
vi.mock('@/api/admin/sharedCredit', () => api)
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: api.groups }))
vi.mock('vue-i18n', async (original) => ({ ...(await original<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string) => key }) }))
const activity = { name: '双十一共享额度', starts_at: null, ends_at: '2026-11-12T00:00:00Z', status: 'active' }
const state = (override = {}) => ({ ...activity, enabled: true, total_usd: '1000.00000001', used_usd: '0.1', reserved_usd: '0.2', remaining_usd: '999.70000001', group_ids: [1], revision: 7, updated_at: '', ...override })
let wrapper: VueWrapper | undefined
beforeEach(() => {
 vi.resetAllMocks()
 api.getState.mockResolvedValue(state())
 api.getUsage.mockResolvedValue({ items: [], total: 0, page: 1 })
 api.groups.mockResolvedValue([{ id: 1, name: 'Pool 1', status: 'active' }, { id: 2, name: 'Pool 2', status: 'active' }])
})
afterEach(() => wrapper?.unmount())
const mountPanel = async () => { wrapper = mount(SharedCreditSettings); await flushPromises(); return wrapper }
describe('Shared credit settings', () => {
 it('loads saved configuration and preserves decimal strings and group choices on save', async () => {
  const w = await mountPanel()
  expect((w.get('[data-testid="shared-credit-total"]').element as HTMLInputElement).value).toBe('1000.00000001')
  expect(w.get('[data-testid="shared-credit-enabled"]').attributes('aria-checked')).toBe('true')
  await w.get('[data-testid="shared-credit-total"]').setValue('1234.00000001')
  await w.get('[data-testid="shared-credit-group-2"] input').setValue(true)
  api.updateState.mockResolvedValue(state({ total_usd: '1234.00000001', group_ids: [1, 2], revision: 8 }))
  await w.get('[data-testid="shared-credit-save"]').trigger('click'); await flushPromises()
  expect(api.updateState).toHaveBeenCalledWith({ name: activity.name, starts_at: null, ends_at: activity.ends_at, enabled: true, total_usd: '1234.00000001', group_ids: [1, 2], revision: 7 })
  expect(w.text()).toContain('sharedCredit.saved')
 })
 it('keeps drafts on conflict and reloads only when explicitly requested', async () => {
  const w = await mountPanel()
  await w.get('[data-testid="shared-credit-total"]').setValue('1200')
  api.updateState.mockRejectedValue({ status: 409 })
  await w.get('[data-testid="shared-credit-save"]').trigger('click'); await flushPromises()
  expect((w.get('[data-testid="shared-credit-total"]').element as HTMLInputElement).value).toBe('1200')
  expect(api.getState).toHaveBeenCalledTimes(1)
  api.getState.mockResolvedValue(state({ total_usd: '1500', revision: 8 }))
  await w.get('[data-testid="shared-credit-reload-conflict"]').trigger('click'); await flushPromises()
  expect((w.get('[data-testid="shared-credit-total"]').element as HTMLInputElement).value).toBe('1500')
 })
 it('accepts exact used plus reserved without float rounding errors', async () => {
  const w = await mountPanel()
  await w.get('[data-testid="shared-credit-total"]').setValue('0.30000000')
  expect(w.get('[data-testid="shared-credit-save"]').attributes('disabled')).toBeUndefined()
  await w.get('[data-testid="shared-credit-total"]').setValue('0.29999999')
  expect(w.get('[data-testid="shared-credit-save"]').attributes('disabled')).toBeDefined()
 })
 it('shows group and ledger loading failures instead of false empty states', async () => {
  api.groups.mockRejectedValue(new Error('offline')); api.getUsage.mockRejectedValue(new Error('offline'))
  const w = await mountPanel()
  expect(w.text()).toContain('sharedCredit.groupsFailed'); expect(w.text()).toContain('sharedCredit.ledgerFailed')
  await w.get('[data-testid="shared-credit-total"]').setValue('1500')
  expect(w.get('[data-testid="shared-credit-save"]').attributes('disabled')).toBeDefined()
 })
 it('edits only the selected group membership using the current revision', async () => {
  wrapper = mount(SharedCreditGroupSetting, { props: { groupId: 2 } }); await flushPromises()
  await wrapper.get('[data-testid="group-shared-credit-toggle"]').trigger('click')
  api.setGroup.mockResolvedValue(state({ group_ids: [1, 2], revision: 8 }))
  await wrapper.get('[data-testid="group-shared-credit-save"]').trigger('click'); await flushPromises()
  expect(api.setGroup).toHaveBeenCalledWith(2, { enabled: true, revision: 7 }); expect(api.updateState).not.toHaveBeenCalled()
 })
 it('prevents group changes without a loaded configuration', async () => {
  api.getState.mockRejectedValue(new Error('offline'))
  wrapper = mount(SharedCreditGroupSetting, { props: { groupId: 2 } }); await flushPromises()
  expect(wrapper.get('[data-testid="group-shared-credit-toggle"]').attributes('disabled')).toBeDefined()
  expect(wrapper.get('[data-testid="group-shared-credit-save"]').attributes('disabled')).toBeDefined()
  expect(api.setGroup).not.toHaveBeenCalled()
 })
 it('requires an end time when enabled and rejects reversed date ranges', async () => {
  const w = await mountPanel()
  await w.get('[data-testid="shared-credit-ends-at"]').setValue('')
  expect(w.text()).toContain('sharedCredit.errors.endRequired')
  expect(w.get('[data-testid="shared-credit-save"]').attributes('disabled')).toBeDefined()
  await w.get('[data-testid="shared-credit-starts-at"]').setValue('2026-11-12T12:00')
  await w.get('[data-testid="shared-credit-ends-at"]').setValue('2026-11-11T12:00')
  expect(w.text()).toContain('sharedCredit.errors.invalidDates')
  expect(w.get('[data-testid="shared-credit-save"]').attributes('disabled')).toBeDefined()
  expect(api.updateState).not.toHaveBeenCalled()
 })
 it('saves local activity times as absolute instants and shows debt eligibility', async () => {
  const w = await mountPanel()
  await w.get('[data-testid="shared-credit-activity-name"]').setValue('限时共享')
  await w.get('[data-testid="shared-credit-starts-at"]').setValue('2026-11-11T09:00')
  await w.get('[data-testid="shared-credit-ends-at"]').setValue('2026-11-11T23:00')
  api.updateState.mockResolvedValue(state())
  await w.get('[data-testid="shared-credit-save"]').trigger('click'); await flushPromises()
  expect(api.updateState).toHaveBeenCalledWith(expect.objectContaining({name:'限时共享', starts_at:new Date('2026-11-11T09:00').toISOString(), ends_at:new Date('2026-11-11T23:00').toISOString()}))
  expect(w.text()).toContain('sharedCredit.activity.balanceNotice')
 })
})
