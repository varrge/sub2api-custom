import { apiClient } from './client'
import type {
  ChargeAllocation,
  EntitlementOrder,
  GroupBuyProduct,
  GroupBuyTeam,
  MonthCard
} from '@/types/groupBuy'

export const groupBuyAPI = {
  async allocations() {
    return (await apiClient.get<ChargeAllocation[]>('/group-buy/allocations'))
      .data
  },
  async products() {
    return (await apiClient.get<GroupBuyProduct[]>('/group-buy/products')).data
  },
  async teams(group_id?: number) {
    return (
      await apiClient.get<GroupBuyTeam[]>('/group-buy/teams', {
        params: { group_id }
      })
    ).data
  },
  async team(code: string) {
    return (
      await apiClient.get<GroupBuyTeam>(
        `/group-buy/teams/${encodeURIComponent(code)}`
      )
    ).data
  },
  async cards() {
    return (await apiClient.get<MonthCard[]>('/group-buy/cards')).data
  },
  async orders() {
    return (await apiClient.get<EntitlementOrder[]>('/group-buy/order')).data
  },
  async setOrder(order: EntitlementOrder) {
    await apiClient.put('/group-buy/order', order)
  }
}
export const adminGroupBuyAPI = {
  async allocations(user_id?: number) {
    return (
      await apiClient.get<ChargeAllocation[]>('/admin/group-buy/allocations', {
        params: { user_id }
      })
    ).data
  },
  async products() {
    return (await apiClient.get<GroupBuyProduct[]>('/admin/group-buy/products'))
      .data
  },
  async saveProduct(product: GroupBuyProduct) {
    return (
      await (product.id
        ? apiClient.put<GroupBuyProduct>(
            `/admin/group-buy/products/${product.id}`,
            product
          )
        : apiClient.post<GroupBuyProduct>('/admin/group-buy/products', product))
    ).data
  },
  async teams() {
    return (await apiClient.get<GroupBuyTeam[]>('/admin/group-buy/teams')).data
  },
  async team(code: string) {
    const [team, cards] = await Promise.all([
      apiClient.get<GroupBuyTeam>(
        `/admin/group-buy/teams/${encodeURIComponent(code)}`
      ),
      apiClient.get<MonthCard[]>(
        `/admin/group-buy/teams/${encodeURIComponent(code)}/cards`
      )
    ])
    return { ...team.data, cards: cards.data }
  },
  async cards(user_id?: number) {
    return (
      await apiClient.get<MonthCard[]>('/admin/group-buy/cards', {
        params: { user_id }
      })
    ).data
  }
}
