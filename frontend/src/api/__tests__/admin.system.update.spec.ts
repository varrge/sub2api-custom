import { beforeEach, describe, expect, it, vi } from 'vitest'
import { performUpdate } from '../admin/system'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { post } }))

describe('online update request', () => {
  beforeEach(() => vi.resetAllMocks())

  it('sends the confirmed version and operation key to the update endpoint', async () => {
    post.mockResolvedValue({ data: { deployment_pending: true, need_restart: false } })
    await performUpdate('0.2.22-custom.7', 'system-update-test')
    expect(post).toHaveBeenCalledWith('/admin/system/update', { version: '0.2.22-custom.7' }, {
      timeout: 15 * 60 * 1000,
      headers: { 'Idempotency-Key': 'system-update-test' }
    })
  })
})
