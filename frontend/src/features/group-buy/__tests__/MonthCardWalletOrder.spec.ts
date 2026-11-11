import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { VueDraggable } from 'vue-draggable-plus'
import MonthCardWalletOrder from '../MonthCardWalletOrder.vue'
import type { WalletGroup } from '../walletModel'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

function groups(): WalletGroup[] {
  return [{
    id: 3,
    name: 'Pool',
    items: [1, 2].map(id => ({ kind: 'legacy', id, group_id: 3, name: `#${id}`, expires_at: null })),
  }]
}

describe('wallet drag order', () => {
  let wrapper: VueWrapper
  afterEach(() => wrapper?.unmount())

  it('saves the current draft when the drag library retains an earlier end callback', async () => {
    wrapper = mount(MonthCardWalletOrder, { props: { groups: groups(), selectedId: null, saving: false } })
    const draggable = wrapper.getComponent(VueDraggable)
    const finish = draggable.props('onEnd') as () => void

    // The wallet clock replaces group objects; production Sortable can retain the old callback.
    await wrapper.setProps({ groups: groups() })
    draggable.vm.$emit('start')
    draggable.vm.$emit('update:modelValue', [...draggable.props('modelValue')].reverse())
    finish()

    expect(wrapper.emitted('reorder')).toEqual([
      [3, [{ kind: 'legacy', id: 2 }, { kind: 'legacy', id: 1 }]],
    ])
  })
})
