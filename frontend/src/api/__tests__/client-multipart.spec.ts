import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import { describe, expect, it } from 'vitest'

import { apiClient } from '../client'

describe('API client multipart requests', () => {
  it('preserves FormData so the browser can add the multipart boundary', async () => {
    const form = new FormData()
    form.append('content', 'Please help')
    let sentConfig: InternalAxiosRequestConfig | undefined

    await apiClient.post('/tickets', form, {
      adapter: async (config): Promise<AxiosResponse> => {
        sentConfig = config
        return {
          data: { code: 0, data: {} },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      },
    })

    expect(sentConfig?.data).toBeInstanceOf(FormData)
    expect(sentConfig?.headers.getContentType()).not.toBe('application/json')
  })
})
