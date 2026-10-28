/** One image decorates the shared card template, regardless of product tier. */
export interface MonthCardAppearance {
  id: string
  name: string
  imageUrl?: string
  textColor?: string
}

export const defaultMonthCardAppearance: MonthCardAppearance = {
  id: 'classic-copper',
  name: 'Classic',
}

export function cardArtworkURL(url?: string): string | undefined {
  if (!url || url.includes('\\') || [...url].some(character => character.charCodeAt(0) < 32)) return undefined
  if (url.startsWith('/') && !url.startsWith('//')) return url
  try { return new URL(url).protocol === 'https:' ? url : undefined } catch { return undefined }
}

export function cardTextColor(color?: string): string {
  return color && /^#[0-9a-fA-F]{6}$/.test(color) ? color : '#edc6a6'
}
