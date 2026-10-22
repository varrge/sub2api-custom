import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { reactive } from 'vue'
import PaymentView from '../PaymentView.vue'
import AmountInput from '@/components/payment/AmountInput.vue'
import SubscriptionPlanCard from '@/components/payment/SubscriptionPlanCard.vue'
import ProductCatalog from '@/features/group-buy/ProductCatalog.vue'
import CouponEntry from '@/features/group-buy/CouponEntry.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import { PAYMENT_RECOVERY_STORAGE_KEY } from '@/components/payment/paymentFlow'
import type { CheckoutInfoResponse, MethodLimit, SubscriptionPlan } from '@/types/payment'
import type { CouponQuote, GroupBuyProduct } from '@/types/groupBuy'

const routeState = vi.hoisted(() => ({
  path: '/purchase',
  query: {} as Record<string, unknown>,
}))

const routerReplace = vi.hoisted(() => vi.fn())
const routerPush = vi.hoisted(() => vi.fn())
const routerResolve = vi.hoisted(() => vi.fn(() => ({ href: '/payment/stripe?mock=1' })))
const createOrder = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())
const fetchActiveSubscriptions = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('@/stores/groupBuy', () => ({ useGroupBuyStore: () => ({ fetchMonthCards: vi.fn().mockResolvedValue(undefined) }) }))
const showError = vi.hoisted(() => vi.fn())
const showInfo = vi.hoisted(() => vi.fn())
const showWarning = vi.hoisted(() => vi.fn())
const getCheckoutInfo = vi.hoisted(() => vi.fn())
const groupBuyProducts = vi.hoisted(() => vi.fn())
const groupBuyTeam = vi.hoisted(() => vi.fn())
const getMonthCardRules = vi.hoisted(() => vi.fn().mockResolvedValue({ documents: [] }))
vi.mock('@/api/monthCardRules', () => ({ monthCardRulesAPI: { get: getMonthCardRules, read: vi.fn() } }))
const isEnterprise = vi.hoisted(() => vi.fn(() => false))
const translate = vi.hoisted(() => vi.fn((key: string) => key))
const authIdentity = reactive({ user: { id: 1, username: 'demo-user', balance: 0 }, enterprise: null as boolean | null })

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeState,
    useRouter: () => ({
      replace: routerReplace,
      push: routerPush,
      resolve: routerResolve,
    }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: translate,
    }),
  }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get user() {
      return {
        ...authIdentity.user,
        account_type: (authIdentity.enterprise ?? isEnterprise()) ? 'enterprise' : 'personal',
      }
    },
    get isEnterprise() { return authIdentity.enterprise ?? isEnterprise() },
    refreshUser,
  }),
}))

vi.mock('@/stores/payment', () => ({
  usePaymentStore: () => ({
    createOrder,
  }),
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    activeSubscriptions: [],
    fetchActiveSubscriptions,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showInfo,
    showWarning,
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getCheckoutInfo,
  },
}))

vi.mock('@/api/groups', () => ({
  userGroupsAPI: {
    getUserGroupRates: vi.fn().mockResolvedValue({}),
  },
}))

vi.mock('@/api/groupBuy', () => ({
  groupBuyAPI: {
    products: groupBuyProducts,
    team: groupBuyTeam,
  },
}))

vi.mock('@/utils/device', () => ({
  isMobileDevice: () => true,
}))

function checkoutInfoFixture(overrides: Partial<CheckoutInfoResponse> = {}) {
  const wxpayMethod: MethodLimit = {
    daily_limit: 0,
    daily_used: 0,
    daily_remaining: 0,
    single_min: 0,
    single_max: 0,
    fee_rate: 0,
    available: true,
  }
  const data: CheckoutInfoResponse = {
    methods: {
      wxpay: wxpayMethod,
    },
    global_min: 0,
    global_max: 0,
    plans: [],
    balance_disabled: false,
    balance_recharge_multiplier: 1,
    subscription_usd_to_cny_rate: 0,
    recharge_fee_rate: 0,
    help_text: '',
    help_image_url: '',
    stripe_publishable_key: '',
  }

  return {
    data: { ...data, ...overrides },
  }
}

async function mountPaymentView() {
  const wrapper = shallowMount(PaymentView, {
    global: {
      stubs: {
        AppLayout: {
          template: '<div><slot /></div>',
        },
        RouterLink: {
          props: ['to'],
          template: '<a :href="to"><slot /></a>',
        },
        Teleport: true,
        Transition: false,
      },
    },
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

describe('PaymentView enterprise account', () => {
  beforeEach(() => {
    authIdentity.user = { id: 1, username: 'demo-user', balance: 0 }
    authIdentity.enterprise = null
    vi.useRealTimers()
    routeState.path = '/purchase'
    routeState.query = {}
    routerReplace.mockReset().mockResolvedValue(undefined)
    routerPush.mockReset().mockResolvedValue(undefined)
    createOrder.mockReset()
    refreshUser.mockReset()
    fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
    showError.mockReset()
    showInfo.mockReset()
    showWarning.mockReset()
    getCheckoutInfo.mockReset().mockResolvedValue(checkoutInfoFixture())
    groupBuyProducts.mockReset().mockResolvedValue([])
    groupBuyTeam.mockReset()
    window.localStorage.clear()
    ;(window as Window & { WeixinJSBridge?: unknown }).WeixinJSBridge = undefined
  })

  it('pins the recharge tab and hides subscription/plan UI for enterprise users', async () => {
    isEnterprise.mockReturnValue(true)
    const wrapper = await mountPaymentView()

    // No tab switcher: the only button is the recharge submit button.
    const buttonTexts = wrapper.findAll('button').map(button => button.text())
    expect(buttonTexts).toHaveLength(1)
    expect(buttonTexts[0]).toContain('payment.createOrder')
    // Recharge flow is visible.
    expect(wrapper.findComponent(AmountInput).exists()).toBe(true)
    // Subscription / month-card purchase UI is absent.
    expect(wrapper.findComponent(ProductCatalog).exists()).toBe(false)
    expect(wrapper.findComponent(SubscriptionPlanCard).exists()).toBe(false)
    expect(wrapper.text()).not.toContain('payment.tabSubscribe')
    expect(wrapper.text()).not.toContain('payment.noPlans')
    wrapper.unmount()
  })

  it('reloads checkout after identity conversion and discards the old draft amount', async () => {
    isEnterprise.mockReturnValue(false)
    getCheckoutInfo.mockResolvedValue(checkoutInfoFixture({ balance_recharge_multiplier: 2 }))
    const wrapper = await mountPaymentView()
    wrapper.findComponent(AmountInput).vm.$emit('update:modelValue', 100)
    await flushPromises()
    expect(wrapper.text()).toContain('$200.00')
    let resolveQuote!: (value: ReturnType<typeof checkoutInfoFixture>) => void
    getCheckoutInfo.mockReturnValueOnce(new Promise(resolve => { resolveQuote = resolve }))
    authIdentity.enterprise = true
    await flushPromises()
    expect(wrapper.findComponent(AmountInput).exists()).toBe(false)
    expect(createOrder).not.toHaveBeenCalled()
    resolveQuote(checkoutInfoFixture({ balance_recharge_multiplier: 0.75 }))
    await flushPromises()
    expect(wrapper.findComponent(AmountInput).props('modelValue')).toBeNull()
    wrapper.findComponent(AmountInput).vm.$emit('update:modelValue', 100)
    await flushPromises()
    expect(wrapper.get('[data-testid="enterprise-recharge-multiplier"]').text()).toContain('×0.75')
    expect(wrapper.text()).toContain('$75.00')
    wrapper.unmount()
  })

  it.each([false, true])('keeps the recharge draft on a same-identity profile replacement (enterprise=%s)', async (enterprise) => {
    isEnterprise.mockReturnValue(enterprise)
    const wrapper = await mountPaymentView()
    wrapper.findComponent(AmountInput).vm.$emit('update:modelValue', 125)
    await flushPromises()
    getCheckoutInfo.mockClear()
    authIdentity.user = { ...authIdentity.user, balance: 99, username: 'refreshed' }
    await flushPromises()
    expect(getCheckoutInfo).not.toHaveBeenCalled()
    expect(wrapper.findComponent(AmountInput).props('modelValue')).toBe(125)
    wrapper.unmount()
  })

  it('keeps the selected subscription on a same-identity profile replacement', async () => {
    isEnterprise.mockReturnValue(false)
    const plan: SubscriptionPlan = {
      id: 7, group_id: 3, name: 'Draft subscription', description: '', price: 128,
      validity_days: 30, validity_unit: 'day', features: [], for_sale: true, sort_order: 1,
      group_platform: 'openai', rate_multiplier: 1,
    }
    getCheckoutInfo.mockResolvedValue(checkoutInfoFixture({ plans: [plan] }))
    routeState.query = { tab: 'subscription', group: '3' }
    const wrapper = await mountPaymentView()
    expect(wrapper.get('h3').text()).toBe(plan.name)
    getCheckoutInfo.mockClear()
    authIdentity.user = { ...authIdentity.user, balance: 99 }
    await flushPromises()
    expect(getCheckoutInfo).not.toHaveBeenCalled()
    expect(wrapper.get('h3').text()).toBe(plan.name)
    expect(wrapper.findComponent(SubscriptionPlanCard).exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps the monthly-card selection and applied coupon on a same-identity profile replacement', async () => {
    isEnterprise.mockReturnValue(false)
    const product: GroupBuyProduct = {
      id: 5, group_id: 3, group_name: 'Group', platform: 'openai', name: 'Draft monthly card',
      description: '', price_cny: 100, base_quota_usd: 200, tiers: [], max_members: 5,
      recruitment_hours: 24, for_sale: true, sort_order: 1,
    }
    const coupon: CouponQuote = { coupon_id: 4, code: 'DRAFT', original_cny: 100, discount_cny: 20, amount_cny: 80 }
    const wrapper = await mountPaymentView()
    await wrapper.findAll('button').find(button => button.text() === 'groupBuy.tab')!.trigger('click')
    wrapper.findComponent(ProductCatalog).vm.$emit('select', { product, mode: 'solo' })
    await flushPromises()
    wrapper.findComponent(CouponEntry).vm.$emit('update:modelValue', coupon)
    await flushPromises()
    getCheckoutInfo.mockClear()
    getMonthCardRules.mockClear()
    authIdentity.user = { ...authIdentity.user, balance: 99 }
    await flushPromises()
    expect(getCheckoutInfo).not.toHaveBeenCalled()
    expect(getMonthCardRules).not.toHaveBeenCalled()
    expect(wrapper.get('h3').text()).toContain(product.name)
    expect(wrapper.findComponent(CouponEntry).props('modelValue')).toEqual(coupon)
    wrapper.unmount()
  })

  it('blocks checkout when reloading fails and allows a deliberate retry', async () => {
    isEnterprise.mockReturnValue(false)
    const wrapper = await mountPaymentView()
    getCheckoutInfo.mockRejectedValueOnce(new Error('offline'))
    authIdentity.enterprise = true
    await flushPromises()
    expect(wrapper.findComponent(AmountInput).exists()).toBe(false)
    expect(wrapper.text()).toContain('enterprisePayment.checkoutLoadFailed')
    expect(createOrder).not.toHaveBeenCalled()
    getCheckoutInfo.mockResolvedValue(checkoutInfoFixture({ balance_recharge_multiplier: 1.25 }))
    await wrapper.get('[data-testid="retry-checkout"]').trigger('click')
    await flushPromises()
    expect(wrapper.findComponent(AmountInput).props('multiplier')).toBe(1.25)
    wrapper.unmount()
  })

  it('ignores an old quote that arrives after switching to another account', async () => {
    isEnterprise.mockReturnValue(false)
    const wrapper = await mountPaymentView()
    let resolveOld!: (value: ReturnType<typeof checkoutInfoFixture>) => void
    getCheckoutInfo.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    authIdentity.enterprise = true
    getCheckoutInfo.mockResolvedValue(checkoutInfoFixture({ balance_recharge_multiplier: 0.6 }))
    authIdentity.user.id = 2
    await flushPromises()
    resolveOld(checkoutInfoFixture({ balance_recharge_multiplier: 8 }))
    await flushPromises()
    expect(wrapper.findComponent(AmountInput).props('multiplier')).toBe(0.6)
    wrapper.unmount()
  })

  it('preserves an existing order snapshot during same-user conversion but clears it on account switch', async () => {
    isEnterprise.mockReturnValue(false)
    window.localStorage.setItem(PAYMENT_RECOVERY_STORAGE_KEY, JSON.stringify({
      orderId: 888, amount: 200, payAmount: 100, qrCode: 'qr', expiresAt: '2099-01-01T00:10:00Z',
      paymentType: 'wxpay', payUrl: '', outTradeNo: 'order_888', clientSecret: '', intentId: '',
      currency: 'USD', countryCode: '', paymentEnv: '', orderType: 'balance', paymentMode: 'qr',
      resumeToken: '', createdAt: Date.now(),
    }))
    const wrapper = await mountPaymentView()
    expect(wrapper.findComponent(PaymentStatusPanel).props('amount')).toBe(200)
    getCheckoutInfo.mockResolvedValue(checkoutInfoFixture({ balance_recharge_multiplier: 0.5 }))
    authIdentity.enterprise = true
    await flushPromises()
    const panel = wrapper.findComponent(PaymentStatusPanel)
    expect(panel.props('orderId')).toBe(888)
    expect(panel.props('amount')).toBe(200)
    expect(panel.props('payAmount')).toBe(100)
    authIdentity.user.id = 2
    await flushPromises()
    expect(wrapper.findComponent(PaymentStatusPanel).exists()).toBe(false)
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
    wrapper.unmount()
  })

  it('keeps the separate subscription and month-card tabs for non-enterprise users', async () => {
    isEnterprise.mockReturnValue(false)
    const wrapper = await mountPaymentView()

    const buttonTexts = wrapper.findAll('button').map(button => button.text())
    expect(buttonTexts.slice(0, 3)).toEqual([
      'payment.tabTopUp',
      'payment.tabSubscribe',
      'groupBuy.tab',
    ])
    expect(buttonTexts[3]).toContain('payment.createOrder')
    expect(wrapper.findComponent(AmountInput).exists()).toBe(true)
    // 0.2.17 keeps month cards separate from subscriptions.
    const subscribeTab = wrapper
      .findAll('button')
      .find(button => button.text().includes('groupBuy.tab'))
    expect(subscribeTab).toBeDefined()
    await subscribeTab!.trigger('click')
    await flushPromises()
    expect(wrapper.findComponent(ProductCatalog).exists()).toBe(true)
    wrapper.unmount()
  })

  it('keeps non-enterprise behavior unchanged when balance recharge is disabled', async () => {
    isEnterprise.mockReturnValue(false)
    getCheckoutInfo.mockResolvedValue(checkoutInfoFixture({ balance_disabled: true }))
    const wrapper = await mountPaymentView()

    expect(wrapper.findAll('button').map(button => button.text()).slice(0, 2)).toEqual([
      'payment.tabSubscribe', 'groupBuy.tab',
    ])
    await wrapper.findAll('button').find(button => button.text() === 'groupBuy.tab')!.trigger('click')
    await flushPromises()
    expect(wrapper.findComponent(ProductCatalog).exists()).toBe(true)
    wrapper.unmount()
  })

  it('does not query group-buy endpoints for enterprise deep links', async () => {
    isEnterprise.mockReturnValue(true)
    routeState.query = { groupBuy: '1', mode: 'create', product_id: '5' }
    const wrapper = await mountPaymentView()

    expect(groupBuyProducts).not.toHaveBeenCalled()
    expect(groupBuyTeam).not.toHaveBeenCalled()
    expect(wrapper.findComponent(ProductCatalog).exists()).toBe(false)
    // Still pinned to the recharge tab.
    const buttonTexts = wrapper.findAll('button').map(button => button.text())
    expect(buttonTexts.some(text => text.includes('payment.tabSubscribe'))).toBe(false)
    wrapper.unmount()
  })

  it.each(['bonus', 'discount'])('does not apply personal recharge %s tiers to enterprise checkout', async (mode) => {
    isEnterprise.mockReturnValue(true)
    getCheckoutInfo.mockResolvedValue(checkoutInfoFixture({
      balance_recharge_multiplier: 1.5,
      recharge_bonus_mode: mode,
      recharge_bonus_tiers: [{ min_amount: 10, bonus_percent: 50 }],
      recharge_bonus_notice: 'Personal bonus promotion',
    }))
    const wrapper = await mountPaymentView()
    wrapper.findComponent(AmountInput).vm.$emit('update:modelValue', 100)
    await flushPromises()
    expect(wrapper.findComponent(AmountInput).props('bonusTiers')).toEqual([])
    expect(wrapper.find('[data-testid="recharge-bonus-notice"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="recharge-bonus-row"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="recharge-discount-row"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="recharge-credited-row"]').text()).toContain('$150.00')
    wrapper.unmount()
  })

  it('does not fetch active subscriptions for enterprise users on mount', async () => {
    isEnterprise.mockReturnValue(true)
    const wrapper = await mountPaymentView()

    expect(fetchActiveSubscriptions).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows the enterprise recharge multiplier and credited USD even when the rate is 1', async () => {
    isEnterprise.mockReturnValue(true)
    const wrapper = await mountPaymentView()

    wrapper.findComponent(AmountInput).vm.$emit('update:modelValue', 100)
    await flushPromises()

    const multiplierRow = wrapper.get('[data-testid="enterprise-recharge-multiplier"]')
    expect(multiplierRow.text()).toContain('enterprisePayment.effectiveMultiplier')
    expect(multiplierRow.text()).toContain('×1.00')
    // Credited USD is shown for enterprise even at the default 1x rate.
    expect(wrapper.text()).toContain('payment.creditedBalance')
    expect(wrapper.text()).toContain('$100.00')
    wrapper.unmount()
  })

  it('reflects a backend-provided enterprise multiplier without client-side conversion', async () => {
    isEnterprise.mockReturnValue(true)
    getCheckoutInfo.mockResolvedValue(checkoutInfoFixture({ balance_recharge_multiplier: 1.5 }))
    const wrapper = await mountPaymentView()

    wrapper.findComponent(AmountInput).vm.$emit('update:modelValue', 100)
    await flushPromises()

    const multiplierRow = wrapper.get('[data-testid="enterprise-recharge-multiplier"]')
    expect(multiplierRow.text()).toContain('×1.50')
    expect(wrapper.text()).toContain('$150.00')
    wrapper.unmount()
  })

  it('hides the enterprise multiplier row and the credited row for personal users at rate 1', async () => {
    isEnterprise.mockReturnValue(false)
    const wrapper = await mountPaymentView()

    wrapper.findComponent(AmountInput).vm.$emit('update:modelValue', 100)
    await flushPromises()

    expect(wrapper.find('[data-testid="enterprise-recharge-multiplier"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('payment.creditedBalance')
    wrapper.unmount()
  })

  it('links enterprise users to their recharge orders from the purchase page', async () => {
    isEnterprise.mockReturnValue(true)
    const wrapper = await mountPaymentView()

    const entry = wrapper.get('[data-testid="enterprise-orders-entry"]')
    expect(entry.attributes('href')).toBe('/orders')
    expect(entry.text()).toBe('enterprisePayment.viewOrders')
    wrapper.unmount()
  })

  it('does not show the in-page orders entry for personal users (sidebar covers it)', async () => {
    isEnterprise.mockReturnValue(false)
    const wrapper = await mountPaymentView()

    expect(wrapper.find('[data-testid="enterprise-orders-entry"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
