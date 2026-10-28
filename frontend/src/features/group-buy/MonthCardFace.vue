<template>
  <div
    class="month-card-stage"
    ref="stage"
    @pointermove="move"
    @pointerleave="reset"
    @pointercancel="reset"
  >
    <div
      class="month-card-face"
      ref="face"
      data-testid="month-card-face"
      :data-appearance="appearance.id"
      :data-card-id="card.id"
      :style="{ color: textColor, '--text-main': textColor }"
      role="group"
      :aria-label="t('groupBuy.wallet.faceLabel', { name: card.product_name })"
    >
      <!-- Base Metallic Brown Background -->
      <div class="month-card-base"></div>

      <!-- Optional Uploaded Artwork + Scrim -->
      <div class="month-card-artwork" v-if="image && !imageFailed" :key="image">
        <img :src="image" @error="imageFailed = true" alt="" draggable="false" referrerpolicy="no-referrer" />
        <div class="month-card-scrim"></div>
      </div>

      <!-- Etched Sakura Engraving (Fallback/Overlay) -->
      <div class="month-card-engraving" v-else aria-hidden="true">
        <svg viewBox="0 0 200 200" class="sakura-pattern" aria-hidden="true">
          <g fill="none" stroke="currentColor" stroke-width=".65">
            <path v-for="rotation in [0,72,144,216,288]" :key="rotation" d="M100 95C65 35 135 35 100 95Z" :transform="`rotate(${rotation} 100 100)`" />
          </g>
        </svg>
      </div>

      <!-- Card Content -->
      <div class="month-card-content">
        <!-- Top Header: Brand & Badges -->
        <header class="pass-header">
          <div class="brand-name">{{ brand }}</div>
          <div class="card-badges">
            <span class="pass-badge exclusive">{{ t('groupBuy.wallet.exclusive') }}</span>
            <span class="pass-badge status" :class="{ 'status-frozen': isFrozen }">
              {{ isFrozen ? t('groupBuy.frozen') : t('groupBuy.active') }}
            </span>
          </div>
        </header>

        <!-- Main Body: Titles and Available Quota -->
        <div class="card-main">
          <div class="card-identity">
            <h2 class="product-title">{{ card.product_name }}</h2>
            <div class="group-title">{{ card.group_name }} · {{ card.platform }}</div>
          </div>
          <div class="card-available">
            <div class="available-label">
              {{ t('groupBuy.wallet.availableLabel') }}
              <span class="available-hint" v-if="!isFrozen">{{ t('groupBuy.wallet.availableHint') }}</span>
            </div>
            <div class="available-amount" data-testid="card-available" :class="{ 'amount-frozen': isFrozen }">
              {{ usd(currentAvailable) }}
            </div>
          </div>
        </div>

        <!-- Progress Bars -->
        <div class="card-progress-section" data-testid="card-quotas">
          <div class="progress-row">
            <div class="progress-info">
              <span class="progress-title">{{ t('groupBuy.weeklyUsage') }}</span>
              <span class="progress-values">{{ usd(card.weekly_used_usd) }} / {{ usd(card.weekly_quota_usd) }}</span>
            </div>
            <div class="progress-track">
              <div
                class="progress-fill"
                :style="{ width: progress(card.weekly_used_usd, card.weekly_quota_usd) + '%' }"
              ></div>
            </div>
          </div>

          <div class="progress-row">
            <div class="progress-info">
              <span class="progress-title">{{ t('groupBuy.totalUsage') }}</span>
              <span class="progress-values">{{ usd(card.total_used_usd) }} / {{ usd(card.total_quota_usd) }}</span>
            </div>
            <div class="progress-track">
              <div
                class="progress-fill"
                :style="{ width: progress(card.total_used_usd, card.total_quota_usd) + '%' }"
              ></div>
            </div>
          </div>
        </div>

        <!-- Spacer to push footer down on large screens -->
        <div class="card-spacer"></div>

        <!-- Info Message (Days Left / Reset / Frozen) -->
        <div class="card-info-message" data-testid="card-time-info">
          <template v-if="!isFrozen">
            <span class="info-highlight">{{ t('groupBuy.wallet.daysLeft', { days: daysLeft }) }}</span>
            <span class="info-dot">•</span>
            <span>{{ isFinalWindow ? t('groupBuy.finalWindow') : `${t('groupBuy.nextReset')}: ${exactDate(card.weekly_window_end)}` }}</span>
          </template>
          <template v-else>
            <span class="info-highlight">{{ t('groupBuy.frozenRemaining', { days: daysLeft }) }}</span>
            <span class="info-dot">•</span>
            <span>{{ t('groupBuy.frozenQuota', { amount: usd(availableQuota(card)) }) }}</span>
          </template>
        </div>

        <hr class="card-divider" />

        <!-- Footer: Holder, Code, Expiry -->
        <footer class="pass-footer">
          <div class="footer-field holder-field">
            <div class="footer-label">{{ t('groupBuy.wallet.holder') }}</div>
            <div class="footer-value">{{ holder }}</div>
          </div>
          <div class="footer-field code-field">
            <div class="footer-label">{{ t('groupBuy.cardCode') }}</div>
            <div class="footer-value code-value">•••• {{ card.code.slice(-8) }}</div>
          </div>
          <div class="footer-field expiry-field">
            <div class="footer-label">{{ t('groupBuy.expires') }}</div>
            <div class="footer-value">{{ isFrozen ? t('groupBuy.frozenExpiry') : shortDate(card.expires_at) }}</div>
          </div>
        </footer>
      </div>

      <!-- 3D Sheen Effect Layer -->
      <div class="month-card-sheen"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonthCard } from '@/types/groupBuy'
import { usd, progress, availableQuota, exactDate } from './model'
import { cardDaysRemaining, spendableQuota } from './walletModel'
import { cardArtworkURL, cardTextColor, defaultMonthCardAppearance, type MonthCardAppearance } from './cardAppearance'
const props = withDefaults(defineProps<{ card: MonthCard; holder: string; brand: string; appearance?: MonthCardAppearance; now?: number }>(), {
  appearance: () => defaultMonthCardAppearance,
})
const { t, locale } = useI18n()
const isFrozen = computed(() => props.card.status === 'frozen')
const currentAvailable = computed(() => spendableQuota(props.card, props.now ?? Date.now()))
const daysLeft = computed(() => cardDaysRemaining(props.card, props.now ?? Date.now()))
const isFinalWindow = computed(() => Date.parse(props.card.weekly_window_end) >= Date.parse(props.card.expires_at))
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
/* Base Variables & Theme */
.month-card-stage {
  --text-main: #fdf2e9;
  --text-muted: #d4bfae;
  --brand-copper: #b87333;
  --brand-gold: #dfa671;
  --base-dark: #1f110b;
  --base-light: #3a261c;
  --track-bg: rgba(15, 8, 5, 0.5);
  --divider-color: rgba(212, 191, 174, 0.15);

  perspective: 1200px;
  width: 100%;
  max-width: 960px;
  margin: 0 auto;
}

/* 3D Face Container */
.month-card-face {
  position: relative;
  width: 100%;
  min-height: 540px;
  transform-origin: 50% 50%;
  height: auto;
  border-radius: 24px;
  transform-style: preserve-3d;
  transform: rotateX(var(--rotate-x, 0deg)) rotateY(var(--rotate-y, 0deg));
  transition: transform 0.15s ease-out, box-shadow 0.15s ease-out;
  box-shadow: 0 20px 40px rgba(62, 40, 26, 0.18), 0 0 0 1px rgba(255, 255, 255, 0.05) inset;
  background-color: var(--base-dark);
  color: var(--text-main);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

/* Backgrounds & Textures */
.month-card-base {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse at 10% 0,#9c77604f,transparent 65%),linear-gradient(128deg,#624c3f,#3d3029 50%,#261e1a);
  z-index: 1;
}

.month-card-artwork {
  position: absolute;
  inset: 0;
  z-index: 2;
}

.month-card-artwork img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0.85;
}

.month-card-scrim {
  position: absolute;
  inset: 0;
  background: rgba(20, 10, 5, 0.5);
  mix-blend-mode: multiply;
}

.month-card-engraving {
  position: absolute;
  inset: 0;
  z-index: 3;
  pointer-events: none;
  overflow: hidden;
}

.sakura-pattern {
  position: absolute;
  right: -10%;
  bottom: -15%;
  width: 60%;
  height: 60%;
  color: var(--text-muted);
  opacity: 0.22;
  transform: rotate(-15deg);
  mix-blend-mode: soft-light;
}

/* Content Layout */
.month-card-content {
  position: relative;
  z-index: 10;
  padding: 48px;
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  box-sizing: border-box;
}

/* Header */
.pass-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}

.brand-name {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.card-badges {
  display: flex;
  gap: 12px;
}

.pass-badge {
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  border: 1px solid var(--divider-color);
  backdrop-filter: blur(4px);
}

.pass-badge.exclusive {
  background: rgba(255, 255, 255, 0.05);
}

.pass-badge.status {
  background: rgba(184, 115, 51, 0.15);
  color: var(--brand-gold);
  border-color: rgba(184, 115, 51, 0.3);
}

.pass-badge.status-frozen {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text-muted);
  border-color: var(--divider-color);
}

/* Main Body */
.card-main {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 32px;
  gap: 32px;
}

.card-identity {
  flex: 1;
}

.product-title {
  font-size: 28px;
  font-weight: 600;
  margin: 0 0 8px 0;
  line-height: 1.2;
}

.group-title {
  font-size: 16px;
  color: var(--text-muted);
}

.card-available {
  text-align: right;
  flex-shrink: 0;
}

.available-label {
  font-size: 14px;
  color: var(--text-muted);
  margin-bottom: 8px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  display: flex;
  flex-direction: column;
}

.available-hint {
  font-size: 11px;
  opacity: 0.7;
  margin-top: 2px;
}

.available-amount {
  font-size: 56px;
  font-weight: 700;
  line-height: 1;
  letter-spacing: -0.02em;
}

.available-amount.amount-frozen {
  font-size: 40px;
  color: var(--text-muted);
}

/* Progress Bars */
.card-progress-section {
  display: flex;
  flex-direction: column;
  gap: 20px;
  width: 100%;
}

.progress-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.progress-info {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  font-weight: 500;
}

.progress-title {
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.progress-values {
  font-variant-numeric: tabular-nums;
}

.progress-track {
  width: 100%;
  height: 6px;
  background-color: var(--track-bg);
  border-radius: 3px;
  overflow: hidden;
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.5);
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--brand-copper), var(--brand-gold));
  border-radius: 3px;
  transition: width 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

/* Spacer */
.card-spacer {
  flex-grow: 1;
  min-height: 28px;
}

/* Info Message */
.card-info-message {
  font-size: 14px;
  color: var(--text-muted);
  margin-bottom: 24px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.info-highlight {
  color: var(--text-main);
  font-weight: 500;
}

.info-dot {
  opacity: 0.5;
}

/* Divider */
.card-divider {
  border: none;
  height: 1px;
  background-color: var(--divider-color);
  margin: 0 0 24px 0;
}

/* Footer */
.pass-footer {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.footer-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.footer-label {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.footer-value {
  font-size: 15px;
  font-weight: 500;
  letter-spacing: 0.02em;
}

.code-value {
  font-family: monospace, monospace;
  font-size: 16px;
  letter-spacing: 0.1em;
}

.expiry-field {
  text-align: right;
}

/* Pointer 3D Highlight Sheen */
.month-card-sheen {
  position: absolute;
  inset: 0;
  z-index: 20;
  pointer-events: none;
  background: radial-gradient(
    circle at var(--light-x, 50%) var(--light-y, 50%),
    rgba(255, 255, 255, var(--light-opacity, 0)) 0%,
    transparent 60%
  );
  mix-blend-mode: overlay;
  transition: background 0.1s ease-out;
}

/* ==========================================================================
   Responsive Breakpoints
   ========================================================================== */

/* Medium Layout (Tablets / Smaller Desktop) */
@media (max-width: 659px) {
  .month-card-content {
    padding: 32px;
  }

  .card-main {
    flex-direction: column;
    gap: 24px;
    margin-bottom: 32px;
  }

  .card-available {
    text-align: left;
  }

  .available-label {
    flex-direction: row;
    gap: 8px;
    align-items: baseline;
  }

  .available-hint {
    margin-top: 0;
  }

  .available-amount {
    font-size: 48px;
  }

  .card-progress-section {
    max-width: 100%;
  }

  .pass-footer {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 24px;
  }

  .expiry-field {
    text-align: left;
  }
}

/* Small Layout (Mobile) */
@media (max-width: 390px) {
  .month-card-face {
    min-height: auto; /* Allow auto height on very small screens */
  }

  .month-card-content {
    padding: 24px;
  }

  .pass-header {
    margin-bottom: 32px;
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }

  .product-title {
    font-size: 24px;
  }

  .available-amount {
    font-size: 40px;
  }

  .progress-info {
    flex-direction: column;
    gap: 4px;
  }

  .card-info-message {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .info-dot {
    display: none;
  }

  .pass-footer {
    grid-template-columns: 1fr;
    gap: 20px;
  }

  .sakura-pattern {
    width: 90%;
    height: 90%;
    right: -20%;
    bottom: -10%;
  }
}

.month-card-artwork,.month-card-base{pointer-events:none}
.month-card-base:after{content:'';position:absolute;width:70%;height:160%;right:-25%;top:-35%;border:1px solid #d4ad7538;border-radius:50%;transform:rotate(-25deg);box-shadow:0 0 0 28px #cda66d07,0 0 0 56px #cda66d07}
.month-card-content{transform:translateZ(6px);min-width:0}
.card-identity{min-width:0}.product-title,.group-title{overflow-wrap:anywhere}
.brand-name{min-width:0;max-width:60%;overflow-wrap:anywhere;text-transform:none}
.pass-header{gap:16px}.card-badges{flex-shrink:0}
.card-available{max-width:48%;min-width:0}.available-amount{font-variant-numeric:tabular-nums;overflow-wrap:anywhere}
.available-hint{max-width:38ch;line-height:1.6;opacity:.85;text-wrap:pretty}
.card-info-message{flex-wrap:wrap;line-height:1.8;font-size:13px}
.pass-footer{gap:28px}.footer-field{min-width:0}.footer-value{overflow-wrap:anywhere}
.month-card-scrim{background:linear-gradient(100deg,#170e08b0,#170e0860);mix-blend-mode:normal}
@media(max-width:1000px) and (min-width:660px){.month-card-content{padding:34px}.pass-header{margin-bottom:28px}.brand-name{font-size:16px}.product-title{font-size:26px}.group-title{font-size:13px}.card-main{gap:24px}.available-amount{font-size:46px}.available-hint{font-size:11px}.pass-badge{font-size:11px;padding:5px 9px}.card-badges{gap:6px}}
@media(max-width:659px){.month-card-face{min-height:0}.month-card-content{padding:28px}.pass-header{gap:12px;flex-wrap:wrap;margin-bottom:26px}.brand-name{font-size:14px;max-width:100%}.card-main{gap:22px;margin-bottom:25px}.product-title{font-size:26px}.group-title{font-size:13px}.card-available{max-width:100%;width:100%}.available-label{flex-direction:column;gap:4px;align-items:flex-start}.available-hint{max-width:none}.available-amount{font-size:48px}.pass-badge{font-size:10px;padding:5px 8px}.card-badges{gap:6px}.card-progress-section{gap:18px}.progress-info{gap:10px;line-height:1.6}.progress-values{white-space:nowrap}.card-spacer{min-height:24px}.card-info-message{margin-bottom:18px}.card-divider{margin-bottom:18px}.pass-footer{gap:18px}.footer-value,.code-value{font-size:13px}.expiry-field{grid-column:1/-1}.sakura-pattern{width:80%;height:70%;opacity:.12}}
@media(max-width:390px){.month-card-content{padding:22px}.pass-header{flex-direction:row;gap:12px;margin-bottom:24px}.brand-name{flex-basis:100%}.product-title{font-size:23px}.available-amount{font-size:43px}.progress-info{flex-direction:row;flex-wrap:wrap;gap:2px}.progress-title,.progress-values{font-size:12px}.pass-footer{grid-template-columns:1fr 1fr;gap:16px}.card-info-message{font-size:12px}.footer-label{font-size:10px}}
@media(prefers-reduced-motion:reduce),(pointer:coarse){.month-card-face{transition:none;transform:none!important}.month-card-sheen{display:none}.progress-fill{transition:none}}
</style>
