import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import AdminGiftCards from '../AdminGiftCards.vue'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import type { AdminGroup } from '@/types'
import type { GiftCardBatch, GiftCardPreview } from '@/types/groupBuy'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/groupBuy', () => ({ adminGroupBuyAPI: { previewGift: vi.fn(), issueGift: vi.fn(), giftBatches: vi.fn() } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
const auth = reactive({ user: { id: 30 } })
const preview: GiftCardPreview = { recipient_count: 3, total_quota_usd: '45', preview_token: 'a'.repeat(64) }
const batch: GiftCardBatch = { id: 9, name: '狂蹬卡', group_id: 2, group_name: 'OpenAI', source_group_id: 0, include_frozen: true, quota_usd: '15', valid_days: 7, actor_user_id: 30, recipient_count: 3, created_at: '2026-11-02T00:00:00Z', expires_at: '2026-11-09T00:00:00Z' }
const options = {
  props: { groups: [{ id: 2, name: 'OpenAI', status: 'active', subscription_type: 'subscription' }] as AdminGroup[] },
  global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show" role="dialog"><slot /><slot name="footer" /></div>' } } }
}
const wrappers: ReturnType<typeof shallowMount>[] = []
function mount() { const wrapper = shallowMount(AdminGiftCards, options); wrappers.push(wrapper); return wrapper }
async function prepare(wrapper: ReturnType<typeof mount>) {
  await wrapper.get('[data-testid="gift-target"]').setValue('2')
  await wrapper.get('[data-testid="gift-quota"]').setValue('15')
  await wrapper.get('[data-testid="gift-preview"]').trigger('click')
  await flushPromises()
}
async function confirm(wrapper: ReturnType<typeof mount>) {
  await wrapper.get('[data-testid="gift-open-confirm"]').trigger('click')
  await wrapper.get('[data-testid="gift-issue"]').trigger('click')
  await flushPromises()
}
beforeEach(() => {
  vi.resetAllMocks(); sessionStorage.clear(); auth.user = { id: 30 }
  vi.mocked(adminGroupBuyAPI.previewGift).mockResolvedValue(preview)
  vi.mocked(adminGroupBuyAPI.issueGift).mockResolvedValue(batch)
  vi.mocked(adminGroupBuyAPI.giftBatches).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
})
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); sessionStorage.clear(); vi.restoreAllMocks() })

describe('batch gift issuance', () => {
  it('previews without issuing and invalidates preview when configuration changes', async () => {
    const wrapper = mount()
    await prepare(wrapper)
    expect(adminGroupBuyAPI.previewGift).toHaveBeenCalledWith({ name: '狂蹬卡', group_id: 2, source_group_id: 0, include_frozen: true, quota_usd: '15', valid_days: 7 })
    expect(adminGroupBuyAPI.issueGift).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="gift-open-confirm"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-testid="gift-quota"]').setValue('20')
    expect(wrapper.get('[data-testid="gift-open-confirm"]').attributes('disabled')).toBeDefined()
  })
  it('requires confirmation, prevents double sends and refreshes the batch history', async () => {
    let resolve!: (result: GiftCardBatch) => void
    vi.mocked(adminGroupBuyAPI.issueGift).mockReturnValue(new Promise(done => { resolve = done }))
    const wrapper = mount()
    await prepare(wrapper)
    await wrapper.get('[data-testid="gift-open-confirm"]').trigger('click')
    expect(adminGroupBuyAPI.issueGift).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="gift-issue"]').trigger('click')
    await wrapper.get('[data-testid="gift-issue"]').trigger('click')
    expect(adminGroupBuyAPI.issueGift).toHaveBeenCalledTimes(1)
    expect(sessionStorage.getItem('month-card-gift-pending:30')).toBeTruthy()
    resolve(batch); await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('groupBuy.gifts.issued')
    expect(sessionStorage.getItem('month-card-gift-pending:30')).toBeNull()
    expect(adminGroupBuyAPI.giftBatches).toHaveBeenLastCalledWith(1)
  })
  it('recovers a real flattened 409 by allowing a new preview and request', async () => {
    vi.mocked(adminGroupBuyAPI.issueGift).mockRejectedValueOnce({ status: 409, message: 'Audience changed' })
    const wrapper = mount()
    await prepare(wrapper); await confirm(wrapper)
    const original = vi.mocked(adminGroupBuyAPI.issueGift).mock.calls[0][0].request_id
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="gift-preview"]').attributes('disabled')).toBeUndefined()
    expect(sessionStorage.getItem('month-card-gift-pending:30')).toBeNull()
    await prepare(wrapper); await confirm(wrapper)
    expect(vi.mocked(adminGroupBuyAPI.issueGift).mock.calls[1][0].request_id).not.toBe(original)
  })
  it('restores an uncertain request after remount and retries the same batch', async () => {
    vi.mocked(adminGroupBuyAPI.issueGift).mockRejectedValueOnce(new Error('network response lost'))
    const wrapper = mount()
    await prepare(wrapper); await confirm(wrapper)
    const request = vi.mocked(adminGroupBuyAPI.issueGift).mock.calls[0][0]
    expect(wrapper.get('[data-testid="gift-quota"]').attributes('disabled')).toBeDefined()
    wrapper.unmount(); wrappers.pop()
    const restored = mount(); await flushPromises()
    expect(restored.get('[role="dialog"]').text()).toContain('groupBuy.gifts.retryHint')
    await restored.get('[data-testid="gift-issue"]').trigger('click'); await flushPromises()
    expect(vi.mocked(adminGroupBuyAPI.issueGift).mock.calls[1][0]).toEqual(request)
    expect(restored.find('[role="dialog"]').exists()).toBe(false)
  })
  it('ignores an old account preview response', async () => {
    let resolve!: (result: GiftCardPreview) => void
    vi.mocked(adminGroupBuyAPI.previewGift).mockReturnValue(new Promise(done => { resolve = done }))
    const wrapper = mount()
    await prepare(wrapper)
    auth.user = { id: 31 }; await flushPromises()
    resolve(preview); await flushPromises()
    expect(wrapper.get('[data-testid="gift-open-confirm"]').attributes('disabled')).toBeDefined()
    expect(adminGroupBuyAPI.issueGift).not.toHaveBeenCalled()
  })
  it('does not send if a durable retry receipt cannot be stored', async () => {
    const wrapper = mount(); await prepare(wrapper)
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('storage disabled') })
    await confirm(wrapper)
    expect(adminGroupBuyAPI.issueGift).not.toHaveBeenCalled()
    expect(wrapper.get('[role="dialog"]').text()).toContain('groupBuy.gifts.receiptFailed')
  })
})
