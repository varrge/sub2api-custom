import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ShakeView from '../ShakeView.vue'
import * as api from '@/api/admin/shake'

vi.mock('@/api/admin/shake', () => ({
  listCampaigns: vi.fn(),
  listRewardJobs: vi.fn(),
  getCampaign: vi.fn(),
  listDraws: vi.fn(),
  grantCards: vi.fn(),
  activateCampaign: vi.fn(),
}))

const showError = vi.fn()
const showSuccess = vi.fn()
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))

const prompt = vi.fn()
vi.mock('@/composables/useStepUp', () => ({
  // Mirrors the real controller: STEP_UP_REQUIRED prompts once, then retries.
  useStepUp: () => ({
    visible: { value: false },
    async run<T>(action: () => Promise<T>) {
      try {
        return await action()
      } catch (err) {
        if ((err as { code?: string }).code !== 'STEP_UP_REQUIRED') throw err
        prompt()
        return await action()
      }
    }
  }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => ''
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key })
}))

const campaign: api.ShakeCampaign = {
  id: 'shake-abc',
  status: 'active',
  stored_status: 'active',
  config_version: 3,
  config: { name: '双十一', starts_at: '2030-11-01T00:00:00+08:00', ends_at: '2030-11-12T00:00:00+08:00', rules: [], prizes: [] },
  created_at: '2030-10-01T00:00:00+08:00',
  ended_at: null,
  end_reason: ''
}

const mountView = async () => {
  const wrapper = mount(ShakeView, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true, BaseDialog: true, TotpStepUpDialog: true, ShakeCampaignEditor: true } }
  })
  await flushPromises()
  return wrapper
}

describe('ShakeView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.listCampaigns).mockResolvedValue([campaign])
    vi.mocked(api.listRewardJobs).mockResolvedValue([])
    vi.mocked(api.getCampaign).mockResolvedValue({ campaign, stats: { cards: [], users: 0, draws: 0, balance_paid: 0 } })
    vi.mocked(api.listDraws).mockResolvedValue([])
  })

  it('lists campaigns with their copyable ID', async () => {
    const wrapper = await mountView()
    const row = wrapper.get('[data-testid="shake-campaign-shake-abc"]')
    expect(row.text()).toContain('双十一')
    expect(row.text()).toContain('shake-abc')
    expect(row.text()).toContain('shakeAdmin.status_active')
  })

  it('retries a manual grant once after TOTP step-up', async () => {
    vi.mocked(api.grantCards)
      .mockRejectedValueOnce({ code: 'STEP_UP_REQUIRED' })
      .mockResolvedValueOnce({ granted: 2 })
    const wrapper = await mountView()
    await wrapper.get('[data-testid="shake-campaign-shake-abc"] button').trigger('click')
    await flushPromises()
    const form = wrapper.get('[data-testid="shake-grant"]')
    const inputs = form.findAll('input')
    await inputs[0].setValue('42')
    await inputs[1].setValue('2')
    await inputs[2].setValue('补偿测试')
    await form.trigger('submit')
    await flushPromises()
    expect(prompt).toHaveBeenCalledTimes(1)
    expect(api.grantCards).toHaveBeenCalledTimes(2)
    expect(api.grantCards).toHaveBeenLastCalledWith('shake-abc', { user_id: 42, card_tier: 'low', quantity: 2, reason: '补偿测试' })
    expect(showSuccess).toHaveBeenCalledWith('shakeAdmin.done_granted')
    expect(showError).not.toHaveBeenCalled()
  })
})
