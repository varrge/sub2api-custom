const STORAGE_KEY = 'registration_invitation_code'

export function loadRegistrationInvitation(): string {
  try {
    return sessionStorage.getItem(STORAGE_KEY)?.trim() || ''
  } catch {
    return ''
  }
}

export function storeRegistrationInvitation(value: unknown): string {
  const raw = Array.isArray(value) ? value[0] : value
  const code = typeof raw === 'string' ? raw.trim() : ''
  try {
    if (code) sessionStorage.setItem(STORAGE_KEY, code)
    else sessionStorage.removeItem(STORAGE_KEY)
  } catch {
    // The current form still carries the code when browser storage is unavailable.
  }
  return code
}

export function clearRegistrationInvitation(): void {
  storeRegistrationInvitation('')
}

/** Share link for a one-use code; honours a non-root deployment base path. */
export function registrationInvitationLink(code: string): string {
  const base = (import.meta.env.BASE_URL || '/').replace(/\/+$/, '')
  const link = new URL(`${base}/register`, window.location.origin)
  link.searchParams.set('invitation_code', code)
  return link.toString()
}
