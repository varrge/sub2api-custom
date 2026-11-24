import { extractApiErrorMessage } from '@/utils/apiError'

const REASONS = [
  'INVITATION_DISABLED', 'INVITATION_QUOTA_EXCEEDED', 'INVITATION_PERSONAL_ONLY', 'INVITATION_INVALID',
  'INVITATION_CONFLICT', 'INVITATION_LEVEL_IN_USE', 'INVITATION_NOT_FOUND'
] as const

/** Localised message for invitation API failures; falls back to the server text. */
export function invitationErrorMessage(err: unknown, t: (key: string) => string, fallback: string): string {
  const map = Object.fromEntries(REASONS.map((reason) => [reason, t(`invitations.errors.${reason}`)]))
  return extractApiErrorMessage(err, fallback, map)
}

export function invitationErrorReason(err: unknown): string {
  return String((err as { reason?: unknown } | null)?.reason ?? '')
}
