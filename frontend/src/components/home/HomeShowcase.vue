<template>
  <figure ref="stage" class="showcase" :aria-label="t('home.marketing.demo.title')" :style="{ '--scroll-progress': progress }">
    <div class="api-type" aria-hidden="true">API</div>
    <div class="product-stage">
      <div class="product-base" aria-hidden="true"></div>
      <div class="connection-panel">
        <div class="panel-heading">
          <div class="window-dots" aria-hidden="true"><i></i><i></i><i></i></div>
          <span>{{ t('home.marketing.demo.title') }}</span>
        </div>
        <div class="panel-copy">
          <p class="project-label">{{ t('home.marketing.demo.project') }}</p>
          <h2>{{ t('home.marketing.demo.headline') }}</h2>
        </div>
        <div class="connection" aria-hidden="true">
          <span class="connection-dot"></span>
          <span>{{ t('home.marketing.demo.application') }}</span>
          <i></i><strong :title="siteName">{{ siteName }}</strong><i></i><span>AI</span>
        </div>
        <div class="scenarios" role="group" :aria-label="t('home.marketing.demo.choose')">
          <button v-for="scenario in scenarios" :key="scenario" type="button" :aria-pressed="selected === scenario" aria-controls="home-demo-content" @click="selected = scenario">
            {{ t(`home.marketing.demo.${scenario}.label`) }}
          </button>
        </div>
        <div id="home-demo-content" class="demo-content" aria-live="polite" aria-atomic="true">
          {{ t(`home.marketing.demo.${selected}.result`) }}
        </div>
      </div>
    </div>
    <figcaption>{{ t('home.marketing.demo.caption') }}</figcaption>
  </figure>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

defineProps<{ siteName: string }>()
const { t } = useI18n()
const scenarios = ['code', 'write', 'build'] as const
const selected = ref<(typeof scenarios)[number]>('code')
const stage = ref<HTMLElement | null>(null)
const progress = ref(0)
let frame = 0
let visible = true
let motionQuery: MediaQueryList | undefined
let observer: IntersectionObserver | undefined

function updateProgress() {
  frame = 0
  progress.value = motionQuery?.matches ? 0 : Math.min(1, Math.max(0, window.scrollY / 500))
}
function scheduleUpdate() {
  if (!visible || motionQuery?.matches || frame) return
  frame = requestAnimationFrame(updateProgress)
}
function updateMotion() {
  cancelAnimationFrame(frame)
  updateProgress()
}
onMounted(() => {
  motionQuery = window.matchMedia('(prefers-reduced-motion: reduce), (max-width: 768px)')
  updateProgress()
  motionQuery.addEventListener?.('change', updateMotion)
  window.addEventListener('scroll', scheduleUpdate, { passive: true })
  window.addEventListener('resize', scheduleUpdate)
  if (stage.value && typeof IntersectionObserver !== 'undefined') {
    observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting
      if (visible) scheduleUpdate()
      else { cancelAnimationFrame(frame); frame = 0 }
    })
    observer.observe(stage.value)
  }
})
onUnmounted(() => {
  window.removeEventListener('scroll', scheduleUpdate)
  window.removeEventListener('resize', scheduleUpdate)
  motionQuery?.removeEventListener?.('change', updateMotion)
  observer?.disconnect()
  cancelAnimationFrame(frame)
})
</script>

<style scoped>
.showcase { position: relative; padding: 100px 0 76px; perspective: 1200px; }
.api-type { position: absolute; top: -30px; left: 0; right: 0; text-align: center; font-size: clamp(130px, 24vw, 290px); line-height: 1; font-weight: 800; letter-spacing: -.055em; background: linear-gradient(150deg,#e8dfd5 10%,#a5a3a1 42%,#f8f7f4 64%,#af9276 90%); background-clip: text; -webkit-text-fill-color: transparent; user-select: none; }
.product-stage { position: relative; width: min(760px, 100%); margin: 0 auto; transform-style: preserve-3d; transform: rotateX(calc(22deg - var(--scroll-progress) * 18deg)) rotateY(calc(-12deg + var(--scroll-progress) * 12deg)) rotateZ(calc(-6deg + var(--scroll-progress) * 6deg)) scale(calc(1 + var(--scroll-progress) * .08)); transition: transform .12s ease-out; }
.product-base { position: absolute; bottom: -9px; left: 5%; right: 5%; height: 24px; background: linear-gradient(#e5e5e7,#b2b2b4); border-radius: 12px; box-shadow: 0 15px 35px #00000014; transform: translateZ(-20px); }
.connection-panel { position: relative; min-height: 300px; padding: 24px 28px 20px; border: 1px solid var(--glass-border); border-radius: 20px; background: var(--glass); backdrop-filter: blur(30px); -webkit-backdrop-filter: blur(30px); box-shadow: 0 35px 70px var(--glass-shadow), inset 0 0 0 1px #4c392014; }
.panel-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding-bottom: 12px; border-bottom: 1px solid var(--border); color: var(--muted); font: 11px/1.5 ui-monospace, monospace; }
.window-dots { display: flex; gap: 6px; }
.window-dots i { width: 8px; height: 8px; border-radius: 50%; background: #ff5f56; }
.window-dots i:nth-child(2) { background: #ffbd2e; }
.window-dots i:nth-child(3) { background: #27c93f; }
.panel-copy { padding: 24px 0 20px; }
.project-label { color: var(--muted); font: 10px/1.5 ui-monospace, monospace; letter-spacing: .08em; margin-bottom: 10px; }
.panel-copy h2 { font-size: clamp(20px,2.5vw,30px); letter-spacing: -.03em; line-height: 1.4; font-weight: 600; color: var(--ink); text-wrap: balance; }
.connection { display: flex; align-items: center; gap: 14px; padding: 14px; color: var(--glass-ink); background: var(--inset); border-radius: 10px; font: 12px/1.5 ui-monospace, monospace; }
.connection strong { min-width: 0; max-width: 40%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 500; }
.connection > span:not(.connection-dot) { flex-shrink: 0; }
.connection i { height: 1px; min-width: 12px; flex: 1; background: var(--border); }
.connection-dot { width: 7px; height: 7px; border-radius: 50%; background: #739675; flex-shrink: 0; }
.scenarios { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 16px; }
.scenarios button { min-height: 40px; padding: 8px 14px; border-radius: 99px; color: var(--muted); font-size: 12px; transition: color .2s, background .2s; }
.scenarios button:hover { background: var(--inset); }
.scenarios button[aria-pressed='true'] { color: var(--glass-ink); background: var(--inset); box-shadow: inset 0 0 0 1px var(--border); }
.scenarios button:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
.demo-content { margin-top: 8px; min-height: 20px; font-size: 12px; line-height: 20px; color: var(--muted); overflow-wrap: anywhere; }
figcaption { max-width: 640px; margin: 60px auto 0; color: var(--muted); font-size: 11px; text-align: center; line-height: 1.65; }
@media (max-width: 768px) {
  .showcase { padding-top: 80px; padding-bottom: 56px; }
  .product-stage { transform: none; }
  .connection-panel { padding: 20px; min-height: 0; }
  .connection { gap: 8px; padding: 12px; font-size: 11px; }
  .connection strong { max-width: 34%; }
  .panel-heading { font-size: 10px; }
  .scenarios { gap: 4px; }
  .scenarios button { padding: 8px 10px; }
  figcaption { margin-top: 28px; }
}
@media (max-width: 375px) {
  .connection-panel { padding: 16px; }
  .connection { gap: 6px; padding: 10px; font-size: 10px; }
  .connection-dot { display: none; }
  .scenarios button { padding-inline: 9px; font-size: 11px; }
}
@media (prefers-reduced-motion: reduce) {
  .product-stage { transform: none; transition: none; }
  .scenarios button { transition: none; }
}
@media (forced-colors: active) {
  .api-type { background: none; -webkit-text-fill-color: CanvasText; }
}
</style>
