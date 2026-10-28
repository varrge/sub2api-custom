<template>
  <div ref="stage" class="month-card-stage" @pointermove="move" @pointerleave="reset" @pointercancel="reset">
    <div ref="face" class="month-card-face" data-testid="month-card-face" :data-appearance="appearance.id"
      :data-card-id="card.id" :style="{ color: textColor }" role="img" :aria-label="t('groupBuy.wallet.faceLabel', { name: card.product_name })">
      <div class="month-card-base"></div>
      <img v-if="image && !imageFailed" :key="image" class="month-card-artwork" :src="image" alt="" draggable="false" referrerpolicy="no-referrer" @error="imageFailed = true" />
      <svg v-else class="month-card-engraving" viewBox="0 0 200 200" aria-hidden="true">
        <g fill="none" stroke="currentColor" stroke-width=".7">
          <path v-for="rotation in [0,72,144,216,288]" :key="rotation" d="M100 95C65 35 135 35 100 95Z" :transform="`rotate(${rotation} 100 100)`" />
        </g>
      </svg>
      <div class="month-card-sheen"></div>
      <div class="month-card-content">
        <div class="month-card-top"><span>{{ brand }}</span><span class="month-card-tag">{{ t('groupBuy.wallet.exclusive') }}</span></div>
        <div class="month-card-title"><span>PRIVATE MEMBERSHIP</span><strong>{{ card.product_name }}</strong></div>
        <div class="month-card-bottom">
          <div class="month-card-number">{{ t('groupBuy.cardCode') }} ···· {{ card.code.slice(-8) }}</div>
          <div class="month-card-meta">
            <div><span>{{ t('groupBuy.wallet.holder') }}</span><strong>{{ holder }}</strong></div>
            <div><span>{{ card.status === 'frozen' ? t('groupBuy.frozen') : t('groupBuy.expires') }}</span>
              <strong>{{ card.status === 'frozen' ? t('groupBuy.wallet.pausedDays', { days: Math.ceil((card.remaining_seconds ?? 0)/86400) }) : shortDate(card.expires_at) }}</strong></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonthCard } from '@/types/groupBuy'
import { cardArtworkURL, cardTextColor, defaultMonthCardAppearance, type MonthCardAppearance } from './cardAppearance'
const props = withDefaults(defineProps<{ card: MonthCard; holder: string; brand: string; appearance?: MonthCardAppearance }>(), {
  appearance: () => defaultMonthCardAppearance,
})
const { t, locale } = useI18n()
const stage = ref<HTMLElement | null>(null), face = ref<HTMLElement | null>(null)
const imageFailed = ref(false)
const image = computed(() => cardArtworkURL(props.appearance.imageUrl))
const textColor = computed(() => image.value && !imageFailed.value ? cardTextColor(props.appearance.textColor) : '#edc6a6')
watch(image, () => { imageFailed.value = false })
let frame = 0
let fine: MediaQueryList | undefined, reduce: MediaQueryList | undefined
const shortDate = (value: string) => new Intl.DateTimeFormat(locale.value, { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(value))
function reset() {
  cancelAnimationFrame(frame); frame = 0
  if (!face.value) return
  face.value.style.transform = 'rotateX(0deg) rotateY(0deg)'
  face.value.style.setProperty('--light-opacity', '0')
}
function move(event: PointerEvent) {
  if (event.pointerType === 'touch' || !fine?.matches || reduce?.matches || !stage.value) return
  const bounds = stage.value.getBoundingClientRect()
  const x = Math.max(-1, Math.min(1, (event.clientX - bounds.left) / bounds.width * 2 - 1))
  const y = Math.max(-1, Math.min(1, (event.clientY - bounds.top) / bounds.height * 2 - 1))
  cancelAnimationFrame(frame)
  frame = requestAnimationFrame(() => {
    if (!face.value) return
    face.value.style.transform = `rotateX(${-y * 4}deg) rotateY(${x * 4}deg)`
    face.value.style.setProperty('--light-x', `${(x+1)*50}%`)
    face.value.style.setProperty('--light-y', `${(y+1)*50}%`)
    face.value.style.setProperty('--light-opacity', '.24')
    frame = 0
  })
}
watch(() => props.card.id, reset)
onMounted(() => {
  fine = window.matchMedia('(hover: hover) and (pointer: fine)')
  reduce = window.matchMedia('(prefers-reduced-motion: reduce)')
  fine.addEventListener('change', reset); reduce.addEventListener('change', reset)
})
onUnmounted(() => { cancelAnimationFrame(frame); fine?.removeEventListener('change', reset); reduce?.removeEventListener('change', reset) })
</script>
<style scoped>
.month-card-stage{perspective:1100px;aspect-ratio:1.62;width:100%;position:relative;container-type:inline-size}
.month-card-face{position:relative;height:100%;width:100%;transform-origin:50% 50%;transform-style:preserve-3d;transform:rotateX(0deg) rotateY(0deg);transition:transform .22s ease-out;border-radius:22px;overflow:hidden;border:1px solid #b99c7630;box-shadow:0 22px 38px #40251a20;user-select:none}
.month-card-base{position:absolute;inset:0;background:radial-gradient(ellipse at 12% 0,#9c77604f,transparent 60%),linear-gradient(128deg,#624c3f,#3d3029 50%,#261e1a)}
.month-card-base:after{content:'';position:absolute;width:80%;height:150%;right:-20%;top:-30%;border:1px solid #d4ad7540;border-radius:50%;transform:rotate(-30deg);box-shadow:0 0 0 20px #cda66d09,0 0 0 40px #cda66d08,0 0 0 60px #cda66d07}
.month-card-artwork{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;pointer-events:none}
.month-card-engraving{position:absolute;left:40%;top:-5%;width:70%;height:110%;opacity:.5;pointer-events:none}
.month-card-sheen{position:absolute;inset:0;pointer-events:none;background:radial-gradient(circle at var(--light-x,50%) var(--light-y,50%),white,transparent 65%);opacity:var(--light-opacity,0);transition:opacity .25s;mix-blend-mode:soft-light;z-index:2}
.month-card-content{position:absolute;inset:0;padding:clamp(17px,5.5cqw,28px);display:flex;flex-direction:column;justify-content:space-between;transform:translateZ(10px);pointer-events:none;z-index:3}
.month-card-top{display:flex;justify-content:space-between;align-items:center;gap:12px;font-size:clamp(11px,3.2cqw,15px);letter-spacing:.05em}.month-card-top>span:first-child{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.month-card-tag{font-size:clamp(10px,2.7cqw,12px);border:1px solid currentColor;border-radius:20px;padding:4px 9px;white-space:nowrap;opacity:.65}
.month-card-title{display:flex;flex-direction:column;gap:7px}.month-card-title>span{font-size:clamp(9px,2.4cqw,11px);letter-spacing:.18em;opacity:.5}
.month-card-title>strong{font-size:clamp(18px,5.4cqw,27px);font-weight:500;letter-spacing:.06em;line-height:1.3;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}
.month-card-number{font-family:ui-monospace,monospace;font-size:clamp(11px,3.2cqw,16px);letter-spacing:.12em;opacity:.8;margin-bottom:16px}
.month-card-meta{display:flex;gap:30px}.month-card-meta>div{display:flex;flex-direction:column;gap:4px;min-width:0}
.month-card-meta span{font-size:clamp(9px,2.2cqw,10px);opacity:.5;letter-spacing:.07em}.month-card-meta strong{font-size:clamp(11px,2.7cqw,13px);font-weight:500;max-width:25ch;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
@media(prefers-reduced-motion:reduce),(pointer:coarse){.month-card-face{transition:none;transform:none!important}.month-card-sheen{display:none}}
@media(max-width:400px){.month-card-meta{gap:18px}.month-card-number{margin-bottom:10px}.month-card-face{border-radius:17px}}
</style>
