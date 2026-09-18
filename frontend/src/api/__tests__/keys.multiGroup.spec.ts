import { beforeEach, describe, expect, it, vi } from 'vitest'
import { create, update } from '@/api/keys'
import { updateApiKeyGroups } from '@/api/admin/apiKeys'
import { getAvailableGroups } from '@/api/admin/users'

const { post, put, get } = vi.hoisted(() => ({ post: vi.fn(), put: vi.fn(), get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post, put, get } }))

beforeEach(() => {
  vi.clearAllMocks()
  post.mockResolvedValue({ data: {} })
  put.mockResolvedValue({ data: {} })
  get.mockResolvedValue({ data: [] })
})

describe('ordered API key group requests', () => {
  it('creates using only ordered group_ids and preserves key-wide limits', async () => {
    await create('mixed', [7, 2, 9], undefined, [], [], 25, 30, { rate_limit_5h: 5 })
    expect(post).toHaveBeenCalledWith('/keys', {
      name: 'mixed', group_ids: [7, 2, 9], quota: 25, expires_in_days: 30, rate_limit_5h: 5
    })
    expect(post.mock.calls[0][1]).not.toHaveProperty('group_id')
  })

  it('updates the supplied priority without silently filtering retained bindings', async () => {
    await update(4, { group_ids: [9, 7, 2] })
    expect(put).toHaveBeenCalledWith('/keys/4', { group_ids: [9, 7, 2] })
  })

  it('does not change bindings during unrelated updates', async () => {
    await update(4, { status: 'inactive' })
    expect(put).toHaveBeenCalledWith('/keys/4', { status: 'inactive' })
  })

  it('allows administrators to explicitly clear all bindings', async () => {
    await updateApiKeyGroups(4, [])
    expect(put).toHaveBeenCalledWith('/admin/api-keys/4', { group_ids: [] })
    expect(put.mock.calls[0][1]).not.toHaveProperty('group_id')
  })

  it('loads eligible choices for the target user', async () => {
    await getAvailableGroups(18)
    expect(get).toHaveBeenCalledWith('/admin/users/18/available-groups')
  })
})
