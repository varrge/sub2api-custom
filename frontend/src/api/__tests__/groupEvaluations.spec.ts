import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { getEvaluationDetail, listEvaluationConfigs, listEvaluationGroups, runEvaluation, saveEvaluationConfig, type EvaluationConfigInput } from '../groupEvaluations'

afterEach(() => vi.restoreAllMocks())

describe('group evaluation API contract', () => {
  it('uses shared authenticated client with GET-only user APIs', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { items: [] } })
    const signal = new AbortController().signal
    await listEvaluationGroups(signal)
    await getEvaluationDetail(9, false, signal)
    await getEvaluationDetail(9, true, signal)
    await listEvaluationConfigs(signal)
    expect(get.mock.calls).toEqual([
      ['/group-evaluations', { signal }],
      ['/group-evaluations/9', { signal }],
      ['/admin/group-evaluations/9', { signal }],
      ['/admin/group-evaluations', { signal }],
    ])
  })

  it('separates configuration persistence from asynchronous execution', async () => {
    const put = vi.spyOn(apiClient, 'put').mockResolvedValue({ data: {} })
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { accepted: true } })
    const input: EvaluationConfigInput = { revision: 4, enabled: true, candy_enabled: true, pelican_enabled: false, automatic: false, interval_seconds: 37, model: 'example-model', effort: 'high', runs: 4, daily_limit: 100 }
    await saveEvaluationConfig(9, input)
    expect(put).toHaveBeenCalledWith('/admin/group-evaluations/9', input)
    expect(post).not.toHaveBeenCalled()
    expect(await runEvaluation(9)).toEqual({ accepted: true })
    expect(post).toHaveBeenCalledWith('/admin/group-evaluations/9/run')
  })
})
