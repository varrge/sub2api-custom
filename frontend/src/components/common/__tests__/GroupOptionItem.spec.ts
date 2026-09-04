import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GroupOptionItem from '../GroupOptionItem.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null }),
}))

describe('GroupOptionItem description layout', () => {
  it('applies multiline and overflow-safe text styles', () => {
    const description = 'First section\nvery-long-unbroken-description-value-that-must-not-overflow'
    const wrapper = mount(GroupOptionItem, {
      props: {
        name: 'Example group',
        platform: 'openai',
        description,
      },
      global: {
        stubs: {
          GroupBadge: true,
        },
      },
    })

    const descriptionElement = wrapper
      .findAll('span')
      .find((element) => element.text() === description)

    expect(descriptionElement).toBeDefined()
    expect(descriptionElement?.classes()).toContain('whitespace-pre-line')
    expect(descriptionElement?.classes()).toContain('[overflow-wrap:anywhere]')
    expect(descriptionElement?.classes()).toContain('line-clamp-3')
    expect(wrapper.find('[title]').attributes('title')).toBe(description)
  })

  it('keeps a user rate above an active temporary rate', () => {
    const wrapper = mount(GroupOptionItem, {
      props: {
        name: 'VIP',
        platform: 'openai',
        rateMultiplier: 0.8,
        userRateMultiplier: 0.7,
        temporaryRateEnabled: true,
        temporaryRateMultiplier: 0.5,
        temporaryRateStartsAt: '2000-01-01T00:00:00Z',
        temporaryRateEndsAt: '2099-01-01T00:00:00Z',
      },
      global: { stubs: { GroupBadge: true } },
    })

    expect(wrapper.text()).toContain('0.5x')
    expect(wrapper.text()).toContain('0.7x')
    expect(wrapper.text()).toContain('common.temporaryRate.userOverride')
  })
})
