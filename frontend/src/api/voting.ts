import { apiClient } from './client'

export type PollStatus = 'draft' | 'open' | 'closed'
export interface PollSummary {
  id: number
  title: string
  description: string
  status: PollStatus
  revision: number
  voter_count: number
  has_voted: boolean
  created_at: string
  updated_at: string
}
export interface PollOption { id: number; title: string; image: string; votes: number }
export interface Poll extends PollSummary { image: string; options: PollOption[]; my_option_ids: number[] }
export interface PollInput {
  title: string
  description: string
  image: string
  status: PollStatus
  revision: number
  options: { title: string; image: string }[]
}
export interface PollPage { items: PollSummary[]; total: number; page: number; page_size: number; pages: number }

export const votingAPI = {
  async list(page = 1, signal?: AbortSignal): Promise<PollPage> {
    return (await apiClient.get<PollPage>('/voting/polls', { params: { page, page_size: 12 }, signal })).data
  },
  async get(id: number, signal?: AbortSignal): Promise<Poll> {
    return (await apiClient.get<Poll>(`/voting/polls/${id}`, { signal })).data
  },
  async vote(id: number, revision: number, optionIDs: number[]): Promise<void> {
    await apiClient.post(`/voting/polls/${id}/votes`, { revision, option_ids: optionIDs })
  }
}

export const adminVotingAPI = {
  async list(page = 1, signal?: AbortSignal): Promise<PollPage> {
    return (await apiClient.get<PollPage>('/admin/voting/polls', { params: { page, page_size: 12 }, signal })).data
  },
  async get(id: number, signal?: AbortSignal): Promise<Poll> {
    return (await apiClient.get<Poll>(`/admin/voting/polls/${id}`, { signal })).data
  },
  async create(input: PollInput): Promise<{ id: number }> {
    return (await apiClient.post<{ id: number }>('/admin/voting/polls', input)).data
  },
  async update(id: number, input: PollInput): Promise<{ id: number }> {
    return (await apiClient.put<{ id: number }>(`/admin/voting/polls/${id}`, input)).data
  },
  async delete(id: number, revision: number): Promise<void> {
    await apiClient.delete(`/admin/voting/polls/${id}`, { params: { revision } })
  }
}
