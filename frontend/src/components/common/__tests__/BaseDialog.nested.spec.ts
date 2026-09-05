import { afterEach, describe, expect, it } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'
import { mount } from '@vue/test-utils'
import BaseDialog from '../BaseDialog.vue'

afterEach(() => { document.body.innerHTML = '' })

describe('nested dialogs preserve the parent draft', () => {
  it('Escape closes only the top dialog and retains scroll lock until the parent closes', async () => {
    const wrapper = mount(defineComponent({
      components: { BaseDialog },
      setup: () => ({ parent: ref(true), child: ref(false), draft: ref('unsaved price') }),
      template: `<BaseDialog :show="parent" title="Channel" @close="parent = false; draft = ''">
        <input v-model="draft" />
        <button id="open-import" @click="child = true">Import</button>
        <BaseDialog :show="child" title="Prices" @close="child = false" />
      </BaseDialog>`
    }), { attachTo: document.body })
    await nextTick()
    expect(document.body.classList.contains('modal-open')).toBe(true)
    document.querySelector<HTMLButtonElement>('#open-import')!.click()
    await nextTick()
    const titles = [...document.querySelectorAll('[role="dialog"]')].map(element => element.getAttribute('aria-labelledby'))
    expect(new Set(titles).size).toBe(2)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(wrapper.vm.parent).toBe(true)
    expect(wrapper.vm.child).toBe(false)
    expect(wrapper.vm.draft).toBe('unsaved price')
    expect(document.body.classList.contains('modal-open')).toBe(true)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(wrapper.vm.parent).toBe(false)
    expect(document.body.classList.contains('modal-open')).toBe(false)
    wrapper.unmount()
  })
})
