import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  userList: vi.fn(),
  adminList: vi.fn(),
  refreshUserUnread: vi.fn(),
  refreshAdminUnread: vi.fn(),
  push: vi.fn(),
  replace: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('../api', () => ({
  isSupportTicketFeatureDisabled: () => false,
  supportTicketsUserAPI: { list: mocks.userList },
  supportTicketsAdminAPI: { list: mocks.adminList },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: mocks.showError, fetchPublicSettings: vi.fn() }),
  useAuthStore: () => ({ isAdmin: false }),
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

import SupportTicketListView from '../SupportTicketListView.vue'

const InputStub = defineComponent({
  name: 'SupportInputStub',
  props: { modelValue: { type: [String, Number], default: '' } },
  emits: ['update:modelValue'],
  setup(props, { attrs, emit }) {
    return () => h('input', {
      ...attrs,
      value: props.modelValue,
      onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value),
    })
  },
})

const SelectStub = defineComponent({
  name: 'SupportSelectStub',
  props: {
    modelValue: { type: [String, Number, Boolean], default: '' },
    options: { type: Array, default: () => [] },
  },
  emits: ['update:modelValue'],
  setup(props, { attrs, emit }) {
    return () => h('select', {
      ...attrs,
      value: props.modelValue,
      onChange: (event: Event) => emit('update:modelValue', (event.target as HTMLSelectElement).value),
    }, [
      h('option', { value: '' }, 'all'),
      h('option', { value: 'billing' }, 'billing'),
      h('option', { value: 'pending' }, 'pending'),
      h('option', { value: 'urgent' }, 'urgent'),
    ])
  },
})

const ticket = {
  id: 12,
  user_id: 3,
  user: { id: 3, username: 'alice', email: 'alice@example.com' },
  title: 'Invoice',
  category: 'billing',
  priority: 'urgent',
  status: 'pending',
  unread: true,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-02T00:00:00Z',
}

function page(title = ticket.title) {
  return { items: [{ ...ticket, title }], total: 1, page: 1, page_size: 20, pages: 1 }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

function mountList(admin = false) {
  return mount(SupportTicketListView, {
    props: { admin },
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/><slot name="pagination"/></div>' },
        Input: InputStub,
        Select: SelectStub,
        DataTable: {
          props: ['data'],
          emits: ['row-click'],
          template: '<div><button v-if="data[0]" data-test="row" @click="$emit(\'row-click\', data[0])"><slot name="cell-title" :row="data[0]"/></button></div>',
        },
        EmptyState: true,
        Pagination: true,
        Icon: true,
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('support ticket list', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.userList.mockResolvedValue(page())
    mocks.adminList.mockResolvedValue(page())
    mocks.refreshUserUnread.mockResolvedValue(1)
    mocks.refreshAdminUnread.mockResolvedValue(1)
  })

  it('loads rows without duplicating the app-level unread fetch and routes user rows', async () => {
    const wrapper = mountList()
    await flushPromises()

    expect(mocks.userList).toHaveBeenCalledOnce()
    expect(mocks.refreshUserUnread).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="ticket-unread"]').exists()).toBe(true)
    await wrapper.get('[data-test="row"]').trigger('click')
    expect(mocks.push).toHaveBeenCalledWith('/tickets/12')
  })

  it('submits all admin filters, adds user search, and refreshes the admin count explicitly', async () => {
    const wrapper = mountList(true)
    await flushPromises()

    await wrapper.get('[data-test="ticket-title-filter"]').setValue('  invoice  ')
    await wrapper.get('[data-test="ticket-user-filter"]').setValue('  alice  ')
    await wrapper.get('[data-test="ticket-category-filter"]').setValue('billing')
    await wrapper.get('[data-test="ticket-status-filter"]').setValue('pending')
    await wrapper.get('[data-test="ticket-priority-filter"]').setValue('urgent')
    await wrapper.get('[data-test="ticket-filters"]').trigger('submit')
    await flushPromises()

    expect(mocks.adminList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 20,
      title: '  invoice  ',
      user_search: '  alice  ',
      category: 'billing',
      status: 'pending',
      priority: 'urgent',
    })

    await wrapper.get('[data-test="refresh-tickets"]').trigger('click')
    await flushPromises()
    expect(mocks.refreshAdminUnread).toHaveBeenCalledOnce()
    expect(mocks.refreshUserUnread).not.toHaveBeenCalled()
  })

  it('clears admin rows and reloads from the user endpoint when the reused route changes scope', async () => {
    const staleAdminPage = deferred<ReturnType<typeof page>>()
    const userPage = deferred<ReturnType<typeof page>>()
    mocks.adminList
      .mockResolvedValueOnce(page('Admin private ticket'))
      .mockReturnValueOnce(staleAdminPage.promise)
    mocks.userList.mockReturnValueOnce(userPage.promise)
    const wrapper = mountList(true)
    await flushPromises()
    expect(wrapper.text()).toContain('Admin private ticket')

    await wrapper.get('[data-test="refresh-tickets"]').trigger('click')
    await wrapper.setProps({ admin: false })
    expect(mocks.adminList).toHaveBeenCalledTimes(2)
    expect(mocks.userList).toHaveBeenCalledOnce()
    expect(wrapper.text()).not.toContain('Admin private ticket')

    userPage.resolve(page('User ticket'))
    await flushPromises()
    expect(wrapper.text()).toContain('User ticket')

    staleAdminPage.resolve(page('Stale admin ticket'))
    await flushPromises()
    expect(wrapper.text()).toContain('User ticket')
    expect(wrapper.text()).not.toContain('Stale admin ticket')
  })
})
