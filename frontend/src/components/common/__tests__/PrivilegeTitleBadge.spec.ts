import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { ref, type Ref } from 'vue'
import PrivilegeTitleBadge from '../PrivilegeTitleBadge.vue'
import { userPrivilegeTitleKey } from '@/composables/useUserPrivilegeTitle'

function mountBadge(provided?: Ref<string | null>) {
  const global = { plugins: [createI18n({ legacy: false, locale: 'zh' })] } as Record<string, unknown>
  if (provided) global.provide = { [userPrivilegeTitleKey as symbol]: provided }
  return mount(PrivilegeTitleBadge, { global })
}

describe('PrivilegeTitleBadge', () => {
  it('renders nothing when the injected title is null', () => {
    const view = mountBadge(ref(null))
    expect(view.find('[data-testid="user-privilege-title"]').exists()).toBe(false)
    expect(view.text()).toBe('')
  })

  it('renders nothing when no title is provided at all', () => {
    const view = mountBadge()
    expect(view.find('[data-testid="user-privilege-title"]').exists()).toBe(false)
  })

  it('shows the injected title as escaped plain text', async () => {
    const title = ref('至尊用户')
    const view = mountBadge(title)
    const badge = view.get('[data-testid="user-privilege-title"]')
    expect(badge.text()).toContain('至尊用户')
    title.value = '<img src=x onerror=alert(1)>'
    await view.vm.$nextTick()
    expect(view.find('img').exists()).toBe(false)
    expect(view.get('[data-testid="user-privilege-title"]').text()).toContain('<img src=x onerror=alert(1)>')
  })

  it('updates when the injected ref changes and hides when cleared', async () => {
    const title = ref<string | null>('早起鸟儿')
    const view = mountBadge(title)
    expect(view.get('[data-testid="user-privilege-title"]').text()).toContain('早起鸟儿')
    title.value = '夜猫子'
    await view.vm.$nextTick()
    expect(view.get('[data-testid="user-privilege-title"]').text()).toContain('夜猫子')
    title.value = null
    await view.vm.$nextTick()
    expect(view.find('[data-testid="user-privilege-title"]').exists()).toBe(false)
  })
})
