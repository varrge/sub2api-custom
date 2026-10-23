<template>
  <section class="box-reel" :class="{ 'is-running': running, 'is-revealed': revealed }" :aria-busy="running || waiting">
    <div class="box-reel__bar">
      <p class="box-reel__status" aria-live="polite">
        {{ running ? t('luckyBox.reelRolling') : waiting ? t('luckyBox.reelLocking') : '\u00a0' }}
      </p>
      <label class="box-reel__sound">
        <input type="checkbox" :checked="soundEnabled" data-testid="box-reel-sound" @change="toggleSound" />
        <span>{{ t('luckyBox.reelSound') }}</span>
      </label>
    </div>
    <div ref="viewportRef" class="box-reel__viewport" data-testid="box-reel-viewport">
      <div class="box-reel__marker" data-testid="box-reel-marker" aria-hidden="true"></div>
      <div ref="trackRef" class="box-reel__track" data-testid="box-reel-track" :style="trackStyle" aria-hidden="true">
        <article
          v-for="(item, index) in items"
          :key="index"
          class="box-reel__item"
          data-testid="box-reel-item"
          :data-rarity="item.prize.rarity"
          :data-target-prize="item.target ? '' : undefined"
          :class="{ 'is-winner': revealed && item.target }"
        >
          <span class="box-reel__item-badge">{{ rarityLabel(item.prize.rarity) }}</span>
          <div class="box-reel__item-media">
            <img
              v-if="imageUrl(item.prize)"
              :src="imageUrl(item.prize)"
              alt=""
              referrerpolicy="no-referrer"
              class="box-reel__item-img"
              @error="iconFailed"
            />
            <span v-else-if="iconText(item.prize)" class="box-reel__item-glyph">{{ iconText(item.prize) }}</span>
            <span v-else-if="item.prize.type === 'balance' && item.prize.amount !== null" class="box-reel__item-amount">
              <span class="box-reel__item-currency">$</span>{{ item.prize.amount.toFixed(2) }}
            </span>
            <Icon v-else :name="prizeIcon(item.prize)" size="xl" class="box-reel__item-icon" />
          </div>
          <div class="box-reel__item-copy">
            <strong class="box-reel__item-name" :title="item.prize.name">{{ item.prize.name }}</strong>
            <span class="box-reel__item-rarity">{{ rarityLabel(item.prize.rarity) }}</span>
          </div>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { BoxCardTier, BoxPrize, BoxRarity } from '@/api/luckyBox'
import Icon from '@/components/icons/Icon.vue'
import { useBoxReel, type ReelPrize } from '@/composables/useBoxReel'

const props = defineProps<{ prizes: BoxPrize[]; tier: BoxCardTier; waiting: boolean }>()
const { t } = useI18n()
const {
  viewportRef, trackRef, items, running, revealed, soundEnabled, trackStyle, play, prepareSound, toggleSound,
} = useBoxReel(() => props.prizes, () => props.tier)
const failedImages = ref(new Set<string>())
const imageUrl = (prize: ReelPrize) => {
  const icon = prize.icon?.trim() ?? ''
  return /^https?:\/\//i.test(icon) && !failedImages.value.has(icon) ? icon : ''
}
const iconText = (prize: ReelPrize) => {
  const icon = prize.icon?.trim() ?? ''
  return icon && !/^(?:https?:|data:|javascript:|\/)/i.test(icon) ? icon : ''
}
const iconFailed = (event: Event) => {
  const src = (event.target as HTMLImageElement).getAttribute('src')
  if (src) failedImages.value.add(src)
}
const rarityLabel = (rarity: BoxRarity) => t(`luckyBox.rarity${rarity.charAt(0).toUpperCase()}${rarity.slice(1)}`)
const prizeIcon = (prize: ReelPrize) => prize.type === 'balance' ? 'dollar' : prize.type === 'extra_draw' ? 'refresh' : 'gift'
defineExpose({ play, prepareSound })
</script>

<style scoped>
.box-reel {
  --reel-item-width: 160px;
  --reel-item-height: 224px;
  --reel-item-gap: 12px;
  --reel-viewport-height: 260px;
  --reel-rarity-common: #a1a1aa;
  --reel-rarity-rare: #22d3ee;
  --reel-rarity-epic: #e879f9;
  --reel-rarity-legendary: #fbbf24;
  --reel-surface: #18181b;
  --reel-text: #fafafa;
  width: 100%;
  min-width: 0;
  letter-spacing: 0;
  color: var(--reel-text);
}
.box-reel__bar { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 28px; margin-bottom: 8px; }
.box-reel__status { margin: 0; min-width: 0; font-size: 13px; font-weight: 600; color: #71717a; }
.box-reel__sound { display: inline-flex; flex-shrink: 0; align-items: center; gap: 6px; font-size: 12px; font-weight: 600; color: #71717a; cursor: pointer; }
:global(.dark) .box-reel__status, :global(.dark) .box-reel__sound { color: #a1a1aa; }
.box-reel__sound input { width: 16px; height: 16px; margin: 0; accent-color: #0891b2; cursor: pointer; }
.box-reel__viewport {
  position: relative;
  width: 100%;
  height: var(--reel-viewport-height);
  overflow: hidden;
  border-radius: 8px;
  border: 1px solid #3f3f46;
  background: var(--reel-surface);
  box-shadow: inset 0 0 32px #0006;
}
.box-reel__viewport::before, .box-reel__viewport::after {
  position: absolute; z-index: 3; top: 0; bottom: 0; width: 12%; content: ''; pointer-events: none;
}
.box-reel__viewport::before { left: 0; background: linear-gradient(90deg, var(--reel-surface), transparent); }
.box-reel__viewport::after { right: 0; background: linear-gradient(270deg, var(--reel-surface), transparent); }
.box-reel__marker {
  position: absolute; z-index: 5; top: 0; bottom: 0; left: 50%; width: 2px;
  transform: translateX(-50%); background: #22d3ee55; box-shadow: 0 0 10px #22d3ee33; pointer-events: none;
}
.box-reel__marker::before, .box-reel__marker::after {
  position: absolute; left: 50%; width: 0; height: 0; content: ''; transform: translateX(-50%);
  border-left: 9px solid transparent; border-right: 9px solid transparent;
}
.box-reel__marker::before { top: 0; border-top: 12px solid #22d3ee; }
.box-reel__marker::after { bottom: 0; border-bottom: 12px solid #22d3ee; }
.box-reel__track { position: relative; display: flex; align-items: center; gap: var(--reel-item-gap); width: max-content; height: 100%; will-change: transform; }
.box-reel__item {
  --reel-rarity: var(--reel-rarity-common);
  position: relative; flex: none; width: var(--reel-item-width); height: var(--reel-item-height);
  display: grid; grid-template-rows: 1fr 60px; overflow: hidden; border-radius: 6px;
  border: 1px solid #ffffff20; border-bottom: 4px solid var(--reel-rarity);
  background: linear-gradient(160deg, #ffffff0f, #ffffff03 60%), #27272a;
  box-shadow: 0 4px 14px #0005; padding: 12px;
}
.box-reel__item[data-rarity='rare'] { --reel-rarity: var(--reel-rarity-rare); }
.box-reel__item[data-rarity='epic'] { --reel-rarity: var(--reel-rarity-epic); }
.box-reel__item[data-rarity='legendary'] { --reel-rarity: var(--reel-rarity-legendary); }
.box-reel__item-badge {
  position: absolute; top: 8px; right: 8px; z-index: 2; max-width: calc(100% - 16px);
  padding: 2px 7px; border-radius: 4px; background: #0006; border: 1px solid var(--reel-rarity);
  color: var(--reel-rarity); font-size: 10px; font-weight: 700;
}
.box-reel__item-media { display: grid; place-items: center; min-width: 0; min-height: 0; color: var(--reel-rarity); padding-top: 15px; }
.box-reel__item-img { width: 84px; height: 84px; object-fit: contain; filter: drop-shadow(0 4px 10px #0007); }
.box-reel__item-glyph { max-width: 100%; font-size: 38px; overflow-wrap: anywhere; line-height: 1.1; }
.box-reel__item-amount { max-width: 100%; font-size: 28px; font-weight: 800; color: var(--reel-text); overflow-wrap: anywhere; line-height: 1.1; }
.box-reel__item-currency { color: var(--reel-rarity); }
.box-reel__item-icon { width: 52px; height: 52px; }
.box-reel__item-copy { min-width: 0; padding-top: 8px; border-top: 1px solid #ffffff14; }
.box-reel__item-name { display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; overflow-wrap: anywhere; font-size: 13px; line-height: 18px; font-weight: 700; }
.box-reel__item-rarity { display: block; margin-top: 3px; font-size: 10px; font-weight: 700; color: var(--reel-rarity); }
.is-revealed .box-reel__item:not(.is-winner) { opacity: 0.45; }
.box-reel__item.is-winner {
  border-color: var(--reel-rarity);
  box-shadow: 0 0 0 1px var(--reel-rarity), 0 0 24px color-mix(in srgb, var(--reel-rarity) 35%, transparent);
}
@media (max-width: 640px) {
  .box-reel { --reel-item-width: 124px; --reel-item-height: 186px; --reel-item-gap: 10px; --reel-viewport-height: 220px; }
  .box-reel__item { padding: 9px; grid-template-rows: 1fr 60px; }
  .box-reel__item-img { width: 64px; height: 64px; }
  .box-reel__item-glyph { font-size: 30px; }
  .box-reel__item-amount { font-size: 22px; }
  .box-reel__item-icon { width: 40px; height: 40px; }
  .box-reel__item-name { font-size: 12px; }
}
@media (prefers-reduced-motion: reduce) {
  .box-reel__item, .box-reel__track { transition: none !important; }
}
</style>
