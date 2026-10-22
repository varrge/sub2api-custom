import { apiClient } from './client'
import type { WelcomeEffect } from '@/utils/welcomeEffects'
export type { WelcomeEffect } from '@/utils/welcomeEffects'

export type PrivilegeRuleKind = 'total_spend' | 'total_recharge' | 'account_age_days' | 'manual'

export interface PersonalPrivilegeRule {
  id: string
  enabled: boolean
  title: string
  kind: PrivilegeRuleKind
  threshold: number
  user_ids: number[]
  welcome_enabled: boolean
  welcome_message: string
  welcome_effect?: WelcomeEffect
}

export interface PrivilegeSettings {
  enterprise_welcome: { enabled: boolean; message: string; effect?: WelcomeEffect }
  personal_rules: PersonalPrivilegeRule[]
}

export interface UserPrivileges {
  welcome: { message: string; effect?: WelcomeEffect } | null
  title?: string | null
}

export async function getPrivilegeSettings(): Promise<PrivilegeSettings> {
  const { data } = await apiClient.get<PrivilegeSettings>('/admin/settings/privileges')
  return data
}

export async function updatePrivilegeSettings(settings: PrivilegeSettings): Promise<PrivilegeSettings> {
  const { data } = await apiClient.put<PrivilegeSettings>('/admin/settings/privileges', settings)
  return data
}

export async function getUserPrivileges(signal?: AbortSignal): Promise<UserPrivileges> {
  const { data } = await apiClient.get<UserPrivileges>('/user/privileges', { signal })
  return data
}
