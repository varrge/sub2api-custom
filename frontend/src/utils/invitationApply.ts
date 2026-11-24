/**
 * Idempotency keys for issuing an invitation code.
 *
 * The key is kept per signed-in user until the server gives a definite answer, so a
 * lost response followed by a reload confirms the same request instead of spending
 * a second slot of the user's quota.
 */
const PREFIX = 'sub2api_invitation_apply:'
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

function key(userId: number | string): string {
  return `${PREFIX}${userId}`
}

export function newRequestId(): string {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  // randomUUID needs a secure context; plain-HTTP self-hosted panels still have getRandomValues.
  const b = crypto.getRandomValues(new Uint8Array(16))
  b[6] = (b[6] & 0x0f) | 0x40
  b[8] = (b[8] & 0x3f) | 0x80
  const h = Array.from(b, (n) => n.toString(16).padStart(2, '0')).join('')
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`
}

/** The unconfirmed request for this user, or '' when there is none. */
export function loadPendingApply(userId: number | string): string {
  try {
    const value = localStorage.getItem(key(userId)) || ''
    return UUID.test(value) ? value : ''
  } catch {
    return ''
  }
}

/** Returns the unconfirmed request id, creating and persisting one if needed. */
export function beginApply(userId: number | string): string {
  const pending = loadPendingApply(userId)
  if (pending) return pending
  const id = newRequestId()
  try {
    localStorage.setItem(key(userId), id)
  } catch {
    // Without storage the id still protects retries made before a reload.
  }
  return id
}

export function clearPendingApply(userId: number | string): void {
  try {
    localStorage.removeItem(key(userId))
  } catch {
    // Nothing to clear.
  }
}

/**
 * True when the server may have issued the code even though the call failed:
 * no response, a timeout, throttling, an expired session or a server error.
 * A plain 4xx business rejection is definite, because the server looks up an
 * existing request before it applies any rule.
 */
export function isUncertainApplyError(err: unknown): boolean {
  const status = Number((err as { status?: unknown } | null)?.status)
  if (!Number.isFinite(status) || status < 400 || status >= 500) return true
  return status === 401 || status === 408 || status === 429
}
