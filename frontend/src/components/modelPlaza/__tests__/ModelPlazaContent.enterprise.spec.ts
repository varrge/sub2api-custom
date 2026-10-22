import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaGroupSection from '../PlazaGroupSection.vue'
import PlazaFilterBar from '../PlazaFilterBar.vue'
import type { ModelPlazaGroup, ModelPlazaResponse } from '@/api/modelPlaza'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({
    t: (key: string, params?: { selected: number; total: number }) =>
      params ? `${key}: ${params.selected}/${params.total}` : key
  })
}))

const authState = { isAuthenticated: true, isEnterprise: true }

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

function group(
  id: number,
  name: string,
  platform: string,
  rate: number,
  overrides: Partial<ModelPlazaGroup> = {}
): ModelPlazaGroup {
  return {
    id,
    name,
    platform,
    description: '',
    subscription_type: 'standard',
    rate_multiplier: rate,
    temporary_rate_enabled: false,
    temporary_rate_multiplier: 1,
    temporary_rate_starts_at: null,
    temporary_rate_ends_at: null,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    long_context_pricing_enabled: true,
    models: ['shared-model', `${platform}-model`].map((name) => ({
      name, platform, pricing: null, official_pricing: null
    })),
    ...overrides
  }
}

/** Alpha/Beta(openai)、Gamma/Delta(anthropic);只有 Alpha、Delta 配置了企业倍率。 */
function response(): ModelPlazaResponse {
  return {
    description: '',
    groups: [
      group(1, 'Alpha', 'openai', 0.5, { enterprise_rate_multiplier: 0.3 }),
      group(2, 'Beta', 'openai', 1),
      group(3, 'Gamma', 'anthropic', 0.5, { enterprise_rate_multiplier: null }),
      group(4, 'Delta', 'anthropic', 2, {
        user_rate_multiplier: 1,
        enterprise_rate_multiplier: 0.8
      })
    ]
  }
}

let wrapper: VueWrapper

beforeEach(() => {
  authState.isAuthenticated = true
  authState.isEnterprise = true
})

afterEach(() => wrapper?.unmount())

function mountPlaza(data: ModelPlazaResponse | null = response(), extraProps: Record<string, unknown> = {}) {
  wrapper = mount(ModelPlazaContent, {
    props: { response: data, loading: false, ...extraProps },
    global: { stubs: { Icon: true, PlatformIcon: true, PlazaGroupSection: true } }
  })
}

function shownGroups(): ModelPlazaGroup[] {
  return wrapper.findAllComponents(PlazaGroupSection).map((section) => section.props('group'))
}

function toggle() {
  return wrapper.find('input[type="checkbox"]')
}

describe('ModelPlazaContent 企业倍率视图', () => {
  it('企业用户默认勾选,只展示配置了企业倍率的分组并按企业倍率排序', () => {
    mountPlaza()
    expect(toggle().exists()).toBe(true)
    expect((toggle().element as HTMLInputElement).checked).toBe(true)
    // Alpha(0.3)、Delta(0.8);Beta 缺字段、Gamma 显式 null 都视为未配置
    expect(shownGroups().map((g) => g.id)).toEqual([1, 4])
    for (const section of wrapper.findAllComponents(PlazaGroupSection)) {
      expect(section.props('enterprise')).toBe(true)
    }
  })

  it('勾选态下筛选项只来自已配置分组(平台/倍率口径均为企业倍率)', () => {
    mountPlaza()
    const text = wrapper.text()
    // 倍率筛选项为 0.3x/0.8x;普通生效倍率(0.5x/1x)与分组倍率 2x 不出现
    expect(text).toContain('0.3x')
    expect(text).toContain('0.8x')
    expect(text).not.toContain('0.5x')
    expect(text).not.toContain('2x')
    // 两个平台都有已配置分组,均可用
    expect(text).toContain('openai')
    expect(text).toContain('anthropic')
  })

  it('取消勾选后恢复普通视图:全部分组 + user_rate_multiplier ?? rate_multiplier 口径', async () => {
    mountPlaza()
    await toggle().setValue(false)
    expect(shownGroups().map((g) => g.id)).toEqual([1, 3, 2, 4])
    for (const section of wrapper.findAllComponents(PlazaGroupSection)) {
      expect(section.props('enterprise')).toBe(false)
    }
    // 普通口径下 Delta 生效倍率为专属 1x,Alpha/Gamma 为 0.5x;企业倍率 0.3x/0.8x 不再混入
    expect(wrapper.text()).toContain('0.5x')
    expect(wrapper.text()).toContain('1x')
    expect(wrapper.text()).not.toContain('0.3x')
    expect(wrapper.text()).not.toContain('0.8x')
    // 重新勾选回到企业视图
    await toggle().setValue(true)
    expect(shownGroups().map((g) => g.id)).toEqual([1, 4])
  })

  it('重新勾选企业视图时清除已隐藏的平台和分组筛选', async () => {
    const data = response()
    data.groups = data.groups.filter((g) => g.id === 1 || g.id === 3)
    mountPlaza(data)
    await toggle().setValue(false)
    const filters = wrapper.findComponent(PlazaFilterBar)
    filters.vm.$emit('update:platform', 'anthropic')
    filters.vm.$emit('update:groupIds', [3])
    await wrapper.vm.$nextTick()
    expect(shownGroups().map((g) => g.id)).toEqual([3])
    await toggle().setValue(true)
    expect(shownGroups().map((g) => g.id)).toEqual([1])
    expect(filters.props('platform')).toBe('all')
    expect(filters.props('groupIds')).toBe('all')
  })

  it('勾选但没有任何已配置分组时显示空状态,且不自动取消勾选', async () => {
    const data = response()
    data.groups.forEach((g) => delete g.enterprise_rate_multiplier)
    mountPlaza(data)
    expect(shownGroups()).toHaveLength(0)
    expect(wrapper.text()).toContain('enterprisePlaza.empty')
    expect(wrapper.text()).toContain('enterprisePlaza.emptyHint')
    expect((toggle().element as HTMLInputElement).checked).toBe(true)
    // 手动取消后恢复普通列表
    await toggle().setValue(false)
    expect(shownGroups()).toHaveLength(4)
    expect(wrapper.text()).not.toContain('enterprisePlaza.empty')
  })

  it('普通用户不显示开关,直接走普通视图', () => {
    authState.isEnterprise = false
    mountPlaza()
    expect(toggle().exists()).toBe(false)
    expect(shownGroups().map((g) => g.id)).toEqual([1, 3, 2, 4])
  })

  it('匿名用户不显示开关', () => {
    authState.isAuthenticated = false
    authState.isEnterprise = false
    mountPlaza()
    expect(toggle().exists()).toBe(false)
    expect(wrapper.text()).toContain('modelPlaza.anonymousHint')
    expect(shownGroups()).toHaveLength(4)
  })

  it('加载失败保持错误展示,不冒充未配置企业倍率', () => {
    mountPlaza(null, { error: true })
    expect(wrapper.text()).toContain('modelPlaza.loadFailed')
    expect(wrapper.text()).not.toContain('enterprisePlaza.empty')
  })
})
