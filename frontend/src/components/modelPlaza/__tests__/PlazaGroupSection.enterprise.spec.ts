import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlazaGroupSection from '../PlazaGroupSection.vue'
import PlazaModelPricingTable from '../PlazaModelPricingTable.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null })
}))

function tokenModel(): PlazaModel {
  return {
    name: 'claude-sonnet',
    platform: 'anthropic',
    pricing: {
      billing_mode: 'token',
      input_price: 3e-6,
      output_price: 1.5e-5,
      cache_write_price: null,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: []
    },
    official_pricing: null
  }
}

function imageModel(): PlazaModel {
  return {
    name: 'gpt-image-2',
    platform: 'openai',
    pricing: {
      billing_mode: 'image',
      input_price: null,
      output_price: null,
      cache_write_price: null,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: null,
      per_request_price: 0.02,
      intervals: []
    },
    official_pricing: null
  }
}

/** 分组 0.5、用户专属 0.9、企业 0.3;生图独立倍率 2、高峰 1.5 全开,用于验证企业口径全部覆盖。 */
function group(overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id: 1,
    name: 'g',
    description: '',
    platform: 'openai',
    subscription_type: 'standard',
    rate_multiplier: 0.5,
    user_rate_multiplier: 0.9,
    enterprise_rate_multiplier: 0.3,
    temporary_rate_enabled: false,
    temporary_rate_multiplier: 1,
    temporary_rate_starts_at: null,
    temporary_rate_ends_at: null,
    peak_rate_enabled: true,
    peak_start: '14:00',
    peak_end: '18:00',
    peak_rate_multiplier: 1.5,
    is_exclusive: false,
    image_rate_independent: true,
    image_rate_multiplier: 2,
    video_rate_independent: true,
    video_rate_multiplier: 3,
    long_context_pricing_enabled: true,
    models: [tokenModel(), imageModel()],
    ...overrides
  }
}

function mountSection(g: ModelPlazaGroup, enterprise: boolean, stubTable = true) {
  return mount(PlazaGroupSection, {
    props: { group: g, enterprise },
    global: {
      stubs: {
        GroupBadge: true,
        Icon: true,
        PlatformIcon: true,
        PlazaModelPricingTable: stubTable
      }
    }
  })
}

describe('PlazaGroupSection 企业倍率口径', () => {
  it('企业模式:表格实付倍率替换为企业倍率,剥离用户/生图独立/峰谷系数', () => {
    const wrapper = mountSection(group(), true)
    const table = wrapper.findComponent(PlazaModelPricingTable)
    expect(table.props('rateMultiplier')).toBe(0.3)
    expect(table.props('userRateMultiplier')).toBeNull()
    expect(table.props('imageRateIndependent')).toBe(false)
    expect(table.props('videoRateIndependent')).toBe(false)
    expect(table.props('peakWindow')).toBe('')
    // 徽章同样按企业倍率展示,不划线对比普通倍率
    const badge = wrapper.findComponent(GroupBadge)
    expect(badge.props('rateMultiplier')).toBe(0.3)
    expect(badge.props('userRateMultiplier')).toBeNull()
    expect(badge.props('peakRateEnabled')).toBe(false)
    expect(badge.props('temporaryRateEnabled')).toBe(false)
    // 峰谷提示行在企业口径下隐藏
    expect(wrapper.text()).not.toContain('modelPlaza.detail.peakNote')
  })

  it('企业模式实付价:token 与按图模型都乘企业倍率,不叠生图独立倍率与峰谷', () => {
    const wrapper = mountSection(group(), true, false)
    const text = wrapper.text()
    // token:3e-6 × 0.3 × 1M = $0.90(不是专属 0.9 的 $2.70,也不是分组 0.5 的 $1.50)
    expect(text).toContain('$0.90')
    expect(text).not.toContain('$2.70')
    expect(text).not.toContain('$1.50')
    // 按图:0.02 × 0.3 = $0.006(不是独立倍率 2 的 $0.04,也不是专属 0.9 的 $0.018)
    expect(text).toContain('$0.006')
    expect(text).not.toContain('$0.04')
    expect(text).not.toContain('$0.018')
    // 倍率列展示企业倍率
    expect(text).toContain('0.3x')
    expect(text).not.toContain('2x')
  })

  it('非企业模式:完全恢复普通口径(专属倍率 + 生图独立倍率 + 峰谷披露)', () => {
    const wrapper = mountSection(group(), false, false)
    const table = wrapper.findComponent(PlazaModelPricingTable)
    expect(table.props('rateMultiplier')).toBe(0.5)
    expect(table.props('userRateMultiplier')).toBe(0.9)
    expect(table.props('imageRateIndependent')).toBe(true)
    expect(table.props('videoRateIndependent')).toBe(true)
    expect(table.props('peakWindow')).toBe('14:00-18:00 ×1.5')
    const text = wrapper.text()
    // token:3e-6 × 0.9 × 1M = $2.70;按图:0.02 × 2 = $0.04;企业 0.3 不混入
    expect(text).toContain('$2.70')
    expect(text).toContain('$0.04')
    expect(text).not.toContain('$0.90')
    expect(text).not.toContain('$0.006')
  })

  it('企业模式但分组未配置企业倍率(字段为 null)时回退普通口径', () => {
    const wrapper = mountSection(group({ enterprise_rate_multiplier: null }), true)
    const table = wrapper.findComponent(PlazaModelPricingTable)
    expect(table.props('rateMultiplier')).toBe(0.5)
    expect(table.props('userRateMultiplier')).toBe(0.9)
    expect(table.props('imageRateIndependent')).toBe(true)
  })
})
