import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ApiKey, Group } from '@/types'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import KeyGroupBadges from '../KeyGroupBadges.vue'
import OrderedKeyGroupSelector from '../OrderedKeyGroupSelector.vue'
import { groupDisplayProps } from '../keyGroups'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: null }) }))
const group = {
  id: 7, name: 'Enterprise group', platform: 'openai', status: 'active',
  subscription_type: 'standard', rate_multiplier: 2,
  temporary_rate_enabled: true, temporary_rate_multiplier: 3, peak_rate_enabled: true, peak_rate_multiplier: 4,
} as Group
const apiKey = { group_ids: [7], groups: [group] } as ApiKey

describe('enterprise key group prices', () => {
  it('replaces the base rate and disables ordinary temporary/peak modifiers only for explicit enterprise prices', () => {
    expect(groupDisplayProps(group, 0.25)).toMatchObject({ rateMultiplier: 0.25, temporaryRateEnabled: false, peakRateEnabled: false })
    expect(groupDisplayProps(group)).toMatchObject({ rateMultiplier: 2, temporaryRateEnabled: true, peakRateEnabled: true })
  })

  it('prevents a personal user rate from overriding the enterprise badge and hides prices on load failure', async () => {
    const wrapper = mount(KeyGroupBadges, {
      props: { apiKey, userRates: { 7: 0.8 }, enterpriseRates: { 7: 0.25 } },
      global: { stubs: { GroupBadge: true } },
    })
    expect(wrapper.findComponent(GroupBadge).props()).toMatchObject({
      rateMultiplier: 0.25, userRateMultiplier: null, temporaryRateEnabled: false, peakRateEnabled: false,
    })
    await wrapper.setProps({ enterpriseRates: {} })
    expect(wrapper.findComponent(GroupBadge).props('userRateMultiplier')).toBe(0.8)
    await wrapper.setProps({ ratesUnavailable: true })
    expect(wrapper.findComponent(GroupBadge).exists()).toBe(false)
    expect(wrapper.text()).toContain(group.name)
    wrapper.unmount()
  })

  it('uses enterprise rates in selected groups and candidates without changing groups with no explicit price', async () => {
    const wrapper = mount(OrderedKeyGroupSelector, {
      props: {
        modelValue: [7], availableGroups: [group, { ...group, id: 8 }],
        userRates: { 7: 0.8, 8: 0.9 }, enterpriseRates: { 7: 0.25 },
      },
      global: { stubs: { GroupOptionItem: true, Icon: true } },
    })
    expect(wrapper.findComponent(GroupOptionItem).props()).toMatchObject({
      rateMultiplier: 0.25, userRateMultiplier: null, temporaryRateEnabled: false, peakRateEnabled: false,
    })
    await wrapper.get('[data-test="expand-groups"]').trigger('click')
    expect(wrapper.findAllComponents(GroupOptionItem)[1].props()).toMatchObject({
      rateMultiplier: 2, userRateMultiplier: 0.9, temporaryRateEnabled: true, peakRateEnabled: true,
    })
    wrapper.unmount()
  })
})
