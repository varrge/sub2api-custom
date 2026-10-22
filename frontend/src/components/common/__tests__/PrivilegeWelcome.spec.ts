import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import PrivilegeWelcome from '../PrivilegeWelcome.vue'
import { welcomeEffects } from '@/utils/welcomeEffects'

describe('PrivilegeWelcome', () => {
  it.each(welcomeEffects)('renders the %s variant with an accessible dismiss button', async effect => {
    const wrapper = mount(PrivilegeWelcome, {
      props: { message: '欢迎伙伴', phase: 'dwell', effect },
      global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] }
    })
    expect(wrapper.attributes('data-effect')).toBe(effect)
    expect(wrapper.classes()).toContain(`pw-effect-${effect}`)
    expect(wrapper.attributes('aria-live')).toBe('polite')
    await wrapper.get('[data-testid="privilege-welcome-close"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('defaults missing and unknown effects to capsule', async () => {
    const wrapper = mount(PrivilegeWelcome, {
      props: { message: '欢迎伙伴', phase: 'dwell' },
      global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] }
    })
    expect(wrapper.attributes('data-effect')).toBe('capsule')
    await wrapper.setProps({ effect: 'unknown' as never })
    expect(wrapper.attributes('data-effect')).toBe('capsule')
    expect(wrapper.classes()).not.toContain('pw-effect-unknown')
    wrapper.unmount()
  })
  it('retains an inaccessible waiting node as the transition starting point', async () => {
    const wrapper = mount(PrivilegeWelcome, {
      props: { message: '欢迎尊贵的企业用户', phase: 'waiting' },
      global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] }
    })
    expect(wrapper.classes()).toContain('pw-stage-waiting')
    expect(wrapper.attributes('aria-hidden')).toBe('true')
    expect(wrapper.attributes('role')).toBe('status')
    expect(wrapper.attributes('aria-live')).toBe('polite')
    await wrapper.setProps({ phase: 'entering' })
    expect(wrapper.classes()).toContain('pw-stage-entering')
    expect(wrapper.attributes('aria-hidden')).toBe('false')
    await wrapper.get('[data-testid="privilege-welcome-close"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('renders administrator text as text, with an English accessible close label', () => {
    const text = '<img src=x onerror=alert(1)>'
    const wrapper = mount(PrivilegeWelcome, {
      props: { message: text, phase: 'dwell' },
      global: { plugins: [createI18n({ legacy: false, locale: 'en' })] }
    })
    expect(wrapper.text()).toContain(text)
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.get('button').attributes('aria-label')).toBe('Dismiss welcome message')
    wrapper.unmount()
  })
})
