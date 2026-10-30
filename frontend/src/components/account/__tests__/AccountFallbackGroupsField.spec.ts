import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountFallbackGroupsField from '../AccountFallbackGroupsField.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('AccountFallbackGroupsField', () => {
  it('shows roles only for selected groups and removes roles when membership is removed', async () => {
    const wrapper = mount(AccountFallbackGroupsField, {
      props: {
        modelValue: [1],
        groupIds: [1, 2],
        groups: [{ id: 1, name: 'A' }, { id: 2, name: 'B' }, { id: 3, name: 'C' }]
      }
    })
    expect(wrapper.findAll('input').map(input => input.attributes('value'))).toEqual(['1', '2'])
    expect(wrapper.get<HTMLInputElement>('input[value="1"]').element.checked).toBe(true)
    expect(wrapper.get<HTMLInputElement>('input[value="2"]').element.checked).toBe(false)
    await wrapper.get('input[value="2"]').setValue(true)
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[1, 2]])
    await wrapper.setProps({ modelValue: [1, 2], groupIds: [2] })
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[2]])
  })
})
