import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import VersionBadge from '../VersionBadge.vue'

const mocks = vi.hoisted(() => ({
  update: vi.fn(), rollback: vi.fn(), restart: vi.fn(),
  getVersion: vi.fn(), fetchVersion: vi.fn(), clearVersionCache: vi.fn()
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
  getVersion: mocks.getVersion, getRollbackVersions: vi.fn().mockResolvedValue({ versions: [] })
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
  needRestart: boolean
  deploymentStatus: string
  dropdownOpen: boolean
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
    if (kind === 'update') {
      expect(api).toHaveBeenNthCalledWith(1, '0.2.22-custom.3', expect.stringMatching(/^system-update-/))
      expect(api.mock.calls[1]).toEqual(api.mock.calls[0])
    }
    if (kind === 'rollback') expect(api).toHaveBeenLastCalledWith('0.2.22-custom.2')
  })

  it('reuses the operation key after a transport failure', async () => {
    mocks.update.mockRejectedValueOnce(new Error('Connection interrupted'))
      .mockResolvedValueOnce({ need_restart: true })
    const wrapper = render()
    const vm = wrapper.vm as unknown as Actions
    await vm.handleUpdate()
    await vm.handleUpdate()
    expect(mocks.update.mock.calls[0][1]).toMatch(/^system-update-/)
    expect(mocks.update.mock.calls[1]).toEqual(mocks.update.mock.calls[0])
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

  it('explains a host deployment lock after verification without claiming an update succeeded', async () => {
    mocks.update.mockRejectedValueOnce({ reason: 'STEP_UP_REQUIRED' })
      .mockRejectedValueOnce({
        status: 403,
        reason: 'WEB_SYSTEM_MUTATIONS_DISABLED',
        message: 'System changes are managed by the deployment pipeline'
      })
    const wrapper = render()
    const vm = wrapper.vm as unknown as Actions
    const pending = vm.handleUpdate()
    await flushPromises()
    await wrapper.get('[data-verify]').trigger('click')
    await pending
    expect(vm.updateSuccess).toBe(false)
    expect(vm.needRestart).toBe(false)
    expect(vm.updateError).toBe('version.deploymentNotConfigured')
    expect(mocks.update).toHaveBeenCalledTimes(2)
    expect(mocks.restart).not.toHaveBeenCalled()
  })

  it('waits for a managed update through temporary outages and never asks for a restart', async () => {
    mocks.update.mockResolvedValue({ deployment_pending: true, target_version: 'v0.2.22-custom.3' })
    mocks.getVersion.mockResolvedValueOnce({ version: '0.2.22-custom.2' })
      .mockRejectedValueOnce(new Error('Service unavailable'))
      .mockResolvedValueOnce({ version: '0.2.22-custom.3' })
    const wrapper = render()
    const vm = wrapper.vm as unknown as Actions
    vm.dropdownOpen = true
    await vm.handleUpdate()
    expect(wrapper.get('[role="status"]').text()).toBe('version.deploymentPending')
    await vi.advanceTimersByTimeAsync(10_000)
    expect(vm.deploymentStatus).toBe('Pending')
    expect(vm.updateSuccess).toBe(false)
    await vi.advanceTimersByTimeAsync(5_000)
    expect(wrapper.get('[role="status"]').text()).toBe('version.deploymentComplete')
    expect(vm.needRestart).toBe(false)
    expect(mocks.restart).not.toHaveBeenCalled()
  })

  it('allows checking an unconfirmed deployment again without submitting another update', async () => {
    mocks.update.mockResolvedValue({ deployment_pending: true, target_version: '0.2.22-custom.3' })
    mocks.getVersion.mockRejectedValue(new Error('Service unavailable'))
    const wrapper = render()
    const vm = wrapper.vm as unknown as Actions
    vm.dropdownOpen = true
    await vm.handleUpdate()
    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(wrapper.get('[role="status"]').text()).toBe('version.deploymentUnconfirmed')
    const recheck = wrapper.findAll('button').find((button) => button.text() === 'version.checkDeployment')
    expect(recheck).toBeDefined()
    await recheck!.trigger('click')
    expect(wrapper.get('[role="status"]').text()).toBe('version.deploymentPending')
    expect(mocks.update).toHaveBeenCalledTimes(1)
  })
})
