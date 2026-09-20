import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import zh from '@/i18n/locales/zh'
import CouponEntry from '../CouponEntry.vue'
import { monthCardFeeCNY } from '../model'
import { groupBuyAPI } from '@/api/groupBuy'
import type { CouponQuote, MonthCardSelection } from '@/types/groupBuy'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => { const message: unknown = key.split('.').reduce<unknown>((obj, part) => (obj as Record<string, unknown>)?.[part], zh); return String(message || key).replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? '')) } }) }))
vi.mock('@/api/groupBuy', () => ({ groupBuyAPI: { previewCoupon: vi.fn() } }))
const selection: MonthCardSelection = { mode: 'create', product: { id: 1, group_id: 2, name: 'Monthly', group_name: 'OpenAI', platform: 'openai', description: '', price_cny: 198, base_quota_usd: 940, tiers: [], max_members: 10, recruitment_hours: 48, for_sale: true, sort_order: 0 } }
const quote: CouponQuote = { coupon_id: 2, code: 'SAVE10', original_cny: 198, discount_cny: 19.8, amount_cny: 178.2 }
function setup() {
  return mount(CouponEntry, { props: { selection, modelValue: null } })
}

describe('payment coupon entry', () => {
  it.each([[30, 1.1, 0.33], [178.2, 1.1, 1.97], [0.01, 0.01, 0.01], [100, 0, 0]])('matches backend fee rounding for %s at %s%%', (amount, rate, fee) => {
    expect(monthCardFeeCNY(amount, rate)).toBe(fee)
  })
  it('starts collapsed, validates on Apply, and allows removing the discount', async () => {
    vi.mocked(groupBuyAPI.previewCoupon).mockResolvedValue(quote)
    const wrapper = setup()
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.get('button').text()).toBe('有优惠码？')
    await wrapper.get('button').trigger('click')
    await wrapper.get('input').setValue(' SAVE10 ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(groupBuyAPI.previewCoupon).toHaveBeenCalledWith(selection, 'SAVE10')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([quote])
    await wrapper.setProps({ modelValue: quote })
    expect(wrapper.text()).toContain('已优惠 ¥19.80')
    await wrapper.findAll('button').find(button => button.text() === '移除')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([null])
    wrapper.unmount()
  })

  it('clears a previous discount and shows an invalid-code error', async () => {
    vi.mocked(groupBuyAPI.previewCoupon).mockRejectedValue({ message: '优惠码已过期' })
    const wrapper = setup()
    await wrapper.setProps({ modelValue: quote })
    await wrapper.get('button').trigger('click')
    await wrapper.get('input').setValue('EXPIRED')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([null])
    await wrapper.setProps({ modelValue: null })
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('优惠码已过期')
    expect(wrapper.emitted('busy')?.at(-1)).toEqual([false])
    wrapper.unmount()
  })

  it.each(['edit', 'switch', 'unmount'] as const)('ignores a stale response after %s', async action => {
    let resolve!: (value: CouponQuote) => void
    vi.mocked(groupBuyAPI.previewCoupon).mockReturnValue(new Promise(done => { resolve = done }))
    const wrapper = setup()
    await wrapper.get('button').trigger('click')
    await wrapper.get('input').setValue('SAVE10')
    await wrapper.get('form').trigger('submit')
    if (action === 'edit') await wrapper.get('input').setValue('SOMETHINGELSE')
    else if (action === 'switch') await wrapper.setProps({ selection: { ...selection, mode: 'solo' } })
    else wrapper.unmount()
    resolve(quote)
    await flushPromises()
    expect(wrapper.emitted('update:modelValue')?.some(([value]) => value !== null) ?? false).toBe(false)
    if (action !== 'unmount') wrapper.unmount()
  })
})
