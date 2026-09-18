import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { Group } from '@/types'
import OrderedKeyGroupSelector from '../OrderedKeyGroupSelector.vue'
import { keyGroupIds, keyGroups } from '../keyGroups'

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: null }) }))

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: { name: string }) => params ? `${key}:${params.name}` : key }) }))

const group = (id: number, status: 'active' | 'inactive' = 'active') => ({
  id, name: `Group ${id}`, status, platform: 'openai', subscription_type: 'standard', rate_multiplier: id,
} as Group)
const makeSelector = (overrides = {}) => mount(OrderedKeyGroupSelector, {
  props: { modelValue: [7, 2], availableGroups: [group(2), group(3)], existingGroups: [group(7, 'inactive')], ...overrides },
  global: { stubs: { GroupOptionItem: { props: ['name', 'rateMultiplier'], template: '<span>{{ name }} {{ rateMultiplier }}x</span>' } } },
})

describe('ordered key group selection', () => {
  it('preserves unavailable selected bindings and explains why they are retained', () => {
    const wrapper = makeSelector()
    expect(wrapper.findAll('li').map(row => row.attributes('data-group-id'))).toEqual(['7', '2'])
    expect(wrapper.get('[data-group-id="7"]').text()).toContain('keys.multiGroup.inactive')
    expect(wrapper.get('[data-group-id="7"]').text()).toContain('Group 7')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('reorders unavailable selections and removes them only on request', async () => {
    const wrapper = makeSelector()
    await wrapper.get('[aria-label="keys.multiGroup.moveDown:Group 7"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[2, 7]])
    await wrapper.setProps({ modelValue: [2, 7] })
    await wrapper.get('[aria-label="keys.multiGroup.remove:Group 7"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[1]).toEqual([[2]])
  })

  it('offers only eligible unselected groups and appends them after current choices', async () => {
    const wrapper = makeSelector({ availableGroups: [group(2), group(3), group(9, 'inactive')] })
    expect(wrapper.findAll('[data-add-group]').map(button => button.attributes('data-add-group'))).toEqual(['3'])
    await wrapper.get('[data-add-group="3"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[7, 2, 3]])
  })

  it('filters additions without marking eligible selections from other providers unavailable', async () => {
    const wrapper = makeSelector({ modelValue: [2], availableGroups: [group(2), group(3)], additionGroupIds: [3] })
    expect(wrapper.findAll('[data-add-group]').map(button => button.attributes('data-add-group'))).toEqual(['3'])
    expect(wrapper.get('[data-group-id="2"]').text()).not.toContain('keys.multiGroup.ineligible')
    await wrapper.setProps({ additionGroupIds: [2] })
    expect(wrapper.findAll('[data-add-group]')).toHaveLength(0)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('retains an active group after access expires, even when omitted from eligible choices', () => {
    const wrapper = makeSelector({ existingGroups: [group(7)] })
    expect(wrapper.get('[data-group-id="7"]').text()).toContain('keys.multiGroup.ineligible')
    expect(wrapper.find('[data-add-group="7"]').exists()).toBe(false)
  })

  it('distinguishes the user minimum from administrator unbinding', async () => {
    const wrapper = makeSelector({ modelValue: [] })
    expect(wrapper.get('[role="status"]').text()).toBe('keys.groupRequired')
    await wrapper.setProps({ allowEmpty: true })
    expect(wrapper.get('[role="status"]').text()).toBe('keys.multiGroup.adminEmpty')
  })

  it('shows the price warning and sticky scope even after reducing to one group', () => {
    const wrapper = makeSelector({ modelValue: [2], multiGroupEnabled: true })
    expect(wrapper.text()).toContain('keys.multiGroup.priceWarning')
    expect(wrapper.text()).toContain('keys.multiGroup.strictScope')
    expect(wrapper.text()).toContain('keys.multiGroup.sharedQuota')
  })

  it('reads legacy single-group records but honors an explicit empty ordered binding', () => {
    expect(keyGroupIds({ group_id: 2 })).toEqual([2])
    expect(keyGroupIds({ group_id: 2, group_ids: [] })).toEqual([])
    expect(keyGroups({ group: group(2) })).toEqual([group(2)])
  })
})
