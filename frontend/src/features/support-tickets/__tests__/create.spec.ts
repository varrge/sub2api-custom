import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  create: vi.fn(),
  refreshUserUnread: vi.fn(),
  refreshAdminUnread: vi.fn(),
  push: vi.fn(),
  replace: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('../api', () => ({
  isSupportTicketFeatureDisabled: () => false,
  supportTicketsUserAPI: { create: mocks.create },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showSuccess: mocks.showSuccess,
    showError: mocks.showError,
    fetchPublicSettings: vi.fn(),
  }),
  useAuthStore: () => ({ isAdmin: true }),
  useSupportTicketStore: () => ({
    refreshUserUnread: mocks.refreshUserUnread,
    refreshAdminUnread: mocks.refreshAdminUnread,
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push, replace: mocks.replace }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

import SupportTicketCreateView from '../SupportTicketCreateView.vue'

const InputStub = defineComponent({
  props: { modelValue: { type: String, default: '' } },
  emits: ['update:modelValue'],
  setup(props, { attrs, emit }) {
    return () => h('input', {
      ...attrs,
      value: props.modelValue,
      onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value),
    })
  },
})

const TextAreaStub = defineComponent({
  props: { modelValue: { type: String, default: '' } },
  emits: ['update:modelValue'],
  setup(props, { attrs, emit }) {
    return () => h('textarea', {
      ...attrs,
      value: props.modelValue,
      onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLTextAreaElement).value),
    })
  },
})

describe('support ticket creation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.create.mockResolvedValue({ id: 42 })
    mocks.refreshUserUnread.mockResolvedValue(0)
    mocks.refreshAdminUnread.mockResolvedValue(1)
  })

  it('refreshes both read roles when an admin creates through the user-side UI', async () => {
    const wrapper = mount(SupportTicketCreateView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Input: InputStub,
          TextArea: TextAreaStub,
          Select: true,
          SupportTicketImagePicker: true,
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })

    await wrapper.get('#ticket-title').setValue('Need help')
    await wrapper.get('#ticket-content').setValue('Details')
    await wrapper.get('[data-test="create-ticket-form"]').trigger('submit')
    await flushPromises()

    expect(mocks.create).toHaveBeenCalledOnce()
    expect(mocks.refreshUserUnread).toHaveBeenCalledOnce()
    expect(mocks.refreshAdminUnread).toHaveBeenCalledOnce()
    expect(mocks.push).toHaveBeenCalledWith('/tickets/42')
  })
})
