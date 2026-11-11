import { apiClient } from './client'
import type { GroupPlatform } from '@/types'

export interface TutorialArticle {
  id: string
  title: string
  kind: 'tutorial' | 'faq'
  platform: GroupPlatform | ''
  client: string
  markdown: string
  status: 'draft' | 'published'
  order: number
}
export interface TutorialRoute {
  id: string
  title: string
  url: string
  platform: GroupPlatform | ''
  status: 'draft' | 'published'
  order: number
}
export interface TutorialCatalog { revision: number; articles: TutorialArticle[]; routes: TutorialRoute[] }
export const tutorialAPI = {
  async get(signal?: AbortSignal): Promise<TutorialCatalog> {
    return (await apiClient.get<TutorialCatalog>('/tutorial', { signal })).data
  },
  async adminGet(): Promise<TutorialCatalog> {
    return (await apiClient.get<TutorialCatalog>('/admin/tutorial')).data
  },
  async save(catalog: TutorialCatalog): Promise<TutorialCatalog> {
    return (await apiClient.put<TutorialCatalog>('/admin/tutorial', catalog)).data
  }
}
