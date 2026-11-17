import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import EndpointDistributionChart from '../EndpointDistributionChart.vue'

const { getUserBreakdown } = vi.hoisted(() => ({ getUserBreakdown: vi.fn() }))

vi.mock('@/api/admin/dashboard', () => ({ getUserBreakdown }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})
vi.mock('vue-chartjs', () => ({
  Doughnut: {
    props: ['data'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  },
}))

const UserBreakdownSubTableStub = defineComponent({
  props: ['showAccountCost'],
  template: '<div />',
})

const endpointStats = [
  { endpoint: 'responses', requests: 8, total_tokens: 1000, cost: 10, actual_cost: 6, account_cost: 1 },
  { endpoint: 'messages', requests: 4, total_tokens: 500, cost: 20, actual_cost: 2, account_cost: 8 },
]

describe('EndpointDistributionChart', () => {
  beforeEach(() => getUserBreakdown.mockReset().mockResolvedValue({ users: [] }))

  it('orders and charts actual charges independently of the account cost column', () => {
    const wrapper = mount(EndpointDistributionChart, {
      props: { endpointStats, metric: 'actual_cost' },
    })

    const chartData = JSON.parse(wrapper.get('.chart-data').text())
    expect(chartData.labels).toEqual(['responses', 'messages'])
    expect(chartData.datasets[0].data).toEqual([6, 2])
    expect(wrapper.findAll('thead th')).toHaveLength(6)
    expect(wrapper.findAll('tbody tr')[0].findAll('td').map(cell => cell.text()))
      .toEqual(['responses', '8', '1.00K', '$6.00', '$1.00', '$10.00'])
    wrapper.unmount()
  })

  it.each([true, false])('keeps breakdown columns aligned when showAccountCost=%s', async showAccountCost => {
    const wrapper = mount(EndpointDistributionChart, {
      props: { endpointStats, showAccountCost, enableBreakdown: true },
      global: { stubs: { UserBreakdownSubTable: UserBreakdownSubTableStub } },
    })

    expect(wrapper.findAll('thead th')).toHaveLength(showAccountCost ? 6 : 5)
    expect(wrapper.text().includes('admin.dashboard.accountCost')).toBe(showAccountCost)
    await wrapper.get('tbody tr').trigger('click')
    await flushPromises()
    expect(getUserBreakdown).toHaveBeenCalledWith(expect.objectContaining({ endpoint: 'responses' }))
    expect(wrapper.get('td[colspan]').attributes('colspan')).toBe(showAccountCost ? '6' : '5')
    expect(wrapper.getComponent(UserBreakdownSubTableStub).props('showAccountCost')).toBe(showAccountCost)
    wrapper.unmount()
  })
})
