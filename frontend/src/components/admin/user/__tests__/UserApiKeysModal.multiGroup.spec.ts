import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { AdminUser, ApiKey, Group } from '@/types'
import UserApiKeysModal from '../UserApiKeysModal.vue'
import GroupReplaceModal from '../GroupReplaceModal.vue'

const { getKeys, getGroups, updateGroups, replaceGroup, showError } = vi.hoisted(() => ({
  getKeys: vi.fn(), getGroups: vi.fn(), updateGroups: vi.fn(), replaceGroup: vi.fn(), showError: vi.fn()
}))
vi.mock('@/api/admin', () => ({ adminAPI: {
  users: { getUserApiKeys: getKeys, getAvailableGroups: getGroups, replaceGroup },
  apiKeys: { updateApiKeyGroups: updateGroups },
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))
const user = { id: 18, email: 'test@example.com', username: 'target' } as AdminUser
const group = (id: number) => ({ id, name: `Group ${id}`, status: 'active', platform: 'openai', subscription_type: 'standard' } as Group)
const key = { id: 4, key: 'sk-test-long-api-key', name: 'mixed', group_id: 7, group_ids: [7, 2], groups: [group(7), group(2)], multi_group_enabled: true } as ApiKey
const global = { stubs: {
  BaseDialog: { props: ['show'], template: '<div v-if="show" role="dialog"><slot /><slot name="footer" /></div>' },
  GroupOptionItem: true, GroupBadge: true, Icon: true,
} }

beforeEach(() => {
  vi.clearAllMocks()
  getKeys.mockResolvedValue({ items: [key] })
  getGroups.mockResolvedValue([group(2), group(3)])
  updateGroups.mockResolvedValue({ api_key: key, auto_granted_group_access: false })
  replaceGroup.mockResolvedValue({ migrated_keys: 1 })
})

describe('admin API key group configuration', () => {
  it('loads only the target user’s eligible groups and preserves existing selections', async () => {
    const wrapper = mount(UserApiKeysModal, { props: { show: true, user }, global })
    await flushPromises()
    expect(getGroups).toHaveBeenCalledWith(18)
    await wrapper.get('[aria-label="keys.multiGroup.editGroups"]').trigger('click')
    const selector = wrapper.findComponent({ name: 'OrderedKeyGroupSelector' })
    expect(selector.props('modelValue')).toEqual([7, 2])
    expect(selector.props('availableGroups').map((item: Group) => item.id)).toEqual([2, 3])
    expect(selector.props('existingGroups').map((item: Group) => item.id)).toEqual([7, 2])
    expect(selector.props('allowEmpty')).toBe(true)
    await selector.vm.$emit('update:modelValue', [])
    const save = wrapper.findAll('button').find(button => button.text() === 'common.save')!
    expect(save.attributes('disabled')).toBeUndefined()
    await save.trigger('click')
    await flushPromises()
    expect(updateGroups).toHaveBeenCalledWith(4, [])
  })

  it('blocks binding edits when eligibility cannot be loaded', async () => {
    getGroups.mockRejectedValue(new Error('network unavailable'))
    const wrapper = mount(UserApiKeysModal, { props: { show: true, user }, global })
    await flushPromises()
    expect(wrapper.get('[aria-label="keys.multiGroup.editGroups"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('keys.multiGroup.loadFailed')
    expect(updateGroups).not.toHaveBeenCalled()
  })

  it('limits batch replacement to eligible targets and submits the selected replacement', async () => {
    const wrapper = mount(GroupReplaceModal, { props: {
      show: true, user, oldGroup: { id: 7, name: 'Group 7' }, allGroups: [],
    }, global })
    await flushPromises()
    expect(getGroups).toHaveBeenCalledWith(18)
    expect(wrapper.findAll('input[type="radio"]').map(input => input.attributes('value'))).toEqual(['2', '3'])
    await wrapper.get('input[value="3"]').setValue()
    const save = wrapper.findAll('button').find(button => button.text() === 'admin.users.replaceGroupConfirm')!
    await save.trigger('click')
    await flushPromises()
    expect(replaceGroup).toHaveBeenCalledWith(18, 7, 3)
  })
})
