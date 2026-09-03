import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  route: { params: { id: '12' } },
  userGet: vi.fn(),
  adminGet: vi.fn(),
  userRead: vi.fn(),
  adminRead: vi.fn(),
  userReply: vi.fn(),
  adminReply: vi.fn(),
  userAttachment: vi.fn(),
  adminAttachment: vi.fn(),
  updateStatus: vi.fn(),
  updatePriority: vi.fn(),
  refreshUserUnread: vi.fn(),
  refreshAdminUnread: vi.fn(),
  replace: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  createObjectURL: vi.fn(() => 'blob:private-image'),
  revokeObjectURL: vi.fn(),
  isAdmin: false,
}))

vi.mock('../api', () => ({
  isSupportTicketFeatureDisabled: () => false,
  supportTicketsUserAPI: {
    get: mocks.userGet,
    markRead: mocks.userRead,
    reply: mocks.userReply,
    attachment: mocks.userAttachment,
  },
  supportTicketsAdminAPI: {
    get: mocks.adminGet,
    markRead: mocks.adminRead,
    reply: mocks.adminReply,
    attachment: mocks.adminAttachment,
    updateStatus: mocks.updateStatus,
    updatePriority: mocks.updatePriority,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: mocks.showError,
    showSuccess: mocks.showSuccess,
    fetchPublicSettings: vi.fn(),
  }),
  useAuthStore: () => ({ isAdmin: mocks.isAdmin }),
  useSupportTicketStore: () => ({
    refreshUserUnread: mocks.refreshUserUnread,
    refreshAdminUnread: mocks.refreshAdminUnread,
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({ replace: mocks.replace }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

import SupportTicketDetailView from '../SupportTicketDetailView.vue'

const TextAreaStub = defineComponent({
  name: 'TextArea',
  props: { modelValue: { type: String, default: '' }, disabled: Boolean },
  emits: ['update:modelValue'],
  setup(props, { attrs, emit }) {
    return () => h('textarea', {
      ...attrs,
      value: props.modelValue,
      disabled: props.disabled,
      onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLTextAreaElement).value),
    })
  },
})

const SelectStub = defineComponent({
  name: 'SupportSelectStub',
  props: {
    modelValue: { type: [String, Number, Boolean], default: '' },
    options: { type: Array, default: () => [] },
  },
  emits: ['change'],
  setup(props, { attrs, emit }) {
    return () => h('select', {
      ...attrs,
      value: props.modelValue,
      onChange: (event: Event) => emit('change', (event.target as HTMLSelectElement).value, null),
    }, [h('option', { value: 'normal' }, 'normal'), h('option', { value: 'urgent' }, 'urgent')])
  },
})

function detail(status: 'pending' | 'in_progress' | 'closed' = 'pending') {
  return {
    id: 12,
    user_id: 3,
    user: { id: 3, username: 'alice', email: 'alice@example.com' },
    title: 'Unsafe-looking text',
    category: 'account',
    priority: 'normal',
    status,
    unread: true,
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-02T00:00:00Z',
    last_opposing_message_id: 5,
    messages: [{
      id: 5,
      author_role: 'user',
      author: { id: 3, username: 'alice' },
      content: '<script>alert(1)</script>\nsecond line',
      created_at: '2026-09-01T00:00:00Z',
      attachments: [{ id: 8, content_type: 'image/png', size: 3, width: 1, height: 1 }],
    }],
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

function mountDetail(admin = false) {
  return mount(SupportTicketDetailView, {
    props: { admin },
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TextArea: TextAreaStub,
        Select: SelectStub,
        SupportTicketImagePicker: defineComponent({
          props: ['modelValue', 'disabled'],
          emits: ['update:modelValue'],
          template: '<div data-test="image-picker" :data-disabled="disabled" />',
        }),
        Icon: true,
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('support ticket detail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('URL', {
      createObjectURL: mocks.createObjectURL,
      revokeObjectURL: mocks.revokeObjectURL,
    })
    mocks.userGet.mockResolvedValue(detail())
    mocks.adminGet.mockResolvedValue(detail())
    mocks.userRead.mockResolvedValue(undefined)
    mocks.adminRead.mockResolvedValue(undefined)
    mocks.userReply.mockResolvedValue({ id: 6 })
    mocks.adminReply.mockResolvedValue({ id: 6 })
    mocks.userAttachment.mockResolvedValue(new Blob(['img'], { type: 'image/png' }))
    mocks.adminAttachment.mockResolvedValue(new Blob(['img'], { type: 'image/png' }))
    mocks.updateStatus.mockImplementation(async (_id, status) => ({ status }))
    mocks.updatePriority.mockImplementation(async (_id, priority) => ({ priority }))
    mocks.refreshUserUnread.mockResolvedValue(0)
    mocks.refreshAdminUnread.mockResolvedValue(0)
    mocks.isAdmin = false
  })

  it('fetches, marks read, refreshes user unread, and renders message text safely', async () => {
    const wrapper = mountDetail()
    await flushPromises()

    expect(mocks.userGet).toHaveBeenCalledWith(12)
    expect(mocks.userGet.mock.invocationCallOrder[0]).toBeLessThan(mocks.userRead.mock.invocationCallOrder[0])
    expect(mocks.userRead).toHaveBeenCalledWith(12, 5)
    expect(mocks.refreshUserUnread).toHaveBeenCalledOnce()
    expect(mocks.userAttachment).toHaveBeenCalledWith(12, 8)
    expect(wrapper.get('[data-test="ticket-conversation"] p').text()).toBe('<script>alert(1)</script>\nsecond line')
    expect(wrapper.html()).not.toContain('<script>alert(1)</script>')

    wrapper.unmount()
    expect(mocks.revokeObjectURL).toHaveBeenCalledWith('blob:private-image')
  })

  it('disables closed replies and exposes only the exact admin status transitions', async () => {
    mocks.adminGet.mockResolvedValue(detail('closed'))
    const wrapper = mountDetail(true)
    await flushPromises()

    expect(wrapper.find('[data-test="ticket-status-in_progress"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="ticket-status-closed"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="submit-ticket-reply"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="image-picker"]').attributes('data-disabled')).toBe('true')
  })

  it('supports admin priority/status changes and reflects the first-reply status after refetch', async () => {
    mocks.adminGet
      .mockResolvedValueOnce(detail('pending'))
      .mockResolvedValueOnce(detail('in_progress'))
    const wrapper = mountDetail(true)
    await flushPromises()

    await wrapper.get('[data-test="ticket-priority"]').setValue('urgent')
    await flushPromises()
    await wrapper.get('[data-test="ticket-status-in_progress"]').trigger('click')
    await flushPromises()
    expect(mocks.updatePriority).toHaveBeenCalledWith(12, 'urgent')
    expect(mocks.updateStatus).toHaveBeenCalledWith(12, 'in_progress')

    await wrapper.get('#ticket-reply-content').setValue('  Admin reply  ')
    await wrapper.get('[data-test="ticket-reply-form"]').trigger('submit')
    await flushPromises()

    expect(mocks.adminReply).toHaveBeenCalledWith(12, { content: '  Admin reply  ', images: [] })
    expect(mocks.adminGet).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-test="ticket-status-in_progress"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="ticket-status-closed"]').exists()).toBe(true)
  })

  it('clears admin detail and reloads the same ID from the user endpoint when scope changes', async () => {
    const adminDetail = detail()
    adminDetail.title = 'Admin private detail'
    const staleAdminDetail = detail()
    staleAdminDetail.title = 'Stale admin detail'
    const userDetail = detail()
    userDetail.title = 'User detail'
    userDetail.messages[0].content = 'User-visible conversation'
    const staleAdminResponse = deferred<ReturnType<typeof detail>>()
    const userResponse = deferred<ReturnType<typeof detail>>()
    mocks.adminGet
      .mockResolvedValueOnce(adminDetail)
      .mockReturnValueOnce(staleAdminResponse.promise)
    mocks.userGet.mockReturnValueOnce(userResponse.promise)
    const wrapper = mountDetail(true)
    await flushPromises()
    expect(wrapper.text()).toContain('Admin private detail')

    await wrapper.get('[data-test="refresh-ticket-detail"]').trigger('click')
    await wrapper.setProps({ admin: false })
    expect(mocks.adminGet).toHaveBeenCalledTimes(2)
    expect(mocks.userGet).toHaveBeenCalledWith(12)
    expect(wrapper.text()).not.toContain('Admin private detail')
    expect(wrapper.find('[data-test="ticket-conversation"]').exists()).toBe(false)

    userResponse.resolve(userDetail)
    await flushPromises()
    expect(wrapper.text()).toContain('User detail')
    expect(wrapper.text()).toContain('User-visible conversation')

    staleAdminResponse.resolve(staleAdminDetail)
    await flushPromises()
    expect(wrapper.text()).toContain('User detail')
    expect(wrapper.text()).not.toContain('Stale admin detail')
  })

  it('limits attachment downloads to three concurrent requests', async () => {
    const attachmentDetail = detail()
    attachmentDetail.messages[0].attachments = Array.from({ length: 7 }, (_, index) => ({
      id: index + 1,
      content_type: 'image/png',
      size: 3,
      width: 1,
      height: 1,
    }))
    mocks.userGet.mockResolvedValue(attachmentDetail)
    const pending: Array<() => void> = []
    let active = 0
    let maximum = 0
    mocks.userAttachment.mockImplementation(() => new Promise<Blob>((resolve) => {
      active++
      maximum = Math.max(maximum, active)
      pending.push(() => {
        active--
        resolve(new Blob(['img'], { type: 'image/png' }))
      })
    }))

    const wrapper = mountDetail()
    await flushPromises()
    expect(mocks.userAttachment).toHaveBeenCalledTimes(3)

    for (let index = 0; index < 7; index++) {
      expect(pending.length).toBeGreaterThan(0)
      pending.shift()?.()
      await flushPromises()
    }

    expect(mocks.userAttachment).toHaveBeenCalledTimes(7)
    expect(maximum).toBe(3)
    wrapper.unmount()
  })

  it('ignores a detail GET that finishes after a successful status PATCH', async () => {
    const staleGet = deferred<ReturnType<typeof detail>>()
    mocks.adminGet
      .mockResolvedValueOnce(detail('pending'))
      .mockReturnValueOnce(staleGet.promise)
    mocks.updateStatus.mockResolvedValue({ status: 'closed' })
    const wrapper = mountDetail(true)
    await flushPromises()

    await wrapper.get('[data-test="refresh-ticket-detail"]').trigger('click')
    await wrapper.get('[data-test="ticket-status-closed"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="submit-ticket-reply"]').attributes('disabled')).toBeDefined()

    staleGet.resolve(detail('pending'))
    await flushPromises()
    expect(wrapper.get('[data-test="submit-ticket-reply"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-test="ticket-status-in_progress"]').exists()).toBe(true)
  })

  it('ignores a detail GET that finishes before its overlapping status PATCH', async () => {
    const staleGet = deferred<ReturnType<typeof detail>>()
    const patch = deferred<{ status: 'closed' }>()
    mocks.adminGet
      .mockResolvedValueOnce(detail('pending'))
      .mockReturnValueOnce(staleGet.promise)
    mocks.updateStatus.mockReturnValueOnce(patch.promise)
    const wrapper = mountDetail(true)
    await flushPromises()

    await wrapper.get('[data-test="refresh-ticket-detail"]').trigger('click')
    await wrapper.get('[data-test="ticket-status-closed"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="refresh-ticket-detail"]').attributes('disabled')).toBeDefined()

    staleGet.resolve(detail('pending'))
    await flushPromises()
    patch.resolve({ status: 'closed' })
    await flushPromises()
    expect(wrapper.get('[data-test="submit-ticket-reply"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('#ticket-reply-content').attributes('disabled')).toBeDefined()
  })

  it('refreshes both read roles when an admin replies from the user-side UI', async () => {
    mocks.isAdmin = true
    const wrapper = mountDetail()
    await flushPromises()
    mocks.refreshUserUnread.mockClear()
    mocks.refreshAdminUnread.mockClear()

    await wrapper.get('#ticket-reply-content').setValue('user-side admin reply')
    await wrapper.get('[data-test="ticket-reply-form"]').trigger('submit')
    await flushPromises()

    expect(mocks.userReply).toHaveBeenCalledOnce()
    expect(mocks.refreshUserUnread).toHaveBeenCalled()
    expect(mocks.refreshAdminUnread).toHaveBeenCalledOnce()
  })
})
