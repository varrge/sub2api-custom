import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import SidebarGroupsSettings from '../SidebarGroupsSettings.vue'
import type { SidebarGroupsConfig } from '@/types'

const mocks = vi.hoisted(() => ({
  getSidebarGroups: vi.fn<() => Promise<SidebarGroupsConfig>>(),
  updateSidebarGroups: vi.fn<(config: SidebarGroupsConfig) => Promise<SidebarGroupsConfig>>(),
  showSuccess: vi.fn(),
  sidebarGroups: { groups: [] as unknown[] },
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: { value: 'zh' },
    t: (key: string) => key,
  }),
}))

vi.mock('@/api/admin/settings', () => ({
  getSidebarGroups: () => mocks.getSidebarGroups(),
  updateSidebarGroups: (config: SidebarGroupsConfig) => mocks.updateSidebarGroups(config),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: mocks.showSuccess, cachedPublicSettings: null }),
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({
    get sidebarGroups() { return mocks.sidebarGroups },
    set sidebarGroups(value: unknown) { mocks.sidebarGroups = value as typeof mocks.sidebarGroups },
  }),
}))

let wrapper: VueWrapper | undefined

function fixture(groups: SidebarGroupsConfig['groups']): SidebarGroupsConfig {
  return { groups }
}

async function mountSettings(props: Record<string, unknown> = {}) {
  wrapper = mount(SidebarGroupsSettings, {
    props,
    global: { stubs: { Icon: { template: '<i />' } } },
  })
  await flushPromises()
  return wrapper
}

describe('SidebarGroupsSettings', () => {
  beforeEach(() => {
    mocks.getSidebarGroups.mockReset()
    mocks.updateSidebarGroups.mockReset()
    mocks.showSuccess.mockReset()
    mocks.getSidebarGroups.mockResolvedValue(fixture([]))
    mocks.updateSidebarGroups.mockImplementation(config => Promise.resolve(config))
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })

  it('loads and shows the empty state with an add affordance', async () => {
    await mountSettings()
    expect(mocks.getSidebarGroups).toHaveBeenCalledTimes(1)
    expect(wrapper!.get('[data-testid="sidebar-groups-empty"]').exists()).toBe(true)
    expect(wrapper!.find('[data-testid="sidebar-groups-save"]').exists()).toBe(true)
  })

  it('shows retry on load failure and hides the editor', async () => {
    mocks.getSidebarGroups.mockRejectedValueOnce(new Error('offline'))
    await mountSettings()
    expect(wrapper!.get('[data-testid="sidebar-groups-load-error"]').text()).toContain('分组加载失败')
    expect(wrapper!.find('[data-testid="sidebar-group-card"]').exists()).toBe(false)

    mocks.getSidebarGroups.mockResolvedValueOnce(fixture([
      { id: 'g1', label: '常用', visibility: 'user', items: ['/keys'] },
    ]))
    await wrapper!.get('[data-testid="sidebar-groups-retry"]').trigger('click')
    await flushPromises()

    expect(wrapper!.find('[data-testid="sidebar-groups-load-error"]').exists()).toBe(false)
    expect((wrapper!.get('[data-testid="sidebar-group-label"]').element as HTMLInputElement).value).toBe('常用')
  })

  it('creates, renames and deletes groups without exposing ids', async () => {
    await mountSettings()
    await wrapper!.get('[data-testid="sidebar-groups-add"]').trigger('click')

    const labelInput = wrapper!.get('[data-testid="sidebar-group-label"]')
    await labelInput.setValue('运营工具')
    expect(wrapper!.html()).not.toContain('group_')

    await wrapper!.get('[data-testid="sidebar-group-remove"]').trigger('click')
    expect(wrapper!.find('[data-testid="sidebar-group-card"]').exists()).toBe(false)
  })

  it('clears items when the visibility scope changes', async () => {
    await mountSettings()
    await wrapper!.get('[data-testid="sidebar-groups-add"]').trigger('click')

    const addPicker = wrapper!.get('[data-testid="sidebar-group-item-add"]')
    await addPicker.setValue('/keys')
    expect(wrapper!.findAll('[data-testid="sidebar-group-item"]')).toHaveLength(1)

    await wrapper!.get('[data-testid="sidebar-group-visibility"]').setValue('admin')
    expect(wrapper!.findAll('[data-testid="sidebar-group-item"]')).toHaveLength(0)
    expect(wrapper!.get('[data-testid="sidebar-group-items-empty"]').exists()).toBe(true)
  })

  it('adds, orders and removes pages inside a group', async () => {
    await mountSettings()
    await wrapper!.get('[data-testid="sidebar-groups-add"]').trigger('click')
    const picker = wrapper!.get('[data-testid="sidebar-group-item-add"]')

    await picker.setValue('/keys')
    await picker.setValue('/usage')
    await picker.setValue('/redeem')
    const labels = () => wrapper!.findAll('[data-testid="sidebar-group-item"]')
      .map(row => row.text())

    expect(labels()).toHaveLength(3)
    expect(labels()[0]).toContain('nav.apiKeys')

    // /keys cannot be added twice: the picker no longer offers it
    const pickerOptions = wrapper!.get('[data-testid="sidebar-group-item-add"]')
      .findAll('option').map(o => o.attributes('value'))
    expect(pickerOptions).not.toContain('/keys')

    // move /usage above /keys
    const moveUps = wrapper!.findAll('[data-testid="sidebar-group-item-move-up"]')
    await moveUps[1].trigger('click')
    expect(labels()[0]).toContain('nav.usage')

    // remove it again
    await wrapper!.findAll('[data-testid="sidebar-group-item-remove"]')[1].trigger('click')
    expect(labels()).toHaveLength(2)
  })

  it('reorders groups with move buttons', async () => {
    mocks.getSidebarGroups.mockResolvedValueOnce(fixture([
      { id: 'a', label: 'A组', visibility: 'user', items: [] },
      { id: 'b', label: 'B组', visibility: 'user', items: [] },
    ]))
    await mountSettings()

    const labels = () => wrapper!.findAll('[data-testid="sidebar-group-label"]')
      .map(input => (input.element as HTMLInputElement).value)
    expect(labels()).toEqual(['A组', 'B组'])

    await wrapper!.findAll('[data-testid="sidebar-group-move-up"]')[1].trigger('click')
    expect(labels()).toEqual(['B组', 'A组'])
  })

  it('offers visibility-matched custom menu pages and saves through the API', async () => {
    await mountSettings({
      customMenuItems: [
        { id: 'lottery', label: '抽奖中心', url: 'https://example.com', visibility: 'user', sort_order: 0 },
        { id: 'boss', label: 'BOSS战', url: 'https://example.com/boss', visibility: 'admin', sort_order: 1 },
      ],
    })
    await wrapper!.get('[data-testid="sidebar-groups-add"]').trigger('click')
    await wrapper!.get('[data-testid="sidebar-group-label"]').setValue('活动')

    const pickerOptions = wrapper!.get('[data-testid="sidebar-group-item-add"]')
      .findAll('option')
    const userOptions = pickerOptions.map(o => o.text())
    expect(userOptions).toContain('抽奖中心')
    expect(userOptions).not.toContain('BOSS战')

    await wrapper!.get('[data-testid="sidebar-group-item-add"]').setValue('/custom/lottery')
    await wrapper!.get('[data-testid="sidebar-group-item-add"]').setValue('/keys')

    await wrapper!.get('[data-testid="sidebar-groups-save"]').trigger('click')
    await flushPromises()

    expect(mocks.updateSidebarGroups).toHaveBeenCalledTimes(1)
    const payload = mocks.updateSidebarGroups.mock.calls[0][0]
    expect(payload.groups).toHaveLength(1)
    expect(payload.groups[0].label).toBe('活动')
    expect(payload.groups[0].visibility).toBe('user')
    expect(payload.groups[0].items).toEqual(['/custom/lottery', '/keys'])
    expect(mocks.showSuccess).toHaveBeenCalledTimes(1)
  })

  it('blocks saving an unnamed group', async () => {
    await mountSettings()
    await wrapper!.get('[data-testid="sidebar-groups-add"]').trigger('click')
    await wrapper!.get('[data-testid="sidebar-groups-save"]').trigger('click')
    await flushPromises()

    expect(mocks.updateSidebarGroups).not.toHaveBeenCalled()
    expect(wrapper!.get('[data-testid="sidebar-groups-error"]').text()).toContain('分组名称')
  })

  it('keeps the draft and shows an error when saving fails', async () => {
    mocks.getSidebarGroups.mockResolvedValueOnce(fixture([
      { id: 'g1', label: '常用', visibility: 'user', items: ['/keys'] },
    ]))
    await mountSettings()
    mocks.updateSidebarGroups.mockRejectedValueOnce(new Error('boom'))

    await wrapper!.get('[data-testid="sidebar-groups-save"]').trigger('click')
    await flushPromises()

    expect(wrapper!.get('[data-testid="sidebar-groups-error"]').text()).toContain('保存失败')
    expect(wrapper!.find('[data-testid="sidebar-group-card"]').exists()).toBe(true)
    expect(wrapper!.find('[data-testid="sidebar-groups-load-error"]').exists()).toBe(false)
  })

  it('prevents Enter inside the group name input from submitting the parent form', async () => {
    await mountSettings()
    await wrapper!.get('[data-testid="sidebar-groups-add"]').trigger('click')
    const event = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
    wrapper!.get('[data-testid="sidebar-group-label"]').element.dispatchEvent(event)
    expect(event.defaultPrevented).toBe(true)
    expect(mocks.updateSidebarGroups).not.toHaveBeenCalled()
  })
})
