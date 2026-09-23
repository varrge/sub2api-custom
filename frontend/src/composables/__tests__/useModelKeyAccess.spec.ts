import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import type { ModelAccessKey } from '@/api/modelKeyAccess'
const mocks = vi.hoisted(() => ({ list: vi.fn(), update: vi.fn(), groups: vi.fn(), options: vi.fn(), success: vi.fn(), leave: vi.fn() }))
vi.mock('@/api/modelKeyAccess', () => ({ modelKeyAccessAPI: { list: mocks.list, update: mocks.update } }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getAvailable: mocks.groups } }))
vi.mock('@/api/keys', () => ({ keysAPI: { getModelOptions: mocks.options } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.success }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-router', () => ({ onBeforeRouteLeave: mocks.leave }))
import { modelPolicyAllows, useModelKeyAccess, validModelAccessID } from '../useModelKeyAccess'
function key(id: number, allowed = true): ModelAccessKey {
  return { id, name: `Key ${id}`, status: 'active', expires_at: null,
    groups: [{ id: 1, name: 'OpenAI', platform: 'openai' }], group_ids: [1],
    model_allowlist: allowed ? { enabled: false, models: [] } : { enabled: true, mode: 'allow', models: ['other'] }, revision: `revision-${id}` }
}
async function setup() {
  let state!: ReturnType<typeof useModelKeyAccess>
  const wrapper = mount(defineComponent({ setup() { state = useModelKeyAccess(); return () => null } }))
  await flushPromises()
  return { state, wrapper }
}
beforeEach(() => {
  vi.clearAllMocks()
  mocks.list.mockResolvedValue({ keys: [key(1), key(2), key(3, false)] })
  mocks.groups.mockResolvedValue([{ id: 1, name: 'OpenAI' }])
  mocks.options.mockResolvedValue({ models: [{ id: 'target', group_ids: [1] }] })
})
describe('model policy semantics', () => {
  it('distinguishes empty allowed list, deny list and disabled policy', () => {
    expect(modelPolicyAllows({ enabled: true, models: [] }, 'target')).toBe(false)
    expect(modelPolicyAllows({ enabled: true, mode: 'deny', models: [] }, 'target')).toBe(true)
    expect(modelPolicyAllows({ enabled: false, models: ['dormant'] }, 'target')).toBe(true)
    expect(modelPolicyAllows({ enabled: true, models: ['Target'] }, 'target')).toBe(false)
  })
  it('requires exact concrete IDs but preserves protocol prefixes and case', () => {
    expect(validModelAccessID(' models/Gemini-pro ')).toBe(true)
    for (const id of ['', '  ', '*', 'gpt[5]', 'target\n', 'target\u0085', 'a'.repeat(257)]) expect(validModelAccessID(id)).toBe(false)
  })
})
describe('model access editor', () => {
  it('saves all rows including hidden keys and retains group display metadata', async () => {
    const { state, wrapper } = await setup()
    state.selectModel('target'); state.toggleKey(1, false); state.keySearch.value = 'Key 3'; state.setVisible(true)
    expect(state.rows.value).toHaveLength(1)
    expect(state.addedCount.value).toBe(1); expect(state.removedCount.value).toBe(1)
    mocks.update.mockResolvedValue({ keys: [
      { ...key(1), groups: [], model_allowlist: { enabled: true, mode: 'deny', models: ['target'] }, revision: 'new-1' },
      key(2), { ...key(3), groups: [], revision: 'new-3' }
    ], updated_count: 2 })
    expect(await state.save()).toBe(true)
    expect(mocks.update).toHaveBeenCalledWith('target', [
      { id: 1, allowed: false, revision: 'revision-1' }, { id: 2, allowed: true, revision: 'revision-2' }, { id: 3, allowed: true, revision: 'revision-3' }
    ])
    expect(state.dirty.value).toBe(false); expect(state.rows.value[0].groups[0].name).toBe('OpenAI')
    wrapper.unmount()
  })
  it('does not switch models or discard drafts when a save conflicts', async () => {
    const { state, wrapper } = await setup()
    state.selectModel('target'); state.toggleKey(1, false); state.selectModel('other')
    expect(state.switchPending.value).toBe(true)
    mocks.update.mockRejectedValue({ status: 409 }); await state.resolveSwitch('save')
    expect(state.selectedModel.value).toBe('target'); expect(state.dirty.value).toBe(true)
    expect(state.error.value).toBe('modelKeyAccess.conflict')
    await state.resolveSwitch('cancel'); expect(state.switchPending.value).toBe(false)
    state.selectModel('other'); await state.resolveSwitch('discard')
    expect(state.selectedModel.value).toBe('other'); expect(state.dirty.value).toBe(false)
    wrapper.unmount()
  })
  it('keeps saved and manually entered models when discovery fails', async () => {
    mocks.options.mockRejectedValue(new Error('catalog failed'))
    const { state, wrapper } = await setup()
    expect(state.catalogWarning.value).toBe(true); expect(state.models.value).toContain('other')
    state.selectModel('custom/Model-X')
    expect(state.selectedModel.value).toBe('custom/Model-X'); expect(state.models.value).toContain('custom/Model-X')
    expect(state.rows.value.every(row => row.catalogState === 'unknown')).toBe(true)
    wrapper.unmount()
  })
  it('guards navigation and does not reload away unsaved work', async () => {
    const { state, wrapper } = await setup()
    state.selectModel('target'); state.toggleKey(1, false); state.reload()
    expect(mocks.list).toHaveBeenCalledTimes(1); await state.resolveSwitch('cancel')
    const leave = mocks.leave.mock.calls[0][0]
    const navigation = leave(); await state.resolveSwitch('cancel')
    expect(await navigation).toBe(false); expect(state.dirty.value).toBe(true)
    const next = leave(); await state.resolveSwitch('discard'); expect(await next).toBe(true)
    wrapper.unmount()
  })
  it('does not edit absent keys when loading fails', async () => {
    mocks.list.mockRejectedValue(new Error('offline'))
    const { state, wrapper } = await setup()
    expect(state.error.value).toBe('modelKeyAccess.loadFailed')
    state.selectModel('target'); state.toggleKey(1, false)
    expect(state.dirty.value).toBe(false); expect(mocks.update).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
