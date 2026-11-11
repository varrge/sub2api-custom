import type { ApiKeyModelAllowlist } from '@/types'

export function modelPolicyAllows(policy: ApiKeyModelAllowlist, model: string): boolean {
  if (!policy?.enabled) return true
  if (policy.mode && policy.mode !== 'allow' && policy.mode !== 'deny') return false
  const listed = (policy.models ?? []).some(id => id.trim() === model.trim())
  return policy.mode === 'deny' ? !listed : listed
}
export function validModelAccessID(value: string): boolean {
  const model = value.trim()
  // Match the backend's concrete, case-sensitive public-ID validation.
  return model.length > 0 && Array.from(model).length <= 256 &&
    !Array.from(value).some(char => {
      const code = char.codePointAt(0)!
      return code <= 31 || (code >= 127 && code <= 159)
    }) && !/[*?\[\]]/.test(model)
}
