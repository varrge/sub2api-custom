import type { ApiKey, Group } from '@/types'

// Older servers/exports may only include the compatibility projection.
export function keyGroupIds(key: Pick<ApiKey, 'group_id'> & Partial<Pick<ApiKey, 'group_ids'>>): number[] {
  return key.group_ids ? [...key.group_ids] : key.group_id ? [key.group_id] : []
}

export function keyGroups(key: Partial<Pick<ApiKey, 'groups' | 'group'>> | null): Group[] {
  return key?.groups ?? (key?.group ? [key.group] : [])
}

export function groupDisplayProps(group: Group) {
  return {
    name: group.name,
    platform: group.platform,
    subscriptionType: group.subscription_type,
    rateMultiplier: group.rate_multiplier,
    temporaryRateEnabled: group.temporary_rate_enabled,
    temporaryRateMultiplier: group.temporary_rate_multiplier,
    temporaryRateStartsAt: group.temporary_rate_starts_at,
    temporaryRateEndsAt: group.temporary_rate_ends_at,
    peakRateEnabled: group.peak_rate_enabled,
    peakStart: group.peak_start,
    peakEnd: group.peak_end,
    peakRateMultiplier: group.peak_rate_multiplier
  }
}
