import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))
import { getUserPrivileges } from '../privileges'

describe('user privileges API', () => {
  beforeEach(() => { get.mockReset() })

  it('passes the abort signal to the HTTP client', async () => {
    const controller = new AbortController()
    get.mockResolvedValue({ data: { welcome: null } })
    await expect(getUserPrivileges(controller.signal)).resolves.toEqual({ welcome: null })
    expect(get).toHaveBeenCalledWith('/user/privileges', { signal: controller.signal })
  })
})
