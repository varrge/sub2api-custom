import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import VersionBadge from '../VersionBadge.vue'

const mocks = vi.hoisted(() => ({
  update: vi.fn(), rollback: vi.fn(), restart: vi.fn(),
  fetchVersion: vi.fn(), clearVersionCache: vi.fn()
}))
vi.mock('@/stores', () => ({
  useAuthStore: () => ({ isAdmin: true }),
  useAppStore: () => ({
    currentVersion: '0.2.22-custom.2', latestVersion: '0.2.22-custom.3',
    hasUpdate: true, buildType: 'release', fetchVersion: mocks.fetchVersion,
    clearVersionCache: mocks.clearVersionCache
  })
}))
vi.mock('@/api/admin/system', () => ({
  performUpdate: mocks.update, rollback: mocks.rollback, restartService: mocks.restart,
  getRollbackVersions: vi.fn().mockResolvedValue({ versions: [] })
}))
vi.mock('@/api', () => ({ totpAPI: { stepUp: vi.fn() } }))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() })
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const dialog = defineComponent({
  props: ['controller'],
  template: `<div v-if="controller.visible.value">
    <button data-verify @click="controller.onVerified()">verify</button>
    <button data-cancel @click="controller.onCancel()">cancel</button>
  </div>`
})
type Actions = {
  handleUpdate: () => Promise<void>
  handleRollback: () => Promise<void>
  handleRestart: () => Promise<void>
  selectedRollbackVersion: string
  restartCountdown: number
  restarting: boolean
  restartError: string
  updateError: string
  updateSuccess: boolean
}

enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers()
})
afterEach(() => { vi.clearAllTimers(); vi.useRealTimers() })

function render() {
  return mount(VersionBadge, {
    global: { stubs: { TotpStepUpDialog: dialog, Icon: true, Transition: true } }
  })
}

describe('system operation verification', () => {
  it.each(['update', 'rollback', 'restart'] as const)('retries %s only after verification', async (kind) => {
    const api = mocks[kind]
    api.mockRejectedValueOnce({ reason: 'STEP_UP_REQUIRED' })
      .mockResolvedValueOnce({ need_restart: true })
    const wrapper = render()
    const vm = wrapper.vm as unknown as Actions
    vm.selectedRollbackVersion = '0.2.22-custom.2'
    const actions = { update: vm.handleUpdate, rollback: vm.handleRollback, restart: vm.handleRestart }
    const pending = actions[kind]()
    await flushPromises()
    expect(api).toHaveBeenCalledTimes(1)
    expect(vm.restartCountdown).toBe(0)
    await wrapper.get('[data-verify]').trigger('click')
    await pending
    expect(api).toHaveBeenCalledTimes(2)
    if (kind === 'rollback') expect(api).toHaveBeenLastCalledWith('0.2.22-custom.2')
  })

  it('does not restart or start a countdown when verification is cancelled', async () => {
    mocks.restart.mockRejectedValue({ reason: 'STEP_UP_REQUIRED' })
    const wrapper = render()
    const vm = wrapper.vm as unknown as Actions
    const pending = vm.handleRestart()
    await flushPromises()
    await wrapper.get('[data-cancel]').trigger('click')
    await pending
    expect(mocks.restart).toHaveBeenCalledTimes(1)
    expect(vm.restartCountdown).toBe(0)
    expect(vm.restarting).toBe(false)
  })

  it('does not treat a forbidden restart as a successful restart', async () => {
    mocks.restart.mockRejectedValue({ status: 403, message: 'Disabled by deployment' })
    const wrapper = render()
    const vm = wrapper.vm as unknown as Actions
    await vm.handleRestart()
    expect(vm.restartCountdown).toBe(0)
    expect(vm.restarting).toBe(false)
    expect(vm.updateError).toBe('')
    expect(vm.restartError).toBe('Disabled by deployment')
  })
})
