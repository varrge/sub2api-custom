import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import TotpStepUpDialog from '../TotpStepUpDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useStepUp } from '@/composables/useStepUp'

const mocks = vi.hoisted(() => ({ stepUp: vi.fn() }))
vi.mock('@/api', () => ({ totpAPI: { stepUp: mocks.stepUp } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

enableAutoUnmount(afterEach)
afterEach(() => { document.body.innerHTML = '' })
beforeEach(() => vi.resetAllMocks())

function render() {
  const controller = useStepUp()
  const wrapper = mount(defineComponent({
    components: { BaseDialog, TotpStepUpDialog },
    setup: () => ({ controller, editing: ref(true), draft: ref('unsaved model') }),
    template: `
      <aside style="position:fixed;transform:translateX(0);z-index:30;width:256px">
        <button id="open-step-up" @click="controller.prompt()">Update</button>
        <TotpStepUpDialog :controller="controller" />
      </aside>
      <BaseDialog :show="editing" title="Model editor" :z-index="80" @close="editing = false">
        <input id="model-draft" v-model="draft" />
      </BaseDialog>
    `
  }), { attachTo: document.body })
  return { wrapper, controller }
}

function verificationDialog() {
  return document.querySelector('input[maxlength="1"]')!
    .closest<HTMLElement>('[role="dialog"], .fixed.inset-0')!
}

describe('step-up above the editor and sidebar', () => {
  it('escapes the transformed sidebar and places verification above existing dialogs', async () => {
    const { wrapper, controller } = render()
    await nextTick()
    const pending = controller.prompt()
    await flushPromises()
    const dialog = verificationDialog()
    expect(dialog.parentElement).toBe(document.body)
    expect(dialog.getAttribute('role')).toBe('dialog')
    expect(wrapper.find('aside input').exists()).toBe(false)
    expect(Number(dialog.style.zIndex)).toBeGreaterThan(80)
    expect(dialog.contains(document.activeElement)).toBe(true)
    controller.onCancel()
    expect(await pending).toBe(false)
  })

  it('Escape cancels only verification, preserving the editor draft and scroll lock', async () => {
    const { wrapper, controller } = render()
    await nextTick()
    const pending = controller.prompt()
    await nextTick()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(controller.visible.value).toBe(false)
    expect(await pending).toBe(false)
    expect(wrapper.vm.editing).toBe(true)
    expect(wrapper.vm.draft).toBe('unsaved model')
    expect(document.body.classList.contains('modal-open')).toBe(true)
  })

  it('retries the original operation only after verification succeeds and keeps the editor open', async () => {
    const { wrapper, controller } = render()
    const action = vi.fn().mockRejectedValueOnce({ reason: 'STEP_UP_REQUIRED' }).mockResolvedValue('saved')
    let completeVerification!: () => void
    mocks.stepUp.mockReturnValue(new Promise<void>(resolve => { completeVerification = resolve }))
    const pending = controller.run(action)
    await flushPromises()
    const dialog = verificationDialog()
    const input = dialog.querySelector<HTMLInputElement>('input[maxlength="1"]')!
    input.dispatchEvent(Object.assign(new Event('paste', { bubbles: true, cancelable: true }), {
      clipboardData: { getData: () => '123456' }
    }))
    await flushPromises()
    expect(mocks.stepUp).toHaveBeenCalledWith('123456')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(controller.visible.value).toBe(true)
    expect(wrapper.vm.editing).toBe(true)
    expect(action).toHaveBeenCalledTimes(1)
    completeVerification()
    expect(await pending).toBe('saved')
    await nextTick()
    expect(action).toHaveBeenCalledTimes(2)
    expect(controller.visible.value).toBe(false)
    expect(wrapper.vm.draft).toBe('unsaved model')
  })

  it('shows a rejected code inside the top dialog without retrying the protected action', async () => {
    const { controller } = render()
    const action = vi.fn().mockRejectedValue({ reason: 'STEP_UP_REQUIRED' })
    mocks.stepUp.mockRejectedValue(new Error('Invalid verification code'))
    const pending = controller.run(action).catch(error => error)
    await flushPromises()
    const dialog = verificationDialog()
    dialog.querySelector('input[maxlength="1"]')!.dispatchEvent(
      Object.assign(new Event('paste', { bubbles: true, cancelable: true }), {
        clipboardData: { getData: () => '123456' }
      })
    )
    await flushPromises()
    expect(dialog.querySelector('[role="alert"]')?.textContent).toBe('Invalid verification code')
    expect(controller.visible.value).toBe(true)
    expect(action).toHaveBeenCalledTimes(1)
    expect(dialog.querySelector<HTMLInputElement>('input[maxlength="1"]')!.value).toBe('')
    controller.onCancel()
    await pending
  })
})
