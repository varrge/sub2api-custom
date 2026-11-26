import type { EvaluationRecord } from '@/api/groupEvaluations'

export function recentEvaluationRecords(records: EvaluationRecord[] = []) {
  return [...records]
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at) || b.id - a.id)
    .slice(0, 10)
}
