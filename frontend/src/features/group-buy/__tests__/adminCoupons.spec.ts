import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminCoupons from '../AdminCoupons.vue'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import type { GroupBuyProduct, MonthCardCoupon } from '@/types/groupBuy'
import zh from '@/i18n/locales/zh'

vi.mock('@/components/common/BaseDialog.vue', () => ({ default: { props: ['show', 'title'], template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /></section>' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => {
  const message = key.split('.').reduce((obj: any, part) => obj?.[part], zh) || key
  return String(message).replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? ''))
} }) }))
vi.mock('@/api/groupBuy', () => ({ adminGroupBuyAPI: { coupons: vi.fn(), saveCoupon: vi.fn() } }))
const products = [ { id: 1, name: '基础月卡' }, { id: 2, name: '高级月卡' }, { id: 3, name: '活动月卡' } ] as GroupBuyProduct[]
const coupon = (overrides: Partial<MonthCardCoupon> = {}): MonthCardCoupon => ({ id: 7, code: 'SAVE10', kind: 'percent', value: 10, product_id: null, product_ids: [], active: true, expires_at: null, max_uses: 0, per_user_limit: 1, used_count: 0, ...overrides })
const button = (view: ReturnType<typeof mount>, text: string) => view.findAll('button').find(b => b.text() === text)!
async function open(existing?: MonthCardCoupon) {
  vi.mocked(adminGroupBuyAPI.coupons).mockResolvedValue(existing ? [existing] : [])
  const view = mount(AdminCoupons, { props: { products } })
  await flushPromises()
  await button(view, existing ? '编辑' : '新增支付优惠码').trigger('click')
  return view
}
beforeEach(() => { vi.clearAllMocks(); vi.mocked(adminGroupBuyAPI.saveCoupon).mockResolvedValue(coupon()) })

describe('admin coupon product scope', () => {
  it('saves multiple selected products across search filters and allows unlimited personal uses', async () => {
    const view = await open()
    expect((view.get('[data-test="coupon-per-user"]').element as HTMLInputElement).value).toBe('1')
    expect(view.get('[data-test="coupon-per-user"]').attributes('min')).toBe('0')
    await view.get('[data-test="coupon-code"]').setValue('save10')
    await view.get('[data-test="coupon-products-selected"]').setValue(true)
    await view.get('[data-product-id="1"]').setValue(true)
    await view.get('[data-test="coupon-product-search"]').setValue('高级')
    expect(view.find('[data-product-id="1"]').exists()).toBe(false)
    await view.get('[data-product-id="2"]').setValue(true)
    await view.get('[data-test="coupon-per-user"]').setValue(0)
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(adminGroupBuyAPI.saveCoupon).toHaveBeenCalledWith(expect.objectContaining({ product_ids: [1, 2], product_id: 1, per_user_limit: 0, code: 'SAVE10' }))
    view.unmount()
  })
  it('rejects an empty selected scope and saves an explicit all-product scope', async () => {
    const view = await open()
    await view.get('[data-test="coupon-products-selected"]').setValue(true)
    await view.get('form').trigger('submit')
    expect(adminGroupBuyAPI.saveCoupon).not.toHaveBeenCalled()
    expect(view.text()).toContain('需至少选择一个商品')
    await view.get('[data-product-id="1"]').setValue(true)
    expect(view.find('[role="alert"]').exists()).toBe(false)
    await view.get('[data-test="coupon-products-all"]').setValue(true)
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(adminGroupBuyAPI.saveCoupon).toHaveBeenCalledWith(expect.objectContaining({ product_ids: [], product_id: null, per_user_limit: 1 }))
    view.unmount()
  })
  it('restores legacy single-product coupons and leaves existing state unchanged on cancel', async () => {
    const existing = coupon({ product_id: 2, product_ids: undefined })
    const view = await open(existing)
    expect((view.get('[data-test="coupon-products-selected"]').element as HTMLInputElement).checked).toBe(true)
    expect((view.get('[data-product-id="2"]').element as HTMLInputElement).checked).toBe(true)
    await view.get('[data-product-id="1"]').setValue(true)
    await button(view, '取消').trigger('click')
    await button(view, '编辑').trigger('click')
    expect((view.get('[data-product-id="1"]').element as HTMLInputElement).checked).toBe(false)
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(adminGroupBuyAPI.saveCoupon).toHaveBeenCalledWith(expect.objectContaining({ product_ids: [2], product_id: 2 }))
    view.unmount()
  })
  it('shows all scoped products and preserves unknown selections during edits and save failures', async () => {
    const existing = coupon({ product_id: 1, product_ids: [1, 2, 99] })
    const view = await open(existing)
    expect(view.get('tbody').text()).toContain('基础月卡、高级月卡、#99')
    await view.get('[data-product-id="1"]').setValue(false)
    expect(existing.product_ids).toEqual([1, 2, 99])
    vi.mocked(adminGroupBuyAPI.saveCoupon).mockRejectedValueOnce(new Error('failed'))
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(view.find('[role="dialog"]').exists()).toBe(true)
    expect((view.get('[data-product-id="99"]').element as HTMLInputElement).checked).toBe(true)
    expect(adminGroupBuyAPI.saveCoupon).toHaveBeenCalledWith(expect.objectContaining({ product_ids: [2, 99] }))
    view.unmount()
  })
})
