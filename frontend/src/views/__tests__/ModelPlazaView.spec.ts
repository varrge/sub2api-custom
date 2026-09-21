import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ModelPlazaView from '../ModelPlazaView.vue'

const state = vi.hoisted(() => ({ authenticated: true, query: {} as Record<string, string> }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: state.query }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: state.authenticated }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ fetchPublicSettings: vi.fn() }) }))
vi.mock('@/api/modelPlaza', () => ({ getModelPlaza: async () => ({ description: '', groups: [] }) }))

let wrapper: VueWrapper
afterEach(() => { wrapper?.unmount(); state.authenticated = true; state.query = {} })
async function render() {
  wrapper = mount(ModelPlazaView, { global: { stubs: {
    AppLayout: { template: '<div data-console><slot /></div>' },
    PlazaNavBar: { template: '<nav data-plaza-nav />' },
    ModelPlazaContent: true
  } } })
  await flushPromises()
}

describe('ModelPlazaView layout', () => {
  it('keeps the direct authenticated entry in the standalone screenshot layout', async () => {
    await render()
    expect(wrapper.find('main').exists()).toBe(true)
    expect(wrapper.find('[data-plaza-nav]').exists()).toBe(true)
    expect(wrapper.find('[data-console]').exists()).toBe(false)
  })
  it('preserves the explicit embedded console entry', async () => {
    state.query = { embedded: '1' }
    await render()
    expect(wrapper.find('[data-console]').exists()).toBe(true)
    expect(wrapper.find('[data-plaza-nav]').exists()).toBe(false)
  })
  it('never embeds anonymous visitors in the console', async () => {
    state.authenticated = false
    state.query = { embedded: '1' }
    await render()
    expect(wrapper.find('main').exists()).toBe(true)
    expect(wrapper.find('[data-console]').exists()).toBe(false)
  })
})
