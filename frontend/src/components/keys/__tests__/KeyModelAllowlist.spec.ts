import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { ApiKeyModelAllowlist, ApiKeyModelOptions } from '@/types'
import KeyModelAllowlist from '../KeyModelAllowlist.vue'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const catalog = (...ids: string[]): ApiKeyModelOptions => ({ models: ids.map(id => ({ id, group_ids: [1] })) })
const deferred = () => {
  let resolve!: (value: ApiKeyModelOptions) => void
  const promise = new Promise<ApiKeyModelOptions>(done => { resolve = done })
  return { promise, resolve }
}
const mountSelector = (loadOptions: (ids: number[]) => Promise<ApiKeyModelOptions>, value: ApiKeyModelAllowlist = { enabled: true, models: ['saved-model'] }) => {
  const wrapper = mount(KeyModelAllowlist, { props: {
    groupIds: [1, 2], modelValue: value, loadOptions,
    'onUpdate:modelValue': (next: ApiKeyModelAllowlist) => { void wrapper.setProps({ modelValue: next }) }
  } })
  return wrapper
}

describe('API key allowed model selector', () => {
  it('loads the selected group union, deduplicates models, and keeps selection explicit', async () => {
    const loadOptions = vi.fn().mockResolvedValue({ models: [
      { id: 'shared', group_ids: [1] }, { id: 'shared', group_ids: [2] },
      { id: 'group-two', group_ids: [2] }
    ] })
    const wrapper = mountSelector(loadOptions, { enabled: false, models: [] })
    await flushPromises()
    expect(loadOptions).toHaveBeenCalledWith([1, 2])
    expect(wrapper.findAll('input[value="shared"]')).toHaveLength(1)
    expect(wrapper.get('input[value="shared"]').element).toMatchObject({ checked: true, disabled: true })
    await wrapper.get('[data-test="model-restriction-enabled"]').setValue(true)
    expect(wrapper.props('modelValue')).toEqual({ enabled: true, models: [] })
    expect(wrapper.get('[data-test="empty-allowlist-hint"]').text()).toBe('modelKeyAccess.allBlocked')
    await wrapper.get('input[value="group-two"]').setValue(true)
    expect(wrapper.props('modelValue')).toEqual({ enabled: true, models: ['group-two'] })
  })

  it('ignores an old request after groups change and does not grant newly available models', async () => {
    const old = deferred()
    const current = deferred()
    const loadOptions = vi.fn().mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const wrapper = mountSelector(loadOptions)
    expect(wrapper.get('[role="status"]').text()).toBe('keys.modelRestriction.loading')
    expect(wrapper.get('[data-test="retained-models"]').text()).toContain('saved-model')
    await wrapper.setProps({ groupIds: [3] })
    current.resolve(catalog('current-model'))
    await flushPromises()
    old.resolve(catalog('stale-model'))
    await flushPromises()
    expect(loadOptions).toHaveBeenLastCalledWith([3])
    expect(wrapper.find('input[value="stale-model"]').exists()).toBe(false)
    expect(wrapper.get('input[value="current-model"]').element).toMatchObject({ checked: false })
    expect(wrapper.get('input[value="saved-model"]').element).toMatchObject({ checked: true })
    expect(wrapper.props('modelValue')).toEqual({ enabled: true, models: ['saved-model'] })
    expect(wrapper.get('[data-test="retained-models"]').text()).toContain('keys.modelRestriction.unavailableHint')
    await wrapper.get('input[value="saved-model"]').setValue(false)
    expect(wrapper.props('modelValue')).toEqual({ enabled: true, models: [] })
  })

  it('keeps selections on errors and on retry without silently enabling or disabling restrictions', async () => {
    const loadOptions = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue(catalog('saved-model', 'new-model'))
    const wrapper = mountSelector(loadOptions)
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('keys.modelRestriction.loadFailed')
    expect(wrapper.get('input[value="saved-model"]').element).toMatchObject({ checked: true })
    expect(wrapper.props('modelValue')).toEqual({ enabled: true, models: ['saved-model'] })
    await wrapper.findAll('button').find(button => button.text() === 'keys.modelRestriction.retry')!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('input[value="saved-model"]').element).toMatchObject({ checked: true })
    expect(wrapper.get('input[value="new-model"]').element).toMatchObject({ checked: false })
  })

  it('searches models, selects all available without dropping unavailable selections, and clears explicitly', async () => {
    const wrapper = mountSelector(vi.fn().mockResolvedValue(catalog('alpha', 'beta')))
    await flushPromises()
    await wrapper.get('input[type="search"]').setValue('ALP')
    expect(wrapper.find('input[value="alpha"]').exists()).toBe(true)
    expect(wrapper.find('input[value="beta"]').exists()).toBe(false)
    await wrapper.findAll('button').find(button => button.text() === 'keys.modelRestriction.selectAll')!.trigger('click')
    expect(wrapper.props('modelValue').models).toEqual(['saved-model', 'alpha', 'beta'])
    await wrapper.findAll('button').find(button => button.text() === 'keys.modelRestriction.clear')!.trigger('click')
    expect(wrapper.props('modelValue')).toEqual({ enabled: true, models: [] })
  })

  it('keeps restrictions and selections when all groups are removed', async () => {
    const loadOptions = vi.fn().mockResolvedValue(catalog('saved-model'))
    const wrapper = mountSelector(loadOptions)
    await flushPromises()
    await wrapper.setProps({ groupIds: [] })
    await flushPromises()
    expect(loadOptions).toHaveBeenCalledTimes(1)
    expect(wrapper.props('modelValue')).toEqual({ enabled: true, models: ['saved-model'] })
    expect(wrapper.get('[data-test="retained-models"]').text()).toContain('keys.modelRestriction.unavailableHint')
  })
})

it('switches selection meaning without changing selected IDs and accepts an empty deny list', async () => {
  const wrapper = mountSelector(vi.fn().mockResolvedValue(catalog('saved-model', 'other')))
  await flushPromises()
  await wrapper.get('[data-test="model-mode-deny"]').trigger('click')
  expect(wrapper.props('modelValue')).toEqual({ enabled: true, mode: 'deny', models: ['saved-model'] })
  expect(wrapper.get('[data-test="model-mode-deny"]').attributes('aria-pressed')).toBe('true')
  expect(wrapper.text()).toContain('keys.modelRestriction.deniedHint')
  await wrapper.get('input[value="saved-model"]').setValue(false)
  expect(wrapper.props('modelValue')).toEqual({ enabled: true, mode: 'deny', models: [] })
  expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  await wrapper.get('[data-test="model-restriction-enabled"]').setValue(false)
  await wrapper.get('[data-test="model-restriction-enabled"]').setValue(true)
  expect(wrapper.props('modelValue').mode).toBe('deny')
  await wrapper.get('[data-test="model-mode-allow"]').trigger('click')
  expect(wrapper.get('[data-test="empty-allowlist-hint"]').text()).toBe('modelKeyAccess.allBlocked')
})
