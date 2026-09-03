import type { TopQuickMenuItemId } from '@/types'

export const MAX_TOP_QUICK_MENU_ITEMS = 3

export const TOP_QUICK_MENU_OPTIONS = [
  {
    id: 'image_generation',
    labelKey: 'nav.imageGeneration',
    icon: 'sparkles',
    path: '/image-generation',
    routeName: 'ImageGeneration',
  },
  {
    id: 'batch_image',
    labelKey: 'nav.batchImage',
    icon: 'grid',
    path: '/batch-image',
    routeName: 'BatchImageGuide',
  },
  {
    id: 'model_plaza',
    labelKey: 'nav.modelPlaza',
    icon: 'cube',
    path: '/model-plaza',
    featureSetting: 'model_plaza_enabled',
  },
  {
    id: 'support_tickets',
    labelKey: 'topQuickMenu.supportTickets',
    icon: 'chat',
    path: '/tickets',
    adminPath: '/admin/tickets',
    featureSetting: 'support_ticket_enabled',
  },
  { id: 'api_keys', labelKey: 'nav.apiKeys', icon: 'key', path: '/keys' },
  { id: 'usage', labelKey: 'nav.usage', icon: 'chart', path: '/usage', adminPath: '/admin/usage' },
] as const satisfies ReadonlyArray<{
  id: TopQuickMenuItemId
  labelKey: string
  icon: 'sparkles' | 'grid' | 'cube' | 'chat' | 'key' | 'chart'
  path: string
  adminPath?: string
  routeName?: string
  featureSetting?: 'model_plaza_enabled' | 'support_ticket_enabled'
}>

const validIDs = new Set<TopQuickMenuItemId>(TOP_QUICK_MENU_OPTIONS.map((item) => item.id))

export function normalizeTopQuickMenuItems(value: unknown): TopQuickMenuItemId[] {
  if (!Array.isArray(value)) return []
  const seen = new Set<TopQuickMenuItemId>()
  const result: TopQuickMenuItemId[] = []
  for (const id of value) {
    if (typeof id !== 'string' || !validIDs.has(id as TopQuickMenuItemId)) continue
    const validID = id as TopQuickMenuItemId
    if (seen.has(validID)) continue
    seen.add(validID)
    result.push(validID)
    if (result.length === MAX_TOP_QUICK_MENU_ITEMS) break
  }
  return result
}
