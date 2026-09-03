import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SupportTicketImagePicker from '../SupportTicketImagePicker.vue'
import {
  SUPPORT_TICKET_CONTENT_MAX,
  SUPPORT_TICKET_IMAGE_MAX,
  SUPPORT_TICKET_TITLE_MAX,
  supportTicketCharacterCount,
  supportTicketTextIsValid,
} from '../types'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

const createObjectURL = vi.fn((file: File) => `blob:${file.name}`)
const revokeObjectURL = vi.fn()

function selectFiles(wrapper: ReturnType<typeof mount>, files: File[]) {
  const input = wrapper.get('input[type="file"]')
  Object.defineProperty(input.element, 'files', { configurable: true, value: files })
  return input.trigger('change')
}

describe('support ticket form validation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('URL', { ...URL, createObjectURL, revokeObjectURL })
  })

  it('counts trimmed Unicode code points and enforces title/content bounds', () => {
    expect(supportTicketCharacterCount('  工单😀  ')).toBe(3)
    expect(supportTicketTextIsValid('   ', SUPPORT_TICKET_TITLE_MAX)).toBe(false)
    expect(supportTicketTextIsValid('x'.repeat(SUPPORT_TICKET_TITLE_MAX), SUPPORT_TICKET_TITLE_MAX)).toBe(true)
    expect(supportTicketTextIsValid('x'.repeat(SUPPORT_TICKET_TITLE_MAX + 1), SUPPORT_TICKET_TITLE_MAX)).toBe(false)
    expect(supportTicketTextIsValid('x'.repeat(SUPPORT_TICKET_CONTENT_MAX), SUPPORT_TICKET_CONTENT_MAX)).toBe(true)
  })

  it('rejects unsupported, oversized, and excess images before emitting', async () => {
    const wrapper = mount(SupportTicketImagePicker, { props: { modelValue: [] } })

    await selectFiles(wrapper, [new File(['x'], 'x.gif', { type: 'image/gif' })])
    expect(wrapper.text()).toContain('supportTickets.errors.imageType')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    const oversized = new File(['x'], 'large.png', { type: 'image/png' })
    Object.defineProperty(oversized, 'size', { value: SUPPORT_TICKET_IMAGE_MAX + 1 })
    await selectFiles(wrapper, [oversized])
    expect(wrapper.text()).toContain('supportTickets.errors.imageSize')

    await wrapper.setProps({
      modelValue: [0, 1].map((index) => new File(['x'], `${index}.png`, { type: 'image/png' })),
    })
    await selectFiles(wrapper, [
      new File(['x'], '2.png', { type: 'image/png' }),
      new File(['x'], '3.png', { type: 'image/png' }),
    ])
    expect(wrapper.text()).toContain('supportTickets.errors.imageCount')
  })

  it('previews valid images and revokes URLs on removal, reset, and unmount', async () => {
    const first = new File(['one'], 'one.png', { type: 'image/png' })
    const second = new File(['two'], 'two.webp', { type: 'image/webp' })
    const wrapper = mount(SupportTicketImagePicker, { props: { modelValue: [] } })

    await selectFiles(wrapper, [first, second])
    const selected = wrapper.emitted('update:modelValue')?.[0][0] as File[]
    await wrapper.setProps({ modelValue: selected })
    await flushPromises()
    expect(wrapper.findAll('img')).toHaveLength(2)

    await wrapper.get('[data-test="remove-ticket-image"]').trigger('click')
    const remaining = wrapper.emitted('update:modelValue')?.at(-1)?.[0] as File[]
    await wrapper.setProps({ modelValue: remaining })
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:one.png')

    await wrapper.setProps({ modelValue: [] })
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:two.webp')

    await wrapper.setProps({ modelValue: [first] })
    wrapper.unmount()
    expect(revokeObjectURL).toHaveBeenLastCalledWith('blob:one.png')
  })
})
