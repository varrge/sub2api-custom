export const welcomeEffects = ['capsule', 'slide', 'halo', 'ribbon'] as const
export type WelcomeEffect = typeof welcomeEffects[number]

export function normalizeWelcomeEffect(value: unknown): WelcomeEffect {
  return welcomeEffects.includes(value as WelcomeEffect) ? value as WelcomeEffect : 'capsule'
}
