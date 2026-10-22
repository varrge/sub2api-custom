import { onScopeDispose, ref } from 'vue'
import { normalizeWelcomeEffect, type WelcomeEffect } from '@/utils/welcomeEffects'

export type WelcomePhase = 'waiting' | 'entering' | 'dwell' | 'exiting'

export function useWelcomePlayback() {
  const message = ref<string | null>(null)
  const effect = ref<WelcomeEffect>('capsule')
  const phase = ref<WelcomePhase>('waiting')
  let timers: ReturnType<typeof setTimeout>[] = []

  function cancelTimers() {
    timers.forEach(clearTimeout)
    timers = []
  }

  function clear() {
    cancelTimers()
    message.value = null
    phase.value = 'waiting'
    effect.value = 'capsule'
  }

  function after(ms: number, callback: () => void) {
    timers.push(setTimeout(callback, ms))
  }

  function dismiss() {
    if (!message.value || phase.value === 'exiting') return
    cancelTimers()
    phase.value = 'exiting'
    after(250, clear)
  }

  function play(text: string, selectedEffect?: WelcomeEffect) {
    clear()
    message.value = text
    effect.value = normalizeWelcomeEffect(selectedEffect)
    after(350, () => {
      phase.value = 'entering'
      after(400, () => {
        phase.value = 'dwell'
        after(3500, dismiss)
      })
    })
  }

  onScopeDispose(clear)
  return { message, effect, phase, play, dismiss, clear }
}
