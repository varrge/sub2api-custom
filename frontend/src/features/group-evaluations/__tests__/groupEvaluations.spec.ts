import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import messages from '@/i18n/locales/en/channelMonitorV2'
import type { EvaluationConfig, EvaluationDetail, EvaluationKind, EvaluationRecord } from '@/api/groupEvaluations'
import GroupEvaluationDialog from '../GroupEvaluationDialog.vue'
import GroupEvaluationEntries from '../GroupEvaluationEntries.vue'
import GroupEvaluationSettingsPanel from '../GroupEvaluationSettingsPanel.vue'
import { recentEvaluationRecords } from '../evaluationHistory'
import { buildSafePreview, PREVIEW_CSP } from '../safePreview'

const api = vi.hoisted(() => ({
  listEvaluationGroups: vi.fn(),
  getEvaluationDetail: vi.fn(),
  listEvaluationConfigs: vi.fn(),
  saveEvaluationConfig: vi.fn(),
  runEvaluation: vi.fn(),
  success: vi.fn(),
}))
vi.mock('@/api/groupEvaluations', () => ({
  listEvaluationGroups: api.listEvaluationGroups,
  getEvaluationDetail: api.getEvaluationDetail,
  listEvaluationConfigs: api.listEvaluationConfigs,
  saveEvaluationConfig: api.saveEvaluationConfig,
  runEvaluation: api.runEvaluation,
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: api.success }) }))

function config(overrides: Partial<EvaluationConfig> = {}): EvaluationConfig {
  return {
    group_id: 7, group_name: 'Authorized group', group_platform: 'openai', revision: 1, enabled: true,
    candy_enabled: true, pelican_enabled: true, automatic: false, interval_seconds: 600,
    model: 'test-model', effort: 'medium', runs: 3, daily_limit: 100, running: false,
    last_run_at: null, next_run_at: null, ...overrides,
  }
}
function record(id: number, kind: EvaluationKind, overrides: Partial<EvaluationRecord> = {}): EvaluationRecord {
  return {
    id, kind, status: kind === 'candy' ? 'passed' : 'generated', model: `model-${id}`,
    effort: 'medium', created_at: new Date(Date.UTC(2026, 10, 26, 0, 0, id)).toISOString(),
    answer: `${id}`, expected_answer: '29', prompt_version: 'candy-shape-v1',
    html: `<svg><text>Picture ${id}</text></svg>`, error: '', duration_ms: 150, ...overrides,
  }
}
function detail(overrides: Partial<EvaluationDetail> = {}): EvaluationDetail {
  return {
    group: config(),
    records: {
      candy: Array.from({ length: 12 }, (_, index) => record(index + 1, 'candy')),
      pelican: Array.from({ length: 13 }, (_, index) => record(index + 20, 'pelican')),
    }, ...overrides,
  }
}
function globals() {
  return {
    plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: { ...messages, common: { refresh: 'Refresh', loading: 'Loading' } } } })],
    stubs: {
      BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><h3>{{ title }}</h3><slot/><slot name="footer"/></div>' },
      TotpStepUpDialog: { props: ['controller'], template: '<button v-if="controller.visible.value" data-testid="verify" @click="controller.onVerified()">Verify</button>' },
    },
  }
}
const wrappers: Array<{ unmount: () => void }> = []
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers() })
beforeEach(() => {
  vi.clearAllMocks()
  api.getEvaluationDetail.mockResolvedValue(detail())
  api.listEvaluationGroups.mockResolvedValue({ items: [config()] })
  api.listEvaluationConfigs.mockResolvedValue({ items: [config()] })
  api.saveEvaluationConfig.mockImplementation(async (id, input) => config({ ...input, group_id: id, revision: input.revision + 1 }))
  api.runEvaluation.mockResolvedValue({ accepted: true })
})

describe('group evaluation results', () => {
  it('keeps each kind at its own latest ten and switches selected pelican history', async () => {
    const wrapper = mount(GroupEvaluationDialog, { props: { show: true, groupId: 7 }, global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    const candy = wrapper.findAll('[data-testid="candy-record"]')
    expect(candy).toHaveLength(10)
    expect(candy[0].text()).toContain('model-12')
    expect(candy[9].text()).toContain('model-3')
    expect(candy[0].text()).toContain('29')
    expect(candy[0].text()).toContain('candy-shape-v1')
    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    const pelicans = wrapper.findAll('[data-testid="pelican-record"]')
    expect(pelicans).toHaveLength(10)
    expect(pelicans[0].text()).toContain('model-32')
    expect(wrapper.get('iframe').attributes('srcdoc')).toContain('Picture 32')
    await pelicans[4].trigger('click')
    expect(wrapper.get('iframe').attributes('srcdoc')).toContain('Picture 28')
    expect(wrapper.get('iframe').attributes('sandbox')).toBe('')
    expect(wrapper.get('iframe').attributes('referrerpolicy')).toBe('no-referrer')
    await wrapper.findAll('button').find(button => button.text() === 'Refresh')!.trigger('click')
    await flushPromises()
    expect(api.getEvaluationDetail).toHaveBeenCalledTimes(2)
    expect(api.runEvaluation).not.toHaveBeenCalled()
    expect(api.saveEvaluationConfig).not.toHaveBeenCalled()
  })

  it('shows fewer records and excludes errors from the stated accuracy denominator', async () => {
    api.getEvaluationDetail.mockResolvedValue(detail({
      group: config({ pelican_enabled: false }),
      records: {
        candy: [record(1, 'candy'), record(2, 'candy', { status: 'mismatch' }), record(3, 'candy', { status: 'error', error: 'Request failed' })],
        pelican: [record(4, 'pelican')],
      },
    }))
    const wrapper = mount(GroupEvaluationDialog, { props: { show: true, groupId: 7 }, global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.findAll('[data-testid="candy-record"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('50%')
    expect(wrapper.text()).toContain('1 / 2 valid answers matched')
    expect(wrapper.text()).toContain('1 failed attempts excluded from accuracy')
    expect(wrapper.find('[role="tab"]').exists()).toBe(false)
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Pelican')
  })

  it('shows only pelican when candy is disabled and reports an empty history honestly', async () => {
    api.getEvaluationDetail.mockResolvedValue(detail({
      group: config({ candy_enabled: false }),
      records: { candy: [record(1, 'candy')], pelican: [] },
    }))
    const wrapper = mount(GroupEvaluationDialog, { props: { show: true, groupId: 7 }, global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.text()).toContain('Pelican test')
    expect(wrapper.text()).toContain('No results yet')
    expect(wrapper.text()).not.toContain('Candy test')
    expect(wrapper.find('[data-testid="candy-record"]').exists()).toBe(false)
    expect(wrapper.find('iframe').exists()).toBe(false)
  })

  it('polls only GET while running and stops on completion or close', async () => {
    vi.useFakeTimers()
    api.getEvaluationDetail.mockResolvedValueOnce(detail({ group: config({ running: true }) })).mockResolvedValue(detail())
    const wrapper = mount(GroupEvaluationDialog, { props: { show: true, groupId: 7, admin: true }, global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(2500)
    await flushPromises()
    expect(api.getEvaluationDetail).toHaveBeenCalledTimes(2)
    expect(api.getEvaluationDetail.mock.calls[0].slice(0, 2)).toEqual([7, true])
    await vi.advanceTimersByTimeAsync(10000)
    expect(api.getEvaluationDetail).toHaveBeenCalledTimes(2)
    await wrapper.setProps({ show: false })
    expect(api.runEvaluation).not.toHaveBeenCalled()
  })

  it('orders equal timestamps by descending ID without mutating the API records', () => {
    const rows = [record(1, 'candy'), record(3, 'candy'), record(2, 'candy')].map(row => ({ ...row, created_at: '2026-11-26T00:00:00Z' }))
    expect(recentEvaluationRecords(rows).map(row => row.id)).toEqual([3, 2, 1])
    expect(rows.map(row => row.id)).toEqual([1, 3, 2])
  })

  it('cancels pending polling when the dialog closes', async () => {
    vi.useFakeTimers()
    api.getEvaluationDetail.mockResolvedValue(detail({ group: config({ running: true }) }))
    const wrapper = mount(GroupEvaluationDialog, { props: { show: true, groupId: 7 }, global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    const signal = api.getEvaluationDetail.mock.calls[0][2] as AbortSignal
    await wrapper.setProps({ show: false })
    await vi.advanceTimersByTimeAsync(10000)
    expect(signal.aborted).toBe(true)
    expect(api.getEvaluationDetail).toHaveBeenCalledTimes(1)
  })

  it('only shows enabled authorized summaries without interpreting monitor names as IDs', async () => {
    api.listEvaluationGroups.mockResolvedValue({ items: [
      config({ group_id: 91, group_name: 'Real group', candy_enabled: false }),
      config({ group_id: 92, group_name: 'Disabled group', enabled: false }),
      config({ group_id: 93, group_name: 'No tests', candy_enabled: false, pelican_enabled: false }),
    ] })
    const wrapper = mount(GroupEvaluationEntries, { global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.text()).toContain('Real group')
    expect(wrapper.text()).not.toContain('Disabled group')
    expect(wrapper.text()).not.toContain('No tests')
    expect(wrapper.text()).not.toContain('Candy test')
    await wrapper.get('button[aria-label]').trigger('click')
    await flushPromises()
    expect(api.getEvaluationDetail.mock.calls[0][0]).toBe(91)
    expect(api.runEvaluation).not.toHaveBeenCalled()
  })

  it('shares authorized summary IDs with matrix callers and rejects unknown row IDs', async () => {
    api.listEvaluationGroups.mockResolvedValue({ items: [
      config({ group_id: 71, group_name: 'Same display name' }),
      config({ group_id: 72, group_name: 'Same display name', enabled: false }),
    ] })
    const wrapper = mount(GroupEvaluationEntries, { global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.emitted('groups-loaded')?.[0]).toEqual([[expect.objectContaining({ group_id: 71 })]])

    wrapper.vm.openGroup(72)
    await flushPromises()
    expect(api.getEvaluationDetail).not.toHaveBeenCalled()
    wrapper.vm.openGroup(71)
    await flushPromises()
    expect(api.getEvaluationDetail).toHaveBeenCalledWith(71, false, expect.any(AbortSignal))
    expect(api.listEvaluationGroups).toHaveBeenCalledTimes(1)
    expect(api.runEvaluation).not.toHaveBeenCalled()

    api.listEvaluationGroups.mockResolvedValue({ items: [] })
    await wrapper.findAll('button').find(button => button.text() === 'Refresh')!.trigger('click')
    await flushPromises()
    expect(wrapper.emitted('groups-loaded')?.at(-1)).toEqual([[]])
    expect(wrapper.find('[data-testid="candy-record"]').exists()).toBe(false)
  })

  it('removes previously loaded results if access is revoked on refresh', async () => {
    const wrapper = mount(GroupEvaluationDialog, { props: { show: true, groupId: 7 }, global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    api.getEvaluationDetail.mockRejectedValueOnce({ status: 403 })
    await wrapper.findAll('button').find(button => button.text() === 'Refresh')!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="candy-record"]').exists()).toBe(false)
    expect(wrapper.get('[role="alert"]').text()).toContain('Failed to load')
  })
})

describe('isolated generated preview', () => {
  it('preserves static head styles and vector artwork', () => {
    const html = buildSafePreview('<!doctype html><html><head><style>.bird{color:red}</style></head><body><svg class="bird" viewBox="0 0 100 100"><circle r="5"/></svg><img src="data:image/png;base64,abcd"></body></html>')
    expect(html).toContain('<style>.bird{color:red}</style>')
    expect(html).toContain('<circle r="5">')
    expect(html).toContain('viewBox="0 0 100 100"')
    expect(html).toContain('src="data:image/png;base64,abcd"')
  })

  it('places restrictive CSP before sanitized content and removes navigation and scripts', () => {
    const html = buildSafePreview(`<html><head><meta http-equiv="refresh" content="0;url=https://evil.test"><meta http-equiv="Content-Security-Policy" content="default-src *"><base href="https://evil.test"></head><body onload="parent.alert(1)"><script>location='https://evil.test'</script><a href="https://evil.test">link</a><iframe src="https://evil.test"></iframe><form action="https://evil.test"><input></form><svg onload="alert(1)"><foreignObject><div>bad</div></foreignObject><set attributeName="href" to="https://evil.test"/><circle cx="5" cy="5" r="3"/></svg><img src="https://evil.test/track"></body></html>`)
    const parsed = new DOMParser().parseFromString(html, 'text/html')
    expect(parsed.head.firstElementChild?.getAttribute('content')).toBe(PREVIEW_CSP)
    expect(parsed.querySelectorAll('meta[http-equiv]')).toHaveLength(1)
    expect(parsed.querySelector('script, iframe, form, base, foreignObject, set, [onload], [href], img[src]')).toBeNull()
    expect(parsed.querySelector('svg circle')).not.toBeNull()
    expect(html).toContain("default-src 'none'")
    expect(html).toContain("script-src 'none'")
    expect(html).toContain("connect-src 'none'")
    expect(html).toContain("img-src data:")
  })
})

describe('administrator group settings', () => {
  it('saves exact interval seconds with revision and allows a manual run only after saving', async () => {
    const wrapper = mount(GroupEvaluationSettingsPanel, { global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    const interval = wrapper.findAll('input[type="number"]')[1]
    // Enable automatic scheduling before editing the interval.
    await wrapper.findAll('input[type="checkbox"]')[3].setValue(true)
    await interval.setValue('37')
    const run = () => wrapper.findAll('button').find(button => button.text() === 'Run now')!
    expect(run().attributes('disabled')).toBeDefined()
    await wrapper.findAll('button').find(button => button.text() === 'Save settings')!.trigger('click')
    await flushPromises()
    expect(api.saveEvaluationConfig).toHaveBeenCalledWith(7, expect.objectContaining({ interval_seconds: 37, revision: 1, automatic: true }))
    expect(api.runEvaluation).not.toHaveBeenCalled()
    expect(run().attributes('disabled')).toBeUndefined()
    await run().trigger('click')
    await flushPromises()
    expect(api.runEvaluation).toHaveBeenCalledWith(7)
    expect(api.getEvaluationDetail.mock.calls[0].slice(0, 2)).toEqual([7, true])
  })

  it('rejects invalid intervals before any mutation', async () => {
    const wrapper = mount(GroupEvaluationSettingsPanel, { global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    await wrapper.findAll('input[type="checkbox"]')[3].setValue(true)
    await wrapper.findAll('input[type="number"]')[1].setValue('9')
    await wrapper.findAll('button').find(button => button.text() === 'Save settings')!.trigger('click')
    expect(api.saveEvaluationConfig).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('10–86400')
  })

  it('keeps unsaved changes on revision conflicts', async () => {
    const wrapper = mount(GroupEvaluationSettingsPanel, { global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    await wrapper.get('input[type="text"]').setValue('new-model')
    api.saveEvaluationConfig.mockRejectedValueOnce({ status: 409, message: 'Configuration changed; refresh and try again' })
    await wrapper.findAll('button').find(button => button.text() === 'Save settings')!.trigger('click')
    await flushPromises()
    expect(wrapper.get('input[type="text"]').element).toHaveProperty('value', 'new-model')
    expect(wrapper.get('[role="alert"]').text()).toContain('Configuration changed')
  })

  it('does not retry execution or invent results when the server rejects a request cap', async () => {
    const wrapper = mount(GroupEvaluationSettingsPanel, { global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    api.runEvaluation.mockRejectedValueOnce({ status: 429, message: 'Daily request cap reached' })
    await wrapper.findAll('button').find(button => button.text() === 'Run now')!.trigger('click')
    await flushPromises()
    expect(api.runEvaluation).toHaveBeenCalledTimes(1)
    expect(api.getEvaluationDetail).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('Daily request cap reached')
  })

  it('uses step-up retry for configuration mutations', async () => {
    const wrapper = mount(GroupEvaluationSettingsPanel, { global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    api.saveEvaluationConfig.mockRejectedValueOnce({ reason: 'STEP_UP_REQUIRED' })
    await wrapper.get('input[type="text"]').setValue('new-model')
    await wrapper.findAll('button').find(button => button.text() === 'Save settings')!.trigger('click')
    await flushPromises()
    expect(api.saveEvaluationConfig).toHaveBeenCalledTimes(1)
    await wrapper.get('[data-testid="verify"]').trigger('click')
    await flushPromises()
    expect(api.saveEvaluationConfig).toHaveBeenCalledTimes(2)
    expect(api.saveEvaluationConfig.mock.calls[1]).toEqual(api.saveEvaluationConfig.mock.calls[0])
  })

  it('does not enable or execute unsupported group providers', async () => {
    api.listEvaluationConfigs.mockResolvedValue({ items: [config({ group_platform: 'anthropic', enabled: false })] })
    const wrapper = mount(GroupEvaluationSettingsPanel, { global: globals() })
    wrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.findAll('input[type="checkbox"]')[0].attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('button').find(button => button.text() === 'Run now')!.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('OpenAI groups only')
    expect(api.runEvaluation).not.toHaveBeenCalled()
  })
})
