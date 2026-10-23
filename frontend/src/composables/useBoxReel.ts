import { computed, nextTick, onMounted, onUnmounted, ref, watch, type CSSProperties } from 'vue'
import type { BoxCardTier, BoxDraw, BoxPrize } from '@/api/luckyBox'

export type ReelPrize = NonNullable<BoxDraw['prize']> & Partial<Pick<BoxPrize, 'icon'>>
export interface ReelItem { prize: ReelPrize; target: boolean }

const DURATION = 5400

export function useBoxReel(prizes: () => BoxPrize[], tier: () => BoxCardTier) {
  const viewportRef = ref<HTMLElement>()
  const trackRef = ref<HTMLElement>()
  const items = ref<ReelItem[]>([])
  const running = ref(false)
  const revealed = ref(false)
  const soundEnabled = ref(false)
  const offset = ref(0)
  const transitioning = ref(false)
  const trackStyle = computed<CSSProperties>(() => ({
    transform: `translate3d(${offset.value}px, 0, 0)`,
    transition: transitioning.value ? `transform ${DURATION}ms cubic-bezier(0.12, 0.72, 0.1, 1)` : 'none',
  }))
  let frame = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let resolvePlay: (() => void) | undefined
  let resizeObserver: ResizeObserver | undefined
  let disposed = false
  let audio: AudioContext | undefined
  let canCelebrate = false

  const pool = () => prizes().filter(prize => (prize.weights?.[tier()] ?? prize.weight) > 0)
  const idle = () => {
    if (running.value) return
    const available = pool()
    items.value = available.length
      ? Array.from({ length: 18 }, (_, index) => ({ prize: available[index % available.length], target: false }))
      : []
    offset.value = 0
    transitioning.value = false
    revealed.value = false
  }
  watch([prizes, tier], idle, { immediate: true })

  const prepareSound = () => {
    if (!soundEnabled.value || typeof window.AudioContext !== 'function') return
    try {
      audio ??= new AudioContext()
      void audio.resume().catch(() => {})
    } catch {
      // Audio is optional; a blocked device must never interrupt a draw.
    }
  }
  const toggleSound = () => {
    soundEnabled.value = !soundEnabled.value
    prepareSound()
    if (!soundEnabled.value) void audio?.suspend().catch(() => {})
  }
  const tone = (frequency: number, duration = 0.035, delay = 0) => {
    if (!soundEnabled.value || !audio || audio.state !== 'running') return
    try {
      const oscillator = audio.createOscillator()
      const gain = audio.createGain()
      const at = audio.currentTime + delay
      oscillator.type = 'triangle'
      oscillator.frequency.value = frequency
      gain.gain.setValueAtTime(0.025, at)
      gain.gain.exponentialRampToValueAtTime(0.0001, at + duration)
      oscillator.connect(gain).connect(audio.destination)
      oscillator.onended = () => { oscillator.disconnect(); gain.disconnect() }
      oscillator.start(at)
      oscillator.stop(at + duration)
    } catch {
      // Result delivery does not depend on audio support.
    }
  }

  const destination = () => {
    const target = trackRef.value?.querySelector<HTMLElement>('[data-target-prize]')
    const viewport = viewportRef.value
    return target && viewport ? viewport.clientWidth / 2 - target.offsetLeft - target.offsetWidth / 2 : 0
  }
  const clearMotion = () => {
    cancelAnimationFrame(frame)
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
    transitioning.value = false
  }
  const finish = () => {
    if (!running.value) return
    clearMotion()
    offset.value = destination()
    running.value = false
    revealed.value = true
    if (canCelebrate) [523.25, 659.25, 783.99].forEach((frequency, i) => tone(frequency, 0.24, i * 0.08))
    resolvePlay?.()
    resolvePlay = undefined
  }

  const play = async (draw: BoxDraw) => {
    if (disposed || !draw.prize || running.value) return
    const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches === true
    const targetIndex = reduced ? 4 : 30
    const available = pool()
    // Fillers are decorative. The target is always the immutable server prize.
    const prize = { icon: prizes().find(item => item.id === draw.prize?.id)?.icon, ...draw.prize }
    const fillers = available.length ? available : [prize]
    clearMotion()
    revealed.value = false
    offset.value = 0
    running.value = true
    canCelebrate = draw.status === 'delivered' && prize.type !== 'empty'
    items.value = Array.from({ length: targetIndex + 6 }, (_, index) => ({
      prize: index === targetIndex ? prize : fillers[(index + 3) % fillers.length],
      target: index === targetIndex,
    }))
    const done = new Promise<void>(resolve => { resolvePlay = resolve })
    await nextTick()
    if (disposed || !running.value) return done
    // The timer also finishes in background tabs, where animation frames may stop.
    timer = setTimeout(finish, reduced ? 60 : DURATION + 150)
    if (reduced) {
      offset.value = destination()
      return done
    }
    frame = requestAnimationFrame(() => {
      frame = requestAnimationFrame(() => {
        transitioning.value = true
        offset.value = destination()
        let lastSlot = -1
        let lastTick = 0
        const tick = (at: number) => {
          if (!running.value || disposed) return
          const track = trackRef.value
          const first = track?.children[0] as HTMLElement | undefined
          if (soundEnabled.value && audio?.state === 'running' && track && first && viewportRef.value) {
            const stride = first.offsetWidth + (parseFloat(getComputedStyle(track).gap) || 0)
            const x = track.getBoundingClientRect().left - viewportRef.value.getBoundingClientRect().left
            const slot = Math.floor(Math.abs(x) / Math.max(1, stride))
            if (slot !== lastSlot && at - lastTick > 35) {
              tone(480)
              lastSlot = slot
              lastTick = at
            }
          }
          frame = requestAnimationFrame(tick)
        }
        frame = requestAnimationFrame(tick)
      })
    })
    return done
  }

  onMounted(() => {
    if (!viewportRef.value || typeof ResizeObserver === 'undefined') return
    let width = viewportRef.value.clientWidth
    resizeObserver = new ResizeObserver(() => {
      const nextWidth = viewportRef.value?.clientWidth
      if (nextWidth === width) return
      width = nextWidth ?? 0
      // Resizing cannot move the pointer onto a different prize.
      if (running.value) finish()
      else if (revealed.value) offset.value = destination()
    })
    resizeObserver.observe(viewportRef.value)
  })
  onUnmounted(() => {
    disposed = true
    clearMotion()
    resizeObserver?.disconnect()
    resolvePlay?.()
    resolvePlay = undefined
    void audio?.close().catch(() => {})
  })

  return { viewportRef, trackRef, items, running, revealed, soundEnabled, trackStyle, play, prepareSound, toggleSound }
}
