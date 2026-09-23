import { apiClient } from './client'
import type { AdminMonthCardRules, MonthCardRuleDraft, MonthCardRules } from '@/types/monthCardRules'

export const monthCardRulesAPI = {
  async get() { return (await apiClient.get<MonthCardRules>('/group-buy/rules')).data },
  async read(id: string, version: string) {
    await apiClient.post('/group-buy/rules/read', { id, version })
  },
  async admin() { return (await apiClient.get<AdminMonthCardRules>('/admin/group-buy/rules')).data },
  async save(draft_revision: number, documents: MonthCardRuleDraft[]) {
    return (await apiClient.put<AdminMonthCardRules>('/admin/group-buy/rules', { draft_revision, documents })).data
  },
  async publish(draft_revision: number) {
    return (await apiClient.post<AdminMonthCardRules>('/admin/group-buy/rules/publish', { draft_revision })).data
  }
}
