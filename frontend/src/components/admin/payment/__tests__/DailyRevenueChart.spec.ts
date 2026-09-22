import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DailyRevenueChart from '../DailyRevenueChart.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-chartjs', () => ({
  Line: { name: 'Line', props: ['data', 'options'], template: '<div />' },
}))

describe('payment revenue chart', () => {
  it('keeps separate currency amounts and order counts while changing theme without a reload', async () => {
    document.documentElement.classList.remove('dark')
    const wrapper = mount(DailyRevenueChart, {
      props: { data: [
        { date: '2026-09-01', amount: { CNY: 198, USD: 12 }, count: 3 },
        { date: '2026-09-02', amount: { CNY: 99 }, count: 1 },
      ] },
    })
    try {
      const chart = wrapper.findComponent({ name: 'Line' })
      const before = chart.props('data')
      expect(before.datasets.map((series: { data: number[] }) => series.data)).toEqual([[198, 99], [12, 0], [3, 1]])
      const light = chart.props('options').scales.y.ticks.color
      document.documentElement.classList.add('dark')
      await flushPromises()
      expect(chart.props('options').scales.y.ticks.color).not.toBe(light)
      expect(chart.props('data')).toEqual(before)
    } finally {
      wrapper.unmount()
      document.documentElement.classList.remove('dark')
    }
  })
})
