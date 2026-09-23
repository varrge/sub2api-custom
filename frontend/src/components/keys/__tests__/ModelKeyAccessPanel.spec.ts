import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ModelAccessRow } from '@/api/modelKeyAccess'
import ModelKeyAccessPanel from '../ModelKeyAccessPanel.vue'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key.startsWith('keys.status.')) return `translated:${key}`
      return params ? `${key} ${JSON.stringify(params)}` : key
    }
  })
}))

const RouterLinkStub = { template: '<a><slot /></a>' }

const row = (overrides: Partial<ModelAccessRow> = {}): ModelAccessRow => ({
  id: 1,
  name: 'Production key',
  status: 'active',
  expires_at: null,
  groups: [{ id: 1, name: 'default', platform: 'anthropic' }],
  group_ids: [1],
  allowed: true,
  originalAllowed: true,
  catalogState: 'listed',
  ...overrides
})

const mountPanel = (props: Partial<InstanceType<typeof ModelKeyAccessPanel>['$props']> = {}) =>
  mount(ModelKeyAccessPanel, {
    props: {
      loading: false,
      saving: false,
      error: '',
      catalogWarning: false,
      selectedModel: '',
      modelSearch: '',
      keySearch: '',
      groupFilter: null,
      models: [],
      rows: [],
      groups: [],
      totalCount: 0,
      allowedCount: 0,
      addedCount: 0,
      removedCount: 0,
      dirty: false,
      switchPending: false,
      ...props
    },
    global: { stubs: { RouterLink: RouterLinkStub, teleport: true } }
  })

describe('ModelKeyAccessPanel', () => {
  it('shows an empty state until a model is selected and selects models from the list', async () => {
    const wrapper = mountPanel({ models: ['claude-sonnet-4', 'gpt-5'] })
    expect(wrapper.text()).toContain('modelKeyAccess.noModelSelectedTitle')
    const options = wrapper.findAll('[data-test="model-list"] button')
    expect(options).toHaveLength(2)
    await options[0].trigger('click')
    expect(wrapper.emitted('select-model')).toEqual([['claude-sonnet-4']])
  })

  it('offers a manual full model ID only when the search is not already selected', async () => {
    const wrapper = mountPanel({ modelSearch: 'custom-model-1' })
    await wrapper.get('[data-test="use-manual-model"]').trigger('click')
    expect(wrapper.emitted('select-model')).toEqual([['custom-model-1']])
    await wrapper.setProps({ modelSearch: 'claude-sonnet-4', selectedModel: 'claude-sonnet-4' })
    expect(wrapper.find('[data-test="use-manual-model"]').exists()).toBe(false)
  })

  it('emits toggle-key from the row switch and marks rows changed from the original', async () => {
    const wrapper = mountPanel({
      selectedModel: 'claude-sonnet-4',
      totalCount: 2,
      allowedCount: 1,
      rows: [
        row(),
        row({ id: 2, name: 'Draft key', allowed: true, originalAllowed: false, catalogState: 'unlisted' })
      ]
    })
    expect(wrapper.findAll('[data-test="modified-badge"]')).toHaveLength(1)
    // Catalog state badges are gone: unlisted/unknown rows are filtered out upstream
    expect(wrapper.text()).not.toContain('modelKeyAccess.catalogUnlisted')
    expect(wrapper.text()).not.toContain('modelKeyAccess.catalogUnknown')
    const toggles = wrapper.findAll('[data-test="allow-toggle"]')
    await toggles[0].trigger('click')
    expect(wrapper.emitted('toggle-key')).toEqual([[1, false]])
  })

  it('scopes allow/block buttons to visible rows and keeps save disabled without changes', async () => {
    const wrapper = mountPanel({ selectedModel: 'claude-sonnet-4', rows: [row()] })
    expect(wrapper.get('[data-test="save"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="discard"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="allow-visible"]').trigger('click')
    await wrapper.get('[data-test="block-visible"]').trigger('click')
    expect(wrapper.emitted('set-visible')).toEqual([[true], [false]])
  })

  it('enables save/discard when dirty and reports added and removed counts', async () => {
    const wrapper = mountPanel({
      selectedModel: 'claude-sonnet-4',
      rows: [row()],
      dirty: true,
      addedCount: 2,
      removedCount: 1
    })
    expect(wrapper.text()).toContain('modelKeyAccess.added {"count":2}')
    expect(wrapper.text()).toContain('modelKeyAccess.removed {"count":1}')
    await wrapper.get('[data-test="save"]').trigger('click')
    await wrapper.get('[data-test="discard"]').trigger('click')
    expect(wrapper.emitted('save')).toHaveLength(1)
    expect(wrapper.emitted('discard')).toHaveLength(1)
  })

  it('offers retry on errors without touching the draft', async () => {
    const wrapper = mountPanel({ error: 'network down' })
    expect(wrapper.get('[role="alert"]').text()).toContain('network down')
    await wrapper.get('[role="alert"] button').trigger('click')
    expect(wrapper.emitted('reload')).toHaveLength(1)
  })

  it('shows catalog warning and distinguishes no-keys from no-filter-matches', () => {
    const warning = mountPanel({ catalogWarning: true })
    expect(warning.text()).toContain('modelKeyAccess.catalogWarning')

    const noKeys = mountPanel({ selectedModel: 'm', totalCount: 0 })
    expect(noKeys.text()).toContain('modelKeyAccess.noKeysTitle')

    const noMatches = mountPanel({ selectedModel: 'm', totalCount: 5, keySearch: 'zzz' })
    expect(noMatches.text()).toContain('modelKeyAccess.noMatchesTitle')
  })

  it('blocks interaction while loading or saving', () => {
    const wrapper = mountPanel({
      loading: true,
      saving: false,
      selectedModel: 'm',
      models: ['m'],
      rows: [row()],
      dirty: true
    })
    expect(wrapper.get('[data-test="allow-toggle"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="save"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="allow-visible"]').attributes('disabled')).toBeDefined()
  })

  it('resolves a pending model switch through the dialog with three choices', async () => {
    const wrapper = mountPanel({ selectedModel: 'claude-sonnet-4', switchPending: true })
    await wrapper.get('[data-test="switch-save"]').trigger('click')
    await wrapper.get('[data-test="switch-discard"]').trigger('click')
    await wrapper.get('[data-test="switch-cancel"]').trigger('click')
    expect(wrapper.emitted('resolve-switch')).toEqual([['save'], ['discard'], ['cancel']])

    await wrapper.setProps({ saving: true })
    expect(wrapper.get('[data-test="switch-save"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="switch-discard"]').attributes('disabled')).toBeDefined()
  })

  it('shows a failed save-and-continue error inside the dialog, keeping the draft reachable', async () => {
    const wrapper = mountPanel({
      selectedModel: 'claude-sonnet-4',
      switchPending: true,
      error: 'conflict: revision mismatch'
    })
    const dialogError = wrapper.get('[data-test="dialog-error"]')
    expect(dialogError.text()).toContain('conflict: revision mismatch')
    // Cancel stays available so the user keeps the draft after a failed save
    await wrapper.get('[data-test="switch-cancel"]').trigger('click')
    expect(wrapper.emitted('resolve-switch')).toEqual([['cancel']])
  })

  it('hides the dialog close affordance while saving', async () => {
    const wrapper = mountPanel({ selectedModel: 'm', switchPending: true })
    expect(wrapper.find('button[aria-label="Close modal"]').exists()).toBe(true)
    await wrapper.setProps({ saving: true })
    expect(wrapper.find('button[aria-label="Close modal"]').exists()).toBe(false)
  })

  it('lays out status and expiry on a second full-width line on mobile', () => {
    const wrapper = mountPanel({
      selectedModel: 'm',
      rows: [row({ expires_at: '2020-01-01T00:00:00Z' })]
    })
    const status = wrapper.get('[data-test="key-row-status"]')
    // w-full forces the wrap onto its own line on small screens; sm+ restores the right column
    expect(status.classes()).toContain('w-full')
    expect(status.classes()).toContain('sm:w-auto')
    expect(status.classes()).toContain('flex-wrap')
  })

  it('shows status and expiration accurately for inactive and expired keys', () => {
    const wrapper = mountPanel({
      selectedModel: 'm',
      rows: [
        row({ id: 1, status: 'inactive' }),
        row({ id: 2, name: 'Expired key', status: 'expired', expires_at: '2020-01-01T00:00:00Z' }),
        row({ id: 3, name: 'Quota key', status: 'quota_exhausted', expires_at: '2999-01-01T00:00:00Z' })
      ]
    })
    expect(wrapper.text()).toContain('translated:keys.status.inactive')
    expect(wrapper.text()).toContain('translated:keys.status.expired')
    expect(wrapper.text()).toContain('translated:keys.status.quota_exhausted')
    const expiryBadges = wrapper.findAll('[data-test="key-row"] .text-red-500')
    expect(expiryBadges.length).toBeGreaterThanOrEqual(1)
  })
})
