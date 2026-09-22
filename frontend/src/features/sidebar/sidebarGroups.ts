import type { CustomMenuItem, SidebarGroupsConfig } from '@/types'

export type SidebarScope = 'user' | 'admin'
export interface SidebarPageOption { path: string; label: string }

// Only rearrange the caller's already-authorized navigation. Configuration can
// never restore an entry hidden by permissions, feature flags, or simple mode.
export function applySidebarGroups<T extends { path: string; label: string }>(
  items: T[], config: SidebarGroupsConfig | undefined, visibility: SidebarScope, folderIcon: unknown,
): T[] {
  if (!Array.isArray(config?.groups)) return items
  const available = new Map(items.map(item => [item.path, item]))
  const used = new Set<string>()
  const groupIDs = new Set<string>()
  const grouped: T[] = []
  for (const group of config.groups) {
    if (!group || group.visibility !== visibility || !group.id || !group.label?.trim() ||
        !Array.isArray(group.items) || groupIDs.has(group.id)) continue
    groupIDs.add(group.id)
    const children: T[] = []
    for (const path of group.items) {
      const item = available.get(path)
      if (item && !used.has(path)) {
        used.add(path)
        children.push(item)
      }
    }
    if (children.length) grouped.push({
      path: `/__sidebar_group__/${group.id}`, label: group.label,
      icon: folderIcon, expandOnly: true, children,
    } as unknown as T)
  }
  return [...grouped, ...items.filter(item => !used.has(item.path))]
}

// These are top-level sidebar entries. Native admin folders remain intact when
// assigned to a custom group; their feature-filtered children travel with them.
const userPages = [
  ['/dashboard', 'nav.dashboard'], ['/keys', 'nav.apiKeys'],
  ['/image-generation', 'nav.imageGeneration'], ['/batch-image', 'nav.batchImage'],
  ['/usage', 'nav.usage'], ['/available-channels', 'nav.availableChannels'],
  ['/monitor', 'nav.channelStatus'], ['/group-buy', 'groupBuy.hall'],
  ['/my-group-buy', 'groupBuy.myGroupBuy'], ['/subscriptions', 'nav.mySubscriptions'],
  ['/tickets', 'supportTickets.myTickets'], ['/purchase', 'nav.buySubscription'],
  ['/orders', 'nav.myOrders'], ['/redeem', 'nav.redeem'],
  ['/affiliate', 'nav.affiliate'], ['/profile', 'nav.profile'],
]
const adminPages = [
  ['/admin/dashboard', 'nav.dashboard'], ['/admin/ops', 'nav.ops'],
  ['/admin/users', 'nav.users'], ['/admin/groups', 'nav.groups'],
  ['/admin/channels', 'nav.channelManagement'], ['/admin/group-buy', 'groupBuy.admin'],
  ['/admin/subscriptions', 'nav.subscriptions'], ['/admin/accounts', 'nav.accounts'],
  ['/admin/plugins', 'nav.plugins'], ['/admin/announcements', 'nav.announcements'],
  ['/admin/tickets', 'supportTickets.management'], ['/admin/proxies', 'nav.proxies'],
  ['/admin/security-audit', 'nav.securityAudit'], ['/admin/redeem', 'nav.redeemCodes'],
  ['/admin/promo-codes', 'nav.promoCodes'], ['/admin/affiliates', 'nav.affiliateManagement'],
  ['/admin/orders', 'nav.orderManagement'], ['/admin/usage', 'nav.usage'],
  ['/admin/audit-logs', 'nav.auditLogs'], ['/admin/settings', 'nav.settings'], ['/keys', 'nav.apiKeys'],
]

export function sidebarPageOptions(
  visibility: SidebarScope, customMenus: CustomMenuItem[], translate: (key: string) => string,
): SidebarPageOption[] {
  const builtins = visibility === 'admin' ? adminPages : userPages
  const custom = customMenus.filter(item => item.visibility === visibility)
    .slice().sort((a, b) => a.sort_order - b.sort_order)
  return [
    ...builtins.map(([path, key]) => ({ path, label: translate(key) })),
    ...custom.map(item => ({ path: `/custom/${item.id}`, label: item.label })),
  ]
}
