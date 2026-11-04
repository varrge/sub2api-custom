import { apiClient } from '@/api/client'

export type PushEvent = 'payment.recharged' | 'subscription.purchased' | 'monthcard.purchased' | 'team.full' | 'ops.alert' | 'shared_credit.low'
export interface PushChannel {
  id: string
  name: string
  enabled: boolean
  type: 'serverchan' | 'webhook'
  credential: string
  configured: boolean
  endpoint_hint: string
  events: PushEvent[]
  title_template: string
  body_template: string
  tags_template: string
  quota_threshold: number
  serverchan_channel: string
  webhook_format: 'json' | 'form'
}
export interface PushConfig { revision: number; enabled: boolean; channels: PushChannel[] }
export interface PushPreview { title: string; body: string; tags: string }
export interface PushDelivery {
  id: number; channel_id: string; channel_name: string; event: string; title: string
  status: 'pending' | 'sending' | 'sent' | 'failed' | 'cancelled'
  attempts: number; error: string; created_at: string; sent_at: string | null
}
const path = '/admin/settings/push-notifications'
export const pushNotificationsApi = {
  async get() { return (await apiClient.get<PushConfig>(path)).data },
  async save(config: PushConfig) { return (await apiClient.put<PushConfig>(path, config)).data },
  async preview(channel: PushChannel, event: PushEvent) { return (await apiClient.post<PushPreview>(`${path}/preview`, { channel, event })).data },
  async test(id: string, revision: number) { return (await apiClient.post(`${path}/${encodeURIComponent(id)}/test`, { revision })).data },
  async history() { return (await apiClient.get<PushDelivery[]>(`${path}/history`)).data },
}
