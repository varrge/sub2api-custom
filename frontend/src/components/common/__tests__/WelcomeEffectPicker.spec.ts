import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import WelcomeEffectPicker from '../WelcomeEffectPicker.vue'

const start = (props = {}) => mount(WelcomeEffectPicker, {
  props, global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] }
})

describe('WelcomeEffectPicker', () => {
  it('leaves an unknown admin draft unselected so it can be corrected directly to A', async () => {
    const view = start({ modelValue: 'future-effect' })
    expect(view.findAll('input').some(input => (input.element as HTMLInputElement).checked)).toBe(false)
    await view.get('input[value="capsule"]').setValue(true)
    expect(view.emitted('update:modelValue')).toEqual([['capsule']])
    view.unmount()
  })
  it('offers four labeled native radios, defaults A, and emits changes', async () => {
    const view = start()
    expect(view.attributes('role')).toBe('radiogroup')
    expect(view.findAll('input[type="radio"]')).toHaveLength(4)
    expect((view.get('input[value="capsule"]').element as HTMLInputElement).checked).toBe(true)
    await view.get('input[value="halo"]').setValue(true)
    expect(view.emitted('update:modelValue')).toEqual([['halo']])
    await view.setProps({ modelValue: 'halo' })
    expect(view.get('[data-effect="halo"]').classes()).toContain('wep-option-selected')
    view.unmount()
  })

  it('uses separate radio groups and blocks changes while disabled', async () => {
    const first = start({ modelValue: 'ribbon' })
    const second = start({ disabled: true })
    expect(first.get('input').attributes('name')).not.toBe(second.get('input').attributes('name'))
    expect(second.findAll('input').every(input => (input.element as HTMLInputElement).disabled)).toBe(true)
    await second.get('input[value="slide"]').trigger('change')
    expect(second.emitted('update:modelValue')).toBeUndefined()
    first.unmount()
    second.unmount()
  })
})
