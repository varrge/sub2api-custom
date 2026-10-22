import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'

import UsageView from '../UsageView.vue'

const publicSettings = reactive<{ server_utc_offset?: string }>({})

const {
  query,
  getStats,
  getDashboardModels,
  getDashboardSnapshotV2,
  list,
  getAvailable,
  exportMonthly,
  saveAs,
  showError,
  showWarning,
  showSuccess,
  showInfo,
} = vi.hoisted(() => ({
  query: vi.fn(),
  getStats: vi.fn(),
  getDashboardModels: vi.fn(),
  getDashboardSnapshotV2: vi.fn(),
  list: vi.fn(),
  getAvailable: vi.fn(),
  exportMonthly: vi.fn(),
  saveAs: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
}))

vi.mock('@/api', () => ({
  usageAPI: {
    query,
    getStats,
    getDashboardModels,
    getDashboardSnapshotV2,
    exportMonthly,
  },
  keysAPI: {
    list,
  },
  userGroupsAPI: {
    getAvailable,
  },
}))

vi.mock('file-saver', () => ({ saveAs }))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess, showInfo, cachedPublicSettings: publicSettings }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const simpleStub = { template: '<div><slot /></div>' }
const chartStub = { template: '<div />' }

function mountUsageView() {
  return mount(UsageView, {
    global: {
      stubs: {
        AppLayout: simpleStub,
        Pagination: true,
        Select: true,
        DateRangePicker: true,
        Icon: true,
        UsageStatsCards: chartStub,
        UsageTable: chartStub,
        ModelDistributionChart: chartStub,
        GroupDistributionChart: chartStub,
        EndpointDistributionChart: chartStub,
        TokenUsageTrend: chartStub,
      },
    },
  })
}

const currentMonth = () => {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

describe('user UsageView monthly export', () => {
  afterEach(() => vi.useRealTimers())
  beforeEach(() => {
    vi.clearAllMocks()
    publicSettings.server_utc_offset = undefined

    query.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStats.mockResolvedValue({
      total_requests: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_cache_tokens: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
      endpoints: [],
      upstream_endpoints: [],
      endpoint_paths: [],
    })
    getDashboardModels.mockResolvedValue({ models: [], start_date: '2026-10-01', end_date: '2026-10-21' })
    getDashboardSnapshotV2.mockResolvedValue({
      generated_at: '2026-10-21T00:00:00Z',
      start_date: '2026-10-20',
      end_date: '2026-10-21',
      granularity: 'hour',
      trend: [],
      groups: [],
    })
    list.mockResolvedValue({ items: [] })
    getAvailable.mockResolvedValue([])
    exportMonthly.mockResolvedValue({
      blob: new Blob(['zip-bytes'], { type: 'application/zip' }),
      filename: `usage-${currentMonth()}-all.zip`,
    })
  })

  it('defaults the month picker to the current month (YYYY-MM)', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    const input = wrapper.get('[data-testid="monthly-export-month"]')
    expect((input.element as HTMLInputElement).value).toBe(currentMonth())
    expect((input.element as HTMLInputElement).value).toMatch(/^\d{4}-\d{2}$/)
  })

  it('defaults to the site month at the UTC month boundary', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-31T20:30:00Z'))
    publicSettings.server_utc_offset = '+08:00'
    const wrapper = mountUsageView()
    await flushPromises()
    expect((wrapper.get('[data-testid="monthly-export-month"]').element as HTMLInputElement).value).toBe('2026-11')
    wrapper.unmount()
  })

  it('updates the initial month when site settings arrive but preserves an explicit month selection', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-31T20:30:00Z'))
    const wrapper = mountUsageView()
    await flushPromises()
    publicSettings.server_utc_offset = '+08:00'
    await flushPromises()
    const input = wrapper.get('[data-testid="monthly-export-month"]')
    expect((input.element as HTMLInputElement).value).toBe('2026-11')
    await input.setValue('2026-06')
    publicSettings.server_utc_offset = '-07:00'
    await flushPromises()
    expect((input.element as HTMLInputElement).value).toBe('2026-06')
    wrapper.unmount()
  })

  it('primary button exports kind=all and saves the returned zip filename', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    await wrapper.get('[data-testid="monthly-export-all"]').trigger('click')
    await flushPromises()

    expect(exportMonthly).toHaveBeenCalledWith({ month: currentMonth(), kind: 'all' })
    expect(saveAs).toHaveBeenCalledTimes(1)
    const [blob, filename] = saveAs.mock.calls[0]
    expect(blob).toBeInstanceOf(Blob)
    expect(filename).toMatch(new RegExp(`^usage-${currentMonth()}-all\\.zip$`))
    expect(showSuccess).toHaveBeenCalled()
    expect(showError).not.toHaveBeenCalled()
  })

  it.each([
    ['summary', 'monthly-export-summary'],
    ['details', 'monthly-export-details'],
    ['conversions', 'monthly-export-conversions'],
  ])('secondary button %s exports its own kind', async (kind, testid) => {
    exportMonthly.mockResolvedValue({
      blob: new Blob(['csv'], { type: 'text/csv' }),
      filename: `usage-${currentMonth()}-${kind}.csv`,
    })

    const wrapper = mountUsageView()
    await flushPromises()

    await wrapper.get(`[data-testid="${testid}"]`).trigger('click')
    await flushPromises()

    expect(exportMonthly).toHaveBeenCalledWith({ month: currentMonth(), kind })
    expect(saveAs).toHaveBeenCalledTimes(1)
    expect(saveAs.mock.calls[0][1]).toBe(`usage-${currentMonth()}-${kind}.csv`)
  })

  it('surfaces an error and does not save when the API fails', async () => {
    exportMonthly.mockRejectedValue(new Error('boom'))

    const wrapper = mountUsageView()
    await flushPromises()

    await wrapper.get('[data-testid="monthly-export-all"]').trigger('click')
    await flushPromises()

    expect(exportMonthly).toHaveBeenCalledWith({ month: currentMonth(), kind: 'all' })
    expect(saveAs).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('enterpriseExport.exportFailed')
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('disables export buttons when no month is selected', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    const input = wrapper.get('[data-testid="monthly-export-month"]')
    await input.setValue('')

    for (const testid of [
      'monthly-export-all',
      'monthly-export-summary',
      'monthly-export-details',
      'monthly-export-conversions',
    ]) {
      expect(wrapper.get(`[data-testid="${testid}"]`).attributes('disabled')).toBeDefined()
    }
  })

  const setExportSelect = async (
    wrapper: ReturnType<typeof mountUsageView>,
    testid: string,
    value: unknown,
  ) => {
    wrapper.findComponent(`[data-testid="${testid}"]`).vm.$emit('update:modelValue', value)
    await flushPromises()
  }

  it('sends group_id and api_key_id for summary/details exports', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    await setExportSelect(wrapper, 'monthly-export-group', 7)
    await setExportSelect(wrapper, 'monthly-export-api-key', 42)

    await wrapper.get('[data-testid="monthly-export-summary"]').trigger('click')
    await flushPromises()
    expect(exportMonthly).toHaveBeenCalledWith({
      month: currentMonth(),
      kind: 'summary',
      group_id: 7,
      api_key_id: 42,
    })

    exportMonthly.mockClear()
    await wrapper.get('[data-testid="monthly-export-details"]').trigger('click')
    await flushPromises()
    expect(exportMonthly).toHaveBeenCalledWith({
      month: currentMonth(),
      kind: 'details',
      group_id: 7,
      api_key_id: 42,
    })
  })

  it.each(['all', 'conversions'] as const)(
    'omits api_key_id for kind=%s, keeps group_id and warns when a key filter is set',
    async (kind) => {
      const wrapper = mountUsageView()
      await flushPromises()

      await setExportSelect(wrapper, 'monthly-export-group', 7)
      await setExportSelect(wrapper, 'monthly-export-api-key', 42)

      await wrapper.get(`[data-testid="monthly-export-${kind}"]`).trigger('click')
      await flushPromises()

      const call = exportMonthly.mock.calls[0][0]
      expect(call).toEqual({ month: currentMonth(), kind, group_id: 7, api_key_id: undefined })
      expect(showWarning).toHaveBeenCalledWith('enterpriseExport.apiKeyIgnoredForFullPack')
      expect(saveAs).toHaveBeenCalledTimes(1)
    },
  )

  it('shows the api-key hint only when a key filter is set', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    expect(wrapper.find('[data-testid="monthly-export-api-key-hint"]').exists()).toBe(false)
    await setExportSelect(wrapper, 'monthly-export-api-key', 42)
    const hint = wrapper.get('[data-testid="monthly-export-api-key-hint"]')
    expect(hint.text()).toBe('enterpriseExport.apiKeyIgnoredForFullPack')
  })

  it('disables the API key select while an all/conversions export is running', async () => {
    let resolveExport!: (value: { blob: Blob; filename: string }) => void
    exportMonthly.mockReturnValue(
      new Promise((resolve) => {
        resolveExport = resolve
      }),
    )

    const wrapper = mountUsageView()
    await flushPromises()

    const keySelect = () => wrapper.get('[data-testid="monthly-export-api-key"]')
    expect(keySelect().attributes('disabled')).toBe('false')

    await wrapper.get('[data-testid="monthly-export-all"]').trigger('click')
    expect(keySelect().attributes('disabled')).toBe('true')

    resolveExport({ blob: new Blob(['zip']), filename: `usage-${currentMonth()}-all.zip` })
    await flushPromises()
    expect(keySelect().attributes('disabled')).toBe('false')
  })
})
